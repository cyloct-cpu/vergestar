package database

import (
	"strings"
	"testing"
	"time"

	"infinite-canvas/backend/internal/model"

	"gorm.io/gorm"
)

// vergestar 分支迁移编号：Story 域 v4-v12，上游 v1.2.4-v1.2.9 顺延为 v13-v19。
// 这里模拟一个“旧版本分支二进制”创建的数据库：只应用了 Story 迁移（v1-v12），
// 上游部分的 v13-v19 尚未应用，升级后必须按新编号补齐并保留既有记录。
func storyOnlyMigratedDatabase(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := Open(Config{Driver: "sqlite", DSN: "file:" + t.Name() + "?mode=memory&cache=shared"})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := db.AutoMigrate(&schemaMigration{}); err != nil {
		t.Fatal(err)
	}
	for _, item := range schemaMigrations {
		if item.version > 12 {
			break
		}
		if err := item.apply(db); err != nil {
			t.Fatal(err)
		}
		if err := db.Create(&schemaMigration{Version: item.version, Name: item.name, Checksum: item.checksum, AppliedAt: time.Date(2026, 9, 8, 0, 0, 0, 0, time.UTC)}).Error; err != nil {
			t.Fatal(err)
		}
	}
	// 基线迁移按当前模型结构建表，会直接带上 v15 的播放副本列；删掉以模拟旧库。
	for _, column := range []string{"playback_status", "playback_object_key", "playback_error"} {
		if db.Migrator().HasColumn(&model.Resource{}, column) {
			if err := db.Migrator().DropColumn(&model.Resource{}, column); err != nil {
				t.Fatal(err)
			}
		}
	}
	return db
}

func TestMigrateSchemaAppliesUpstreamMigrationsAfterStoryBlock(t *testing.T) {
	db := storyOnlyMigratedDatabase(t)
	var before schemaMigration
	if err := db.First(&before, "version = 12").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO asset_folders (id, user_id, name) VALUES ('kept-folder', 'owner', 'Keep me')").Error; err != nil {
		t.Fatal(err)
	}
	for attempt := 0; attempt < 2; attempt++ {
		if err := MigrateSchema(db); err != nil {
			t.Fatal(err)
		}
		if err := RequireSchemaVersion(db); err != nil {
			t.Fatal(err)
		}
	}
	var after schemaMigration
	if err := db.First(&after, "version = 12").Error; err != nil {
		t.Fatal(err)
	}
	if before.Name != after.Name || before.Checksum != after.Checksum || !before.AppliedAt.Equal(after.AppliedAt) {
		t.Fatalf("historical record changed: before=%+v after=%+v", before, after)
	}
	var playback schemaMigration
	if err := db.First(&playback, "version = 15").Error; err != nil {
		t.Fatal(err)
	}
	if playback.Name != "resource_playback_variant" || playback.Checksum != resourcePlaybackChecksum {
		t.Fatalf("unexpected playback migration: %+v", playback)
	}
	var folders schemaMigration
	if err := db.First(&folders, "version = 16").Error; err != nil {
		t.Fatal(err)
	}
	if folders.Name != "asset_library_folders" || folders.Checksum != assetLibraryFoldersChecksum {
		t.Fatalf("unexpected folders migration: %+v", folders)
	}
	for _, column := range []string{"playback_status", "playback_object_key", "playback_error"} {
		if !db.Migrator().HasColumn(&model.Resource{}, column) {
			t.Fatalf("missing playback column %s", column)
		}
	}
	var name string
	if err := db.Raw("SELECT name FROM asset_folders WHERE id = 'kept-folder'").Scan(&name).Error; err != nil || name != "Keep me" {
		t.Fatalf("folder data changed: %q %v", name, err)
	}
}

func TestMigrateSchemaRejectsUnknownStoryLineage(t *testing.T) {
	for _, scenario := range []string{"checksum", "name", "extra-story-version"} {
		t.Run(scenario, func(t *testing.T) {
			db := storyOnlyMigratedDatabase(t)
			switch scenario {
			case "checksum":
				if err := db.Model(&schemaMigration{}).Where("version = 4").Update("checksum", "unknown").Error; err != nil {
					t.Fatal(err)
				}
			case "name":
				if err := db.Model(&schemaMigration{}).Where("version = 4").Update("name", "unknown").Error; err != nil {
					t.Fatal(err)
				}
			case "extra-story-version":
				if err := db.Create(&schemaMigration{Version: 13, Name: "story_foundation_agent_session", Checksum: storyFoundationAgentSessionChecksum, AppliedAt: time.Now().UTC()}).Error; err != nil {
					t.Fatal(err)
				}
			}
			if err := MigrateSchema(db); err == nil || !strings.Contains(err.Error(), "不一致") {
				t.Fatalf("expected lineage rejection, got %v", err)
			}
			if db.Migrator().HasColumn(&model.Resource{}, "playback_status") {
				t.Fatal("rejected migration changed resource schema")
			}
		})
	}
}
