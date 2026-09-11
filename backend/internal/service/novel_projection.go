package service

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"infinite-canvas/backend/internal/model"
	"infinite-canvas/backend/internal/repository"
)

var chapterFileNamePattern = regexp.MustCompile(`^(\d{1,6})_(.+)\.md$`)

// Memory projections preserve the InkOS truth files that the story editor
// and downstream screenplay work rely on. New proven files are appended to
// this table so old source IDs remain stable.
var novelMemorySources = []struct {
	path string
	kind string
}{
	{path: "story/current_state.md", kind: "state"},
	{path: "story/pending_hooks.md", kind: "hook"},
	{path: "story/emotional_arcs.md", kind: "emotional_arc"},
	{path: "story/chapter_summaries.md", kind: "chapter_summary"},
	{path: "story/subplot_board.md", kind: "subplot"},
	{path: "story/outline/volume_map.md", kind: "outline"},
}

type novelAgentCharacterProjection struct {
	Name       string
	SourceID   string
	Definition map[string]any
}

type novelAgentMemoryProjection struct {
	Kind     string
	SourceID string
	Content  string
}

type novelAgentChapterMetadata struct {
	Number      int      `json:"number"`
	Title       string   `json:"title"`
	Status      string   `json:"status"`
	WordCount   int      `json:"wordCount"`
	AuditIssues []string `json:"auditIssues"`
}

// syncNovelAgentProjection turns stable InkOS foundation files into native
// Vergestar records. It is intentionally additive and source-idempotent: a
// later job poll must never create a second copy of a role or memory.
func (s *Service) syncNovelAgentProjection(userID, projectID, sourceHash string, files map[string]string) error {
	characters, memories := projectNovelFoundation(files)
	chapters := projectNovelChapters(files)

	existingUnits, err := s.repo.ProjectUnits(projectID)
	if err != nil {
		return err
	}
	if err := s.syncNovelChapterUnits(userID, projectID, existingUnits, chapters); err != nil {
		return err
	}
	if err := s.syncNovelChapterMetadata(projectID, files); err != nil {
		return err
	}

	existingAssets, err := s.repo.ProjectAssets(userID, projectID)
	if err != nil {
		return err
	}
	existingCharacterNames := make(map[string]model.Asset, len(existingAssets))
	for _, asset := range existingAssets {
		if asset.Category == model.AssetCategoryCharacter {
			existingCharacterNames[strings.TrimSpace(asset.Title)] = asset
		}
	}
	for _, character := range characters {
		definition := character.Definition
		definition["sourceType"] = "inkos-agent"
		definition["sourceId"] = character.SourceID
		definition["sourceHash"] = sourceHash
		existing, exists := existingCharacterNames[character.Name]
		if !exists {
			detail, err := s.CreateProjectCharacter(userID, projectID, CreateProjectCharacterRequest{
				Name: character.Name, Definition: definition,
			})
			if err != nil {
				return err
			}
			existingCharacterNames[character.Name] = model.Asset{ID: detail.Asset.ID, Title: detail.Asset.Title, PrimaryVersionID: detail.Asset.PrimaryVersionID}
			continue
		}
		if changed, err := novelCharacterDefinitionChanged(s.repo, existing, character.SourceID, character.Definition["roleCategory"], definition); err != nil {
			return err
		} else if !changed {
			continue
		}
		if _, err := s.UpdateProjectCharacter(userID, projectID, existing.ID, UpdateProjectCharacterRequest{
			Name: character.Name, Definition: definition,
		}); err != nil {
			return err
		}
	}

	existingMemories, err := s.repo.StoryMemories(projectID, 200)
	if err != nil {
		return err
	}
	existingMemorySources := make(map[string]model.StoryMemory, len(existingMemories))
	for _, memory := range existingMemories {
		if memory.SourceType == "inkos-file" {
			existingMemorySources[memory.SourceID] = memory
		}
	}
	for _, memory := range memories {
		existing, exists := existingMemorySources[memory.SourceID]
		if exists {
			if existing.Content == memory.Content {
				continue
			}
			if err := s.repo.UpdateStoryMemoryContent(existing.ID, projectID, memory.Kind, memory.Content, sourceHash); err != nil {
				return err
			}
			continue
		}
		now := time.Now()
		if err := s.repo.CreateStoryMemory(&model.StoryMemory{
			ID: newID(), ProjectID: projectID, Kind: memory.Kind, Content: memory.Content,
			SourceType: "inkos-file", SourceID: memory.SourceID, SourceHash: sourceHash,
			CreatedAt: now, UpdatedAt: now,
		}); err != nil {
			return err
		}
		existingMemorySources[memory.SourceID] = model.StoryMemory{ID: newID(), ProjectID: projectID, Kind: memory.Kind, Content: memory.Content, SourceHash: sourceHash}
	}
	return nil
}

