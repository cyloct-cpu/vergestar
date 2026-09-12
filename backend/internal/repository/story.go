package repository

import (
	"strings"
	"time"

	"infinite-canvas/backend/internal/model"

	"gorm.io/gorm"
)

func (r *Repository) StoryChapterVersions(projectID, unitID string) ([]model.StoryChapterVersion, error) {
	var versions []model.StoryChapterVersion
	err := r.db.Where("project_id = ? AND unit_id = ?", projectID, unitID).Order("number desc").Find(&versions).Error
	return versions, err
}

func (r *Repository) StoryChapterVersion(projectID, unitID, versionID string) (*model.StoryChapterVersion, error) {
	var version model.StoryChapterVersion
	if err := r.db.First(&version, "id = ? AND project_id = ? AND unit_id = ?", versionID, projectID, unitID).Error; err != nil {
		return nil, err
	}
	return &version, nil
}

func (r *Repository) CreateStoryChapterVersion(version *model.StoryChapterVersion) error {
	return r.db.Create(version).Error
}

func (r *Repository) RestoreStoryChapterVersion(unit *model.ProjectUnit, updatedAt time.Time) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.ProjectUnit{}).Where("id = ? AND project_id = ?", unit.ID, unit.ProjectID).Updates(map[string]any{
			"title": unit.Title, "source_text": unit.SourceText, "word_count": unit.WordCount, "updated_at": updatedAt,
		}).Error; err != nil {
			return err
		}
		return tx.Model(&model.Project{}).Where("id = ?", unit.ProjectID).Updates(map[string]any{"revision": gorm.Expr("revision + 1"), "updated_at": updatedAt}).Error
	})
}

func (r *Repository) StoryReviews(projectID, unitID string) ([]model.StoryReview, error) {
	var reviews []model.StoryReview
	err := r.db.Where("project_id = ? AND unit_id = ?", projectID, unitID).Order("created_at desc").Find(&reviews).Error
	return reviews, err
}

func (r *Repository) CreateStoryReview(review *model.StoryReview) error {
	return r.db.Create(review).Error
}

func (r *Repository) UpdateStoryReviewContent(reviewID, projectID, status, summary, issuesJSON string) error {
	return r.db.Model(&model.StoryReview{}).Where("id = ? AND project_id = ?", reviewID, projectID).Updates(map[string]any{
		"status": status, "summary": summary, "issues_json": issuesJSON, "updated_at": time.Now(),
	}).Error
}
func (r *Repository) StoryMemories(projectID string, limit int) ([]model.StoryMemory, error) {
	var memories []model.StoryMemory
	err := r.db.Where("project_id = ?", projectID).Order("updated_at desc").Limit(limit).Find(&memories).Error
	return memories, err
}

func (r *Repository) CreateStoryMemory(memory *model.StoryMemory) error {
	return r.db.Create(memory).Error
}

func (r *Repository) UpdateStoryMemoryContent(memoryID, projectID, kind, content, sourceHash string) error {
	return r.db.Model(&model.StoryMemory{}).Where("id = ? AND project_id = ?", memoryID, projectID).Updates(map[string]any{
		"kind": kind, "content": content, "source_hash": sourceHash, "updated_at": time.Now(),
	}).Error
}
func (r *Repository) DeleteStoryMemory(projectID, memoryID string) error {
	return r.db.Delete(&model.StoryMemory{}, "id = ? AND project_id = ?", memoryID, projectID).Error
}

func (r *Repository) StoryBranches(projectID string) ([]model.StoryBranch, error) {
	var branches []model.StoryBranch
	err := r.db.Where("project_id = ?", projectID).Order("updated_at desc").Find(&branches).Error
	return branches, err
}

func (r *Repository) CreateStoryBranch(branch *model.StoryBranch) error {
	return r.db.Create(branch).Error
}

func (r *Repository) StoryScenes(projectID, unitID string) ([]model.StoryScene, error) {
	var scenes []model.StoryScene
	err := r.db.Where("project_id = ? AND unit_id = ?", projectID, unitID).Order("position asc, created_at asc").Find(&scenes).Error
	return scenes, err
}

