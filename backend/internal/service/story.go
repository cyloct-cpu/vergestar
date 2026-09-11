package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"sort"
	"strings"
	"time"

	"infinite-canvas/backend/internal/model"

	"gorm.io/gorm"
)

type CreateStoryReviewRequest struct {
	Summary string          `json:"summary"`
	Issues  json.RawMessage `json:"issues"`
}

type CreateStoryMemoryRequest struct {
	Kind    string `json:"kind"`
	Content string `json:"content"`
}

type CreateStoryBranchRequest struct {
	Title string          `json:"title"`
	Plan  json.RawMessage `json:"plan"`
}

type CreateStorySceneRequest struct {
	Title             string   `json:"title"`
	Location          string   `json:"location"`
	TimeOfDay         string   `json:"timeOfDay"`
	Action            string   `json:"action"`
	Dialogue          string   `json:"dialogue"`
	Emotion           string   `json:"emotion"`
	VisualIntent      string   `json:"visualIntent"`
	CharacterAssetIDs []string `json:"characterAssetIds"`
}

type CreateStorySceneShotRequest struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	DurationMs  int64  `json:"durationMs"`
}

// StoryFoundationView exposes only the reviewed foundation documents that
// belong to the authenticated Vergestar project. It intentionally omits the
// internal Novel Agent workspace path and any model configuration.
type StoryFoundationView struct {
	ID             string            `json:"id"`
	ProjectID      string            `json:"projectId"`
	InkosBookID    string            `json:"inkosBookId"`
	AgentSessionID string            `json:"agentSessionId,omitempty"`
	Title          string            `json:"title"`
	SourceHash     string            `json:"sourceHash"`
	Files          map[string]string `json:"files"`
	CreatedAt      time.Time         `json:"createdAt"`
	UpdatedAt      time.Time         `json:"updatedAt"`
}

func (s *Service) StoryFoundation(userID, projectID string) (StoryFoundationView, error) {
	project, err := s.repo.ProjectForUser(userID, projectID)
	if err != nil {
		return StoryFoundationView{}, err
	}
	if project.Type != "novel" {
		return StoryFoundationView{}, BadAuthRequest("当前项目不是小说项目")
	}
	foundation, err := s.repo.StoryFoundationForProject(projectID)
	if err != nil {
		return StoryFoundationView{}, err
	}
	files := map[string]string{}
	if strings.TrimSpace(foundation.FilesJSON) != "" {
		if err := json.Unmarshal([]byte(foundation.FilesJSON), &files); err != nil {
			return StoryFoundationView{}, BadAuthRequest("小说基础设定文件格式无效")
		}
	}
	return StoryFoundationView{
		ID: foundation.ID, ProjectID: foundation.ProjectID, InkosBookID: foundation.InkosBookID,
		AgentSessionID: foundation.AgentSessionID,
		Title:          foundation.Title, SourceHash: foundation.SourceHash, Files: files,
		CreatedAt: foundation.CreatedAt, UpdatedAt: foundation.UpdatedAt,
	}, nil
}

func (s *Service) storyProjectUnit(userID, projectID, unitID string) (*model.ProjectUnit, error) {
	project, err := s.repo.ProjectForUser(userID, projectID)
	if err != nil {
		return nil, err
	}
	if project.Type != "novel" {
		return nil, BadAuthRequest("当前项目不是小说项目")
	}
	unit, err := s.repo.ProjectUnit(projectID, unitID)
	if err != nil {
		return nil, err
	}
	return unit, nil
}

func storyHash(title, content string) string {
	digest := sha256.Sum256([]byte(title + "\n" + content))
	return "sha256:" + hex.EncodeToString(digest[:])
}

func (s *Service) SnapshotStoryChapter(userID, projectID, unitID string) (model.StoryChapterVersion, error) {
	unit, err := s.storyProjectUnit(userID, projectID, unitID)
	if err != nil {
		return model.StoryChapterVersion{}, err
	}
	versions, err := s.repo.StoryChapterVersions(projectID, unitID)
	if err != nil {
		return model.StoryChapterVersion{}, err
	}
	hash := storyHash(unit.Title, unit.SourceText)
	if len(versions) > 0 && versions[0].SourceHash == hash {
		return versions[0], nil
	}
	version := model.StoryChapterVersion{ID: newID(), ProjectID: projectID, UnitID: unitID, Number: len(versions) + 1, Title: unit.Title, Content: unit.SourceText, SourceHash: hash, CreatedBy: userID, CreatedAt: time.Now()}
	if err := s.repo.CreateStoryChapterVersion(&version); err != nil {
		return model.StoryChapterVersion{}, err
	}
	return version, nil
}

