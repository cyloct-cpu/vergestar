package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"infinite-canvas/backend/internal/model"

	"gorm.io/gorm"
)

type NovelAgentSkill struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Source      string `json:"source"`
}

type NovelAgentModelRequest struct {
	Provider    string  `json:"provider"`
	Service     string  `json:"service"`
	BaseURL     string  `json:"baseUrl"`
	APIKey      string  `json:"apiKey"`
	Model       string  `json:"model"`
	APIFormat   string  `json:"apiFormat"`
	Temperature float64 `json:"temperature"`
}

type NovelAgentTurnRequest struct {
	SessionID              string                 `json:"sessionId"`
	BookID                 string                 `json:"bookId"`
	Mode                   string                 `json:"mode"`
	Message                string                 `json:"message"`
	RequestedSkills        []string               `json:"requestedSkills"`
	ConfirmedIntent        string                 `json:"confirmedIntent"`
	ConfirmedActionPayload map[string]any         `json:"confirmedActionPayload"`
	Model                  NovelAgentModelRequest `json:"model"`
}

type NovelAgentTurnResult struct {
	Session         model.StoryAgentSession `json:"session"`
	Text            string                  `json:"text"`
	Skills          []NovelAgentSkill       `json:"skills"`
	MissingSkillIDs []string                `json:"missingSkillIds"`
	Usage           struct {
		PromptTokens     int `json:"promptTokens"`
		CompletionTokens int `json:"completionTokens"`
		TotalTokens      int `json:"totalTokens"`
	} `json:"usage"`
	Confirmation *NovelAgentConfirmation `json:"confirmation,omitempty"`
	Artifacts    *NovelAgentArtifacts    `json:"artifacts,omitempty"`
}

type NovelAgentSessionMessage struct {
	ID           string                  `json:"id"`
	Role         string                  `json:"role"`
	Content      string                  `json:"content"`
	Skills       []NovelAgentSkill       `json:"skills"`
	Confirmation *NovelAgentConfirmation `json:"confirmation,omitempty"`
	CreatedAt    time.Time               `json:"createdAt"`
}

type NovelAgentSessionView struct {
	Session  model.StoryAgentSession    `json:"session"`
	Messages []NovelAgentSessionMessage `json:"messages"`
}

type NovelAgentJob struct {
	ID             string                `json:"id"`
	Status         string                `json:"status"`
	Stage          string                `json:"stage"`
	Logs           []string              `json:"logs"`
	Result         *NovelAgentTurnResult `json:"result,omitempty"`
	Error          string                `json:"error,omitempty"`
	StoryProjectID string                `json:"storyProjectId,omitempty"`
	CreatedAt      string                `json:"createdAt"`
	UpdatedAt      string                `json:"updatedAt"`
}

type NovelAgentArtifacts struct {
	BookID         string            `json:"bookId"`
	AgentSessionID string            `json:"agentSessionId"`
	Title          string            `json:"title"`
	BookJSON       string            `json:"bookJson"`
	ChapterIndex   string            `json:"chapterIndex"`
	Files          map[string]string `json:"files"`
	ProductionFiles map[string]string `json:"productionFiles,omitempty"`
	ProductionUnitID string             `json:"productionUnitId,omitempty"`
}

type NovelAgentConfirmation struct {
	Action          string         `json:"action"`
	Title           string         `json:"title"`
	Summary         string         `json:"summary"`
	Instruction     string         `json:"instruction"`
	RequestedSkills []string       `json:"requestedSkills"`
	ActionPayload   map[string]any `json:"actionPayload"`
}

type novelAgentClient struct {
	endpoint string
	token    string
	http     *http.Client
}

func (s *Service) ConfigureNovelAgent(endpoint, token string) error {
	endpoint = strings.TrimRight(strings.TrimSpace(endpoint), "/")
	if endpoint == "" {
		s.novelAgent = nil
		return nil
	}
	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Scheme != "http" || parsed.Host == "" {
		return errors.New("Novel Agent 地址必须是有效的 http 地址")
	}
	if strings.TrimSpace(token) == "" {
		return errors.New("启用 Novel Agent 必须配置内部访问令牌")
	}
	// Architect 会执行多轮基础设定生成、审稿和重试，不能用普通聊天的短
	// HTTP 超时。取消仍由请求 context 控制；生产环境后续应迁移到持久化 Job。
	s.novelAgent = &novelAgentClient{endpoint: endpoint, token: token, http: &http.Client{Timeout: 45 * time.Minute}}
	return nil
}

