package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strings"

	"gorm.io/gorm"

	"infinite-canvas/backend/internal/model"
)

// InkOS BookDetail 管理操作的后端服务层：通过章节、拒绝并回滚、书籍设置、删除书。
// 桥接 Novel Agent 的 /agent/chapters/* 与 /agent/books/*，产物回传后重投影，
// 保持 Vergestar DB 是单一真相。语义与 InkOS Studio 服务端一致。

type novelBridgeRequest struct {
	UserID     string `json:"userId"`
	SessionID  string `json:"sessionId"`
	BookID     string `json:"bookId"`
	ChapterNum int    `json:"chapterNumber,omitempty"`
}

func (s *Service) novelBridgeCall(ctx context.Context, userID, path string, payload any, out any) error {
	if s.novelAgent == nil {
		return NewAppError(http.StatusServiceUnavailable, "小说 Agent 尚未配置")
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return WrapAppError(http.StatusInternalServerError, "序列化小说 Agent 请求失败", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.novelAgent.endpoint+path, bytes.NewReader(body))
	if err != nil {
		return WrapAppError(http.StatusInternalServerError, "创建小说 Agent 请求失败", err)
	}
	req.Header.Set("content-type", "application/json")
	req.Header.Set("x-vergestar-novel-agent-token", s.novelAgent.token)
	req.Header.Set("x-vergestar-novel-agent-user", userID)
	resp, err := s.novelAgent.http.Do(req)
	if err != nil {
		return WrapAppError(http.StatusServiceUnavailable, "小说 Agent 暂时不可用", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		msg := strings.TrimSpace(string(raw))
		if msg == "" {
			msg = "小说 Agent 操作失败"
		}
		return NewAppError(http.StatusBadGateway, msg)
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return WrapAppError(http.StatusBadGateway, "小说 Agent 响应无效", err)
	}
	return nil
}

// foundationForProject loads the InkOS binding for a novel project.
func (s *Service) foundationForProject(projectID string) (*model.StoryFoundation, error) {
	foundation, err := s.repo.StoryFoundationForProject(projectID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, NewAppError(http.StatusNotFound, "该小说项目没有绑定 InkOS 书籍")
		}
		return nil, err
	}
	return foundation, nil
}

func (s *Service) resyncProject(userID string, artifacts NovelAgentArtifacts) error {
	if _, err := s.syncNovelAgentArtifacts(userID, artifacts, artifacts.AgentSessionID); err != nil {
		return err
	}
	return nil
}

// ApproveNovelChapter marks an InkOS chapter as approved and re-projects the book.
func (s *Service) ApproveNovelChapter(ctx context.Context, userID, projectID string, chapterNumber int) error {
	foundation, err := s.foundationForProject(projectID)
	if err != nil {
		return err
	}
	if chapterNumber <= 0 {
		return BadAuthRequest("章节号无效")
	}
	var body struct {
		Artifacts NovelAgentArtifacts `json:"artifacts"`
	}
	if err := s.novelBridgeCall(ctx, userID, "/agent/chapters/approve", novelBridgeRequest{UserID: userID, SessionID: foundation.AgentSessionID, BookID: foundation.InkosBookID, ChapterNum: chapterNumber}, &body); err != nil {
		return err
	}
	if err := s.resyncProject(userID, body.Artifacts); err != nil {
		log.Printf("novel approve projection failed: project=%s chapter=%d err=%v", projectID, chapterNumber, err)
		return err
	}
	return nil
}

// RejectNovelChapter rolls the InkOS book back to the previous chapter, discarding the rejected one.
func (s *Service) RejectNovelChapter(ctx context.Context, userID, projectID string, chapterNumber int) error {
	foundation, err := s.foundationForProject(projectID)
	if err != nil {
		return err
	}
	if chapterNumber <= 0 {
		return BadAuthRequest("章节号无效")
	}
	var body struct {
		Artifacts NovelAgentArtifacts `json:"artifacts"`
	}
	if err := s.novelBridgeCall(ctx, userID, "/agent/chapters/reject", novelBridgeRequest{UserID: userID, SessionID: foundation.AgentSessionID, BookID: foundation.InkosBookID, ChapterNum: chapterNumber}, &body); err != nil {
		return err
	}
	if err := s.resyncProject(userID, body.Artifacts); err != nil {
		log.Printf("novel reject projection failed: project=%s chapter=%d err=%v", projectID, chapterNumber, err)
		return err
	}
	return nil
}

type NovelBookSettingsRequest struct {
	ChapterWordCount  *int    `json:"chapterWordCount,omitempty"`
	TargetChapters    *int    `json:"targetChapters,omitempty"`
	Status            *string `json:"status,omitempty"`
	ChapterReviewMode *string `json:"chapterReviewMode,omitempty"`
}

// UpdateNovelBookSettings writes book.json settings (word count target, chapter target,
// status, review mode) through the bridge.
func (s *Service) UpdateNovelBookSettings(ctx context.Context, userID, projectID string, request NovelBookSettingsRequest) (map[string]any, error) {
	foundation, err := s.foundationForProject(projectID)
	if err != nil {
		return nil, err
	}
	payload := map[string]any{"userId": userID, "sessionId": foundation.AgentSessionID, "bookId": foundation.InkosBookID}
	if request.ChapterWordCount != nil && *request.ChapterWordCount > 0 {
		payload["chapterWordCount"] = *request.ChapterWordCount
	}
	if request.TargetChapters != nil && *request.TargetChapters > 0 {
		payload["targetChapters"] = *request.TargetChapters
	}
	if request.Status != nil && strings.TrimSpace(*request.Status) != "" {
		payload["status"] = strings.TrimSpace(*request.Status)
	}
	if request.ChapterReviewMode != nil && (*request.ChapterReviewMode == "auto" || *request.ChapterReviewMode == "manual") {
		payload["chapterReviewMode"] = *request.ChapterReviewMode
	}
	var body struct {
		Config map[string]any `json:"config"`
	}
	if err := s.novelBridgeCall(ctx, userID, "/agent/books/settings", payload, &body); err != nil {
		return nil, err
	}
	return body.Config, nil
}

// DeleteNovelBook removes the InkOS book directory and the Vergestar novel project.
func (s *Service) DeleteNovelBook(ctx context.Context, userID, projectID string) error {
	foundation, err := s.foundationForProject(projectID)
	if err != nil {
		return err
	}
	if err := s.novelBridgeCall(ctx, userID, "/agent/books/delete", novelBridgeRequest{UserID: userID, SessionID: foundation.AgentSessionID, BookID: foundation.InkosBookID}, &struct{}{}); err != nil {
		return err
	}
	return s.DeleteProject(userID, projectID)
}
