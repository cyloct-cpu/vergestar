package service

import (
	"context"
)

// --- T2：真相文件读写 / 导出回传 / 质量工具 ---
// 本文件的桥接调用复用 novel_book_management.go 中的 novelBridgeCall 与 foundationForProject。

func (s *Service) ReadNovelTruth(ctx context.Context, userID, projectID, truthPath string) (string, error) {
	foundation, err := s.foundationForProject(projectID)
	if err != nil {
		return "", err
	}
	var body struct {
		Content string `json:"content"`
	}
	if err := s.novelBridgeCall(ctx, userID, "/agent/books/truth/read", map[string]any{"userId": userID, "sessionId": foundation.AgentSessionID, "bookId": foundation.InkosBookID, "path": truthPath}, &body); err != nil {
		return "", err
	}
	return body.Content, nil
}

func (s *Service) WriteNovelTruth(ctx context.Context, userID, projectID, truthPath, content string) error {
	foundation, err := s.foundationForProject(projectID)
	if err != nil {
		return err
	}
	var body struct {
		Ok bool `json:"ok"`
	}
	if err := s.novelBridgeCall(ctx, userID, "/agent/books/truth/write", map[string]any{"userId": userID, "sessionId": foundation.AgentSessionID, "bookId": foundation.InkosBookID, "path": truthPath, "content": content}, &body); err != nil {
		return err
	}
	return nil
}

func (s *Service) ListNovelExports(ctx context.Context, userID, projectID string) ([]map[string]any, error) {
	foundation, err := s.foundationForProject(projectID)
	if err != nil {
		return nil, err
	}
	var body struct {
		Files []map[string]any `json:"files"`
	}
	if err := s.novelBridgeCall(ctx, userID, "/agent/books/exports/list", map[string]any{"userId": userID, "sessionId": foundation.AgentSessionID}, &body); err != nil {
		return nil, err
	}
	return body.Files, nil
}

func (s *Service) ReadNovelExport(ctx context.Context, userID, projectID, exportPath string) (string, error) {
	foundation, err := s.foundationForProject(projectID)
	if err != nil {
		return "", err
	}
	var body struct {
		Content  string `json:"content"`
		Encoding string `json:"encoding"`
	}
	if err := s.novelBridgeCall(ctx, userID, "/agent/books/exports/read", map[string]any{"userId": userID, "sessionId": foundation.AgentSessionID, "path": exportPath}, &body); err != nil {
		return "", err
	}
	return body.Content, nil
}

func (s *Service) EvaluateNovelBook(ctx context.Context, userID, projectID string) (map[string]any, error) {
	foundation, err := s.foundationForProject(projectID)
	if err != nil {
		return nil, err
	}
	var body struct {
		Eval map[string]any `json:"eval"`
	}
	if err := s.novelBridgeCall(ctx, userID, "/agent/books/eval", map[string]any{"userId": userID, "sessionId": foundation.AgentSessionID, "bookId": foundation.InkosBookID}, &body); err != nil {
		return nil, err
	}
	return body.Eval, nil
}

func (s *Service) DetectNovelChapter(ctx context.Context, userID, projectID string, chapterNumber int, content string) (map[string]any, error) {
	foundation, err := s.foundationForProject(projectID)
	if err != nil {
		return nil, err
	}
	var body map[string]any
	if err := s.novelBridgeCall(ctx, userID, "/agent/books/detect", map[string]any{"userId": userID, "sessionId": foundation.AgentSessionID, "bookId": foundation.InkosBookID, "chapterNumber": chapterNumber, "content": content}, &body); err != nil {
		return nil, err
	}
	return body, nil
}
