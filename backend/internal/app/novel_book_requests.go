package app

import "context"

// T2 请求体定义。

type NovelTruthRequest struct {
	Path string `json:"path"`
}

type NovelTruthWriteRequest struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

type NovelDetectRequest struct {
	ChapterNumber int    `json:"chapterNumber"`
	Content       string `json:"content"`
}

var _ = context.Background