// syncNovelChapterMetadata mirrors the InkOS chapters/index.json quality
// state into Vergestar's chapter status, immutable version, and review tables.
// It is source-hash idempotent so polling a completed job cannot duplicate
// versions or reviews.
func (s *Service) syncNovelChapterMetadata(projectID string, files map[string]string) error {
	raw := strings.TrimSpace(files["chapters/index.json"])
	if raw == "" || raw == "[]" {
		return nil
	}
	var metadata []novelAgentChapterMetadata
	if err := json.Unmarshal([]byte(raw), &metadata); err != nil {
		return fmt.Errorf("解析 InkOS 章节索引失败：%w", err)
	}
	units, err := s.repo.ProjectUnits(projectID)
	if err != nil {
		return err
	}
	byPosition := make(map[int]model.ProjectUnit, len(units))
	for _, unit := range units {
		byPosition[unit.Position+1] = unit
	}
	for _, item := range metadata {
		if item.Number < 1 {
			continue
		}
		unit, exists := byPosition[item.Number]
		if !exists {
			continue
		}
		content := chapterContentForNumber(files, item.Number)
		if content == "" {
			continue
		}
		desiredStatus := projectUnitStatusForInkOS(item.Status)
		if unit.Status != desiredStatus || (item.Title != "" && unit.Title != item.Title) {
			unit.Title = strings.TrimSpace(item.Title)
			if unit.Title == "" {
				unit.Title = fmt.Sprintf("第 %d 章", item.Number)
			}
			unit.SourceText = content
			unit.WordCount = model.ProjectUnitWordCount(content)
			unit.Status = desiredStatus
			unit.UpdatedAt = time.Now()
			if err := s.repo.UpdateProjectUnit(&unit, false); err != nil {
				return err
			}
		}
		versionHash := storyHash(unit.Title, content)
		versions, err := s.repo.StoryChapterVersions(projectID, unit.ID)
		if err != nil {
			return err
		}
		versionExists := false
		var versionID string
		for _, version := range versions {
			if version.SourceHash == versionHash {
				versionExists = true
				versionID = version.ID
				break
			}
		}
		if !versionExists {
			version := model.StoryChapterVersion{ID: newID(), ProjectID: projectID, UnitID: unit.ID, Number: len(versions) + 1, Title: unit.Title, Content: content, SourceHash: versionHash, CreatedAt: time.Now()}
			if err := s.repo.CreateStoryChapterVersion(&version); err != nil {
				return err
			}
			versionID = version.ID
		}
		if len(item.AuditIssues) > 0 || item.Status == "audit-passed" || item.Status == "ready-for-review" {
			if err := upsertNovelStoryReview(projectID, unit.ID, versionID, item, s.repo); err != nil {
				return err
			}
		}
	}
	return nil
}

func upsertNovelStoryReview(projectID, unitID, versionID string, item novelAgentChapterMetadata, repo *repository.Repository) error {
	issues, err := json.Marshal(item.AuditIssues)
	if err != nil {
		return fmt.Errorf("序列化 InkOS 审稿问题失败：%w", err)
	}
	summary := strings.Join(item.AuditIssues, "；")
	status := "passed"
	if len(item.AuditIssues) > 0 || item.Status == "audit-failed" {
		status = "failed"
	}
	reviews, err := repo.StoryReviews(projectID, unitID)
	if err != nil {
		return err
	}
	for _, review := range reviews {
		if review.ChapterVersionID != versionID {
			continue
		}
		if review.Status == status && review.Summary == summary && review.IssuesJSON == string(issues) {
			return nil
		}
		return repo.UpdateStoryReviewContent(review.ID, projectID, status, summary, string(issues))
	}
	now := time.Now()
	return repo.CreateStoryReview(&model.StoryReview{
		ID: newID(), ProjectID: projectID, UnitID: unitID, ChapterVersionID: versionID,
		Status: status, Summary: summary, IssuesJSON: string(issues), CreatedAt: now, UpdatedAt: now,
	})
}

func chapterContentForNumber(files map[string]string, number int) string {
	for filePath, content := range files {
		if !strings.HasPrefix(filePath, "chapters/") || filepath.Dir(filePath) != "chapters" {
			continue
		}
		match := chapterFileNamePattern.FindStringSubmatch(filepath.Base(filePath))
		if len(match) != 3 {
			continue
		}
		var current int
		if _, err := fmt.Sscanf(match[1], "%d", &current); err == nil && current == number {
			return strings.TrimSpace(content)
		}
	}
	return ""
}

func projectUnitStatusForInkOS(status string) model.ProjectUnitStatus {
	switch strings.TrimSpace(status) {
	case "approved", "published", "audit-passed":
		return model.ProjectUnitStatusCompleted
	case "ready-for-review", "drafted", "imported":
		return model.ProjectUnitStatusReady
	default:
		return model.ProjectUnitStatusDraft
	}
}

// syncNovelChapterUnits applies only new or changed chapter markdown files.
// InkOS owns the chapter index and markdown; Vergestar keeps the current
// ProjectUnit projection without replacing unrelated manual metadata.
func (s *Service) syncNovelChapterUnits(userID, projectID string, existing []model.ProjectUnit, chapters []CreateProjectUnitRequest) error {
	byPosition := make(map[int]model.ProjectUnit, len(existing))
	for _, unit := range existing {
		byPosition[unit.Position] = unit
	}
	newUnits := make([]CreateProjectUnitRequest, 0)
	for _, chapter := range chapters {
		current, exists := byPosition[chapter.Position]
		if !exists {
			newUnits = append(newUnits, chapter)
			continue
		}
		if current.Title == chapter.Title && current.SourceText == chapter.SourceText {
			continue
		}
		if _, err := s.UpdateProjectUnit(userID, projectID, current.ID, UpdateProjectUnitRequest{
			Title: chapter.Title, SourceText: chapter.SourceText, Status: string(current.Status),
		}); err != nil {
			return err
		}
	}
	if len(newUnits) > 0 {
		if _, err := s.ImportProjectUnits(userID, projectID, ImportProjectUnitsRequest{Units: newUnits}); err != nil {
			return err
		}
	}
	return nil
}

