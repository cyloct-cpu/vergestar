package app

import (
	"context"
	"strings"
)

// Play 游玩 HUD（T5 富 UI）：读取世界状态/场景投影与建议行动。
// 桥接 Bridge 的 POST /agent/play/state；数据真相在 Bridge 侧 PlayStore。

type NovelPlayStateRequest struct {
	SessionID string `json:"sessionId"`
}

func (s *Service) ReadNovelPlayState(ctx context.Context, userID, sessionID string) (map[string]any, error) {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" || len(sessionID) > 120 {
		return nil, BadAuthRequest("会话 ID 无效")
	}
	var body map[string]any
	if err := s.novelBridgeCall(ctx, userID, "/agent/play/state", map[string]any{"userId": userID, "sessionId": sessionID}, &body); err != nil {
		return nil, err
	}
	return body, nil
}
