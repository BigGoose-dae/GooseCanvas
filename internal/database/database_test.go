package database

import (
	"path/filepath"
	"testing"

	"github.com/BigGoose-dae/GooseCanvas/internal/domain"
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
		if count != 3 {
			t.Fatalf("attempt %d: builtin model count = %d, want 3", attempt+1, count)
		}
		sqlDB, err := db.DB()
		if err != nil {
			t.Fatal(err)
		}
		_ = sqlDB.Close()
	}
}