func projectNovelChapters(files map[string]string) []CreateProjectUnitRequest {
	type chapter struct {
		position int
		title    string
		content  string
	}
	chapters := make([]chapter, 0)
	for filePath, content := range files {
		if !strings.HasPrefix(filePath, "chapters/") || filepath.Dir(filePath) != "chapters" {
			continue
		}
		match := chapterFileNamePattern.FindStringSubmatch(filepath.Base(filePath))
		if len(match) != 3 {
			continue
		}
		position := 0
		if _, err := fmt.Sscanf(match[1], "%d", &position); err != nil || position < 1 {
			continue
		}
		title := strings.TrimSpace(match[2])
		if title == "" {
			title = fmt.Sprintf("第 %d 章", position)
		}
		chapters = append(chapters, chapter{position: position, title: title, content: content})
	}
	sort.Slice(chapters, func(left, right int) bool { return chapters[left].position < chapters[right].position })
	result := make([]CreateProjectUnitRequest, 0, len(chapters))
	for position, item := range chapters {
		result = append(result, CreateProjectUnitRequest{Kind: string(model.ProjectUnitKindChapter), Title: item.title, SourceText: item.content, Position: position})
	}
	return result
}

func projectNovelFoundation(files map[string]string) ([]novelAgentCharacterProjection, []novelAgentMemoryProjection) {
	characters := make([]novelAgentCharacterProjection, 0)
	memories := make([]novelAgentMemoryProjection, 0, 3)

	for filePath, content := range files {
		if !strings.HasPrefix(filePath, "story/roles/") || !strings.HasSuffix(filePath, ".md") {
			continue
		}
		parts := strings.Split(filePath, "/")
		if len(parts) != 4 || strings.TrimSpace(parts[2]) == "" {
			continue
		}
		name := strings.TrimSpace(strings.TrimSuffix(parts[3], ".md"))
		if name == "" || len([]rune(name)) > 240 {
			continue
		}
		content = truncateNovelProjectionText(content, 24000)
		characters = append(characters, novelAgentCharacterProjection{
			Name: name, SourceID: filePath,
			Definition: map[string]any{"roleCategory": parts[2], "sourceMarkdown": content},
		})
	}
	sort.Slice(characters, func(left, right int) bool {
		return characters[left].SourceID < characters[right].SourceID
	})

	for _, source := range novelMemorySources {
		content := strings.TrimSpace(files[source.path])
		if content == "" {
			continue
		}
		memories = append(memories, novelAgentMemoryProjection{
			Kind: source.kind, SourceID: source.path,
			Content: truncateNovelProjectionText(content, 6000),
		})
	}
	return characters, memories
}

func truncateNovelProjectionText(value string, maxRunes int) string {
	runes := []rune(strings.TrimSpace(value))
	if len(runes) <= maxRunes {
		return string(runes)
	}
	return string(runes[:maxRunes]) + "\n\n[内容已截断，完整来源保存在 StoryFoundation。]"
}

func encodeNovelProjectionDefinition(value map[string]any) string {
	encoded, _ := json.Marshal(value)
	return string(encoded)
}

// novelCharacterDefinitionChanged compares the projected InkOS source with the
// current character version. Character updates remain immutable: a changed
// role file creates a new asset version instead of overwriting history.
func novelCharacterDefinitionChanged(repo *repository.Repository, asset model.Asset, sourceID string, roleCategory any, nextDefinition map[string]any) (bool, error) {
	version, err := repo.AssetVersion(asset.PrimaryVersionID)
	if err != nil {
		return true, nil
	}
	var current map[string]any
	if err := json.Unmarshal([]byte(version.DefinitionJSON), &current); err != nil {
		return true, nil
	}
	currentSource, _ := current["sourceId"].(string)
	currentCategory, _ := current["roleCategory"]
	currentContent, _ := current["sourceMarkdown"].(string)
	nextContent, _ := nextDefinition["sourceMarkdown"].(string)
	return currentSource != sourceID || currentCategory != roleCategory || currentContent != nextContent, nil
}

