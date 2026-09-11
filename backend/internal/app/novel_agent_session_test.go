package app

import (
	"testing"
	"time"

	"infinite-canvas/backend/internal/model"
	"infinite-canvas/backend/internal/repository"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestNovelAgentSessionViewRestoresSkillsAndConfirmation(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.StoryAgentSession{}, &model.StoryAgentMessage{}); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	session := model.StoryAgentSession{ID: "session-1", UserID: "user-1", Title: "雨夜档案", Mode: "inkos-long-writing", Status: "active", CreatedAt: now, UpdatedAt: now}
	if err := db.Create(&session).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.StoryAgentMessage{
		ID: "message-1", SessionID: session.ID, Role: "assistant", Content: "请确认建书", SkillsJSON: `[{"id":"inkos-long-writing","name":"长篇小说","description":"","source":"inkos"}]`, ConfirmationJSON: `{"action":"create_book","title":"确认创建","summary":"建立小说基础设定","instruction":"执行建书","requestedSkills":["inkos-long-writing"],"actionPayload":{"title":"雨夜档案"}}`, CreatedAt: now,
	}).Error; err != nil {
		t.Fatal(err)
	}

	view, err := (&Service{repo: repository.New(db)}).NovelAgentSessionView("user-1", session.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(view.Messages) != 1 || len(view.Messages[0].Skills) != 1 || view.Messages[0].Confirmation == nil || view.Messages[0].Confirmation.Action != "create_book" {
		t.Fatalf("restored session view = %#v", view)
	}
	if err := (&Service{repo: repository.New(db)}).repo.ConsumeStoryAgentConfirmations("user-1", session.ID); err != nil {
		t.Fatal(err)
	}
	view, err = (&Service{repo: repository.New(db)}).NovelAgentSessionView("user-1", session.ID)
	if err != nil {
		t.Fatal(err)
	}
	if view.Messages[0].Confirmation != nil {
		t.Fatal("accepted confirmation must not reappear after session restore")
	}
}
