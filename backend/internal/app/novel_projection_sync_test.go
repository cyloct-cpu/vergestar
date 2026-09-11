package app

import (
	"encoding/json"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"infinite-canvas/backend/internal/model"
	"infinite-canvas/backend/internal/repository"
	"slices"
	"testing"
	"time"
)

func TestSyncNovelProductionArtifactsMapsStoryboardShotsToScriptScenesIdempotently(t *testing.T) {
	svc, db := newProjectWorkflowV2TestService(t)
	if err := db.AutoMigrate(&model.StoryScene{}, &model.StorySceneShotLink{}); err != nil {
		t.Fatal(err)
	}
	project, unit := seedWorkflowProject(t, db)
	if err := db.Model(&model.Project{}).Where("id = ?", project.ID).Update("type", "novel").Error; err != nil {
		t.Fatal(err)
	}
	guNing, err := svc.CreateProjectCharacter("user-1", project.ID, CreateProjectCharacterRequest{Name: "顾宁", Definition: map[string]any{}})
	if err != nil {
		t.Fatal(err)
	}
	dayShift, err := svc.CreateProjectCharacter("user-1", project.ID, CreateProjectCharacterRequest{Name: "白班同事", Definition: map[string]any{}})
	if err != nil {
		t.Fatal(err)
	}
	manifestRaw, err := json.Marshal(novelProductionAssetsManifest{
		ProjectID: "book-1",
		Assets: []novelProductionShot{
			{ShotID: "shot-001", Status: "prompt_ready", Prompt: "地下库房入口，顾宁按下工号，门禁打开"},
			{ShotID: "shot-002", Status: "prompt_ready", Prompt: "库房交接窗口，白班同事把铜色钥匙推给顾宁"},
			{ShotID: "shot-003", Status: "prompt_ready", Prompt: "B-14架前，顾宁核对卷宗档号"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	artifacts := NovelAgentArtifacts{
		BookID:           "book-1",
		ProductionUnitID: unit.ID,
		ProductionFiles: map[string]string{
			"dramas/book-1-ch-0001/script.md": `## 剧本正文

**场次1 地下库房入口 夜**
人物：顾宁、白班同事
动作：顾宁在签到机按下工号，白班同事把铜色钥匙推过来。
对白：顾宁：移交栏空着。

**场次2 B-14架前 夜**
人物：顾宁
动作：顾宁核对卷宗档号，发现冻结卷宗回到普通架。
对白：顾宁：不该在这儿。`,
			"storyboards/book-1-ch-0001/storyboard.md": "# 分镜\n",
			"storyboards/book-1-ch-0001/assets.json":   string(manifestRaw),
		},
	}
	if err := svc.syncNovelProductionArtifacts("user-1", project.ID, artifacts); err != nil {
		t.Fatal(err)
	}
	scenes, err := svc.repo.StoryScenes(project.ID, unit.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(scenes) != 2 {
		t.Fatalf("scenes = %#v, want two script scenes", scenes)
	}
	var firstSceneCharacterIDs []string
	if err := json.Unmarshal([]byte(scenes[0].CharacterAssetIDsJSON), &firstSceneCharacterIDs); err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(firstSceneCharacterIDs, guNing.Asset.ID) || !slices.Contains(firstSceneCharacterIDs, dayShift.Asset.ID) {
		t.Fatalf("first script scene character assets = %#v", firstSceneCharacterIDs)
	}
	var secondSceneCharacterIDs []string
	if err := json.Unmarshal([]byte(scenes[1].CharacterAssetIDsJSON), &secondSceneCharacterIDs); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(secondSceneCharacterIDs, []string{guNing.Asset.ID}) {
		t.Fatalf("second script scene character assets = %#v", secondSceneCharacterIDs)
	}
	links, err := svc.repo.StorySceneShotLinks(project.ID, unit.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(links) != 3 {
		t.Fatalf("links = %#v, want three storyboard shots", links)
	}
	shotCountByScene := map[string]int{}
	shotPositionsByScene := map[string][]int{}
	shots, err := svc.repo.ProjectShots(project.ID)
	if err != nil {
		t.Fatal(err)
	}
	shotPositionByID := make(map[string]int, len(shots))
	for _, shot := range shots {
		shotPositionByID[shot.ID] = shot.Position
	}
	for _, link := range links {
		shotCountByScene[link.SceneID]++
		shotPositionsByScene[link.SceneID] = append(shotPositionsByScene[link.SceneID], shotPositionByID[link.ShotID])
	}
	if shotCountByScene[scenes[0].ID] != 2 || shotCountByScene[scenes[1].ID] != 1 {
		t.Fatalf("shot count by script scene = %#v", shotCountByScene)
	}
	for _, positions := range shotPositionsByScene {
		slices.Sort(positions)
	}
	if !slices.Equal(shotPositionsByScene[scenes[0].ID], []int{0, 1}) || !slices.Equal(shotPositionsByScene[scenes[1].ID], []int{2}) {
		t.Fatalf("shot positions by script scene = %#v", shotPositionsByScene)
	}
	if err := svc.syncNovelProductionArtifacts("user-1", project.ID, artifacts); err != nil {
		t.Fatal(err)
	}
	links, err = svc.repo.StorySceneShotLinks(project.ID, unit.ID)
	if err != nil {
		t.Fatal(err)
	}
	shots, err = svc.repo.ProjectShots(project.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(links) != 3 || len(shots) != 3 {
		t.Fatalf("repeat projection links/shots = %d/%d, want 3/3", len(links), len(shots))
	}
}

func TestSyncNovelProductionArtifactsCreatesPendingCandidatesForUnmatchedScriptCharacters(t *testing.T) {
	svc, db := newProjectWorkflowV2TestService(t)
	if err := db.AutoMigrate(&model.StoryScene{}, &model.StorySceneShotLink{}, &model.StoryFoundation{}); err != nil {
		t.Fatal(err)
	}
	project, unit := seedWorkflowProject(t, db)
	if err := db.Model(&model.Project{}).Where("id = ?", project.ID).Update("type", "novel").Error; err != nil {
		t.Fatal(err)
	}
	artifacts := NovelAgentArtifacts{
		BookID:           "book-1",
		ProductionUnitID: unit.ID,
		ProductionFiles: map[string]string{
			"dramas/book-1-ch-0001/script.md": `## 剧本正文

**场次1 终端前 夜**
人物：顾宁、终端系统声
动作：顾宁听见终端的系统提示。
对白：终端系统声：补验单生成。`,
		},
	}
	filesJSON, err := json.Marshal(artifacts.ProductionFiles)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	foundation := model.StoryFoundation{ID: "foundation-1", ProjectID: project.ID, UserID: "user-1", InkosBookID: artifacts.BookID, Title: "候选角色验收", FilesJSON: string(filesJSON), SourceHash: "sha256:test", CreatedAt: now, UpdatedAt: now}
	if err := db.Create(&foundation).Error; err != nil {
		t.Fatal(err)
	}
	if err := svc.syncNovelProductionArtifacts("user-1", project.ID, artifacts); err != nil {
		t.Fatal(err)
	}
	candidates, err := svc.repo.ProjectAssetCandidates(project.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) != 2 {
		t.Fatalf("candidates = %#v, want two unmatched script characters", candidates)
	}
	for _, candidate := range candidates {
		if candidate.Category != model.AssetCategoryCharacter || candidate.Status != "pending_confirmation" || candidate.Source != assetCandidateSourceInkOSScriptCharacter || candidate.UnitID != unit.ID {
			t.Fatalf("candidate metadata = %#v", candidate)
		}
	}
	if candidates[0].Name != "终端系统声" && candidates[1].Name != "终端系统声" {
		t.Fatalf("terminal system voice candidate missing: %#v", candidates)
	}
	var guNingCandidate model.ProjectAssetCandidate
	for _, candidate := range candidates {
		if candidate.Name == "顾宁" {
			guNingCandidate = candidate
			break
		}
	}
	if guNingCandidate.ID == "" {
		t.Fatalf("顾宁 candidate missing: %#v", candidates)
	}
	confirmed, err := svc.ConfirmProjectAssetCandidate("user-1", project.ID, guNingCandidate.ID, ConfirmProjectAssetCandidateRequest{})
	if err != nil {
		t.Fatal(err)
	}
	scenes, err := svc.repo.StoryScenes(project.ID, unit.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(scenes) != 1 {
		t.Fatalf("scenes = %#v", scenes)
	}
	var characterIDs []string
	if err := json.Unmarshal([]byte(scenes[0].CharacterAssetIDsJSON), &characterIDs); err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(characterIDs, confirmed.ID) {
		t.Fatalf("confirmed character was not projected to the script scene: %#v", characterIDs)
	}
}

func TestSyncNovelChapterMetadataUpdatesReviewWithoutNewVersion(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.Project{}, &model.ProjectUnit{}, &model.StoryChapterVersion{}, &model.StoryReview{}); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	unit := model.ProjectUnit{ID: "unit-1", ProjectID: "project-1", Kind: model.ProjectUnitKindChapter, Title: "签名", SourceText: "第一章正文", WordCount: 5, Status: model.ProjectUnitStatusDraft, Position: 0, CreatedAt: now, UpdatedAt: now}
	if err := db.Create(&unit).Error; err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"chapters/0001_签名.md": "第一章正文",
		"chapters/index.json": `[{"number":1,"title":"签名","status":"ready-for-review","auditIssues":["旧审稿"]}]`,
	}
	svc := &Service{repo: repository.New(db)}
	if err := svc.syncNovelChapterMetadata("project-1", files); err != nil {
		t.Fatal(err)
	}
	reviews, err := svc.repo.StoryReviews("project-1", unit.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(reviews) != 1 || reviews[0].Summary != "旧审稿" {
		t.Fatalf("initial reviews = %#v", reviews)
	}
	files["chapters/index.json"] = `[{"number":1,"title":"签名","status":"ready-for-review","auditIssues":["新审稿"]}]`
	if err := svc.syncNovelChapterMetadata("project-1", files); err != nil {
		t.Fatal(err)
	}
	reviews, err = svc.repo.StoryReviews("project-1", unit.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(reviews) != 1 || reviews[0].Summary != "新审稿" {
		t.Fatalf("updated reviews = %#v", reviews)
	}
	versions, err := svc.repo.StoryChapterVersions("project-1", unit.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(versions) != 1 {
		t.Fatalf("versions = %#v, want immutable version reused", versions)
	}
}
func TestSyncNovelAgentProjectionUpdatesExistingMemoryAndImportsNewTruthFiles(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.Asset{}, &model.ProjectAssetLink{}, &model.Project{}, &model.ProjectUnit{}, &model.StoryChapterVersion{}, &model.StoryReview{}, &model.StoryMemory{}, &model.Shot{}, &model.ShotAssetReference{}, &model.AssetRepresentation{}, &model.CharacterVoiceBinding{}, &model.Shot{}, &model.ShotAssetReference{}, &model.AssetRepresentation{}, &model.CharacterVoiceBinding{}); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	memory := model.StoryMemory{ID: "memory-1", ProjectID: "project-1", Kind: "state", Content: "旧状态", SourceType: "inkos-file", SourceID: "story/current_state.md", SourceHash: "old", CreatedAt: now, UpdatedAt: now}
	if err := db.Create(&memory).Error; err != nil {
		t.Fatal(err)
	}
	svc := &Service{repo: repository.New(db)}
	files := map[string]string{
		"story/current_state.md":     "新状态",
		"story/chapter_summaries.md": "# 章节摘要\n\n第 1 章：签名。",
		"story/subplot_board.md":     "# 支线\n\n外包工号线。",
		"chapters/index.json":        "[]",
	}
	if err := svc.syncNovelAgentProjection("user-1", "project-1", "sha256:new", files); err != nil {
		t.Fatal(err)
	}
	memories, err := svc.repo.StoryMemories("project-1", 100)
	if err != nil {
		t.Fatal(err)
	}
	bySource := make(map[string]model.StoryMemory, len(memories))
	for _, item := range memories {
		bySource[item.SourceID] = item
	}
	if len(memories) != 3 {
		t.Fatalf("memories = %#v, want state plus two new truth files", memories)
	}
	if bySource["story/current_state.md"].Content != "新状态" || bySource["story/current_state.md"].SourceHash != "sha256:new" {
		t.Fatalf("current state not updated: %#v", bySource["story/current_state.md"])
	}
	if bySource["story/chapter_summaries.md"].Kind != "chapter_summary" || bySource["story/subplot_board.md"].Kind != "subplot" {
		t.Fatalf("new memory kinds = %#v", bySource)
	}
	if err := svc.syncNovelAgentProjection("user-1", "project-1", "sha256:new", files); err != nil {
		t.Fatal(err)
	}
	memories, err = svc.repo.StoryMemories("project-1", 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(memories) != 3 {
		t.Fatalf("repeat projection duplicated memories: %#v", memories)
	}
}

func TestSyncNovelAgentProjectionUpdatesChangedCharacterVersion(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.Asset{}, &model.AssetVersion{}, &model.Project{}, &model.ProjectAssetLink{}, &model.ProjectUnit{}, &model.StoryChapterVersion{}, &model.StoryReview{}, &model.StoryMemory{}, &model.Shot{}, &model.ShotAssetReference{}, &model.AssetRepresentation{}, &model.CharacterVoiceBinding{}); err != nil {
		t.Fatal(err)
	}
	svc := &Service{repo: repository.New(db)}
	project := model.Project{ID: "project-1", UserID: "user-1", Name: "角色投影", Type: "novel", CreatedAt: time.Now(), UpdatedAt: time.Now()}
	if err := db.Create(&project).Error; err != nil {
		t.Fatal(err)
	}
	oldRole := "# 顾宁\n\n旧角色卡。"
	files := map[string]string{
		"story/roles/主要角色/顾宁.md": oldRole,
		"chapters/index.json":    "[]",
	}
	if err := svc.syncNovelAgentProjection("user-1", "project-1", "sha256:old", files); err != nil {
		t.Fatal(err)
	}
	assets, err := svc.repo.ProjectAssets("user-1", "project-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(assets) != 1 {
		t.Fatalf("assets = %#v, want one role", assets)
	}
	oldVersion, err := svc.repo.AssetVersion(assets[0].PrimaryVersionID)
	if err != nil {
		t.Fatal(err)
	}
	newRole := "# 顾宁\n\n新角色卡。"
	files["story/roles/主要角色/顾宁.md"] = newRole
	if err := svc.syncNovelAgentProjection("user-1", "project-1", "sha256:new", files); err != nil {
		t.Fatal(err)
	}
	assets, err = svc.repo.ProjectAssets("user-1", "project-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(assets) != 1 || assets[0].ID != oldVersion.AssetID {
		t.Fatalf("assets after changed role = %#v, want same asset", assets)
	}
	newVersion, err := svc.repo.AssetVersion(assets[0].PrimaryVersionID)
	if err != nil {
		t.Fatal(err)
	}
	if newVersion.Version != oldVersion.Version+1 {
		t.Fatalf("versions = %d/%d, want immutable increment", oldVersion.Version, newVersion.Version)
	}
	var current map[string]any
	if err := json.Unmarshal([]byte(newVersion.DefinitionJSON), &current); err != nil {
		t.Fatal(err)
	}
	if current["sourceMarkdown"] != newRole || current["sourceHash"] != "sha256:new" {
		t.Fatalf("updated definition = %#v", current)
	}
	if err := svc.syncNovelAgentProjection("user-1", "project-1", "sha256:new", files); err != nil {
		t.Fatal(err)
	}
	versions, err := svc.repo.AssetVersions(oldVersion.AssetID)
	if err != nil {
		t.Fatal(err)
	}
	if len(versions) != 2 {
		t.Fatalf("repeat projection versions = %d, want no duplicate", len(versions))
	}
}