func novelProductionSourceHash(files map[string]string) string {
	keys := make([]string, 0, len(files))
	for key := range files {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	value := ""
	for _, key := range keys {
		value += key + "\n" + files[key]
	}
	digest := sha256.Sum256([]byte(value))
	return "sha256:" + hex.EncodeToString(digest[:])
}

func novelProductionFilesForUnit(files map[string]string, unit model.ProjectUnit) map[string]string {
	if len(files) == 0 {
		return map[string]string{}
	}
	marker := fmt.Sprintf("-ch-%04d", unit.Position+1)
	filtered := make(map[string]string)
	fallback := make(map[string]string)
	for path, content := range files {
		if !strings.HasPrefix(path, "dramas/") && !strings.HasPrefix(path, "storyboards/") {
			continue
		}
		fallback[path] = content
		if strings.Contains(path, marker) {
			filtered[path] = content
		}
	}
	if len(filtered) > 0 {
		return filtered
	}
	return fallback
}

func projectNovelStoryboardSections(storyboard string) []novelProductionSection {
	lines := strings.Split(storyboard, "\n")
	sections := make([]novelProductionSection, 0, 32)
	var current *novelProductionSection
	flush := func() {
		if current != nil {
			sections = append(sections, *current)
			current = nil
		}
	}
	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "#") {
			flush()
			title := strings.TrimSpace(strings.TrimLeft(line, "# "))
			current = &novelProductionSection{Title: title}
			continue
		}
		if current == nil {
			current = &novelProductionSection{}
		}
		switch {
		case strings.HasPrefix(line, "**场景") || strings.HasPrefix(line, "**Scene"):
			current.Location = strings.TrimSpace(strings.Trim(line, "* "))
		case strings.HasPrefix(line, "**时间") || strings.HasPrefix(line, "**Time"):
			current.Time = strings.TrimSpace(strings.Trim(line, "* "))
		case strings.HasPrefix(line, "**画面") || strings.HasPrefix(line, "**Visual"):
			current.Action = strings.TrimSpace(strings.Trim(line, "* "))
		case strings.HasPrefix(line, "**台词") || strings.HasPrefix(line, "**Dialogue"):
			current.Dialogue = strings.TrimSpace(strings.Trim(line, "* "))
		case strings.HasPrefix(line, "**镜头") || strings.HasPrefix(line, "**Camera"):
			current.Visual = strings.TrimSpace(strings.Trim(line, "* "))
		}
	}
	flush()
	return sections
}

type novelProductionShot struct {
	ShotID string `json:"shotId"`
	Prompt string `json:"prompt"`
	Status string `json:"status"`
}

type novelProductionAssetsManifest struct {
	Title      string                `json:"title"`
	ProjectID  string                `json:"projectId"`
	Storyboard string                `json:"storyboardPath"`
	Prompts    string                `json:"imagePromptsPath"`
	Assets     []novelProductionShot `json:"assets"`
}

type novelProductionSection struct {
	Title      string
	Location   string
	Time       string
	Action     string
	Dialogue   string
	Visual     string
	Characters string
}

