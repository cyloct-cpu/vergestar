package app

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"infinite-canvas/backend/internal/model"
)

// --- T3：Job 历史/断线恢复（DB 镜像）与会话管理 ---

// mirrorNovelAgentJob upserts the DB mirror of a bridge job after each poll.
// The bridge owns execution + artifacts; the DB owns user-facing history.
func (s *Service) mirrorNovelAgentJob(userID string, job NovelAgentJob, requestMeta NovelAgentJobRequestMeta) {
	if job.ID == "" {
		return
	}
	// 轮询路径没有完整请求上下文：从已有镜像行回填归属字段。
	existing, err := s.repo.StoryAgentJobForUser(userID, job.ID)
	if err == nil && existing != nil && requestMeta.SessionID == "" {
		requestMeta = NovelAgentJobRequestMeta{SessionID: existing.SessionID, BookID: existing.BookID, Mode: existing.Mode, ConfirmedIntent: existing.ConfirmedIntent}
	}
	mirror := model.StoryAgentJob{
		ID:              job.ID,
		UserID:          userID,
		SessionID:       strings.TrimSpace(requestMeta.SessionID),
		BookID:          strings.TrimSpace(requestMeta.BookID),
		Mode:            strings.TrimSpace(requestMeta.Mode),
		ConfirmedIntent: strings.TrimSpace(requestMeta.ConfirmedIntent),
		Status:          job.Status,
		Stage:           strings.TrimSpace(job.Stage),
		Error:           strings.TrimSpace(job.Error),
		CreatedAt:       parseBridgeTime(job.CreatedAt),
		UpdatedAt:       parseBridgeTime(job.UpdatedAt),
	}
	if logs, err := json.Marshal(job.Logs); err == nil {
		mirror.LogsJSON = string(logs)
	}
	if job.Result != nil {
		mirror.ResultSummary = strings.TrimSpace(job.Result.Text)
		if len(mirror.ResultSummary) > 800 {
			mirror.ResultSummary = mirror.ResultSummary[:800]
		}
	}
	if err == nil && existing != nil {
		mirror.CreatedAt = existing.CreatedAt
	}
	_ = s.repo.StoryAgentJobUpsert(&mirror)
}

// NovelAgentJobRequestMeta carries the request context captured when a job is
// started, so the mirror row can attribute it even before the first poll.
type NovelAgentJobRequestMeta struct {
	SessionID       string `json:"sessionId,omitempty"`
	BookID          string `json:"bookId,omitempty"`
	Mode            string `json:"mode,omitempty"`
	ConfirmedIntent string `json:"confirmedIntent,omitempty"`
}

func requestMetaFromTurn(request NovelAgentTurnRequest) NovelAgentJobRequestMeta {
	return NovelAgentJobRequestMeta{
		SessionID:       request.SessionID,
		BookID:          request.BookID,
		Mode:            request.Mode,
		ConfirmedIntent: request.ConfirmedIntent,
	}
}

// NovelAgentJobHistoryView is the history-list row: no logs, no result body.
type NovelAgentJobHistoryView struct {
	ID              string `json:"id"`
	Status          string `json:"status"`
	Stage           string `json:"stage"`
	Error           string `json:"error,omitempty"`
	Mode            string `json:"mode,omitempty"`
	ConfirmedIntent string `json:"confirmedIntent,omitempty"`
	SessionID       string `json:"sessionId,omitempty"`
	BookID          string `json:"bookId,omitempty"`
	ResultSummary   string `json:"resultSummary,omitempty"`
	CreatedAt       string `json:"createdAt"`
	UpdatedAt       string `json:"updatedAt"`
}

func jobHistoryView(job model.StoryAgentJob) NovelAgentJobHistoryView {
	return NovelAgentJobHistoryView{
		ID:              job.ID,
		Status:          job.Status,
		Stage:           job.Stage,
		Error:           job.Error,
		Mode:            job.Mode,
		ConfirmedIntent: job.ConfirmedIntent,
		SessionID:       job.SessionID,
		BookID:          job.BookID,
		ResultSummary:   job.ResultSummary,
		CreatedAt:       job.CreatedAt.Format(time.RFC3339),
		UpdatedAt:       job.UpdatedAt.Format(time.RFC3339),
	}
}

