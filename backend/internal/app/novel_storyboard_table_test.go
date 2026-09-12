package app

import (
	"encoding/json"
	"testing"

	"infinite-canvas/backend/internal/model"
)

func TestParseNovelStoryboardTableExtractsStructuredColumns(t *testing.T) {
	storyboard := `# 架上签名 分镜

## 分镜表

| 镜号 | 场景/画面 | 人物/物件 | 动作 | 景别/机位 | 对白/字幕 | 时长建议 | 备注 |
|---|---|---|---|---|---|---|---|
| 1 | 地下库房入口，签到机与门禁 | 顾宁、签到机、工牌 | 顾宁按下工号，屏幕跳出绿灯 | 特写接中景 | 字幕：晚上八点十分，B区夜班开始 | 3秒 | 开场交代时间、地点、身份 |
| 8 | B-14架前 | 卷宗档号、回架清单 | 顾宁低头核对档号，翻回清单 | 过肩特写 | 顾宁：三个月前失踪案？ | 4秒 | 强化悬念 |
| 十二 | 无效行号行 | 顾宁 | 无动作 | 中景 | 无 | 3秒 | 应被忽略 |
`
	rows := parseNovelStoryboardTable(storyboard)
	if len(rows) != 2 {
		t.Fatalf("rows = %d, want 2 (numbered rows only)", len(rows))
	}
	row1, ok := rows[1]
	if !ok {
		t.Fatal("row 1 missing")
	}
	if row1.ShotSize == "" || row1.Dialogue == "" || row1.DurationMs != 3000 {
		t.Fatalf("row1 = %#v, want shot size/dialogue/3000ms", row1)
	}
	if row1.Action == "" || row1.ContinuityNotes == "" || row1.Scene == "" {
		t.Fatalf("row1 missing scene/action/notes: %#v", row1)
	}
	row8, ok := rows[8]
	if !ok {
		t.Fatal("row 8 missing")
	}
	if row8.DurationMs != 4000 {
		t.Fatalf("row8 duration = %d, want 4000", row8.DurationMs)
	}
	if row8.Dialogue == "" || !contains(row8.Dialogue, "三个月前失踪案") {
		t.Fatalf("row8 dialogue = %q", row8.Dialogue)
	}
	if _, ok := rows[12]; ok {
		t.Fatal("non-numeric shot number must be ignored")
	}
}

func contains(haystack, needle string) bool {
	return len(needle) > 0 && len(haystack) >= len(needle) && indexOf(haystack, needle) >= 0
}

func indexOf(haystack, needle string) int {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}

func TestSyncNovelProductionArtifactsFillsStructuredShotFieldsFromStoryboardTable(t *testing.T) {
	svc, db := newProjectWorkflowV2TestService(t)
	if err := db.AutoMigrate(&model.StoryScene{}, &model.StorySceneShotLink{}, &model.Shot{}, &model.ShotRevision{}, &model.ShotArtifact{}); err != nil {
		t.Fatal(err)
	}
	project, unit := seedWorkflowProject(t, db)
	if err := db.Model(&model.Project{}).Where("id = ?", project.ID).Update("type", "novel").Error; err != nil {
		t.Fatal(err)
	}
	storyboard := `# 架上签名 分镜

## 分镜表

| 镜号 | 场景/画面 | 人物/物件 | 动作 | 景别/机位 | 对白/字幕 | 时长建议 | 备注 |
|---|---|---|---|---|---|---|---|
| 1 | 地下库房入口，签到机与门禁 | 顾宁、签到机 | 顾宁按下工号，门禁打开 | 特写接中景 | 字幕：晚上八点十分，B区夜班开始 | 3秒 | 开场交代 |
| 2 | 库房交接窗口 | 白班同事、铜色钥匙 | 白班同事把钥匙推给顾宁 | 近景过肩 | 白班同事：B区钥匙，你今晚只巡库 | 4秒 | 建立限制 |
`
	manifestRaw, err := json.Marshal(novelProductionAssetsManifest{
		ProjectID: "book-1",
		Assets: []novelProductionShot{
			{ShotID: "shot-001", Status: "prompt_ready", Prompt: "镜1：地下库房入口签到机特写，顾宁按工号，绿色门禁灯亮起"},
			{ShotID: "shot-002", Status: "prompt_ready", Prompt: "镜2：库房交接窗口近景，白班同事把铜色钥匙推给顾宁"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	artifacts := NovelAgentArtifacts{
		BookID:           "book-1",
		ProductionUnitID: unit.ID,
		ProductionFiles: map[string]string{
			"storyboards/book-1-ch-0001/storyboard.md": storyboard,
			"storyboards/book-1-ch-0001/assets.json":   string(manifestRaw),
		},
	}
	if err := svc.syncNovelProductionArtifacts("user-1", project.ID, artifacts); err != nil {
		t.Fatal(err)
	}
	shots, err := svc.repo.ProjectShots(project.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(shots) != 2 {
		t.Fatalf("shots = %d, want 2", len(shots))
	}
	byPosition := map[int]model.Shot{}
	for _, shot := range shots {
		byPosition[shot.Position] = shot
	}
	shot1 := byPosition[0]
	if shot1.DurationMs != 3000 {
		t.Fatalf("shot1 duration = %d, want 3000", shot1.DurationMs)
	}
	if shot1.CurrentRevisionID == "" {
		t.Fatal("shot1 has no current revision")
	}
	revisions, revErr := svc.repo.ProjectUnitShotRevisions(project.ID, unit.ID)
	if revErr != nil {
		t.Fatal(revErr)
	}
	revision, found := findRevisionForShot(revisions, shot1.CurrentRevisionID)
	if !found {
		t.Fatal("shot1 revision not found")
	}
	if revision.ShotSize == "" || revision.Dialogue == "" {
		t.Fatalf("revision shot size/dialogue not filled: %#v", revision)
	}
	if revision.ContinuityNotes != "开场交代" {
		t.Fatalf("revision continuity notes = %q, want table value", revision.ContinuityNotes)
	}
	shot2 := byPosition[1]
	if shot2.DurationMs != 4000 {
		t.Fatalf("shot2 duration = %d, want 4000", shot2.DurationMs)
	}
	revision2, found2 := findRevisionForShot(revisions, shot2.CurrentRevisionID)
	if !found2 {
		t.Fatal("shot2 revision not found")
	}
	if revision2.ShotSize == "" {
		t.Fatalf("shot2 revision shot size not filled: %#v", revision2)
	}
	if revision2.ContinuityNotes == "开场交代" {
		t.Fatal("shot2 revision notes must not reuse shot1 row")
	}
}

func findRevisionForShot(revisions []model.ShotRevision, revisionID string) (model.ShotRevision, bool) {
	for _, revision := range revisions {
		if revision.ID == revisionID {
			return revision, true
		}
	}
	return model.ShotRevision{}, false
}