// syncNovelProductionArtifacts projects InkOS script/storyboard deliverables
// into Vergestar's native scene and shot records. The projection is additive
// and keyed by a deterministic InkOS source ID, so repeated job polling cannot
// duplicate scenes or shots.
func (s *Service) syncNovelProductionArtifacts(userID, projectID string, artifacts NovelAgentArtifacts) error {
	if len(artifacts.ProductionFiles) == 0 {
		return nil
	}
	// 短篇产物（shorts/*）只进入 StoryFoundation 文件集合，没有场次/镜头语义。
	shortOnly := true
	for path := range artifacts.ProductionFiles {
		if !strings.HasPrefix(path, "shorts/") {
			shortOnly = false
			break
		}
	}
	if shortOnly {
		return nil
	}
	unitID := strings.TrimSpace(artifacts.ProductionUnitID)
	if unitID == "" {
		units, err := s.repo.ProjectUnits(projectID)
		if err != nil {
			return err
		}
		if len(units) > 0 {
			chapter := novelProductionChapter(artifacts.ProductionFiles)
			unitID = units[len(units)-1].ID
			if chapter > 0 {
				for _, unit := range units {
					if unit.Position+1 == chapter {
						unitID = unit.ID
						break
					}
				}
			}
		}
	}
	if unitID == "" {
		return NewAppError(http.StatusBadGateway, "影视生产产物缺少目标章节")
	}
	if _, err := s.storyProjectUnit(userID, projectID, unitID); err != nil {
		return err
	}
	// InkOS writes production artifacts with forward-slash relative paths. Do
	// not assume one hard-coded project id; select the first valid manifest.
	storyboardRaw := ""
	assetsRaw := ""
	scriptRaw := ""
	if storyboardRaw == "" || assetsRaw == "" {
		for path, content := range artifacts.ProductionFiles {
			if storyboardRaw == "" && strings.HasPrefix(path, "storyboards/") && strings.HasSuffix(path, "/storyboard.md") {
				storyboardRaw = content
			}
			if assetsRaw == "" && strings.HasPrefix(path, "storyboards/") && strings.HasSuffix(path, "/assets.json") {
				assetsRaw = content
			}
			if scriptRaw == "" && strings.HasPrefix(path, "dramas/") && strings.HasSuffix(path, "/script.md") {
				scriptRaw = content
			}
		}
	}
	if (storyboardRaw == "" || assetsRaw == "") && scriptRaw != "" {
		// Script-only runs still have deterministic scene boundaries. Project
		// them first so users can see the production skeleton before InkOS
		// emits the storyboard manifest. Shots are created only with prompts.
		sourceHash := novelProductionSourceHash(artifacts.ProductionFiles)
		sections := projectNovelScriptSections(scriptRaw)
		if err := s.syncNovelScriptCharacterCandidates(userID, projectID, unitID, artifacts.BookID, sections); err != nil {
			return err
		}
		for index, section := range sections {
			sourceID := fmt.Sprintf("inkos-script:%s:scene-%d", artifacts.BookID, index+1)
			if _, err := s.upsertNovelProductionScene(userID, projectID, unitID, sourceID, sourceHash, section, section.Visual, index); err != nil {
				return err
			}
		}
		return nil
	}
	if scriptRaw == "" {
		if foundation, err := s.repo.StoryFoundationForProject(projectID); err == nil && strings.TrimSpace(foundation.FilesJSON) != "" {
			files := map[string]string{}
			if json.Unmarshal([]byte(foundation.FilesJSON), &files) == nil {
				for path, content := range files {
					if strings.HasPrefix(path, "dramas/") && strings.HasSuffix(path, "/script.md") {
						scriptRaw = content
						break
					}
				}
			}
		}
	}
	scriptSections := projectNovelScriptSections(scriptRaw)
	if len(scriptSections) > 0 {
		if err := s.syncNovelScriptCharacterCandidates(userID, projectID, unitID, artifacts.BookID, scriptSections); err != nil {
			return err
		}
		scriptHash := novelProductionSourceHash(map[string]string{"script.md": scriptRaw})
		for index, section := range scriptSections {
			sourceID := fmt.Sprintf("inkos-script:%s:scene-%d", artifacts.BookID, index+1)
			if _, err := s.upsertNovelProductionScene(userID, projectID, unitID, sourceID, scriptHash, section, section.Visual, index); err != nil {
				return err
			}
		}
	}
	if storyboardRaw == "" || assetsRaw == "" {
		return nil
	}
	var manifest novelProductionAssetsManifest
	if err := json.Unmarshal([]byte(assetsRaw), &manifest); err != nil {
		return NewAppError(http.StatusBadGateway, "InkOS 分镜清单格式无效")
	}
	if len(manifest.Assets) > 200 {
		return NewAppError(http.StatusBadGateway, "InkOS 分镜数量超出限制")
	}
	sourceHash := novelProductionSourceHash(artifacts.ProductionFiles)
	projectAssets, err := s.repo.ProjectAssets(userID, projectID)
	if err != nil {
		return err
	}
	assetVersions := make(map[string]string, len(projectAssets))
	for _, asset := range projectAssets {
		if asset.Category == model.AssetCategoryCharacter && strings.TrimSpace(asset.PrimaryVersionID) != "" {
			assetVersions[asset.ID] = asset.PrimaryVersionID
		}
	}
	existingReferences, err := s.repo.ProjectShotAssetReferences(projectID)
	if err != nil {
		return err
	}
	referenceKeys := make(map[string]struct{}, len(existingReferences))
	for _, reference := range existingReferences {
		referenceKeys[novelProductionShotReferenceKey(reference.ShotID, reference.AssetVersionID)] = struct{}{}
	}
	existingScenes, err := s.repo.StoryScenes(projectID, unitID)
	if err != nil {
		return err
	}
	links, err := s.repo.StorySceneShotLinks(projectID, unitID)
	if err != nil {
		return err
	}
	shots, err := s.repo.ProjectShots(projectID)
	if err != nil {
		return err
	}
	revisions, err := s.repo.ProjectShotRevisions(projectID)
	if err != nil {
		return err
	}
	scenesByStatus := make(map[string]model.StoryScene, len(existingScenes))
	scriptScenes := make([]model.StoryScene, 0, len(scriptSections))
	for _, scene := range existingScenes {
		scenesByStatus[scene.Status] = scene
		if strings.HasPrefix(scene.Status, "inkos:inkos-script:"+artifacts.BookID+":scene-") {
			scriptScenes = append(scriptScenes, scene)
		}
	}
	sort.Slice(scriptScenes, func(left, right int) bool { return scriptScenes[left].Position < scriptScenes[right].Position })
	linksByScene := make(map[string][]model.StorySceneShotLink, len(links))
	linksByShot := make(map[string][]model.StorySceneShotLink, len(links))
	for _, link := range links {
		linksByScene[link.SceneID] = append(linksByScene[link.SceneID], link)
		linksByShot[link.ShotID] = append(linksByShot[link.ShotID], link)
	}
	shotByID := make(map[string]model.Shot, len(shots))
	for _, shot := range shots {
		shotByID[shot.ID] = shot
	}
	shotBySourceID := make(map[string]model.Shot, len(revisions))
	for _, revision := range revisions {
		if sourceID := novelStoryboardSourceIDFromNotes(revision.ContinuityNotes); sourceID != "" {
			if shot, exists := shotByID[revision.ShotID]; exists {
				shotBySourceID[sourceID] = shot
			}
		}
	}
	fallbackSections := projectNovelStoryboardSections(storyboardRaw)
	prompts := make([]string, len(manifest.Assets))
	for index, asset := range manifest.Assets {
		prompts[index] = asset.Prompt
	}
	storyboardTargets := novelStoryboardTargetScenes(scriptScenes, prompts)
	for index, asset := range manifest.Assets {
		if asset.ShotID == "" || asset.Status != "prompt_ready" || strings.TrimSpace(asset.Prompt) == "" {
			continue
		}
		sourceID := fmt.Sprintf("inkos-storyboard:%s:%s", manifest.ProjectID, asset.ShotID)
		legacyStatus := "inkos:" + sourceID
		target := model.StoryScene{}
		if index < len(storyboardTargets) {
			target = storyboardTargets[index]
		}
		if target.ID == "" {
			section := novelProductionSection{Title: fmt.Sprintf("镜头 %02d", index+1)}
			if index < len(fallbackSections) {
				section = fallbackSections[index]
			}
			created, createErr := s.upsertNovelProductionScene(userID, projectID, unitID, sourceID, sourceHash, section, asset.Prompt, index)
			if createErr != nil {
				return createErr
			}
			target = *created
		}
		if legacy, exists := scenesByStatus[legacyStatus]; exists && legacy.ID != target.ID {
			for _, link := range linksByScene[legacy.ID] {
				if shot, exists := shotByID[link.ShotID]; exists && shot.CurrentRevisionID != "" {
					if err := s.repo.UpdateShotRevisionContinuityNotes(shot.CurrentRevisionID, "inkosStoryboard:"+sourceID); err != nil {
						return err
					}
					shotBySourceID[sourceID] = shot
				}
				link.SceneID = target.ID
				link.SourceHash = target.SourceHash
				link.UpdatedAt = time.Now()
				if err := s.repo.UpdateStorySceneShotLink(&link); err != nil {
					return err
				}
				linksByShot[link.ShotID] = []model.StorySceneShotLink{link}
			}
			if err := s.repo.DeleteStoryScene(legacy.ID); err != nil {
				return err
			}
			delete(scenesByStatus, legacyStatus)
		}
		if shot, exists := shotBySourceID[sourceID]; exists {
			for _, link := range linksByShot[shot.ID] {
				if link.SceneID == target.ID {
					continue
				}
				link.SceneID = target.ID
				link.SourceHash = target.SourceHash
				link.UpdatedAt = time.Now()
				if err := s.repo.UpdateStorySceneShotLink(&link); err != nil {
					return err
				}
				linksByShot[shot.ID] = []model.StorySceneShotLink{link}
			}
			if err := s.ensureNovelProductionShotReferences(projectID, shot, target, assetVersions, referenceKeys); err != nil {
				return err
			}
			continue
		}
		shot, createErr := s.CreateProjectShot(userID, projectID, CreateProjectShotRequest{
			UnitID: unitID, Title: fmt.Sprintf("SH.%02d", index+1), Description: asset.Prompt, Position: index, DurationMs: 4000,
			Revision: ShotRevisionInput{PlotDescription: asset.Prompt, ImagePrompt: asset.Prompt, VideoPrompt: asset.Prompt, ContinuityNotes: "inkosStoryboard:" + sourceID},
		})
		if createErr != nil {
			return createErr
		}
		now := time.Now()
		if err := s.repo.CreateStorySceneShotLink(&model.StorySceneShotLink{ID: newID(), ProjectID: projectID, UnitID: unitID, SceneID: target.ID, ShotID: shot.ID, SourceHash: target.SourceHash, CreatedAt: now, UpdatedAt: now}); err != nil {
			return err
		}
		shotBySourceID[sourceID] = shot
		linksByShot[shot.ID] = []model.StorySceneShotLink{{SceneID: target.ID, ShotID: shot.ID, SourceHash: target.SourceHash}}
		if err := s.ensureNovelProductionShotReferences(projectID, shot, target, assetVersions, referenceKeys); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) upsertNovelProductionScene(userID, projectID, unitID, sourceID, sourceHash string, section novelProductionSection, imagePrompt string, position int) (*model.StoryScene, error) {
	title := strings.TrimSpace(section.Title)
	if title == "" {
		title = fmt.Sprintf("镜头 %02d", position+1)
	}
	action := truncateNovelProjectionText(section.Action, 6000)
	dialogue := truncateNovelProjectionText(section.Dialogue, 6000)
	visual := truncateNovelProjectionText(imagePrompt, 6000)
	characterAssetIDs, err := s.novelProductionCharacterAssetIDs(userID, projectID, section.Characters)
	if err != nil {
		return nil, err
	}
	characterAssetIDsJSON, err := json.Marshal(characterAssetIDs)
	if err != nil {
		return nil, err
	}
	sceneHash := storyHash(sourceID, strings.Join([]string{title, section.Location, section.Time, action, dialogue, visual, section.Characters, string(characterAssetIDsJSON)}, "\n"))
	existing, err := s.repo.StoryScenes(projectID, unitID)
	if err != nil {
		return nil, err
	}
	for index := range existing {
		if existing[index].Status != "inkos:"+sourceID {
			continue
		}
		updated := existing[index]
		if updated.SourceHash == sceneHash {
			return &updated, nil
		}
		updated.Title = title
		updated.Location = strings.TrimSpace(section.Location)
		updated.TimeOfDay = strings.TrimSpace(section.Time)
		updated.Action = action
		updated.Dialogue = dialogue
		updated.Emotion = strings.TrimSpace(section.Characters)
		updated.VisualIntent = visual
		updated.CharacterAssetIDsJSON = string(characterAssetIDsJSON)
		updated.SourceHash = sceneHash
		updated.UpdatedAt = time.Now()
		if err := s.repo.UpdateStoryScene(&updated); err != nil {
			return nil, err
		}
		return &updated, nil
	}
	now := time.Now()
	sceneEmotion := strings.TrimSpace(section.Characters)
	if sceneEmotion == "" {
		sceneEmotion = strings.TrimSpace(section.Title)
	}
	scene := model.StoryScene{ID: newID(), ProjectID: projectID, UnitID: unitID, Position: position, Title: title, Location: strings.TrimSpace(section.Location), TimeOfDay: strings.TrimSpace(section.Time), Action: action, Dialogue: dialogue, Emotion: sceneEmotion, VisualIntent: visual, CharacterAssetIDsJSON: string(characterAssetIDsJSON), SourceHash: sceneHash, Status: "inkos:" + sourceID, CreatedAt: now, UpdatedAt: now}
	if err := s.repo.CreateStoryScene(&scene); err != nil {
		return nil, err
	}
	return &scene, nil
}

func (s *Service) novelProductionCharacterAssetIDs(userID, projectID, characters string) ([]string, error) {
	characters = strings.TrimSpace(characters)
	if characters == "" {
		return []string{}, nil
	}
	assets, err := s.repo.ProjectAssets(userID, projectID)
	if err != nil {
		return nil, err
	}
	assetIDByName := make(map[string]string, len(assets))
	for _, asset := range assets {
		if asset.Category != model.AssetCategoryCharacter || strings.TrimSpace(asset.ID) == "" {
			continue
		}
		assetIDByName[model.AssetCandidateNameKey(asset.Title)] = asset.ID
	}
	parts := strings.FieldsFunc(characters, func(char rune) bool {
		switch char {
		case '、', '，', ',', '；', ';':
			return true
		default:
			return false
		}
	})
	ids := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		id := assetIDByName[model.AssetCandidateNameKey(part)]
		if id == "" {
			continue
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	return ids, nil
}

func (s *Service) syncNovelScriptCharacterCandidates(userID, projectID, unitID, bookID string, sections []novelProductionSection) error {
	assets, err := s.repo.ProjectAssets(userID, projectID)
	if err != nil {
		return err
	}
	known := make(map[string]struct{}, len(assets))
	for _, asset := range assets {
		if asset.Category == model.AssetCategoryCharacter {
			known[model.AssetCandidateNameKey(asset.Title)] = struct{}{}
		}
	}
	candidates := make([]AssetCandidateInput, 0, len(sections))
	seen := make(map[string]struct{}, len(sections))
	for index, section := range sections {
		for _, name := range novelProductionCharacterNames(section.Characters) {
			key := model.AssetCandidateNameKey(name)
			if key == "" {
				continue
			}
			if _, exists := known[key]; exists {
				continue
			}
			if _, exists := seen[key]; exists {
				continue
			}
			seen[key] = struct{}{}
			candidates = append(candidates, AssetCandidateInput{
				UnitID: unitID, Name: name, Category: string(model.AssetCategoryCharacter),
				Details: map[string]any{
					"sourceType": "inkos-script", "sourceId": fmt.Sprintf("%s:scene-%d", bookID, index+1),
					"role": "剧本人物待确认", "sceneTitle": section.Title, "sceneLocation": section.Location,
				},
			})
		}
	}
	if len(candidates) == 0 {
		return nil
	}
	_, err = s.CreateProjectAssetCandidates(userID, projectID, CreateAssetCandidatesRequest{Source: assetCandidateSourceInkOSScriptCharacter, Candidates: candidates})
	return err
}

func novelProductionCharacterNames(characters string) []string {
	parts := strings.FieldsFunc(characters, func(char rune) bool {
		switch char {
		case '、', '，', ',', '；', ';':
			return true
		default:
			return false
		}
	})
	result := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		name := strings.TrimSpace(part)
		key := model.AssetCandidateNameKey(name)
		if name == "" || key == "" {
			continue
		}
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, name)
	}
	return result
}