// NovelAgentJobHistory returns the user's recent jobs from the DB mirror.
func (s *Service) NovelAgentJobHistory(ctx context.Context, userID string, limit int) ([]NovelAgentJobHistoryView, error) {
	_ = ctx
	jobs, err := s.repo.StoryAgentJobsForUser(userID, limit)
	if err != nil {
		return nil, err
	}
	views := make([]NovelAgentJobHistoryView, 0, len(jobs))
	for _, job := range jobs {
		views = append(views, jobHistoryView(job))
	}
	return views, nil
}

// NovelAgentActiveJobs returns the user's queued/running jobs (disconnect recovery).
func (s *Service) NovelAgentActiveJobs(ctx context.Context, userID string) ([]NovelAgentJobHistoryView, error) {
	_ = ctx
	jobs, err := s.repo.StoryAgentActiveJobsForUser(userID)
	if err != nil {
		return nil, err
	}
	views := make([]NovelAgentJobHistoryView, 0, len(jobs))
	for _, job := range jobs {
		views = append(views, jobHistoryView(job))
	}
	return views, nil
}

// --- 会话管理（重命名/删除）：Vergestar DB 为用户可见真相，Bridge 工作区同步 ---

func (s *Service) RenameNovelAgentSession(ctx context.Context, userID, sessionID, title string) error {
	sessionID = strings.TrimSpace(sessionID)
	title = strings.TrimSpace(title)
	if sessionID == "" || title == "" {
		return BadAuthRequest("会话 ID 和标题不能为空")
	}
	if _, err := s.repo.StoryAgentSessionForUser(userID, sessionID); err != nil {
		return err
	}
	if s.novelAgent != nil {
		req, err := http.NewRequestWithContext(ctx, http.MethodPut, s.novelAgent.endpoint+"/agent/sessions/"+strings.TrimSpace(sessionID), strings.NewReader(jsonify(map[string]any{"title": title})))
		if err == nil {
			req.Header.Set("content-type", "application/json")
			req.Header.Set("x-vergestar-novel-agent-token", s.novelAgent.token)
			req.Header.Set("x-vergestar-novel-agent-user", userID)
			resp, err := s.novelAgent.http.Do(req)
			if err == nil {
				_ = resp.Body.Close()
			}
			// Bridge 侧失败不阻断 DB 重命名：UI 列表以 DB 为真相。
		}
	}
	return s.repo.StoryAgentSessionRename(userID, sessionID, title)
}

func (s *Service) DeleteNovelAgentSession(ctx context.Context, userID, sessionID string) error {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return BadAuthRequest("会话 ID 不能为空")
	}
	if _, err := s.repo.StoryAgentSessionForUser(userID, sessionID); err != nil {
		return err
	}
	if s.novelAgent != nil {
		req, err := http.NewRequestWithContext(ctx, http.MethodDelete, s.novelAgent.endpoint+"/agent/sessions/"+sessionID, nil)
		if err == nil {
			req.Header.Set("x-vergestar-novel-agent-token", s.novelAgent.token)
			req.Header.Set("x-vergestar-novel-agent-user", userID)
			resp, err := s.novelAgent.http.Do(req)
			if err == nil {
				_ = resp.Body.Close()
			}
		}
	}
	return s.repo.StoryAgentSessionDelete(userID, sessionID)
}

// sessionIDOf 在轮询路径上尽力还原请求上下文：优先用已有镜像行的归属字段。
func sessionIDOf(job NovelAgentJob) string {
	if job.Result != nil && job.Result.Session.ID != "" {
		return job.Result.Session.ID
	}
	return ""
}

func parseBridgeTime(value string) time.Time {
	parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(value))
	if err != nil {
		return time.Now()
	}
	return parsed
}

func jsonify(value map[string]any) string {
	data, _ := json.Marshal(value)
	return string(data)
}
