package handler

import (
	"net/http"
	"strconv"

	"infinite-canvas/backend/internal/service"

	"github.com/gin-gonic/gin"
)

// RegisterNovelAgentRoutes makes the vendored InkOS Skill catalog available
// through Vergestar's existing authenticated API boundary. The browser never
// sees the internal Novel Agent endpoint or its access token.
func RegisterNovelAgentRoutes(r *gin.RouterGroup, svc *service.Service) {
	r.GET("/novel-agent/sessions/search", func(c *gin.Context) {
		user, err := currentUser(c, svc)
		if err != nil {
			failService(c, err)
			return
		}
		page := 1
		if v, err := strconv.Atoi(c.DefaultQuery("page", "1")); err == nil {
			page = v
		}
		pageSize := 20
		if v, err := strconv.Atoi(c.DefaultQuery("page_size", "20")); err == nil {
			pageSize = v
		}
		result, err := svc.SearchNovelAgentSessions(c.Request.Context(), user.ID, c.Query("q"), page, pageSize)
		if err != nil {
			failService(c, err)
			return
		}
		ok(c, result)
	})
	r.GET("/novel-agent/sessions", func(c *gin.Context) {
		user, err := currentUser(c, svc)
		if err != nil {
			failService(c, err)
			return
		}
		sessions, err := svc.ListNovelAgentSessions(user.ID)
		if err != nil {
			failService(c, err)
			return
		}
		ok(c, gin.H{"sessions": sessions})
	})
	r.GET("/novel-agent/sessions/:sessionId/messages", func(c *gin.Context) {
		user, err := currentUser(c, svc)
		if err != nil {
			failService(c, err)
			return
		}
		view, err := svc.NovelAgentSessionView(user.ID, c.Param("sessionId"))
		if err != nil {
			if service.IsProjectNotFound(err) {
				fail(c, http.StatusNotFound, err)
				return
			}
			failService(c, err)
			return
		}
		ok(c, view)
	})
	r.GET("/novel-agent/skills", func(c *gin.Context) {
		if _, err := currentUser(c, svc); err != nil {
			failService(c, err)
			return
		}
		skills, err := svc.NovelAgentSkills(c.Request.Context())
		if err != nil {
			failService(c, err)
			return
		}
		ok(c, gin.H{"skills": skills})
	})
	r.POST("/novel-agent/turn", func(c *gin.Context) {
		user, err := currentUser(c, svc)
		if err != nil {
			failService(c, err)
			return
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 128<<10)
		var request service.NovelAgentTurnRequest
		if err := c.ShouldBindJSON(&request); err != nil {
			fail(c, http.StatusBadRequest, err)
			return
		}
		result, err := svc.RunNovelAgentTurn(c.Request.Context(), user.ID, request)
		if err != nil {
			failService(c, err)
			return
		}
		ok(c, result)
	})
	r.POST("/novel-agent/jobs", func(c *gin.Context) {
		user, err := currentUser(c, svc)
		if err != nil {
			failService(c, err)
			return
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 128<<10)
		var request service.NovelAgentTurnRequest
		if err := c.ShouldBindJSON(&request); err != nil {
			fail(c, http.StatusBadRequest, err)
			return
		}
		job, err := svc.StartNovelAgentJob(c.Request.Context(), user.ID, request)
		if err != nil {
			failService(c, err)
			return
		}
		c.JSON(http.StatusAccepted, gin.H{"code": 0, "data": gin.H{"job": job}, "msg": "accepted"})
	})
	r.POST("/novel-agent/jobs/:jobId/cancel", func(c *gin.Context) {
		user, err := currentUser(c, svc)
		if err != nil {
			failService(c, err)
			return
		}
		if err := svc.CancelNovelAgentJob(c.Request.Context(), user.ID, c.Param("jobId")); err != nil {
			failService(c, err)
			return
		}
		ok(c, gin.H{"ok": true})
	})
	r.POST("/novel-agent/sessions/:sessionId/play-state", func(c *gin.Context) {
		user, err := currentUser(c, svc)
		if err != nil {
			failService(c, err)
			return
		}
		result, err := svc.ReadNovelPlayState(c.Request.Context(), user.ID, c.Param("sessionId"))
		if err != nil {
			failService(c, err)
			return
		}
		ok(c, result)
	})
	r.GET("/novel-agent/jobs/history", func(c *gin.Context) {
		user, err := currentUser(c, svc)
		if err != nil {
			failService(c, err)
			return
		}
		jobs, err := svc.NovelAgentJobHistory(c.Request.Context(), user.ID, 30)
		if err != nil {
			failService(c, err)
			return
		}
		ok(c, gin.H{"jobs": jobs})
	})
	r.GET("/novel-agent/jobs/active", func(c *gin.Context) {
		user, err := currentUser(c, svc)
		if err != nil {
			failService(c, err)
			return
		}
		jobs, err := svc.NovelAgentActiveJobs(c.Request.Context(), user.ID)
		if err != nil {
			failService(c, err)
			return
		}
		ok(c, gin.H{"jobs": jobs})
	})
	r.PUT("/novel-agent/sessions/:sessionId", func(c *gin.Context) {
		user, err := currentUser(c, svc)
		if err != nil {
			failService(c, err)
			return
		}
		var body struct {
			Title string `json:"title"`
		}
		if err := c.ShouldBindJSON(&body); err != nil || body.Title == "" {
			failService(c, service.BadAuthRequest("会话标题不能为空"))
			return
		}
		if err := svc.RenameNovelAgentSession(c.Request.Context(), user.ID, c.Param("sessionId"), body.Title); err != nil {
			failService(c, err)
			return
		}
		ok(c, gin.H{"ok": true})
	})
	r.DELETE("/novel-agent/sessions/:sessionId", func(c *gin.Context) {
		user, err := currentUser(c, svc)
		if err != nil {
			failService(c, err)
			return
		}
		if err := svc.DeleteNovelAgentSession(c.Request.Context(), user.ID, c.Param("sessionId")); err != nil {
			failService(c, err)
			return
		}
		ok(c, gin.H{"ok": true})
	})
	r.GET("/novel-agent/jobs/:jobId", func(c *gin.Context) {
		user, err := currentUser(c, svc)
		if err != nil {
			failService(c, err)
			return
		}
		job, err := svc.NovelAgentJob(c.Request.Context(), user.ID, c.Param("jobId"))
		if err != nil {
			failService(c, err)
			return
		}
		ok(c, gin.H{"job": job})
	})
}