func (s *Service) ensureNovelProductionShotReferences(projectID string, shot model.Shot, scene model.StoryScene, assetVersions map[string]string, existing map[string]struct{}) error {
	var assetIDs []string
	if strings.TrimSpace(scene.CharacterAssetIDsJSON) == "" || json.Unmarshal([]byte(scene.CharacterAssetIDsJSON), &assetIDs) != nil {
		return nil
	}
	for _, assetID := range assetIDs {
		versionID := assetVersions[assetID]
		if versionID == "" {
			continue
		}
		key := novelProductionShotReferenceKey(shot.ID, versionID)
		if _, exists := existing[key]; exists {
			continue
		}
		now := time.Now()
		reference := model.ShotAssetReference{ID: newID(), ShotID: shot.ID, AssetVersionID: versionID, Role: "reference", Status: "linked", CreatedAt: now}
		if err := s.repo.UpsertShotAssetReferenceAndInvalidate(projectID, &reference, now); err != nil {
			return err
		}
		existing[key] = struct{}{}
	}
	return nil
}

func novelProductionShotReferenceKey(shotID, assetVersionID string) string {
	return shotID + "\x00" + assetVersionID
}

func novelProductionChapter(files map[string]string) int {
	productionPattern := regexp.MustCompile(`-ch-(\d{1,6})`)
	for path := range files {
		match := productionPattern.FindStringSubmatch(path)
		if len(match) == 2 {
			var number int
			if _, err := fmt.Sscanf(match[1], "%d", &number); err == nil && number > 0 {
				return number
			}
		}
	}
	return 0
}