func (s *Service) NovelAgentSkills(ctx context.Context) ([]NovelAgentSkill, error) {
	if s.novelAgent == nil {
		return nil, NewAppError(http.StatusServiceUnavailable, "小说 Agent 尚未配置")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.novelAgent.endpoint+"/skills", nil)
	if err != nil {
		return nil, WrapAppError(http.StatusInternalServerError, "创建小说 Agent 请求失败", err)
	}
	req.Header.Set("x-vergestar-novel-agent-token", s.novelAgent.token)
	resp, err := s.novelAgent.http.Do(req)
	if err != nil {
		return nil, WrapAppError(http.StatusServiceUnavailable, "小说 Agent 暂时不可用", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, NewAppError(http.StatusServiceUnavailable, "小说 Agent 暂时不可用")
	}
	var body struct {
		Skills []NovelAgentSkill `json:"skills"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, WrapAppError(http.StatusBadGateway, "小说 Agent 返回格式无效", err)
	}
	if len(body.Skills) == 0 {
		return nil, NewAppError(http.StatusServiceUnavailable, fmt.Sprintf("小说 Agent 没有可用 Skill"))
	}
	return body.Skills, nil
}

func (s *Service) RunNovelAgentTurn(ctx context.Context, userID string, request NovelAgentTurnRequest) (NovelAgentTurnResult, error) {
	if s.novelAgent == nil {
		return NovelAgentTurnResult{}, NewAppError(http.StatusServiceUnavailable, "小说 Agent 尚未配置")
	}
	message := strings.TrimSpace(request.Message)
	if message == "" || len([]rune(message)) > 12_000 {
		return NovelAgentTurnResult{}, BadAuthRequest("小说 Agent 输入必须在 1 到 12000 字之间")
	}
	if len(request.RequestedSkills) > 8 {
		return NovelAgentTurnResult{}, BadAuthRequest("一次最多指定 8 个小说 Skill")
	}
	for _, skillID := range request.RequestedSkills {
		if len([]rune(strings.TrimSpace(skillID))) > 160 {
			return NovelAgentTurnResult{}, BadAuthRequest("Skill 标识过长")
		}
	}
	baseURL := strings.TrimSpace(request.Model.BaseURL)
	if _, err := s.ValidateChannelOutboundURL(baseURL, false, false); err != nil {
		return NovelAgentTurnResult{}, err
	}
	if strings.TrimSpace(request.Model.APIKey) == "" || len(request.Model.APIKey) > 4_000 {
		return NovelAgentTurnResult{}, BadAuthRequest("小说 Agent 需要当前模型的有效密钥")
	}
	if strings.TrimSpace(request.Model.Model) == "" || len(request.Model.Model) > 500 {
		return NovelAgentTurnResult{}, BadAuthRequest("小说 Agent 模型标识无效")
	}
	if request.Model.Provider != "openai" && request.Model.Provider != "anthropic" && request.Model.Provider != "custom" {
		request.Model.Provider = "custom"
	}
	if request.Model.APIFormat != "responses" {
		request.Model.APIFormat = "chat"
	}
	if request.Model.Temperature < 0 || request.Model.Temperature > 2 {
		return NovelAgentTurnResult{}, BadAuthRequest("小说 Agent 温度必须在 0 到 2 之间")
	}
	mode := strings.TrimSpace(request.Mode)
	if mode == "" {
		mode = "long-novel"
	}
	if len([]rune(mode)) > 48 {
		return NovelAgentTurnResult{}, BadAuthRequest("小说 Agent 创作模式无效")
	}
	if request.ConfirmedIntent != "" && request.ConfirmedIntent != "create_book" && request.ConfirmedIntent != "write_next" && request.ConfirmedIntent != "repair_state" && request.ConfirmedIntent != "revise_chapter" && request.ConfirmedIntent != "create_script" && request.ConfirmedIntent != "create_storyboard" && request.ConfirmedIntent != "short_run" && request.ConfirmedIntent != "draft_next" && request.ConfirmedIntent != "plan_chapter" && request.ConfirmedIntent != "compose_chapter" && request.ConfirmedIntent != "fanfic_init" && request.ConfirmedIntent != "continuation_import" && request.ConfirmedIntent != "spinoff_create" && request.ConfirmedIntent != "style_imitation" && request.ConfirmedIntent != "translation_create" && request.ConfirmedIntent != "interactive_film_create" && request.ConfirmedIntent != "play_start" && request.ConfirmedIntent != "play_step" {
		return NovelAgentTurnResult{}, BadAuthRequest("不支持的小说 Agent 确认动作")
	}
	if request.ConfirmedIntent == "create_book" && len(request.ConfirmedActionPayload) == 0 {
		return NovelAgentTurnResult{}, BadAuthRequest("确认创建小说缺少结构化动作参数")
	}
	if request.ConfirmedIntent == "write_next" || request.ConfirmedIntent == "repair_state" || request.ConfirmedIntent == "revise_chapter" || request.ConfirmedIntent == "create_script" || request.ConfirmedIntent == "create_storyboard" || request.ConfirmedIntent == "draft_next" || request.ConfirmedIntent == "plan_chapter" || request.ConfirmedIntent == "compose_chapter" {
		if strings.TrimSpace(request.BookID) == "" {
			return NovelAgentTurnResult{}, BadAuthRequest("小说状态操作缺少绑定的 InkOS 书籍")
		}
		if len(request.ConfirmedActionPayload) == 0 {
			return NovelAgentTurnResult{}, BadAuthRequest("确认写作缺少结构化动作参数")
		}
		if _, err := s.repo.StoryFoundationForBook(userID, strings.TrimSpace(request.BookID)); err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return NovelAgentTurnResult{}, Forbidden("当前用户没有权限操作该 InkOS 书籍")
			}
			return NovelAgentTurnResult{}, err
		}
	}
	var session *model.StoryAgentSession
	var history []model.StoryAgentMessage
	if sessionID := strings.TrimSpace(request.SessionID); sessionID != "" {
		loaded, err := s.repo.StoryAgentSessionForUser(userID, sessionID)
		if err != nil {
			return NovelAgentTurnResult{}, err
		}
		session = loaded
		history, err = s.repo.StoryAgentMessages(session.ID)
		if err != nil {
			return NovelAgentTurnResult{}, err
		}
	}
	bridgeSessionID := strings.TrimSpace(request.SessionID)
	if bridgeSessionID == "" {
		bridgeSessionID = newID()
	}
	bridgePayload := struct {
		UserID                 string                 `json:"userId"`
		SessionID              string                 `json:"sessionId"`
		BookID                 string                 `json:"bookId,omitempty"`
		Message                string                 `json:"message"`
		RequestedSkills        []string               `json:"requestedSkills"`
		History                []map[string]string    `json:"history"`
		Model                  NovelAgentModelRequest `json:"model"`
		ConfirmedIntent        string                 `json:"confirmedIntent,omitempty"`
		ConfirmedActionPayload map[string]any         `json:"confirmedActionPayload,omitempty"`
	}{UserID: userID, SessionID: bridgeSessionID, BookID: strings.TrimSpace(request.BookID), Message: message, RequestedSkills: request.RequestedSkills, Model: request.Model, ConfirmedIntent: request.ConfirmedIntent, ConfirmedActionPayload: request.ConfirmedActionPayload}
	for _, item := range history {
		if item.Role == "user" || item.Role == "assistant" {
			bridgePayload.History = append(bridgePayload.History, map[string]string{"role": item.Role, "content": item.Content})
		}
	}
	payload, err := json.Marshal(bridgePayload)
	if err != nil {
		return NovelAgentTurnResult{}, WrapAppError(http.StatusInternalServerError, "序列化小说 Agent 请求失败", err)
	}
	internalRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, s.novelAgent.endpoint+"/agent/turn", bytes.NewReader(payload))
	if err != nil {
		return NovelAgentTurnResult{}, WrapAppError(http.StatusInternalServerError, "创建小说 Agent 请求失败", err)
	}
	internalRequest.Header.Set("content-type", "application/json")
	internalRequest.Header.Set("x-vergestar-novel-agent-token", s.novelAgent.token)
	response, err := s.novelAgent.http.Do(internalRequest)
	if err != nil {
		return NovelAgentTurnResult{}, WrapAppError(http.StatusServiceUnavailable, "小说 Agent 暂时不可用", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return NovelAgentTurnResult{}, NewAppError(http.StatusBadGateway, "小说 Agent 本轮创作失败")
	}
	var bridgeResult struct {
		Text            string            `json:"text"`
		Skills          []NovelAgentSkill `json:"skills"`
		MissingSkillIDs []string          `json:"missingSkillIds"`
		Usage           struct {
			PromptTokens     int `json:"promptTokens"`
			CompletionTokens int `json:"completionTokens"`
			TotalTokens      int `json:"totalTokens"`
		} `json:"usage"`
		Confirmation *NovelAgentConfirmation `json:"confirmation"`
		Artifacts *NovelAgentArtifacts `json:"artifacts"`
	}
	if err := json.NewDecoder(response.Body).Decode(&bridgeResult); err != nil {
		return NovelAgentTurnResult{}, WrapAppError(http.StatusBadGateway, "小说 Agent 返回格式无效", err)
	}
	if strings.TrimSpace(bridgeResult.Text) == "" && bridgeResult.Confirmation == nil {
		return NovelAgentTurnResult{}, NewAppError(http.StatusBadGateway, "小说 Agent 没有返回内容")
	}
	skillsJSON, err := json.Marshal(bridgeResult.Skills)
	if err != nil {
		return NovelAgentTurnResult{}, WrapAppError(http.StatusInternalServerError, "序列化小说 Agent Skill 失败", err)
	}
	now := time.Now()
	userMessage := model.StoryAgentMessage{ID: newID(), Role: "user", Content: message, CreatedAt: now}
	assistantContent := bridgeResult.Text
	if bridgeResult.Confirmation != nil {
		assistantContent = bridgeResult.Confirmation.Summary
	}
	confirmationJSON := ""
	if bridgeResult.Confirmation != nil {
		encoded, marshalErr := json.Marshal(bridgeResult.Confirmation)
		if marshalErr != nil {
			return NovelAgentTurnResult{}, WrapAppError(http.StatusInternalServerError, "序列化小说 Agent 确认动作失败", marshalErr)
		}
		confirmationJSON = string(encoded)
	}
	artifactsJSON := ""
	if bridgeResult.Artifacts != nil {
		encoded, marshalErr := json.Marshal(bridgeResult.Artifacts)
		if marshalErr != nil {
			return NovelAgentTurnResult{}, WrapAppError(http.StatusInternalServerError, "序列化小说 Agent 产物失败", marshalErr)
		}
		artifactsJSON = string(encoded)
	}
	assistantMessage := model.StoryAgentMessage{ID: newID(), Role: "assistant", Content: assistantContent, SkillsJSON: string(skillsJSON), ConfirmationJSON: confirmationJSON, ArtifactsJSON: artifactsJSON, CreatedAt: now}
	if session == nil {
		session = &model.StoryAgentSession{ID: bridgeSessionID, UserID: userID, Title: novelAgentSessionTitle(message), Mode: mode, Status: "active", CreatedAt: now, UpdatedAt: now}
		userMessage.SessionID, assistantMessage.SessionID = session.ID, session.ID
		if err := s.repo.CreateStoryAgentSession(session, []model.StoryAgentMessage{userMessage, assistantMessage}); err != nil {
			return NovelAgentTurnResult{}, err
		}
	} else {
		userMessage.SessionID, assistantMessage.SessionID = session.ID, session.ID
		session.UpdatedAt = now
		if err := s.repo.AppendStoryAgentMessages(session, []model.StoryAgentMessage{userMessage, assistantMessage}); err != nil {
			return NovelAgentTurnResult{}, err
		}
	}
	var artifacts *NovelAgentArtifacts
	if assistantMessage.ArtifactsJSON != "" {
		artifacts = &NovelAgentArtifacts{}
		if err := json.Unmarshal([]byte(assistantMessage.ArtifactsJSON), artifacts); err != nil {
			return NovelAgentTurnResult{}, WrapAppError(http.StatusInternalServerError, "解析小说 Agent 产物失败", err)
		}
	}
	return NovelAgentTurnResult{Session: *session, Text: assistantContent, Skills: bridgeResult.Skills, MissingSkillIDs: bridgeResult.MissingSkillIDs, Usage: bridgeResult.Usage, Confirmation: bridgeResult.Confirmation, Artifacts: artifacts}, nil
}

type NovelAgentSessionPage struct {
	Sessions []model.StoryAgentSession `json:"sessions"`
	Total    int64                     `json:"total"`
	Page     int                       `json:"page"`
	PageSize int                       `json:"pageSize"`
}

func (s *Service) SearchNovelAgentSessions(ctx context.Context, userID, query string, page, pageSize int) (NovelAgentSessionPage, error) {
	_ = ctx
	result := NovelAgentSessionPage{Page: page, PageSize: pageSize}
	sessions, total, err := s.repo.StoryAgentSessionsSearch(userID, query, page, pageSize)
	if err != nil {
		return result, err
	}
	result.Sessions = sessions
	result.Total = total
	return result, nil
}
func (s *Service) ListNovelAgentSessions(userID string) ([]model.StoryAgentSession, error) {
	return s.repo.StoryAgentSessionsForUser(userID)
}

func (s *Service) NovelAgentSessionView(userID, sessionID string) (NovelAgentSessionView, error) {
	session, err := s.repo.StoryAgentSessionForUser(userID, strings.TrimSpace(sessionID))
	if err != nil {
		return NovelAgentSessionView{}, err
	}
	messages, err := s.repo.StoryAgentMessages(session.ID)
	if err != nil {
		return NovelAgentSessionView{}, err
	}
	view := NovelAgentSessionView{Session: *session, Messages: make([]NovelAgentSessionMessage, 0, len(messages))}
	for _, item := range messages {
		if item.Role != "user" && item.Role != "assistant" {
			continue
		}
		var skills []NovelAgentSkill
		if strings.TrimSpace(item.SkillsJSON) != "" {
			if err := json.Unmarshal([]byte(item.SkillsJSON), &skills); err != nil {
				return NovelAgentSessionView{}, WrapAppError(http.StatusInternalServerError, "小说 Agent 会话技能格式无效", err)
			}
		}
		var confirmation *NovelAgentConfirmation
		if strings.TrimSpace(item.ConfirmationJSON) != "" {
			confirmation = &NovelAgentConfirmation{}
			if err := json.Unmarshal([]byte(item.ConfirmationJSON), confirmation); err != nil {
				return NovelAgentSessionView{}, WrapAppError(http.StatusInternalServerError, "小说 Agent 会话确认动作格式无效", err)
			}
		}
		view.Messages = append(view.Messages, NovelAgentSessionMessage{ID: item.ID, Role: item.Role, Content: item.Content, Skills: skills, Confirmation: confirmation, CreatedAt: item.CreatedAt})
	}
	return view, nil
}

func (s *Service) StartNovelAgentJob(ctx context.Context, userID string, request NovelAgentTurnRequest) (NovelAgentJob, error) {
	if s.novelAgent == nil {
		return NovelAgentJob{}, NewAppError(http.StatusServiceUnavailable, "小说 Agent 尚未配置")
	}
	if (request.ConfirmedIntent == "write_next" || request.ConfirmedIntent == "repair_state" || request.ConfirmedIntent == "revise_chapter" || request.ConfirmedIntent == "create_script" || request.ConfirmedIntent == "create_storyboard") && strings.TrimSpace(request.BookID) == "" {
		return NovelAgentJob{}, BadAuthRequest("写作下一章缺少绑定的 InkOS 书籍")
	}
	if _, err := s.validateNovelAgentTurnRequest(userID, request); err != nil {
		return NovelAgentJob{}, err
	}
	payload, err := json.Marshal(requestWithOwner(request, userID))
	if err != nil {
		return NovelAgentJob{}, WrapAppError(http.StatusInternalServerError, "序列化小说 Agent 任务失败", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.novelAgent.endpoint+"/agent/jobs", bytes.NewReader(payload))
	if err != nil {
		return NovelAgentJob{}, WrapAppError(http.StatusInternalServerError, "创建小说 Agent 任务请求失败", err)
	}
	req.Header.Set("content-type", "application/json")
	req.Header.Set("x-vergestar-novel-agent-token", s.novelAgent.token)
	req.Header.Set("x-vergestar-novel-agent-user", userID)
	resp, err := s.novelAgent.http.Do(req)
	if err != nil {
		return NovelAgentJob{}, WrapAppError(http.StatusServiceUnavailable, "小说 Agent 暂时不可用", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		return NovelAgentJob{}, NewAppError(http.StatusBadGateway, "小说 Agent 后台任务创建失败")
	}
	var body struct {
		Job NovelAgentJob `json:"job"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return NovelAgentJob{}, WrapAppError(http.StatusBadGateway, "小说 Agent 任务响应无效", err)
	}
	if err := s.ensureNovelTurnUserMessage(userID, request); err != nil {
		log.Printf("novel turn user message persist failed: user=%s err=%v", userID, err)
	}
	if strings.TrimSpace(request.ConfirmedIntent) != "" {
		if err := s.repo.ConsumeStoryAgentConfirmations(userID, request.SessionID); err != nil {
			return NovelAgentJob{}, WrapAppError(http.StatusInternalServerError, "保存小说 Agent 确认状态失败", err)
		}
	}
	// DB 镜像：任务历史与断线恢复的真相源（Bridge 仅执行侧）。
	s.mirrorNovelAgentJob(userID, body.Job, requestMetaFromTurn(request))
	return body.Job, nil
}

func (s *Service) NovelAgentJob(ctx context.Context, userID, jobID string) (NovelAgentJob, error) {
	if s.novelAgent == nil {
		return NovelAgentJob{}, NewAppError(http.StatusServiceUnavailable, "小说 Agent 尚未配置")
	}
	if strings.TrimSpace(jobID) == "" || len(jobID) > 80 {
		return NovelAgentJob{}, BadAuthRequest("小说 Agent 任务 ID 无效")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.novelAgent.endpoint+"/agent/jobs/"+url.PathEscape(jobID), nil)
	if err != nil {
		return NovelAgentJob{}, WrapAppError(http.StatusInternalServerError, "创建小说 Agent 查询请求失败", err)
	}
	req.Header.Set("x-vergestar-novel-agent-token", s.novelAgent.token)
	req.Header.Set("x-vergestar-novel-agent-user", userID)
	resp, err := s.novelAgent.http.Do(req)
	if err != nil {
		return NovelAgentJob{}, WrapAppError(http.StatusServiceUnavailable, "小说 Agent 暂时不可用", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return NovelAgentJob{}, NewAppError(http.StatusNotFound, "小说 Agent 任务不存在")
	}
	var body struct {
		Job NovelAgentJob `json:"job"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return NovelAgentJob{}, WrapAppError(http.StatusBadGateway, "小说 Agent 任务响应无效", err)
	}
	// Planning jobs intentionally have no book artifacts. They only return a
	// structured confirmation card and must not enter the projection path.
	if body.Job.Status == "succeeded" && body.Job.Result != nil && body.Job.Result.Artifacts != nil {
		artifacts, artifactErr := s.novelAgentArtifacts(ctx, userID, jobID)
		if artifactErr != nil {
			log.Printf("novel agent artifact fetch failed: job=%s err=%v", jobID, artifactErr)
			return NovelAgentJob{}, artifactErr
		}
		fallbackSessionID := ""
		if body.Job.Result != nil {
			fallbackSessionID = body.Job.Result.Session.ID
		}
		project, syncErr := s.syncNovelAgentArtifacts(userID, artifacts, fallbackSessionID)
		if syncErr != nil {
			log.Printf("novel agent artifact projection failed: job=%s book=%s files=%d err=%v", jobID, artifacts.BookID, len(artifacts.Files), syncErr)
			return NovelAgentJob{}, syncErr
		}
		body.Job.StoryProjectID = project.ID
	}
	s.mirrorNovelAgentJob(userID, body.Job, NovelAgentJobRequestMeta{SessionID: sessionIDOf(body.Job)})
	s.persistNovelJobAssistantMessage(userID, body.Job)
	return body.Job, nil
}

func (s *Service) novelAgentArtifacts(ctx context.Context, userID, jobID string) (NovelAgentArtifacts, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.novelAgent.endpoint+"/agent/jobs/"+url.PathEscape(jobID)+"/artifacts", nil)
	if err != nil {
		return NovelAgentArtifacts{}, WrapAppError(http.StatusInternalServerError, "创建小说 Agent 产物请求失败", err)
	}
	req.Header.Set("x-vergestar-novel-agent-token", s.novelAgent.token)
	req.Header.Set("x-vergestar-novel-agent-user", userID)
	resp, err := s.novelAgent.http.Do(req)
	if err != nil {
		return NovelAgentArtifacts{}, WrapAppError(http.StatusServiceUnavailable, "小说 Agent 产物暂时不可用", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return NovelAgentArtifacts{}, NewAppError(http.StatusBadGateway, "小说 Agent 产物不可用")
	}
	var body struct {
		Artifacts NovelAgentArtifacts `json:"artifacts"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return NovelAgentArtifacts{}, WrapAppError(http.StatusBadGateway, "小说 Agent 产物格式无效", err)
	}
	return body.Artifacts, nil
}

func (s *Service) syncNovelAgentArtifacts(userID string, artifacts NovelAgentArtifacts, fallbackSessionID string) (model.Project, error) {
	if strings.TrimSpace(artifacts.AgentSessionID) == "" {
		artifacts.AgentSessionID = strings.TrimSpace(fallbackSessionID)
	}
	title := strings.TrimSpace(artifacts.Title)
	bookID := strings.TrimSpace(artifacts.BookID)
	bookJSON := strings.TrimSpace(artifacts.BookJSON)
	if title == "" || bookID == "" || bookJSON == "" || len([]byte(bookJSON)) > 128<<10 {
		return model.Project{}, NewAppError(http.StatusBadGateway, "小说 Agent 书籍产物缺少有效元数据")
	}
	var book map[string]any
	if err := json.Unmarshal([]byte(bookJSON), &book); err != nil {
		return model.Project{}, NewAppError(http.StatusBadGateway, "小说 Agent book.json 格式无效")
	}
	if len(artifacts.Files) > 128 {
		return model.Project{}, NewAppError(http.StatusBadGateway, "小说 Agent 基础设定文件数量超出限制")
	}
	files := make(map[string]string, len(artifacts.Files))
	keys := make([]string, 0, len(artifacts.Files))
	totalBytes := len([]byte(bookJSON))
	for rawPath, content := range artifacts.Files {
		path := strings.TrimSpace(rawPath)
		if !validNovelArtifactPath(path) || len([]byte(content)) > 512<<10 {
			return model.Project{}, NewAppError(http.StatusBadGateway, "小说 Agent 基础设定文件路径或大小无效")
		}
		totalBytes += len([]byte(content))
		if totalBytes > 4<<20 {
			return model.Project{}, NewAppError(http.StatusBadGateway, "小说 Agent 基础设定总大小超出限制")
		}
		files[path] = content
		keys = append(keys, path)
	}
	if len(artifacts.ProductionFiles) > 32 {
		return model.Project{}, NewAppError(http.StatusBadGateway, "小说 Agent 影视产物文件数量超出限制")
	}
	for rawPath, content := range artifacts.ProductionFiles {
		path := strings.TrimSpace(rawPath)
		if !validNovelArtifactPath(path) || len([]byte(content)) > 768<<10 {
			return model.Project{}, NewAppError(http.StatusBadGateway, "小说 Agent 影视产物路径或大小无效")
		}
		totalBytes += len([]byte(content))
		if totalBytes > 6<<20 {
			return model.Project{}, NewAppError(http.StatusBadGateway, "小说 Agent 产物总大小超出限制")
		}
		files[path] = content
		keys = append(keys, path)
	}
	if existing, err := s.repo.StoryFoundationForBook(userID, bookID); err == nil {
		project, projectErr := s.repo.ProjectForUser(userID, existing.ProjectID)
		if projectErr != nil {
			return model.Project{}, projectErr
		}
		if strings.TrimSpace(artifacts.AgentSessionID) != "" && existing.AgentSessionID != strings.TrimSpace(artifacts.AgentSessionID) {
			existing.AgentSessionID = strings.TrimSpace(artifacts.AgentSessionID)
			existing.UpdatedAt = time.Now()
			if err := s.repo.UpdateStoryFoundation(existing); err != nil {
				return model.Project{}, err
			}
		}
		if strings.TrimSpace(artifacts.AgentSessionID) != "" {
			if err := s.repo.BindStoryAgentSessionProject(userID, strings.TrimSpace(artifacts.AgentSessionID), project.ID); err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
				return model.Project{}, err
			}
		}
		mergedFiles := mergeNovelArtifactFiles(existing.FilesJSON, artifacts)
		if projectionErr := s.syncNovelAgentProjection(userID, project.ID, sourceHashForFoundation(existing), mergedFiles); projectionErr != nil {
			return model.Project{}, projectionErr
		}
		if productionErr := s.syncNovelProductionArtifacts(userID, project.ID, artifacts); productionErr != nil {
			return model.Project{}, productionErr
		}
		if encoded, err := json.Marshal(mergedFiles); err == nil {
			existing.SourceHash = novelFoundationSourceHash(bookJSON, mergedFiles)
			existing.FilesJSON = string(encoded)
			existing.BookJSON = bookJSON
			existing.Title = title
			existing.UpdatedAt = time.Now()
			if err := s.repo.UpdateStoryFoundation(existing); err != nil {
				return model.Project{}, err
			}
		}
		return *project, nil
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return model.Project{}, err
	}
	sort.Strings(keys)
	hashInput := bookJSON
	for _, key := range keys {
		hashInput += "\n" + key + "\n" + files[key]
	}
	sourceHash := novelFoundationSourceHash(bookJSON, files)
	filesJSON, err := json.Marshal(files)
	if err != nil {
		return model.Project{}, WrapAppError(http.StatusInternalServerError, "序列化小说基础设定失败", err)
	}
	descriptionJSON, err := json.Marshal(map[string]any{"version": 1, "kind": "novel", "source": "inkos-agent", "inkosBookId": bookID, "sourceHash": sourceHash, "title": title})
	if err != nil {
		return model.Project{}, WrapAppError(http.StatusInternalServerError, "序列化小说项目来源失败", err)
	}
	project, err := s.CreateProject(userID, CreateProjectRequest{Name: title, Type: "novel", AspectRatio: "16:9", SourceType: "inkos-agent", Description: string(descriptionJSON)})
	if err != nil {
		return model.Project{}, err
	}
	foundation := model.StoryFoundation{ID: newID(), ProjectID: project.ID, UserID: userID, InkosBookID: bookID, AgentSessionID: strings.TrimSpace(artifacts.AgentSessionID), Title: title, BookJSON: bookJSON, FilesJSON: string(filesJSON), SourceHash: sourceHash, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	if err := s.repo.CreateStoryFoundation(&foundation); err != nil {
		_ = s.DeleteProject(userID, project.ID)
		return model.Project{}, err
	}
	if err := s.syncNovelAgentProjection(userID, project.ID, sourceHash, files); err != nil {
		return model.Project{}, err
	}
	if productionErr := s.syncNovelProductionArtifacts(userID, project.ID, artifacts); productionErr != nil {
		return model.Project{}, productionErr
	}
	if strings.TrimSpace(artifacts.AgentSessionID) != "" {
		if err := s.repo.BindStoryAgentSessionProject(userID, strings.TrimSpace(artifacts.AgentSessionID), project.ID); err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return model.Project{}, err
		}
	}
	return project, nil
}

func novelFoundationSourceHash(bookJSON string, files map[string]string) string {
	keys := make([]string, 0, len(files))
	for key := range files {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	hashInput := bookJSON
	for _, key := range keys {
		hashInput += "\n" + key + "\n" + files[key]
	}
	digest := sha256.Sum256([]byte(hashInput))
	return "sha256:" + hex.EncodeToString(digest[:])
}

func mergeNovelArtifactFiles(existingJSON string, artifacts NovelAgentArtifacts) map[string]string {
	files := decodeNovelFoundationFiles(existingJSON)
	for path, content := range artifacts.Files {
		files[path] = content
	}
	for path, content := range artifacts.ProductionFiles {
		if validNovelArtifactPath(path) {
			files[path] = content
		}
	}
	return files
}

func sourceHashForFoundation(foundation *model.StoryFoundation) string {
	if foundation == nil {
		return ""
	}
	return foundation.SourceHash
}

func decodeNovelFoundationFiles(filesJSON string) map[string]string {
	files := map[string]string{}
	if strings.TrimSpace(filesJSON) == "" {
		return files
	}
	_ = json.Unmarshal([]byte(filesJSON), &files)
	return files
}

func validNovelArtifactPath(path string) bool {
	if path == "" || strings.Contains(path, "\\") || strings.Contains(path, "..") || strings.HasPrefix(path, "/") || len(path) > 240 {
		return false
	}
	// The chapter index is a JSON control file, while chapter bodies are
	// markdown. Check the exact allowlist before the chapters/*.md branch.
	if path == "chapters/index.json" {
		return true
	}
	if strings.HasPrefix(path, "story/roles/") {
		return strings.HasSuffix(path, ".md")
	}
	if strings.HasPrefix(path, "chapters/") {
		return strings.HasSuffix(path, ".md")
	}
	if (strings.HasPrefix(path, "dramas/") || strings.HasPrefix(path, "storyboards/")) && (strings.HasSuffix(path, ".md") || strings.HasSuffix(path, ".json")) {
		return true
	}
	if strings.HasPrefix(path, "shorts/") && (strings.HasSuffix(path, ".md") || strings.HasSuffix(path, ".json")) {
		return true
	}
	for _, allowed := range []string{
		"story/author_intent.md", "story/brief.md", "story/book_rules.md", "story/character_matrix.md",
		"story/current_state.md", "story/pending_hooks.md", "story/emotional_arcs.md", "story/chapter_summaries.md", "story/subplot_board.md", "story/story_bible.md", "story/style_guide.md",
		"story/outline/story_frame.md", "story/outline/volume_map.md",
	} {
		if path == allowed {
			return true
		}
	}
	return false
}

func requestWithOwner(request NovelAgentTurnRequest, userID string) map[string]any {
	return map[string]any{"userId": userID, "sessionId": request.SessionID, "bookId": request.BookID, "mode": request.Mode, "message": request.Message, "requestedSkills": request.RequestedSkills, "confirmedIntent": request.ConfirmedIntent, "confirmedActionPayload": request.ConfirmedActionPayload, "model": request.Model}
}

func (s *Service) validateNovelAgentTurnRequest(userID string, request NovelAgentTurnRequest) (string, error) {
	if strings.TrimSpace(request.Message) == "" {
		return "", BadAuthRequest("小说 Agent 输入不能为空")
	}
	if _, err := s.repo.StoryAgentSessionForUser(userID, strings.TrimSpace(request.SessionID)); err != nil {
		return "", err
	}
	return userID, nil
}

func novelAgentSessionTitle(message string) string {
	runes := []rune(strings.TrimSpace(message))
	if len(runes) > 40 {
		return string(runes[:40])
	}
	return string(runes)
}

func (s *Service) CancelNovelAgentJob(ctx context.Context, userID, jobID string) error {
	if s.novelAgent == nil {
		return NewAppError(http.StatusServiceUnavailable, "小说 Agent 尚未配置")
	}
	if strings.TrimSpace(jobID) == "" || len(jobID) > 80 {
		return BadAuthRequest("小说 Agent 任务 ID 无效")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.novelAgent.endpoint+"/agent/jobs/"+url.PathEscape(jobID)+"/cancel", nil)
	if err != nil {
		return WrapAppError(http.StatusInternalServerError, "创建小说 Agent 取消请求失败", err)
	}
	req.Header.Set("x-vergestar-novel-agent-token", s.novelAgent.token)
	req.Header.Set("x-vergestar-novel-agent-user", userID)
	resp, err := s.novelAgent.http.Do(req)
	if err != nil {
		return WrapAppError(http.StatusServiceUnavailable, "小说 Agent 暂时不可用", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return NewAppError(http.StatusNotFound, "小说 Agent 任务不存在或已结束")
	}
	var body struct {
		OK bool `json:"ok"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return WrapAppError(http.StatusBadGateway, "小说 Agent 取消响应无效", err)
	}
	if !body.OK {
		return NewAppError(http.StatusNotFound, "任务已结束，不能取消")
	}
	return nil
}