func (r *Repository) CreateStoryScene(scene *model.StoryScene) error {
	return r.db.Create(scene).Error
}

func (r *Repository) StoryScene(projectID, unitID, sceneID string) (*model.StoryScene, error) {
	var scene model.StoryScene
	if err := r.db.First(&scene, "id = ? AND project_id = ? AND unit_id = ?", sceneID, projectID, unitID).Error; err != nil {
		return nil, err
	}
	return &scene, nil
}

func (r *Repository) CreateStorySceneShotLink(link *model.StorySceneShotLink) error {
	return r.db.Create(link).Error
}

func (r *Repository) UpdateStorySceneShotLink(link *model.StorySceneShotLink) error {
	return r.db.Model(&model.StorySceneShotLink{}).Where("id = ?", link.ID).Updates(map[string]any{
		"scene_id": link.SceneID, "source_hash": link.SourceHash, "updated_at": link.UpdatedAt,
	}).Error
}

func (r *Repository) DeleteStoryScene(sceneID string) error {
	return r.db.Delete(&model.StoryScene{}, "id = ?", sceneID).Error
}

func (r *Repository) StoryAgentSessions(userID string) ([]model.StoryAgentSession, error) {
	sessions := make([]model.StoryAgentSession, 0)
	err := r.db.Where("user_id = ?", userID).Order("updated_at desc").Limit(100).Find(&sessions).Error
	return sessions, err
}

func (r *Repository) StoryAgentSessionForUser(userID, sessionID string) (*model.StoryAgentSession, error) {
	var session model.StoryAgentSession
	if err := r.db.First(&session, "id = ? AND user_id = ?", sessionID, userID).Error; err != nil {
		return nil, err
	}
	return &session, nil
}

func (r *Repository) StoryAgentMessages(sessionID string) ([]model.StoryAgentMessage, error) {
	messages := make([]model.StoryAgentMessage, 0)
	err := r.db.Where("session_id = ?", sessionID).Order("created_at asc").Find(&messages).Error
	return messages, err
}

// StoryAgentSessionsSearch supports the novel sidebar search + pagination (T3).
func (r *Repository) StoryAgentSessionsSearch(userID, query string, page, pageSize int) ([]model.StoryAgentSession, int64, error) {
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}
	if page <= 0 {
		page = 1
	}
	db := r.db.Model(&model.StoryAgentSession{}).Where("user_id = ?", userID)
	if query = strings.TrimSpace(query); query != "" {
		db = db.Where("title LIKE ?", "%"+query+"%")
	}
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	sessions := make([]model.StoryAgentSession, 0)
	err := db.Order("updated_at desc").Offset((page - 1) * pageSize).Limit(pageSize).Find(&sessions).Error
	return sessions, total, err
}

func (r *Repository) StoryAgentSessionsForUser(userID string) ([]model.StoryAgentSession, error) {
	return r.StoryAgentSessions(userID)
}

func (r *Repository) CreateStoryAgentSession(session *model.StoryAgentSession, messages []model.StoryAgentMessage) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(session).Error; err != nil {
			return err
		}
		return tx.Create(&messages).Error
	})
}

func (r *Repository) AppendStoryAgentMessages(session *model.StoryAgentSession, messages []model.StoryAgentMessage) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&messages).Error; err != nil {
			return err
		}
		return tx.Model(&model.StoryAgentSession{}).Where("id = ? AND user_id = ?", session.ID, session.UserID).Update("updated_at", session.UpdatedAt).Error
	})
}

// ConsumeStoryAgentConfirmations makes an accepted production action
// non-replayable in the user-facing transcript. The bridge job remains the
// source of execution status; this only removes an already accepted button.
func (r *Repository) ConsumeStoryAgentConfirmations(userID, sessionID string) error {
	return r.db.Model(&model.StoryAgentMessage{}).
		Where("session_id = ? AND confirmation_json <> '' AND EXISTS (SELECT 1 FROM story_agent_sessions WHERE story_agent_sessions.id = story_agent_messages.session_id AND story_agent_sessions.user_id = ?)", sessionID, userID).
		Update("confirmation_json", "").Error
}