func novelStoryboardSourceIDFromNotes(notes string) string {
	const marker = "inkosStoryboard:"
	index := strings.Index(notes, marker)
	if index < 0 {
		return ""
	}
	value := strings.TrimSpace(notes[index+len(marker):])
	if value == "" {
		return ""
	}
	return strings.Fields(value)[0]
}

func novelStoryboardTargetScenes(scenes []model.StoryScene, prompts []string) []model.StoryScene {
	targets := make([]model.StoryScene, len(prompts))
	if len(scenes) == 0 || len(prompts) == 0 {
		return targets
	}
	sceneTerms := make([]map[string]struct{}, len(scenes))
	for index, scene := range scenes {
		sceneTerms[index] = novelProductionTerms(strings.Join([]string{scene.Title, scene.Location, scene.Action, scene.Dialogue, scene.VisualIntent}, "\n"))
	}
	scores := make([][]int, len(prompts))
	for promptIndex, prompt := range prompts {
		promptTerms := novelProductionTerms(prompt)
		scores[promptIndex] = make([]int, len(scenes))
		for sceneIndex, terms := range sceneTerms {
			for term := range promptTerms {
				if _, exists := terms[term]; exists {
					scores[promptIndex][sceneIndex]++
				}
			}
		}
	}
	const negativeInfinity = -1 << 30
	dp := make([][]int, len(prompts))
	previous := make([][]int, len(prompts))
	for promptIndex := range prompts {
		dp[promptIndex] = make([]int, len(scenes))
		previous[promptIndex] = make([]int, len(scenes))
		for sceneIndex := range scenes {
			dp[promptIndex][sceneIndex] = negativeInfinity
		}
	}
	for sceneIndex := range scenes {
		// The first storyboard shot should not skip earlier script scenes unless
		// its semantic evidence is clearly stronger.
		dp[0][sceneIndex] = scores[0][sceneIndex] - sceneIndex*2
	}
	for promptIndex := 1; promptIndex < len(prompts); promptIndex++ {
		for sceneIndex := range scenes {
			bestScore, bestPrevious := negativeInfinity, 0
			for previousSceneIndex := 0; previousSceneIndex <= sceneIndex; previousSceneIndex++ {
				candidate := dp[promptIndex-1][previousSceneIndex] - (sceneIndex - previousSceneIndex)
				if candidate > bestScore {
					bestScore, bestPrevious = candidate, previousSceneIndex
				}
			}
			dp[promptIndex][sceneIndex] = bestScore + scores[promptIndex][sceneIndex]
			previous[promptIndex][sceneIndex] = bestPrevious
		}
	}
	bestScene, bestScore := 0, negativeInfinity
	for sceneIndex := range scenes {
		if dp[len(prompts)-1][sceneIndex] > bestScore {
			bestScene, bestScore = sceneIndex, dp[len(prompts)-1][sceneIndex]
		}
	}
	for promptIndex := len(prompts) - 1; promptIndex >= 0; promptIndex-- {
		targets[promptIndex] = scenes[bestScene]
		if promptIndex > 0 {
			bestScene = previous[promptIndex][bestScene]
		}
	}
	return targets
}

