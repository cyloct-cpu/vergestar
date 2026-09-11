package service

import (
	"encoding/json"
	"slices"
	"testing"

	"infinite-canvas/backend/internal/model"
)

func TestProjectNovelFoundationExtractsRolesAndMemorySources(t *testing.T) {
	characters, memories := projectNovelFoundation(map[string]string{
		"story/roles/主要角色/林川.md":  "## 核心标签\n维修工、谨慎。",
		"story/roles/次要角色/错误.md":  "## 角色\n被系统遗漏。",
		"story/current_state.md":  "# 当前状态\n\n林川尚未完成验收。",
		"story/pending_hooks.md":  "# 伏笔池\n\nH001 待回收。",
		"story/emotional_arcs.md": "# 情感弧线\n\n母子关系从疏离到共同签字。",
		"story/brief.md":          "不会导入为记忆。",
	})
	if len(characters) != 2 {
		t.Fatalf("characters = %d, want 2", len(characters))
	}
	if !slices.ContainsFunc(characters, func(character novelAgentCharacterProjection) bool {
		return character.Name == "林川" && character.Definition["roleCategory"] == "主要角色"
	}) {
		t.Fatalf("main character projection missing: %#v", characters)
	}
	if len(memories) != 3 || memories[0].SourceID != "story/current_state.md" {
		t.Fatalf("memories = %#v", memories)
	}
}

func TestProjectNovelFoundationTruncatesLargeSourceText(t *testing.T) {
	large := make([]rune, 7000)
	for index := range large {
		large[index] = '字'
	}
	_, memories := projectNovelFoundation(map[string]string{"story/current_state.md": string(large)})
	if len(memories) != 1 {
		t.Fatalf("memories = %#v", memories)
	}
	if len([]rune(memories[0].Content)) <= 6000 || len([]rune(memories[0].Content)) >= 6200 {
		t.Fatalf("truncated content length = %d", len([]rune(memories[0].Content)))
	}
}

func TestProjectNovelChaptersImportsOnlyRealChapterFilesInOrder(t *testing.T) {
	chapters := projectNovelChapters(map[string]string{
		"chapters/0002_雨夜.md":         "第二章正文",
		"chapters/0001_签名.md":         "第一章正文",
		"chapters/index.json":         "[]",
		"chapters/.trash/0003_废稿.md":  "不应导入",
		"story/outline/volume_map.md": "第 1 章不是正文文件",
	})
	if len(chapters) != 2 {
		t.Fatalf("chapters = %#v, want 2 real chapter files", chapters)
	}
	if chapters[0].Title != "签名" || chapters[0].SourceText != "第一章正文" || chapters[0].Position != 0 {
		t.Fatalf("first chapter = %#v", chapters[0])
	}
	if chapters[1].Title != "雨夜" || chapters[1].SourceText != "第二章正文" || chapters[1].Position != 1 {
		t.Fatalf("second chapter = %#v", chapters[1])
	}
}

func TestProjectNovelChapterSyncAddsLaterChapterAndUpdatesChangedChapter(t *testing.T) {
	chapters := projectNovelChapters(map[string]string{
		"chapters/0001_签名.md": "第一章修订正文",
		"chapters/0002_雨夜.md": "第二章正文",
	})
	if len(chapters) != 2 || chapters[0].Position != 0 || chapters[1].Position != 1 {
		t.Fatalf("chapters = %#v", chapters)
	}
	if chapters[0].Title != "签名" || chapters[1].Title != "雨夜" {
		t.Fatalf("chapter titles = %#v", chapters)
	}
}

func TestValidNovelArtifactPathAcceptsApprovedFoundationRoleAndChapterFiles(t *testing.T) {
	for _, filePath := range []string{
		"story/emotional_arcs.md",
		"story/roles/主要角色/顾宁.md",
		"chapters/0001_档案签名.md",
		"chapters/index.json",
	} {
		if !validNovelArtifactPath(filePath) {
			t.Fatalf("expected %q to be an allowed artifact path", filePath)
		}
	}
	if validNovelArtifactPath("story/snapshots/0/current_state.md") {
		t.Fatal("snapshot paths must not be exposed through the foundation projection")
	}
}

func TestChapterMetadataMapsInkOSStatusAndFindsChapterContent(t *testing.T) {
	files := map[string]string{
		"chapters/index.json": `[{"number":1,"title":"签名","status":"audit-failed","wordCount":12,"auditIssues":["冲突释放不足"]}]`,
		"chapters/0001_签名.md": "第一章正文",
	}
	var metadata []novelAgentChapterMetadata
	if err := json.Unmarshal([]byte(files["chapters/index.json"]), &metadata); err != nil {
		t.Fatal(err)
	}
	if len(metadata) != 1 || metadata[0].Status != "audit-failed" {
		t.Fatalf("metadata = %#v", metadata)
	}
	if got := chapterContentForNumber(files, 1); got != "第一章正文" {
		t.Fatalf("chapter content = %q", got)
	}
	if got := projectUnitStatusForInkOS("audit-failed"); got != model.ProjectUnitStatusDraft {
		t.Fatalf("audit-failed status = %q", got)
	}
	if got := projectUnitStatusForInkOS("approved"); got != model.ProjectUnitStatusCompleted {
		t.Fatalf("approved status = %q", got)
	}
}

func TestProjectNovelScriptSectionsParsesProductionScenes(t *testing.T) {
	script := `# 架上签名

## 人物

- **顾宁**：巡库员。

## 剧本正文

**第1集 架上签名**

**场次1 地下库房入口 夜**  
人物：顾宁、白班同事  

动作：签到机绿灯。门禁开。  

对白：  
白班同事：B区钥匙。  
顾宁：移交栏空着。  

动作：同事离开。  

**场次2 B区库房主通道 夜**  
人物：顾宁  

动作：第三排，B-14架第三层露出一盒卷宗。  

对白：  
顾宁：B-14，第三层。  
`
	sections := projectNovelScriptSections(script)
	if len(sections) != 2 {
		t.Fatalf("sections = %d, want 2", len(sections))
	}
	first := sections[0]
	if first.Title != "场次 1" || first.Location != "地下库房入口 夜" {
		t.Fatalf("first section header = %#v", first)
	}
	if first.Characters != "顾宁、白班同事" {
		t.Fatalf("first characters = %q", first.Characters)
	}
	if first.Action != "签到机绿灯。门禁开。\n同事离开。" {
		t.Fatalf("first action = %q", first.Action)
	}
	if first.Dialogue != "白班同事：B区钥匙。\n顾宁：移交栏空着。" {
		t.Fatalf("first dialogue = %q", first.Dialogue)
	}
	if sections[1].Title != "场次 2" || sections[1].Action == "" || sections[1].Dialogue == "" {
		t.Fatalf("second section = %#v", sections[1])
	}
}