func (r *Repository) StoryFoundationForBook(userID, inkosBookID string) (*model.StoryFoundation, error) {
	var foundation model.StoryFoundation
	if err := r.db.First(&foundation, "user_id = ? AND inkos_book_id = ?", userID, inkosBookID).Error; err != nil {
		return nil, err
	}
	return &foundation, nil
}

func (r *Repository) StoryFoundationForProject(projectID string) (*model.StoryFoundation, error) {
	var foundation model.StoryFoundation
	if err := r.db.First(&foundation, "project_id = ?", projectID).Error; err != nil {
		return nil, err
	}
	return &foundation, nil
}

func (r *Repository) CreateStoryFoundation(foundation *model.StoryFoundation) error {
	return r.db.Create(foundation).Error
}

func (r *Repository) UpdateStoryFoundation(foundation *model.StoryFoundation) error {
	return r.db.Model(&model.StoryFoundation{}).Where("id = ?", foundation.ID).Updates(map[string]any{
		"agent_session_id": foundation.AgentSessionID,
		"title":            foundation.Title,
		"book_json":        foundation.BookJSON,
		"files_json":       foundation.FilesJSON,
		"source_hash":      foundation.SourceHash,
		"updated_at":       foundation.UpdatedAt,
	}).Error
}

func (r *Repository) BindStoryAgentSessionProject(userID, sessionID, projectID string) error {
	result := r.db.Model(&model.StoryAgentSession{}).
		Where("id = ? AND user_id = ?", sessionID, userID).
		Update("project_id", projectID)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *Repository) UpdateStoryScene(scene *model.StoryScene) error {
	return r.db.Model(&model.StoryScene{}).Where("id = ? AND project_id = ?", scene.ID, scene.ProjectID).Updates(map[string]any{
		"title": scene.Title, "location": scene.Location, "time_of_day": scene.TimeOfDay, "action": scene.Action,
		"dialogue": scene.Dialogue, "emotion": scene.Emotion, "visual_intent": scene.VisualIntent,
		"character_asset_ids_json": scene.CharacterAssetIDsJSON, "source_hash": scene.SourceHash, "updated_at": scene.UpdatedAt,
	}).Error
}

func (r *Repository) StorySceneShotLinks(projectID, unitID string) ([]model.StorySceneShotLink, error) {
	links := make([]model.StorySceneShotLink, 0)
	err := r.db.Where("project_id = ? AND unit_id = ?", projectID, unitID).Find(&links).Error
	return links, err
}

// StoryAgentMessageExists reports whether a message with the given ID exists
// in the session (used for idempotent job-result persistence).
func (r *Repository) StoryAgentMessageExists(sessionID, messageID string) (bool, error) {
	var count int64
	err := r.db.Model(&model.StoryAgentMessage{}).Where("session_id = ? AND id = ?", sessionID, messageID).Count(&count).Error
	return count > 0, err
}

// ShotRevisionByID fetches a single shot revision by ID (backfill read path).
func (r *Repository) ShotRevisionByID(revisionID string) (*model.ShotRevision, error) {
	var revision model.ShotRevision
	if err := r.db.First(&revision, "id = ?", revisionID).Error; err != nil {
		return nil, err
	}
	return &revision, nil
}

// UpdateShotRevisionFields persists structured storyboard fields on an
// existing revision (T6 backfill). Only the columns provided are updated.
func (r *Repository) UpdateShotRevisionFields(revision *model.ShotRevision) error {
	return r.db.Model(&model.ShotRevision{}).Where("id = ?", revision.ID).Updates(map[string]any{
		"shot_size":    revision.ShotSize,
		"camera_angle": revision.CameraAngle,
		"dialogue":     revision.Dialogue,
		"action":       revision.Action,
	}).Error
}
