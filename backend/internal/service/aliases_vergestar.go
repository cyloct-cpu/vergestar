package service

import "infinite-canvas/backend/internal/app"

// vergestar 分支新增的 Story / Novel Agent 领域类型随 app 包导出，
// handler 层继续通过 service 别名引用。
type (
	ComfyBridgeControlSettings  = app.ComfyBridgeControlSettings
	CreateStoryBranchRequest    = app.CreateStoryBranchRequest
	CreateStoryMemoryRequest    = app.CreateStoryMemoryRequest
	CreateStoryReviewRequest    = app.CreateStoryReviewRequest
	CreateStorySceneRequest     = app.CreateStorySceneRequest
	CreateStorySceneShotRequest = app.CreateStorySceneShotRequest
	NovelAgentTurnRequest       = app.NovelAgentTurnRequest
	NovelBookSettingsRequest    = app.NovelBookSettingsRequest
	NovelDetectRequest          = app.NovelDetectRequest
	NovelRadarRequest           = app.NovelRadarRequest
	NovelTruthRequest           = app.NovelTruthRequest
	NovelTruthWriteRequest      = app.NovelTruthWriteRequest
)
