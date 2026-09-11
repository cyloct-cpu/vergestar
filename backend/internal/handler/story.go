package handler

import (
	"fmt"
	"net/http"

	"infinite-canvas/backend/internal/service"

	"github.com/gin-gonic/gin"
)

// RegisterStoryRoutes exposes the native story domain. It deliberately does
// not proxy InkOS or expose local filesystem paths to browsers.
func RegisterStoryRoutes(r *gin.RouterGroup, svc *service.Service) {
	withUser := func(c *gin.Context) (*string, bool) {
		user, err := currentUser(c, svc)
		if err != nil {
			failService(c, err)
			return nil, false
		}
		return &user.ID, true
	}
	r.GET("/story/projects/:projectId/foundation", func(c *gin.Context) {
		userID, authorized := withUser(c)
		if !authorized {
			return
		}
		foundation, err := svc.StoryFoundation(*userID, c.Param("projectId"))
		if err != nil {
			if service.IsProjectNotFound(err) {
				fail(c, http.StatusNotFound, err)
				return
			}
			failService(c, err)
			return
		}
		ok(c, foundation)
	})
	r.GET("/story/projects/:projectId/chapters/:unitId/versions", func(c *gin.Context) {
		userID, authorized := withUser(c)
		if !authorized {
			return
		}
		versions, err := svc.StoryChapterVersions(*userID, c.Param("projectId"), c.Param("unitId"))
		if err != nil {
			failService(c, err)
			return
		}
		ok(c, gin.H{"versions": versions})
	})
	r.POST("/story/projects/:projectId/chapters/:unitId/snapshot", func(c *gin.Context) {
		userID, authorized := withUser(c)
		if !authorized {
			return
		}
		version, err := svc.SnapshotStoryChapter(*userID, c.Param("projectId"), c.Param("unitId"))
		if err != nil {
			failService(c, err)
			return
		}
		ok(c, gin.H{"version": version})
	})
	r.POST("/story/projects/:projectId/chapters/:unitId/versions/:versionId/restore", func(c *gin.Context) {
		userID, authorized := withUser(c)
		if !authorized {
			return
		}
		unit, err := svc.RestoreStoryChapterVersion(*userID, c.Param("projectId"), c.Param("unitId"), c.Param("versionId"))
		if err != nil {
			failService(c, err)
			return
		}
		ok(c, gin.H{"unit": unit})
	})
	r.GET("/story/projects/:projectId/chapters/:unitId/reviews", func(c *gin.Context) {
		userID, authorized := withUser(c)
		if !authorized {
			return
		}
		reviews, err := svc.StoryReviews(*userID, c.Param("projectId"), c.Param("unitId"))
		if err != nil {
			failService(c, err)
			return
		}
		ok(c, gin.H{"reviews": reviews})
	})
	r.POST("/story/projects/:projectId/chapters/:unitId/reviews", func(c *gin.Context) {
		userID, authorized := withUser(c)
		if !authorized {
			return
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 256<<10)
		var request service.CreateStoryReviewRequest
		if err := c.ShouldBindJSON(&request); err != nil {
			fail(c, http.StatusBadRequest, err)
			return
		}
		review, err := svc.CreateStoryReview(*userID, c.Param("projectId"), c.Param("unitId"), request)
		if err != nil {
			failService(c, err)
			return
		}
		ok(c, gin.H{"review": review})
	})
	r.GET("/story/projects/:projectId/memories", func(c *gin.Context) {
		userID, authorized := withUser(c)
		if !authorized {
			return
		}
		memories, err := svc.StoryMemories(*userID, c.Param("projectId"))
		if err != nil {
			failService(c, err)
			return
		}
		ok(c, gin.H{"memories": memories})
	})
	r.POST("/story/projects/:projectId/memories", func(c *gin.Context) {
		userID, authorized := withUser(c)
		if !authorized {
			return
		}
		var request service.CreateStoryMemoryRequest
		if err := c.ShouldBindJSON(&request); err != nil {
			fail(c, http.StatusBadRequest, err)
			return
		}
		memory, err := svc.CreateStoryMemory(*userID, c.Param("projectId"), request)
		if err != nil {
			failService(c, err)
			return
		}
		ok(c, gin.H{"memory": memory})
	})
	r.DELETE("/story/projects/:projectId/memories/:memoryId", func(c *gin.Context) {
		userID, authorized := withUser(c)
		if !authorized {
			return
		}
		if err := svc.DeleteStoryMemory(*userID, c.Param("projectId"), c.Param("memoryId")); err != nil {
			failService(c, err)
			return
		}
		ok(c, gin.H{"id": c.Param("memoryId")})
	})
	r.GET("/story/projects/:projectId/branches", func(c *gin.Context) {
		userID, authorized := withUser(c)
		if !authorized {
			return
		}
		branches, err := svc.StoryBranches(*userID, c.Param("projectId"))
		if err != nil {
			failService(c, err)
			return
		}
		ok(c, gin.H{"branches": branches})
	})
	r.POST("/story/projects/:projectId/chapters/:unitId/branches", func(c *gin.Context) {
		userID, authorized := withUser(c)
		if !authorized {
			return
		}
		var request service.CreateStoryBranchRequest
		if err := c.ShouldBindJSON(&request); err != nil {
			fail(c, http.StatusBadRequest, err)
			return
		}
		branch, err := svc.CreateStoryBranch(*userID, c.Param("projectId"), c.Param("unitId"), request)
		if err != nil {
			failService(c, err)
			return
		}
		ok(c, gin.H{"branch": branch})
	})
	r.GET("/story/projects/:projectId/chapters/:unitId/scenes", func(c *gin.Context) {
		userID, authorized := withUser(c)
		if !authorized {
			return
		}
		scenes, err := svc.StoryScenes(*userID, c.Param("projectId"), c.Param("unitId"))
		if err != nil {
			failService(c, err)
			return
		}
		ok(c, gin.H{"scenes": scenes})
	})
	r.POST("/story/projects/:projectId/chapters/:unitId/scenes", func(c *gin.Context) {
		userID, authorized := withUser(c)
		if !authorized {
			return
		}
		var request service.CreateStorySceneRequest
		if err := c.ShouldBindJSON(&request); err != nil {
			fail(c, http.StatusBadRequest, err)
			return
		}
		scene, err := svc.CreateStoryScene(*userID, c.Param("projectId"), c.Param("unitId"), request)
		if err != nil {
			failService(c, err)
			return
		}
		ok(c, gin.H{"scene": scene})
	})
	r.POST("/story/projects/:projectId/chapters/:unitId/scenes/:sceneId/shots", func(c *gin.Context) {
		userID, authorized := withUser(c)
		if !authorized {
			return
		}
		var request service.CreateStorySceneShotRequest
		if err := c.ShouldBindJSON(&request); err != nil {
			fail(c, http.StatusBadRequest, err)
			return
		}
		shot, err := svc.CreateStorySceneShot(*userID, c.Param("projectId"), c.Param("unitId"), c.Param("sceneId"), request)
		if err != nil {
			failService(c, err)
			return
		}
		ok(c, gin.H{"shot": shot})
	})

	// --- InkOS 书籍管理（BookDetail 对齐）：通过/拒绝回滚/书籍设置/删除书 ---
	r.POST("/story/projects/:projectId/inkos/chapters/:chapterNumber/approve", func(c *gin.Context) {
		userID, okUser := withUser(c)
		if !okUser {
			return
		}
		chapterNumber := 0
		if _, err := fmt.Sscanf(c.Param("chapterNumber"), "%d", &chapterNumber); err != nil {
			failService(c, service.BadAuthRequest("章节号无效"))
			return
		}
		if err := svc.ApproveNovelChapter(c.Request.Context(), *userID, c.Param("projectId"), chapterNumber); err != nil {
			failService(c, err)
			return
		}
		ok(c, gin.H{"ok": true, "chapterNumber": chapterNumber, "status": "approved"})
	})
	r.POST("/story/projects/:projectId/inkos/chapters/:chapterNumber/reject", func(c *gin.Context) {
		userID, okUser := withUser(c)
		if !okUser {
			return
		}
		chapterNumber := 0
		if _, err := fmt.Sscanf(c.Param("chapterNumber"), "%d", &chapterNumber); err != nil {
			failService(c, service.BadAuthRequest("章节号无效"))
			return
		}
		if err := svc.RejectNovelChapter(c.Request.Context(), *userID, c.Param("projectId"), chapterNumber); err != nil {
			failService(c, err)
			return
		}
		ok(c, gin.H{"ok": true, "chapterNumber": chapterNumber, "status": "rejected"})
	})
	r.PUT("/story/projects/:projectId/book-settings", func(c *gin.Context) {
		userID, okUser := withUser(c)
		if !okUser {
			return
		}
		var request service.NovelBookSettingsRequest
		if err := c.ShouldBindJSON(&request); err != nil {
			failService(c, service.BadAuthRequest("书籍设置格式无效"))
			return
		}
		config, err := svc.UpdateNovelBookSettings(c.Request.Context(), *userID, c.Param("projectId"), request)
		if err != nil {
			failService(c, err)
			return
		}
		ok(c, gin.H{"config": config})
	})
	r.POST("/story/projects/:projectId/truth/read", func(c *gin.Context) {
		userID, okUser := withUser(c)
		if !okUser {
			return
		}
		var request service.NovelTruthRequest
		if err := c.ShouldBindJSON(&request); err != nil || request.Path == "" {
			failService(c, service.BadAuthRequest("真相文件路径无效"))
			return
		}
		content, err := svc.ReadNovelTruth(c.Request.Context(), *userID, c.Param("projectId"), request.Path)
		if err != nil {
			failService(c, err)
			return
		}
		ok(c, gin.H{"content": content})
	})
	r.POST("/story/projects/:projectId/truth/write", func(c *gin.Context) {
		userID, okUser := withUser(c)
		if !okUser {
			return
		}
		var request service.NovelTruthWriteRequest
		if err := c.ShouldBindJSON(&request); err != nil || request.Path == "" {
			failService(c, service.BadAuthRequest("真相文件路径无效"))
			return
		}
		if err := svc.WriteNovelTruth(c.Request.Context(), *userID, c.Param("projectId"), request.Path, request.Content); err != nil {
			failService(c, err)
			return
		}
		ok(c, gin.H{"ok": true})
	})
	r.GET("/story/projects/:projectId/exports", func(c *gin.Context) {
		userID, okUser := withUser(c)
		if !okUser {
			return
		}
		files, err := svc.ListNovelExports(c.Request.Context(), *userID, c.Param("projectId"))
		if err != nil {
			failService(c, err)
			return
		}
		ok(c, gin.H{"files": files})
	})
	r.GET("/story/projects/:projectId/exports/content", func(c *gin.Context) {
		userID, okUser := withUser(c)
		if !okUser {
			return
		}
		exportPath := c.Query("path")
		if exportPath == "" {
			failService(c, service.BadAuthRequest("导出文件路径无效"))
			return
		}
		content, err := svc.ReadNovelExport(c.Request.Context(), *userID, c.Param("projectId"), exportPath)
		if err != nil {
			failService(c, err)
			return
		}
		ok(c, gin.H{"content": content, "encoding": "base64"})
	})
	r.POST("/story/projects/:projectId/radar", func(c *gin.Context) {
		user, err := currentUser(c, svc)
		if err != nil {
			failService(c, err)
			return
		}
		var request service.NovelRadarRequest
		if err := c.ShouldBindJSON(&request); err != nil {
			failService(c, service.BadAuthRequest("市场雷达请求无效"))
			return
		}
		result, err := svc.NovelMarketRadar(c.Request.Context(), user.ID, request)
		if err != nil {
			failService(c, err)
			return
		}
		ok(c, gin.H{"radar": result})
	})
	r.POST("/story/projects/:projectId/eval", func(c *gin.Context) {
		userID, okUser := withUser(c)
		if !okUser {
			return
		}
		result, err := svc.EvaluateNovelBook(c.Request.Context(), *userID, c.Param("projectId"))
		if err != nil {
			failService(c, err)
			return
		}
		ok(c, gin.H{"eval": result})
	})
	r.POST("/story/projects/:projectId/detect", func(c *gin.Context) {
		userID, okUser := withUser(c)
		if !okUser {
			return
		}
		var request service.NovelDetectRequest
		if err := c.ShouldBindJSON(&request); err != nil {
			failService(c, service.BadAuthRequest("检测请求无效"))
			return
		}
		result, err := svc.DetectNovelChapter(c.Request.Context(), *userID, c.Param("projectId"), request.ChapterNumber, request.Content)
		if err != nil {
			failService(c, err)
			return
		}
		ok(c, result)
	})
	r.POST("/story/projects/:projectId/delete-inkos-book", func(c *gin.Context) {
		userID, okUser := withUser(c)
		if !okUser {
			return
		}
		if err := svc.DeleteNovelBook(c.Request.Context(), *userID, c.Param("projectId")); err != nil {
			failService(c, err)
			return
		}
		ok(c, gin.H{"ok": true})
	})
}
