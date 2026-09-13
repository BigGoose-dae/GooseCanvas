package database

import (
	"path/filepath"
	"testing"

	"github.com/BigGoose-dae/GooseCanvas/internal/domain"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestOpenSeedsBuiltinModelsIdempotently(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	for attempt := 0; attempt < 2; attempt++ {
		db, err := Open(path)
		if err != nil {
			t.Fatal(err)
		}
		var count int64
		if err := db.Model(&domain.ModelDefinition{}).Where("builtin=? AND deleted_at IS NULL", true).Count(&count).Error; err != nil {
			t.Fatal(err)
		}
		if count != 4 {
			t.Fatalf("attempt %d: builtin model count = %d, want 4", attempt+1, count)
		}
		sqlDB, err := db.DB()
		if err != nil {
			t.Fatal(err)
		}
		_ = sqlDB.Close()
	}
}

func TestOpenMigratesLegacyTextPromptToContentOnce(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy.db")
	legacy, err := gorm.Open(sqlite.Open(path), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	statements := []string{
		`CREATE TABLE nodes (id integer primary key, workspace_id integer not null, node_type text not null, title text, prompt text, model_key text, params text, pos_x real, pos_y real, width real, height real, current_asset_id integer, version integer not null default 1, created_at datetime, updated_at datetime, deleted_at datetime)`,
		`CREATE TABLE node_versions (id integer primary key, node_id integer not null, version integer not null, asset_id integer, prompt text, model_key text, params text, created_at datetime)`,
		`CREATE TABLE generation_sessions (id integer primary key, workspace_id integer not null, node_id integer not null, task_type text not null, model_key text not null, prompt text, node_prompt text, params text, status text not null, error_message text, result_asset_id integer, result_text text, created_at datetime, updated_at datetime, started_at datetime, finished_at datetime)`,
		`INSERT INTO nodes (id, workspace_id, node_type, prompt, version) VALUES (1, 1, 'text', 'legacy body', 1)`,
		`INSERT INTO node_versions (id, node_id, version, prompt) VALUES (1, 1, 1, 'legacy body')`,
		`INSERT INTO generation_sessions (id, workspace_id, node_id, task_type, model_key, prompt, node_prompt, status) VALUES (1, 1, 1, 'text', 'model', 'legacy body', 'legacy body', 'succeeded')`,
	}
	for _, statement := range statements {
		if err := legacy.Exec(statement).Error; err != nil {
			t.Fatal(err)
		}
	}
	legacySQL, _ := legacy.DB()
	_ = legacySQL.Close()

	db, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	var node domain.Node
	var version domain.NodeVersion
	var session domain.GenerationSession
	db.First(&node, 1)
	db.First(&version, 1)
	db.First(&session, 1)
	if node.Prompt != "" || node.Content != "legacy body" || version.Prompt != "" || version.Content != "legacy body" || session.NodePrompt != "" || session.NodeContent != "legacy body" {
		t.Fatalf("legacy fields were not migrated: node=%+v version=%+v session=%+v", node, version, session)
	}
	if err := db.Model(&node).Updates(map[string]any{"prompt": "new instruction", "content": "new body"}).Error; err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	_ = sqlDB.Close()

	db, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { sqlDB, _ := db.DB(); _ = sqlDB.Close() }()
	if err := db.First(&node, 1).Error; err != nil {
		t.Fatal(err)
	}
	if node.Prompt != "new instruction" || node.Content != "new body" {
		t.Fatalf("migration ran more than once: %+v", node)
	}
}
