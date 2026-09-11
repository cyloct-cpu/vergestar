package handler

import (
	"io"
	"net/http"

	"infinite-canvas/backend/internal/service"

	"github.com/gin-gonic/gin"
)

// RegisterNovelAgentJobStream proxies the Novel Agent bridge SSE stream for a
// running job to the browser. The bridge owns event emission; this handler is
// a pass-through with flushing so token deltas arrive unbuffered. Job state
// truth remains the polling endpoint — the stream is additive UX.
func RegisterNovelAgentJobStream(r *gin.RouterGroup, svc *service.Service) {
	r.GET("/novel-agent/jobs/:jobId/stream", func(c *gin.Context) {
		user, err := currentUser(c, svc)
		if err != nil {
			failService(c, err)
			return
		}
		jobID := c.Param("jobId")
		if jobID == "" || len(jobID) > 80 {
			failService(c, service.BadAuthRequest("任务 ID 无效"))
			return
		}
		stream, cancel, err := svc.StreamNovelAgentJob(c.Request.Context(), user.ID, jobID)
		if err != nil {
			failService(c, err)
			return
		}
		defer stream.Close()
		defer cancel()

		c.Header("Content-Type", "text/event-stream; charset=utf-8")
		c.Header("Cache-Control", "no-cache")
		c.Header("X-Accel-Buffering", "no")
		c.Writer.WriteHeader(http.StatusOK)
		flusher, ok := c.Writer.(http.Flusher)
		if !ok {
			return
		}
		flusher.Flush()
		buf := make([]byte, 4096)
		for {
			n, readErr := stream.Read(buf)
			if n > 0 {
				if _, writeErr := c.Writer.Write(buf[:n]); writeErr != nil {
					return
				}
				flusher.Flush()
			}
			if readErr != nil {
				if readErr != io.EOF {
					// 上游中断：正常 SSE 关闭语义，前端以轮询兜底。
				}
				return
			}
			select {
			case <-c.Request.Context().Done():
				return
			default:
			}
		}
	})
}