func (s *Service) StoryChapterVersions(userID, projectID, unitID string) ([]model.StoryChapterVersion, error) {
	if _, err := s.storyProjectUnit(userID, projectID, unitID); err != nil {
		return nil, err
	}
	return s.repo.StoryChapterVersions(projectID, unitID)
}

func (s *Service) RestoreStoryChapterVersion(userID, projectID, unitID, versionID string) (model.ProjectUnit, error) {
	unit, err := s.storyProjectUnit(userID, projectID, unitID)
	if err != nil {
		return model.ProjectUnit{}, err
	}
	version, err := s.repo.StoryChapterVersion(projectID, unitID, versionID)
	if err != nil {
		return model.ProjectUnit{}, err
	}
	unit.Title, unit.SourceText, unit.WordCount = version.Title, version.Content, model.ProjectUnitWordCount(version.Content)
	updatedAt := time.Now()
	if err := s.repo.RestoreStoryChapterVersion(unit, updatedAt); err != nil {
		return model.ProjectUnit{}, err
	}
	unit.UpdatedAt = updatedAt

	// For InkOS-originated books, restore the matching state snapshot after
	// the DB pointer is restored. This keeps current_state/hooks aligned with
	// the selected prose instead of leaving a stale production state behind.
	foundation, foundationErr := s.repo.StoryFoundationForProject(projectID)
	if foundationErr == nil {
		if syncErr := s.restoreNovelChapterSnapshot(userID, foundation, unit.Position+1, version.Content); syncErr != nil {
			log.Printf("novel chapter snapshot sync failed: user=%s project=%s unit=%s err=%v", userID, projectID, unitID, syncErr)
		}
	} else if !errors.Is(foundationErr, gorm.ErrRecordNotFound) {
		return model.ProjectUnit{}, foundationErr
	}
	return *unit, nil
}

func (s *Service) CreateStoryReview(userID, projectID, unitID string, request CreateStoryReviewRequest) (model.StoryReview, error) {
	version, err := s.SnapshotStoryChapter(userID, projectID, unitID)
	if err != nil {
		return model.StoryReview{}, err
	}
	issues := request.Issues
	if len(issues) == 0 {
		issues = json.RawMessage("[]")
	}
	if !json.Valid(issues) {
		return model.StoryReview{}, BadAuthRequest("审稿问题格式必须是 JSON")
	}
	now := time.Now()
	review := model.StoryReview{ID: newID(), ProjectID: projectID, UnitID: unitID, ChapterVersionID: version.ID, Status: "completed", Summary: strings.TrimSpace(request.Summary), IssuesJSON: string(issues), CreatedAt: now, UpdatedAt: now}
	if err := s.repo.CreateStoryReview(&review); err != nil {
		return model.StoryReview{}, err
	}
	return review, nil
}

func (s *Service) StoryReviews(userID, projectID, unitID string) ([]model.StoryReview, error) {
	if _, err := s.storyProjectUnit(userID, projectID, unitID); err != nil {
		return nil, err
	}
	return s.repo.StoryReviews(projectID, unitID)
}

func (s *Service) StoryMemories(userID, projectID string) ([]model.StoryMemory, error) {
	project, err := s.repo.ProjectForUser(userID, projectID)
	if err != nil {
		return nil, err
	}
	if project.Type != "novel" {
		return nil, BadAuthRequest("当前项目不是小说项目")
	}
	return s.repo.StoryMemories(projectID, 200)
}

func (s *Service) CreateStoryMemory(userID, projectID string, request CreateStoryMemoryRequest) (model.StoryMemory, error) {
	project, err := s.repo.ProjectForUser(userID, projectID)
	if err != nil {
		return model.StoryMemory{}, err
	}
	if project.Type != "novel" {
		return model.StoryMemory{}, BadAuthRequest("当前项目不是小说项目")
	}
	content := strings.TrimSpace(request.Content)
	if content == "" || len([]rune(content)) > 6000 {
		return model.StoryMemory{}, BadAuthRequest("故事记忆必须在 1 到 6000 字之间")
	}
	kind := strings.TrimSpace(request.Kind)
	if kind == "" {
		kind = "fact"
	}
	now := time.Now()
	memory := model.StoryMemory{ID: newID(), ProjectID: projectID, Kind: kind, Content: content, SourceType: "manual", SourceHash: storyHash(kind, content), CreatedAt: now, UpdatedAt: now}
	if err := s.repo.CreateStoryMemory(&memory); err != nil {
		return model.StoryMemory{}, err
	}
	return memory, nil
}

func (s *Service) DeleteStoryMemory(userID, projectID, memoryID string) error {
	if _, err := s.StoryMemories(userID, projectID); err != nil {
		return err
	}
	return s.repo.DeleteStoryMemory(projectID, memoryID)
}

