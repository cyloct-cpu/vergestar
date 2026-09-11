package app

import (
	"encoding/json"
	"log"
	"strings"
	"time"

	"infinite-canvas/backend/internal/model"
)

// Job 结果消息持久化（T3 补全）：确认动作走异步 Job，此前结果只在轮询响应里，
// 刷新即丢。现在发起任务时落用户消息，任务成功后按 jobID 幂等补写助手消息
// （含确认卡与产物元数据），会话历史在刷新后完整可回放。

// ensureNovelTurnUserMessage persists the user's message when a job starts.
// The session row is created on demand (bridge creates its side lazily too).
func (s *Service) ensureNovelTurnUserMessage(userID string, request NovelAgentTurnRequest) error {
	if strings.TrimSpace(request.Message) == "" || strings.TrimSpace(request.SessionID) == "" {
		// 新会话首条消息由 Bridge 生成 session 后，在结果消息持久化时一并落库。
		return nil
	}
	mode := strings.TrimSpace(request.Mode)
	if mode == "" {
		mode = "long-novel"
	}
	now := time.Now()
	session, err := s.repo.StoryAgentSessionForUser(userID, strings.TrimSpace(request.SessionID))
	if err != nil {
		session = &model.StoryAgentSession{ID: strings.TrimSpace(request.SessionID), UserID: userID, Title: novelAgentSessionTitle(request.Message), Mode: mode, Status: "active", CreatedAt: now, UpdatedAt: now}
		if err := s.repo.CreateStoryAgentSession(session, []model.StoryAgentMessage{}); err != nil {
			// 并发创建冲突视为已存在。
			session, err = s.repo.StoryAgentSessionForUser(userID, strings.TrimSpace(request.SessionID))
			if err != nil {
				return err
			}
		}
	}
	userMessage := model.StoryAgentMessage{ID: newID(), SessionID: session.ID, Role: "user", Content: strings.TrimSpace(request.Message), CreatedAt: now}
	return s.repo.AppendStoryAgentMessages(session, []model.StoryAgentMessage{userMessage})
}

// persistNovelJobAssistantMessage appends the job's assistant message exactly
// once per job (idempotent by using the job ID as the message ID), so repeated
// polling after success does not duplicate the transcript.
func (s *Service) persistNovelJobAssistantMessage(userID string, job NovelAgentJob) {
	if job.Status != "succeeded" || job.Result == nil {
		return
	}
	sessionID := strings.TrimSpace(job.Result.Session.ID)
	if sessionID == "" {
		sessionID = strings.TrimSpace(s.novelJobSessionHint(userID, job.ID))
	}
	if sessionID == "" {
		return
	}
	if _, err := s.repo.StoryAgentMessageExists(sessionID, job.ID); err == nil {
		return // 已写入，幂等跳过
	}
	session, err := s.repo.StoryAgentSessionForUser(userID, sessionID)
	if err != nil {
		session = &model.StoryAgentSession{ID: sessionID, UserID: userID, Title: strings.TrimSpace(job.Result.Session.Title), Mode: job.Result.Session.Mode, Status: "active", CreatedAt: time.Now(), UpdatedAt: time.Now()}
		if err := s.repo.CreateStoryAgentSession(session, []model.StoryAgentMessage{}); err != nil {
			log.Printf("novel job assistant session create failed: job=%s err=%v", job.ID, err)
			return
		}
	}
	skillsJSON, _ := json.Marshal(job.Result.Skills)
	confirmationJSON := []byte("null")
	if job.Result.Confirmation != nil {
		if data, err := json.Marshal(job.Result.Confirmation); err == nil {
			confirmationJSON = data
		}
	}
	artifactsJSON := []byte("null")
	now := time.Now()
	assistantMessage := model.StoryAgentMessage{
		ID:               job.ID, // 幂等键：一个 Job 一条助手消息
		SessionID:        session.ID,
		Role:             "assistant",
		Content:          job.Result.Text,
		SkillsJSON:       string(skillsJSON),
		ConfirmationJSON: string(confirmationJSON),
		ArtifactsJSON:    string(artifactsJSON),
		CreatedAt:        now,
	}
	session.UpdatedAt = now
	if err := s.repo.AppendStoryAgentMessages(session, []model.StoryAgentMessage{assistantMessage}); err != nil {
		log.Printf("novel job assistant message append failed: job=%s err=%v", job.ID, err)
	}
}

// novelJobSessionHint reads the persisted mirror row to recover the session.
func (s *Service) novelJobSessionHint(userID, jobID string) string {
	job, err := s.repo.StoryAgentJobForUser(userID, jobID)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(job.SessionID)
}
