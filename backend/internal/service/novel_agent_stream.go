package service

import (
	"bufio"
	"context"
	"io"
	"net/http"
	"strings"
)

// streamReader wraps the bridge SSE body with Close.
type streamReader struct {
	io.ReadCloser
}

// StreamNovelAgentJob opens the bridge SSE stream for a job and returns the
// raw body for pass-through streaming. Authentication headers mirror the
// polling path; the caller must Close the reader and the cancel func.
func (s *Service) StreamNovelAgentJob(ctx context.Context, userID, jobID string) (io.ReadCloser, context.CancelFunc, error) {
	if s.novelAgent == nil {
		return nil, nil, NewAppError(http.StatusServiceUnavailable, "小说 Agent 尚未配置")
	}
	jobID = strings.TrimSpace(jobID)
	if jobID == "" || len(jobID) > 80 {
		return nil, nil, BadAuthRequest("任务 ID 无效")
	}
	// SSE 长连接：不复用 45 分钟超时的轮询 client，单独用带取消的请求。
	streamCtx, cancel := context.WithCancel(ctx)
	req, err := http.NewRequestWithContext(streamCtx, http.MethodGet, s.novelAgent.endpoint+"/agent/jobs/"+jobID+"/stream", nil)
	if err != nil {
		cancel()
		return nil, nil, WrapAppError(http.StatusInternalServerError, "创建流式请求失败", err)
	}
	req.Header.Set("accept", "text/event-stream")
	req.Header.Set("x-vergestar-novel-agent-token", s.novelAgent.token)
	req.Header.Set("x-vergestar-novel-agent-user", userID)
	resp, err := s.novelAgent.http.Do(req)
	if err != nil {
		cancel()
		return nil, nil, WrapAppError(http.StatusServiceUnavailable, "小说 Agent 暂时不可用", err)
	}
	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close()
		cancel()
		return nil, nil, NewAppError(http.StatusBadGateway, "任务流不可用（任务可能已结束）")
	}
	reader := bufio.NewReaderSize(resp.Body, 8192)
	return struct {
		io.Reader
		io.Closer
	}{reader, resp.Body}, cancel, nil
}
