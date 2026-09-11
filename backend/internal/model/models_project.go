package model

import (
	"html"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

var projectUnitHTMLTagPattern = regexp.MustCompile(`<[^>]+>`)

func ProjectUnitWordCount(sourceText string) int {
	plainText := projectUnitHTMLTagPattern.ReplaceAllString(sourceText, "")
	return utf8.RuneCountInString(strings.TrimSpace(html.UnescapeString(plainText)))
}

const AssetIDMaxLength = 80

type Resource struct {
	ID       string         `json:"id" gorm:"primaryKey;size:36"`
	UserID   string         `json:"userId" gorm:"index;size:36;index:idx_resources_user_created,priority:1"`
	Kind     string         `json:"kind" gorm:"index;size:24"`
	Status   ResourceStatus `json:"status" gorm:"index;size:24"`
	Provider string         `json:"provider" gorm:"size:24"`
	Endpoint string         `json:"endpoint"`
	Bucket   string         `json:"bucket" gorm:"size:160"`
	// 用户 OSS 每次修改都会生成新版本，资源固定引用创建时的存储与密钥；只有同一存储位置才可复用当前 CDN。
	StorageSettingID string    `json:"-" gorm:"index;size:36"`
	ObjectKey        string    `json:"objectKey" gorm:"index"`
	PublicURL        string    `json:"publicUrl"`
	MimeType         string    `json:"mimeType" gorm:"size:120"`
	Size             int64     `json:"size"`
	Width            int       `json:"width"`
	Height           int       `json:"height"`
	DurationMs       int64     `json:"durationMs"`
	ETag             string    `json:"etag" gorm:"size:160"`
	Error            string    `json:"error"`
	CreatedAt        time.Time `json:"createdAt" gorm:"index:idx_resources_user_created,priority:2"`
	UpdatedAt        time.Time `json:"updatedAt"`
}

// ResourceDeletionJob is the durable handoff between database deletion and
// physical object cleanup. Storage fields are frozen because the Resource row
// is removed in the same transaction that creates this job.
type ResourceDeletionJob struct {
	ID               string                 `json:"id" gorm:"primaryKey;size:36"`
	UserID           string                 `json:"userId" gorm:"index;size:36"`
	ResourceID       string                 `json:"resourceId" gorm:"index;size:36"`
	Provider         string                 `json:"provider" gorm:"size:24"`
	Endpoint         string                 `json:"endpoint"`
	Bucket           string                 `json:"bucket" gorm:"size:160"`
	StorageSettingID string                 `json:"-" gorm:"index;size:36"`
	ObjectKey        string                 `json:"objectKey" gorm:"index"`
	Status           ResourceDeletionStatus `json:"status" gorm:"index:idx_resource_deletion_jobs_due,priority:1;size:24"`
	Attempts         int                    `json:"attempts"`
	LastError        string                 `json:"lastError" gorm:"type:text"`
	NextAttemptAt    time.Time              `json:"nextAttemptAt" gorm:"index:idx_resource_deletion_jobs_due,priority:2"`
	LeaseOwner       string                 `json:"-" gorm:"index;size:120"`
	LeaseExpiresAt   *time.Time             `json:"-" gorm:"index"`
	CreatedAt        time.Time              `json:"createdAt"`
	UpdatedAt        time.Time              `json:"updatedAt"`
}

// AnnouncementImageDraft marks an uploaded image as temporary until an
// announcement create or update transaction consumes it.
type AnnouncementImageDraft struct {
	ResourceID string    `json:"resourceId" gorm:"primaryKey;size:36"`
	UserID     string    `json:"userId" gorm:"index;size:36"`
	CreatedAt  time.Time `json:"createdAt" gorm:"index"`
}

type Asset struct {
	ID               string             `json:"id" gorm:"primaryKey;size:80"`
	UserID           string             `json:"userId" gorm:"index;size:36;index:idx_assets_user_updated,priority:1"`
	Kind             string             `json:"kind" gorm:"index;size:24"`
	Category         AssetCategory      `json:"category" gorm:"index;size:32"`
	Status           AssetVersionStatus `json:"status" gorm:"index;size:24"`
	PrimaryVersionID string             `json:"primaryVersionId,omitempty" gorm:"index;size:36"`
	Title            string             `json:"title" gorm:"size:240"`
	PayloadJSON      string             `json:"payloadJson" gorm:"type:text"`
	CreatedAt        time.Time          `json:"createdAt"`
	UpdatedAt        time.Time          `json:"updatedAt" gorm:"index:idx_assets_user_updated,priority:2"`
}

type ProjectAssetLink struct {
	ID        string    `json:"id" gorm:"primaryKey;size:36"`
	ProjectID string    `json:"projectId" gorm:"index;size:36;uniqueIndex:idx_project_asset_links_unique,priority:1;index:idx_project_asset_links_project_folder_position,priority:1"`
	AssetID   string    `json:"assetId" gorm:"index;size:80;uniqueIndex:idx_project_asset_links_unique,priority:2"`
	FolderID  string    `json:"folderId,omitempty" gorm:"index;size:36;index:idx_project_asset_links_project_folder_position,priority:2"`
	Position  int       `json:"position" gorm:"index;index:idx_project_asset_links_project_folder_position,priority:3"`
	CreatedAt time.Time `json:"createdAt"`
}

// ProjectAssetFolder 只保存项目内的目录结构；真实媒体仍由 Asset/Resource 唯一持有。
type ProjectAssetFolder struct {
	ID        string    `json:"id" gorm:"primaryKey;size:36"`
	ProjectID string    `json:"projectId" gorm:"index;size:36;uniqueIndex:idx_project_asset_folders_sibling_name,priority:1"`
	ParentID  string    `json:"parentId,omitempty" gorm:"index;size:36;uniqueIndex:idx_project_asset_folders_sibling_name,priority:2"`
	Name      string    `json:"name" gorm:"size:240"`
	NameKey   string    `json:"-" gorm:"size:240;uniqueIndex:idx_project_asset_folders_sibling_name,priority:3"`
	Style     string    `json:"style" gorm:"size:24"`
	Theme     string    `json:"theme" gorm:"size:24"`
	Position  int       `json:"position" gorm:"index"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type ProjectAssetCandidate struct {
	ID              string        `json:"id" gorm:"primaryKey;size:36"`
	ProjectID       string        `json:"projectId" gorm:"index;size:36;index:idx_project_asset_candidates_project_unit_status,priority:1;index:idx_project_asset_candidates_project_status_category,priority:1"`
	UnitID          string        `json:"unitId,omitempty" gorm:"index;size:36;index:idx_project_asset_candidates_project_unit_status,priority:2"`
	ShotID          string        `json:"shotId,omitempty" gorm:"index;size:36"`
	Name            string        `json:"name" gorm:"size:240"`
	NameKey         string        `json:"-" gorm:"index;size:240"`
	Category        AssetCategory `json:"category" gorm:"index;size:32;index:idx_project_asset_candidates_project_status_category,priority:3"`
	Status          string        `json:"status" gorm:"index;size:32;index:idx_project_asset_candidates_project_unit_status,priority:3;index:idx_project_asset_candidates_project_status_category,priority:2"`
	Source          string        `json:"source,omitempty" gorm:"index;size:48"`
	DetailsJSON     string        `json:"detailsJson" gorm:"type:text"`
	ResolvedAssetID string        `json:"resolvedAssetId,omitempty" gorm:"index;size:80"`
	CreatedAt       time.Time     `json:"createdAt"`
	UpdatedAt       time.Time     `json:"updatedAt"`
}

type AssetVersion struct {
	ID             string             `json:"id" gorm:"primaryKey;size:36"`
	AssetID        string             `json:"assetId" gorm:"index;size:80;uniqueIndex:idx_asset_versions_number,priority:1"`
	Version        int                `json:"version" gorm:"uniqueIndex:idx_asset_versions_number,priority:2"`
	Status         AssetVersionStatus `json:"status" gorm:"index;size:24"`
	DefinitionJSON string             `json:"definitionJson" gorm:"type:text"`
	Prompt         string             `json:"prompt" gorm:"type:text"`
	Note           string             `json:"note" gorm:"size:500"`
	CreatedAt      time.Time          `json:"createdAt"`
	UpdatedAt      time.Time          `json:"updatedAt"`
}

type AssetRepresentation struct {
	ID             string    `json:"id" gorm:"primaryKey;size:36"`
	TaskID         string    `json:"taskId,omitempty" gorm:"index;size:36;uniqueIndex:idx_asset_representations_task_role,priority:1"`
	AssetVersionID string    `json:"assetVersionId" gorm:"index;size:36;uniqueIndex:idx_asset_representations_version_role,priority:1"`
	ResourceID     string    `json:"resourceId,omitempty" gorm:"index;size:36"`
	MediaType      string    `json:"mediaType" gorm:"index;size:24"`
	Role           string    `json:"role" gorm:"index;size:32;uniqueIndex:idx_asset_representations_task_role,priority:2;uniqueIndex:idx_asset_representations_version_role,priority:2"`
	MetadataJSON   string    `json:"metadataJson" gorm:"type:text"`
	CreatedAt      time.Time `json:"createdAt"`
}

// VoiceProfile 是可复用的声音身份；试听音频只是表现资源，不等同于声音本身。
type VoiceProfile struct {
	ID                   string    `json:"id" gorm:"primaryKey;size:36"`
	UserID               string    `json:"userId" gorm:"index;size:36;uniqueIndex:idx_voice_profiles_user_provider_key,priority:1"`
	Name                 string    `json:"name" gorm:"size:160"`
	Provider             string    `json:"provider" gorm:"size:48;uniqueIndex:idx_voice_profiles_user_provider_key,priority:2"`
	VoiceKey             string    `json:"voiceKey" gorm:"size:160;uniqueIndex:idx_voice_profiles_user_provider_key,priority:3"`
	Language             string    `json:"language" gorm:"size:80"`
	Timbre               string    `json:"timbre" gorm:"size:240"`
	SampleResourceID     string    `json:"sampleResourceId,omitempty" gorm:"index;size:36"`
	CompatibleModelsJSON string    `json:"compatibleModelsJson" gorm:"type:text"`
	Status               string    `json:"status" gorm:"index;size:24"`
	CreatedAt            time.Time `json:"createdAt"`
	UpdatedAt            time.Time `json:"updatedAt"`
}

type CharacterVoiceBinding struct {
	ID             string    `json:"id" gorm:"primaryKey;size:36"`
	AssetVersionID string    `json:"assetVersionId" gorm:"uniqueIndex;size:36"`
	VoiceProfileID string    `json:"voiceProfileId" gorm:"index;size:36"`
	Instructions   string    `json:"instructions" gorm:"type:text"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

// Project 是短剧领域聚合根；CanvasProject 仍代表可独立创作的画布文档。
type Project struct {
	ID                string        `json:"id" gorm:"primaryKey;size:36"`
	UserID            string        `json:"userId" gorm:"index;size:36;uniqueIndex:idx_projects_user_name,priority:1"`
	Name              string        `json:"name" gorm:"size:240;uniqueIndex:idx_projects_user_name,priority:2"`
	Type              string        `json:"type" gorm:"size:32;index"`
	AspectRatio       string        `json:"aspectRatio" gorm:"size:16"`
	SourceType        string        `json:"sourceType" gorm:"size:32"`
	Description       string        `json:"description" gorm:"type:text"`
	CoverResourceID   string        `json:"coverResourceId,omitempty" gorm:"index;size:36"`
	StylePresetID     string        `json:"stylePresetId" gorm:"size:64"`
	StyleProfileJSON  string        `json:"styleProfileJson" gorm:"type:text"`
	DefaultImageModel string        `json:"defaultImageModel,omitempty" gorm:"size:500"`
	DefaultVideoModel string        `json:"defaultVideoModel,omitempty" gorm:"size:500"`
	Status            ProjectStatus `json:"status" gorm:"index;size:24"`
	Revision          int64         `json:"revision"`
	CreatedAt         time.Time     `json:"createdAt"`
	UpdatedAt         time.Time     `json:"updatedAt" gorm:"index"`
}

// StyleProfile 是用户可持续编辑的风格源；项目只保存应用当时的 JSON 快照，避免源对象更新污染历史生成。
type StyleProfile struct {
	ID          string     `json:"id" gorm:"primaryKey;size:36"`
	UserID      string     `json:"userId" gorm:"index;size:36;index:idx_style_profiles_user_updated,priority:1"`
	Name        string     `json:"name" gorm:"size:160"`
	Description string     `json:"description" gorm:"size:500"`
	CoverURL    string     `json:"coverUrl" gorm:"type:text"`
	TagsJSON    string     `json:"tagsJson" gorm:"type:text"`
	ProfileJSON string     `json:"profileJson" gorm:"type:text"`
	Favorite    bool       `json:"favorite" gorm:"index"`
	LastUsedAt  *time.Time `json:"lastUsedAt,omitempty" gorm:"index"`
	Revision    int64      `json:"revision"`
	CreatedAt   time.Time  `json:"createdAt"`
	UpdatedAt   time.Time  `json:"updatedAt" gorm:"index:idx_style_profiles_user_updated,priority:2"`
}

type ProjectUnit struct {
	ID         string            `json:"id" gorm:"primaryKey;size:36"`
	ProjectID  string            `json:"projectId" gorm:"index;size:36;index:idx_project_units_project_position,priority:1"`
	ParentID   string            `json:"parentId,omitempty" gorm:"index;size:36"`
	Kind       ProjectUnitKind   `json:"kind" gorm:"index;size:24"`
	Title      string            `json:"title" gorm:"size:240"`
	SourceText string            `json:"sourceText" gorm:"type:text"`
	WordCount  int               `json:"wordCount"`
	Status     ProjectUnitStatus `json:"status" gorm:"index;size:24"`
	Position   int               `json:"position" gorm:"index:idx_project_units_project_position,priority:2"`
	CreatedAt  time.Time         `json:"createdAt"`
	UpdatedAt  time.Time         `json:"updatedAt"`
}

// StoryChapterVersion stores an immutable prose snapshot. ProjectUnit remains
// the current chapter pointer used by the existing screenplay workflow.
type StoryChapterVersion struct {
	ID         string    `json:"id" gorm:"primaryKey;size:36"`
	ProjectID  string    `json:"projectId" gorm:"index;size:36;uniqueIndex:idx_story_chapter_versions_unit_number,priority:1"`
	UnitID     string    `json:"unitId" gorm:"index;size:36;uniqueIndex:idx_story_chapter_versions_unit_number,priority:2"`
	Number     int       `json:"number" gorm:"uniqueIndex:idx_story_chapter_versions_unit_number,priority:3"`
	Title      string    `json:"title" gorm:"size:240"`
	Content    string    `json:"content" gorm:"type:text"`
	SourceHash string    `json:"sourceHash" gorm:"size:96;index"`
	CreatedBy  string    `json:"createdBy,omitempty" gorm:"index;size:36"`
	CreatedAt  time.Time `json:"createdAt" gorm:"index"`
}

// StoryReview records review output separately from chapter prose so a later
// revision can be compared to the original review and source version.
type StoryReview struct {
	ID               string    `json:"id" gorm:"primaryKey;size:36"`
	ProjectID        string    `json:"projectId" gorm:"index;size:36;index:idx_story_reviews_project_unit_created,priority:1"`
	UnitID           string    `json:"unitId" gorm:"index;size:36;index:idx_story_reviews_project_unit_created,priority:2"`
	ChapterVersionID string    `json:"chapterVersionId" gorm:"index;size:36"`
	Status           string    `json:"status" gorm:"index;size:24"`
	Summary          string    `json:"summary" gorm:"type:text"`
	IssuesJSON       string    `json:"issuesJson" gorm:"type:text"`
	CreatedAt        time.Time `json:"createdAt" gorm:"index:idx_story_reviews_project_unit_created,priority:3"`
	UpdatedAt        time.Time `json:"updatedAt"`
}

// StoryMemory is a user-approved fact or summary. Its source reference keeps
// retrieval traceable when a chapter, review, or branch later changes.
type StoryMemory struct {
	ID         string    `json:"id" gorm:"primaryKey;size:36"`
	ProjectID  string    `json:"projectId" gorm:"index;size:36;index:idx_story_memories_project_kind_created,priority:1"`
	Kind       string    `json:"kind" gorm:"size:32;index:idx_story_memories_project_kind_created,priority:2"`
	Content    string    `json:"content" gorm:"type:text"`
	SourceType string    `json:"sourceType" gorm:"size:32"`
	SourceID   string    `json:"sourceId" gorm:"index;size:36"`
	SourceHash string    `json:"sourceHash" gorm:"size:96;index"`
	CreatedAt  time.Time `json:"createdAt" gorm:"index:idx_story_memories_project_kind_created,priority:3"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

// StoryBranch is an isolated future-plan candidate. It never mutates the
// current chapter or story state until an explicit future adoption operation.
type StoryBranch struct {
	ID             string    `json:"id" gorm:"primaryKey;size:36"`
	ProjectID      string    `json:"projectId" gorm:"index;size:36;index:idx_story_branches_project_created,priority:1"`
	BaseUnitID     string    `json:"baseUnitId" gorm:"index;size:36"`
	BaseSourceHash string    `json:"baseSourceHash" gorm:"size:96;index"`
	Title          string    `json:"title" gorm:"size:240"`
	PlanJSON       string    `json:"planJson" gorm:"type:text"`
	Status         string    `json:"status" gorm:"index;size:24"`
	CreatedAt      time.Time `json:"createdAt" gorm:"index:idx_story_branches_project_created,priority:2"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

// StoryScene is the screenplay boundary between prose and the existing shot
// and canvas production workflow.
type StoryScene struct {
	ID                    string    `json:"id" gorm:"primaryKey;size:36"`
	ProjectID             string    `json:"projectId" gorm:"index;size:36;index:idx_story_scenes_project_unit_position,priority:1"`
	UnitID                string    `json:"unitId" gorm:"index;size:36;index:idx_story_scenes_project_unit_position,priority:2"`
	Position              int       `json:"position" gorm:"index:idx_story_scenes_project_unit_position,priority:3"`
	Title                 string    `json:"title" gorm:"size:240"`
	Location              string    `json:"location" gorm:"size:240"`
	TimeOfDay             string    `json:"timeOfDay" gorm:"size:80"`
	Action                string    `json:"action" gorm:"type:text"`
	Dialogue              string    `json:"dialogue" gorm:"type:text"`
	Emotion               string    `json:"emotion" gorm:"size:240"`
	VisualIntent          string    `json:"visualIntent" gorm:"type:text"`
	CharacterAssetIDsJSON string    `json:"characterAssetIdsJson" gorm:"type:text"`
	EnvironmentAssetID    string    `json:"environmentAssetId,omitempty" gorm:"index;size:80"`
	SourceHash            string    `json:"sourceHash" gorm:"size:96;index"`
	Status                string    `json:"status" gorm:"index;size:24"`
	CreatedAt             time.Time `json:"createdAt"`
	UpdatedAt             time.Time `json:"updatedAt"`
}

// StorySceneShotLink records the source scene for a production shot. The hash
// makes stale detection deterministic when story scenes later become editable.
type StorySceneShotLink struct {
	ID         string    `json:"id" gorm:"primaryKey;size:36"`
	ProjectID  string    `json:"projectId" gorm:"index;size:36"`
	UnitID     string    `json:"unitId" gorm:"index;size:36"`
	SceneID    string    `json:"sceneId" gorm:"index;size:36;uniqueIndex:idx_story_scene_shot_unique,priority:1"`
	ShotID     string    `json:"shotId" gorm:"index;size:36;uniqueIndex:idx_story_scene_shot_unique,priority:2"`
	SourceHash string    `json:"sourceHash" gorm:"size:96;index"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

// StoryAgentSession is the durable user-facing Novel Agent conversation.
// It intentionally stores no model credential: credentials belong only to an
// individual in-memory turn forwarded through Vergestar's channel boundary.
type StoryAgentSession struct {
	ID        string    `json:"id" gorm:"primaryKey;size:36"`
	UserID    string    `json:"userId" gorm:"index;size:36;index:idx_story_agent_sessions_user_updated,priority:1"`
	ProjectID string    `json:"projectId,omitempty" gorm:"index;size:36"`
	Title     string    `json:"title" gorm:"size:240"`
	Mode      string    `json:"mode" gorm:"size:48"`
	Status    string    `json:"status" gorm:"index;size:24"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt" gorm:"index:idx_story_agent_sessions_user_updated,priority:2"`
}

// StoryAgentJob is the Vergestar DB mirror of a Novel Agent Bridge job.
// It is the history/recovery source of truth for the task list UI; the
// bridge .jobs snapshot remains the execution-side record. Result content
// is not mirrored (bridge owns artifacts); a succeeded row points pollers
// back to the bridge for full artifacts.
type StoryAgentJob struct {
    ID             string    `json:"id" gorm:"primaryKey;size:36"`
    UserID         string    `json:"userId" gorm:"index;size:36;index:idx_story_agent_jobs_user_created,priority:1"`
    SessionID      string    `json:"sessionId" gorm:"index;size:120"`
    BookID         string    `json:"bookId,omitempty" gorm:"size:160"`
    Mode           string    `json:"mode" gorm:"size:48"`
    ConfirmedIntent string   `json:"confirmedIntent,omitempty" gorm:"size:48"`
    Status         string    `json:"status" gorm:"index;size:24"`
    Stage          string    `json:"stage" gorm:"size:240"`
    Error          string    `json:"error,omitempty" gorm:"type:text"`
    LogsJSON       string    `json:"-" gorm:"type:text"`
    ResultSummary  string    `json:"resultSummary,omitempty" gorm:"type:text"`
    CreatedAt      time.Time `json:"createdAt"`
    UpdatedAt      time.Time `json:"updatedAt" gorm:"index:idx_story_agent_jobs_user_created,priority:2"`
}
type StoryAgentMessage struct {
	ID         string `json:"id" gorm:"primaryKey;size:36"`
	SessionID  string `json:"sessionId" gorm:"index;size:36;index:idx_story_agent_messages_session_created,priority:1"`
	Role       string `json:"role" gorm:"size:24"`
	Content    string `json:"content" gorm:"type:text"`
	SkillsJSON string `json:"skillsJson" gorm:"type:text"`
	// ConfirmationJSON keeps the structured action available after a page
	// refresh without putting model credentials or internal paths in history.
	ConfirmationJSON string    `json:"confirmationJson,omitempty" gorm:"type:text"`
	// ArtifactsJSON stores a bounded production projection payload for
	// completed confirmed jobs. It contains no credentials or workspace paths.
	ArtifactsJSON string `json:"artifactsJson,omitempty" gorm:"type:text"`
	CreatedAt        time.Time `json:"createdAt" gorm:"index:idx_story_agent_messages_session_created,priority:2"`
}

// StoryFoundation is the Vergestar-owned projection of an InkOS book
// foundation. FilesJSON preserves the reviewed source documents while the
// project remains the user-facing aggregate used by the rest of the app.
type StoryFoundation struct {
	ID             string    `json:"id" gorm:"primaryKey;size:36"`
	ProjectID      string    `json:"projectId" gorm:"uniqueIndex;size:36"`
	UserID         string    `json:"userId" gorm:"index;size:36;uniqueIndex:idx_story_foundation_user_book,priority:1"`
	InkosBookID    string    `json:"inkosBookId" gorm:"size:240;uniqueIndex:idx_story_foundation_user_book,priority:2"`
	AgentSessionID string    `json:"agentSessionId,omitempty" gorm:"index;size:36"`
	Title          string    `json:"title" gorm:"size:240"`
	BookJSON       string    `json:"bookJson" gorm:"type:text"`
	FilesJSON      string    `json:"filesJson" gorm:"type:text"`
	SourceHash     string    `json:"sourceHash" gorm:"size:96;index"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

type CanvasUnitLink struct {
	ID        string    `json:"id" gorm:"primaryKey;size:36"`
	ProjectID string    `json:"projectId" gorm:"index;size:36;uniqueIndex:idx_canvas_unit_links_unique,priority:1;index:idx_canvas_unit_links_project_unit,priority:1"`
	CanvasID  string    `json:"canvasId" gorm:"index;size:80;uniqueIndex:idx_canvas_unit_links_unique,priority:2"`
	UnitID    string    `json:"unitId" gorm:"index;size:36;uniqueIndex:idx_canvas_unit_links_unique,priority:3;index:idx_canvas_unit_links_project_unit,priority:2"`
	Role      string    `json:"role" gorm:"size:32"`
	CreatedAt time.Time `json:"createdAt"`
}

type Shot struct {
	ID                string    `json:"id" gorm:"primaryKey;size:36"`
	ProjectID         string    `json:"projectId" gorm:"index;size:36;index:idx_shots_project_unit_position,priority:1"`
	UnitID            string    `json:"unitId" gorm:"index;size:36;index:idx_shots_project_unit_position,priority:2"`
	CurrentRevisionID string    `json:"currentRevisionId,omitempty" gorm:"index;size:36"`
	Title             string    `json:"title" gorm:"size:240"`
	Description       string    `json:"description" gorm:"type:text"`
	Position          int       `json:"position" gorm:"index:idx_shots_project_unit_position,priority:3"`
	DurationMs        int64     `json:"durationMs"`
	Status            string    `json:"status" gorm:"index;size:24"`
	CreatedAt         time.Time `json:"createdAt"`
	UpdatedAt         time.Time `json:"updatedAt"`
}

// ShotRevision 保存可复现的分镜脚本版本；Shot 只保留稳定身份、排序和当前版本指针。
type ShotRevision struct {
	ID              string    `json:"id" gorm:"primaryKey;size:36"`
	ShotID          string    `json:"shotId" gorm:"index;size:36;uniqueIndex:idx_shot_revisions_version,priority:1"`
	Version         int       `json:"version" gorm:"uniqueIndex:idx_shot_revisions_version,priority:2"`
	PlotDescription string    `json:"plotDescription" gorm:"type:text"`
	Action          string    `json:"action" gorm:"type:text"`
	Dialogue        string    `json:"dialogue" gorm:"type:text"`
	ShotSize        string    `json:"shotSize" gorm:"size:80"`
	CameraAngle     string    `json:"cameraAngle" gorm:"size:80"`
	CameraMovement  string    `json:"cameraMovement" gorm:"size:120"`
	DurationMs      int64     `json:"durationMs"`
	ImagePrompt     string    `json:"imagePrompt" gorm:"type:text"`
	VideoPrompt     string    `json:"videoPrompt" gorm:"type:text"`
	NegativePrompt  string    `json:"negativePrompt" gorm:"type:text"`
	ContinuityNotes string    `json:"continuityNotes" gorm:"type:text"`
	ActionBeatsJSON string    `json:"actionBeatsJson" gorm:"type:text"`
	CreatedBy       string    `json:"createdBy,omitempty" gorm:"index;size:36"`
	CreatedAt       time.Time `json:"createdAt"`
}

// ShotArtifact 是镜头的版本化生产产物。修改分镜或资产引用时只标记 stale，不删除历史。
type ShotArtifact struct {
	ID           string    `json:"id" gorm:"primaryKey;size:36"`
	ProjectID    string    `json:"projectId" gorm:"index;size:36;index:idx_shot_artifacts_project_unit,priority:1"`
	UnitID       string    `json:"unitId" gorm:"index;size:36;index:idx_shot_artifacts_project_unit,priority:2"`
	ShotID       string    `json:"shotId" gorm:"index;size:36;uniqueIndex:idx_shot_artifacts_version,priority:1"`
	RevisionID   string    `json:"revisionId,omitempty" gorm:"index;size:36"`
	TaskID       string    `json:"taskId,omitempty" gorm:"index;size:36"`
	Type         string    `json:"type" gorm:"index;size:40;uniqueIndex:idx_shot_artifacts_version,priority:2"`
	Version      int       `json:"version" gorm:"uniqueIndex:idx_shot_artifacts_version,priority:3"`
	ResourceID   string    `json:"resourceId,omitempty" gorm:"index;size:36"`
	Status       string    `json:"status" gorm:"index;size:24"`
	Selected     bool      `json:"selected" gorm:"index"`
	MetadataJSON string    `json:"metadataJson" gorm:"type:text"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

type ShotAssetReference struct {
	ID             string    `json:"id" gorm:"primaryKey;size:36"`
	ShotID         string    `json:"shotId" gorm:"index;size:36;uniqueIndex:idx_shot_asset_reference_unique,priority:1"`
	AssetVersionID string    `json:"assetVersionId" gorm:"index;size:36;uniqueIndex:idx_shot_asset_reference_unique,priority:2"`
	Role           string    `json:"role" gorm:"index;size:32;uniqueIndex:idx_shot_asset_reference_unique,priority:3"`
	Status         string    `json:"status" gorm:"index;size:24"`
	CreatedAt      time.Time `json:"createdAt"`
}

type WorkflowTemplateVersion struct {
	ID             string    `json:"id" gorm:"primaryKey;size:36"`
	TemplateKey    string    `json:"templateKey" gorm:"size:80;uniqueIndex:idx_workflow_template_version,priority:1"`
	Name           string    `json:"name" gorm:"size:160"`
	Version        int       `json:"version" gorm:"uniqueIndex:idx_workflow_template_version,priority:2"`
	DefinitionJSON string    `json:"definitionJson" gorm:"type:text"`
	CreatedAt      time.Time `json:"createdAt"`
}

type WorkflowInstance struct {
	ID                string         `json:"id" gorm:"primaryKey;size:36"`
	ProjectID         string         `json:"projectId" gorm:"index;size:36;uniqueIndex:idx_workflow_instance_scope,priority:1"`
	UnitID            string         `json:"unitId,omitempty" gorm:"index;size:36;uniqueIndex:idx_workflow_instance_scope,priority:2"`
	TemplateVersionID string         `json:"templateVersionId" gorm:"index;size:36;uniqueIndex:idx_workflow_instance_scope,priority:3"`
	Scope             string         `json:"scope" gorm:"index;size:24"`
	Status            WorkflowStatus `json:"status" gorm:"index;size:24"`
	Revision          int64          `json:"revision"`
	CreatedAt         time.Time      `json:"createdAt"`
	UpdatedAt         time.Time      `json:"updatedAt"`
}

type WorkflowStepInstance struct {
	ID                 string             `json:"id" gorm:"primaryKey;size:36"`
	WorkflowInstanceID string             `json:"workflowInstanceId" gorm:"index;size:36;uniqueIndex:idx_workflow_steps_instance_key,priority:1"`
	StepKey            string             `json:"stepKey" gorm:"size:80;uniqueIndex:idx_workflow_steps_instance_key,priority:2"`
	Name               string             `json:"name" gorm:"size:160"`
	Position           int                `json:"position"`
	Status             WorkflowStepStatus `json:"status" gorm:"index;size:24"`
	InputJSON          string             `json:"inputJson" gorm:"type:text"`
	OutputJSON         string             `json:"outputJson" gorm:"type:text"`
	Error              string             `json:"error" gorm:"type:text"`
	StartedAt          *time.Time         `json:"startedAt"`
	CompletedAt        *time.Time         `json:"completedAt"`
	CreatedAt          time.Time          `json:"createdAt"`
	UpdatedAt          time.Time          `json:"updatedAt"`
}

type WorkflowStepTask struct {
	ID             string    `json:"id" gorm:"primaryKey;size:36"`
	WorkflowStepID string    `json:"workflowStepId" gorm:"index;size:36;uniqueIndex:idx_workflow_step_tasks_unique,priority:1"`
	TaskID         string    `json:"taskId" gorm:"index;size:36;uniqueIndex:idx_workflow_step_tasks_unique,priority:2"`
	CreatedAt      time.Time `json:"createdAt"`
}

// ProductionTaskLink 显式区分领域项目、画布和镜头上下文，避免继续复用 Task.ProjectID 表达多种身份。
type ProductionTaskLink struct {
	ID             string    `json:"id" gorm:"primaryKey;size:36"`
	TaskID         string    `json:"taskId" gorm:"index;size:36;uniqueIndex:idx_production_task_context,priority:1"`
	ProjectID      string    `json:"projectId" gorm:"index;size:36"`
	CanvasID       string    `json:"canvasId,omitempty" gorm:"index;size:80"`
	UnitID         string    `json:"unitId,omitempty" gorm:"index;size:36"`
	ShotID         string    `json:"shotId,omitempty" gorm:"index;size:36;uniqueIndex:idx_production_task_context,priority:2"`
	WorkflowStepID string    `json:"workflowStepId,omitempty" gorm:"index;size:36"`
	ArtifactType   string    `json:"artifactType,omitempty" gorm:"index;size:40;uniqueIndex:idx_production_task_context,priority:3"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

type CanvasProject struct {
	ID          string    `json:"id" gorm:"primaryKey;size:80"`
	UserID      string    `json:"userId" gorm:"index;size:36;index:idx_canvas_projects_user_updated,priority:1;index:idx_canvas_projects_user_project_updated,priority:1"`
	ProjectID   string    `json:"projectId,omitempty" gorm:"index;size:36;index:idx_canvas_projects_user_project_updated,priority:2"`
	Title       string    `json:"title" gorm:"size:240"`
	PayloadJSON string    `json:"payloadJson" gorm:"type:text"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt" gorm:"index:idx_canvas_projects_user_updated,priority:2;index:idx_canvas_projects_user_project_updated,priority:3"`
}

type CanvasShare struct {
	ID          string     `json:"id" gorm:"primaryKey;size:36"`
	UserID      string     `json:"userId" gorm:"index;size:36;uniqueIndex:idx_canvas_share_owner_project,priority:1"`
	ProjectID   string     `json:"projectId" gorm:"index;size:80;uniqueIndex:idx_canvas_share_owner_project,priority:2"`
	TokenHash   string     `json:"-" gorm:"uniqueIndex;size:64"`
	TokenCipher string     `json:"-" gorm:"type:text"`
	Enabled     bool       `json:"enabled" gorm:"index"`
	ExpiresAt   *time.Time `json:"expiresAt" gorm:"index"`
	CreatedAt   time.Time  `json:"createdAt"`
	UpdatedAt   time.Time  `json:"updatedAt"`
}

type PromptTemplate struct {
	ID         string    `json:"id" gorm:"primaryKey;size:36"`
	Operation  string    `json:"operation" gorm:"size:64;index;uniqueIndex:idx_prompt_template_operation_version,priority:1"`
	Name       string    `json:"name" gorm:"size:120"`
	Version    int       `json:"version" gorm:"uniqueIndex:idx_prompt_template_operation_version,priority:2"`
	Content    string    `json:"content" gorm:"type:text"`
	OutputType string    `json:"outputType" gorm:"size:24"`
	Enabled    bool      `json:"enabled" gorm:"index;index:idx_prompt_template_active,priority:2"`
	CreatedBy  string    `json:"createdBy" gorm:"index;size:36"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

// UserPromptCustomization 只保存用户的创作策略层，动态上下文和输出契约始终由服务端编译器注入。
type UserPromptCustomization struct {
	ID             string    `json:"id" gorm:"primaryKey;size:36"`
	UserID         string    `json:"userId" gorm:"size:36;index;uniqueIndex:idx_user_prompt_operation,priority:1"`
	Operation      string    `json:"operation" gorm:"size:64;index;uniqueIndex:idx_user_prompt_operation,priority:2"`
	Mode           string    `json:"mode" gorm:"size:24"`
	Content        string    `json:"content" gorm:"type:text"`
	BaseTemplateID string    `json:"baseTemplateId" gorm:"size:36;index"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

type Announcement struct {
	ID              string             `json:"id" gorm:"primaryKey;size:36"`
	Title           string             `json:"title" gorm:"size:120"`
	Content         string             `json:"content" gorm:"type:text"`
	ImageResourceID string             `json:"imageResourceId,omitempty" gorm:"index;size:36"`
	ImageURL        string             `json:"imageUrl,omitempty" gorm:"-"`
	Level           AnnouncementLevel  `json:"level" gorm:"index;size:24"`
	Pinned          bool               `json:"pinned" gorm:"index"`
	Status          AnnouncementStatus `json:"status" gorm:"index;size:24;index:idx_announcements_status_published,priority:1"`
	CreatedBy       string             `json:"createdBy" gorm:"index;size:36"`
	PublishedAt     time.Time          `json:"publishedAt" gorm:"index:idx_announcements_status_published,priority:2"`
	ClosedAt        *time.Time         `json:"closedAt"`
	CreatedAt       time.Time          `json:"createdAt"`
	UpdatedAt       time.Time          `json:"updatedAt"`
}

type UserAnnouncementRead struct {
	ID             string    `json:"id" gorm:"primaryKey;size:36"`
	UserID         string    `json:"userId" gorm:"index;size:36;uniqueIndex:idx_user_announcement_read,priority:1"`
	AnnouncementID string    `json:"announcementId" gorm:"index;size:36;uniqueIndex:idx_user_announcement_read,priority:2"`
	ReadAt         time.Time `json:"readAt"`
}