func (s *Service) StoryBranches(userID, projectID string) ([]model.StoryBranch, error) {
	if _, err := s.StoryMemories(userID, projectID); err != nil {
		return nil, err
	}
	return s.repo.StoryBranches(projectID)
}

func (s *Service) CreateStoryBranch(userID, projectID, unitID string, request CreateStoryBranchRequest) (model.StoryBranch, error) {
	version, err := s.SnapshotStoryChapter(userID, projectID, unitID)
	if err != nil {
		return model.StoryBranch{}, err
	}
	title := strings.TrimSpace(request.Title)
	if title == "" {
		return model.StoryBranch{}, BadAuthRequest("分支标题不能为空")
	}
	plan := request.Plan
	if len(plan) == 0 {
		plan = json.RawMessage("{}")
	}
	if !json.Valid(plan) {
		return model.StoryBranch{}, BadAuthRequest("分支计划格式必须是 JSON")
	}
	now := time.Now()
	branch := model.StoryBranch{ID: newID(), ProjectID: projectID, BaseUnitID: unitID, BaseSourceHash: version.SourceHash, Title: title, PlanJSON: string(plan), Status: "candidate", CreatedAt: now, UpdatedAt: now}
	if err := s.repo.CreateStoryBranch(&branch); err != nil {
		return model.StoryBranch{}, err
	}
	return branch, nil
}

func (s *Service) StoryScenes(userID, projectID, unitID string) ([]StorySceneView, error) {
	if _, err := s.storyProjectUnit(userID, projectID, unitID); err != nil {
		return nil, err
	}
	scenes, err := s.repo.StoryScenes(projectID, unitID)
	if err != nil {
		return nil, err
	}
	links, err := s.repo.StorySceneShotLinks(projectID, unitID)
	if err != nil {
		return nil, err
	}
	shots, err := s.repo.ProjectShots(projectID)
	if err != nil {
		return nil, err
	}
	shotByID := make(map[string]model.Shot, len(shots))
	for _, shot := range shots {
		if shot.UnitID == unitID {
			shotByID[shot.ID] = shot
		}
	}
	shotIDs := make(map[string][]string, len(links))
	for _, link := range links {
		if _, exists := shotByID[link.ShotID]; exists {
			shotIDs[link.SceneID] = append(shotIDs[link.SceneID], link.ShotID)
		}
	}
	result := make([]StorySceneView, 0, len(scenes))
	for _, scene := range scenes {
		ids := shotIDs[scene.ID]
		sort.Slice(ids, func(left, right int) bool {
			leftShot, rightShot := shotByID[ids[left]], shotByID[ids[right]]
			return leftShot.Position < rightShot.Position || leftShot.Position == rightShot.Position && leftShot.ID < rightShot.ID
		})
		view := StorySceneView{StoryScene: scene, ShotIDs: ids}
		if len(ids) > 0 {
			view.ShotID = ids[0]
		}
		result = append(result, view)
	}
	return result, nil
}

func (s *Service) CreateStoryScene(userID, projectID, unitID string, request CreateStorySceneRequest) (model.StoryScene, error) {
	if _, err := s.storyProjectUnit(userID, projectID, unitID); err != nil {
		return model.StoryScene{}, err
	}
	title := strings.TrimSpace(request.Title)
	if title == "" {
		return model.StoryScene{}, BadAuthRequest("场次标题不能为空")
	}
	if len(request.CharacterAssetIDs) > 12 {
		return model.StoryScene{}, BadAuthRequest("一个场次最多关联 12 个角色资产")
	}
	ids := make([]string, 0, len(request.CharacterAssetIDs))
	for _, rawID := range request.CharacterAssetIDs {
		id := strings.TrimSpace(rawID)
		if id == "" {
			continue
		}
		asset, assetErr := s.repo.ProjectCharacterAsset(userID, projectID, id)
		if assetErr != nil || asset == nil {
			return model.StoryScene{}, BadAuthRequest("场次角色资产不可用")
		}
		ids = append(ids, id)
	}
	idsJSON, _ := json.Marshal(ids)
	scenes, err := s.repo.StoryScenes(projectID, unitID)
	if err != nil {
		return model.StoryScene{}, err
	}
	now := time.Now()
	scene := model.StoryScene{ID: newID(), ProjectID: projectID, UnitID: unitID, Position: len(scenes), Title: title, Location: strings.TrimSpace(request.Location), TimeOfDay: strings.TrimSpace(request.TimeOfDay), Action: strings.TrimSpace(request.Action), Dialogue: strings.TrimSpace(request.Dialogue), Emotion: strings.TrimSpace(request.Emotion), VisualIntent: strings.TrimSpace(request.VisualIntent), CharacterAssetIDsJSON: string(idsJSON), SourceHash: storyHash(title, strings.Join([]string{request.Location, request.Action, request.Dialogue, request.VisualIntent, string(idsJSON)}, "\n")), Status: "draft", CreatedAt: now, UpdatedAt: now}
	if err := s.repo.CreateStoryScene(&scene); err != nil {
		return model.StoryScene{}, err
	}
	return scene, nil
}