func novelProductionTerms(value string) map[string]struct{} {
	runes := make([]rune, 0, len(value))
	for _, char := range []rune(strings.ToLower(value)) {
		if char >= '0' && char <= '9' || char >= 'a' && char <= 'z' || char >= '\u4e00' && char <= '\u9fff' {
			runes = append(runes, char)
		}
	}
	terms := make(map[string]struct{}, len(runes))
	for index := 0; index+1 < len(runes); index++ {
		terms[string(runes[index:index+2])] = struct{}{}
	}
	return terms
}

func projectNovelScriptSections(script string) []novelProductionSection {
	scenePattern := regexp.MustCompile(`^\*\*场次\s*(\d+)\s*(.*?)\s*\*\*$`)
	lines := strings.Split(script, "\n")
	inScriptBody := false
	sections := make([]novelProductionSection, 0, 32)
	current := (*novelProductionSection)(nil)
	activeField := ""
	flush := func() {
		if current != nil && (strings.TrimSpace(current.Action) != "" || strings.TrimSpace(current.Dialogue) != "") {
			sections = append(sections, *current)
		}
		current = nil
		activeField = ""
	}
	appendField := func(value string) {
		switch activeField {
		case "action":
			current.Action = strings.TrimSpace(strings.Join([]string{current.Action, value}, "\n"))
		case "dialogue":
			current.Dialogue = strings.TrimSpace(strings.Join([]string{current.Dialogue, value}, "\n"))
		}
	}
	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "## 剧本正文" {
			inScriptBody = true
			continue
		}
		if !inScriptBody {
			// Be tolerant of scripts that omit the wrapper heading, but never
			// import the cast list before the body begins.
			if strings.Contains(line, "场次") && strings.HasPrefix(line, "**场次") {
				inScriptBody = true
			} else {
				continue
			}
		}
		if strings.HasPrefix(line, "#") {
			flush()
			continue
		}
		if match := scenePattern.FindStringSubmatch(line); len(match) == 3 {
			flush()
			parts := strings.Fields(match[2])
			current = &novelProductionSection{Title: "场次 " + match[1]}
			if len(parts) > 0 {
				current.Location = strings.Join(parts, " ")
			}
			continue
		}
		if current == nil {
			continue
		}
		switch {
		case strings.HasPrefix(line, "人物：") || strings.HasPrefix(line, "人物:"):
			current.Characters = strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(line, "人物："), "人物:"))
			activeField = ""
		case strings.HasPrefix(line, "动作：") || strings.HasPrefix(line, "动作:"):
			activeField = "action"
			appendField(strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(line, "动作："), "动作:")))
		case strings.HasPrefix(line, "对白：") || strings.HasPrefix(line, "对白:"):
			activeField = "dialogue"
			appendField(strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(line, "对白："), "对白:")))
		case strings.HasPrefix(line, "**集尾钩子"):
			// The final hook is not a discrete scene; keep it in scene action.
			activeField = "action"
		default:
			if line != "" {
				appendField(line)
			}
		}
	}
	flush()
	return sections
}