func (s *Service) CreateStorySceneShot(userID, projectID, unitID, sceneID string, request CreateStorySceneShotRequest) (model.Shot, error) {
	if _, err := s.storyProjectUnit(userID, projectID, unitID); err != nil {
		return model.Shot{}, err
	}
	scene, err := s.repo.StoryScene(projectID, unitID, sceneID)
	if err != nil {
		return model.Shot{}, err
	}
	description := strings.TrimSpace(request.Description)
	if description == "" {
		description = strings.TrimSpace(scene.Action)
	}
	if description == "" {
		return model.Shot{}, BadAuthRequest("场次缺少可用于分镜的行动描述")
	}
	title := strings.TrimSpace(request.Title)
	if title == "" {
		title = scene.Title
	}
	shot, err := s.CreateProjectShot(userID, projectID, CreateProjectShotRequest{
		UnitID: unitID, Title: title, Description: description, Position: scene.Position, DurationMs: request.DurationMs,
		Revision: ShotRevisionInput{PlotDescription: description, Action: scene.Action, Dialogue: scene.Dialogue, VideoPrompt: scene.VisualIntent, ContinuityNotes: "storyScene:" + scene.ID + " sourceHash:" + scene.SourceHash},
	})
	if err != nil {
		return model.Shot{}, err
	}
	now := time.Now()
	link := model.StorySceneShotLink{ID: newID(), ProjectID: projectID, UnitID: unitID, SceneID: scene.ID, ShotID: shot.ID, SourceHash: scene.SourceHash, CreatedAt: now, UpdatedAt: now}
	if err := s.repo.CreateStorySceneShotLink(&link); err != nil {
		return model.Shot{}, err
	}
	return shot, nil
}

type restoreChapterSnapshotRequest struct {
	UserID        string `json:"userId"`
	SessionID     string `json:"sessionId"`
	BookID        string `json:"bookId"`
	ChapterNumber int    `json:"chapterNumber"`
}

// restoreNovelChapterSnapshot asks the Novel Agent bridge to restore InkOS
// state files for the chapter. The returned artifacts are checked against the
// restored Vergestar version before they are projected, so a stale workspace
// cannot overwrite the selected DB version.
func (s *Service) restoreNovelChapterSnapshot(userID string, foundation *model.StoryFoundation, chapterNumber int, expectedContent string) error {
	if s.novelAgent == nil || foundation == nil {
		return nil
	}
	sessionID := strings.TrimSpace(foundation.AgentSessionID)
	bookID := strings.TrimSpace(foundation.InkosBookID)
	if sessionID == "" || bookID == "" {
		return nil
	}
	payload, err := json.Marshal(restoreChapterSnapshotRequest{
		UserID: userID, SessionID: sessionID, BookID: bookID, ChapterNumber: chapterNumber,
	})
	if err != nil {
		return WrapAppError(http.StatusInternalServerError, "序列化章节状态回溯请求失败", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.novelAgent.endpoint+"/agent/chapters/restore-snapshot", bytes.NewReader(payload))
	if err != nil {
		return WrapAppError(http.StatusInternalServerError, "创建章节状态回溯请求失败", err)
	}
	req.Header.Set("content-type", "application/json")
	req.Header.Set("x-vergestar-novel-agent-token", s.novelAgent.token)
	req.Header.Set("x-vergestar-novel-agent-user", userID)
	resp, err := s.novelAgent.http.Do(req)
	if err != nil {
		return WrapAppError(http.StatusServiceUnavailable, "章节状态回溯服务暂不可用", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return NewAppError(http.StatusBadGateway, "章节状态回溯失败")
	}
	var body struct {
		Artifacts NovelAgentArtifacts `json:"artifacts"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return WrapAppError(http.StatusBadGateway, "章节状态回溯响应无效", err)
	}
	content := chapterContentForNumber(body.Artifacts.Files, chapterNumber)
	if content == "" || strings.TrimSpace(content) != strings.TrimSpace(expectedContent) {
		return NewAppError(http.StatusConflict, "InkOS 章节文件与回溯版本不一致，已停止状态投影")
	}
	if _, err := s.syncNovelAgentArtifacts(userID, body.Artifacts, body.Artifacts.AgentSessionID); err != nil {
		return err
	}
	return nil
}

type StorySceneView struct {
	model.StoryScene
	ShotID  string   `json:"shotId,omitempty"`
	ShotIDs []string `json:"shotIds,omitempty"`
}
