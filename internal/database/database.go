package database

import (
	_ "embed"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/BigGoose-dae/GooseCanvas/internal/domain"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

//go:embed sql/001_seed_models.sql
var seedModelsSQL string

func Open(path string) (*gorm.DB, error) {
	// Create the file privately before SQLite creates its WAL/SHM sidecars.
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	if err := file.Chmod(0o600); err != nil {
		file.Close()
		return nil, err
	}
	if err := file.Close(); err != nil {
		return nil, err
	}
	for _, suffix := range []string{"-wal", "-shm"} {
		if err := os.Chmod(path+suffix, 0o600); err != nil && !os.IsNotExist(err) {
			return nil, err
		}
	}

	dbLogger := logger.New(log.New(os.Stdout, "", log.LstdFlags), logger.Config{
		SlowThreshold:             time.Second,
		LogLevel:                  logger.Error,
		IgnoreRecordNotFoundError: true,
		ParameterizedQueries:      true,
		Colorful:                  true,
	})
	db, err := gorm.Open(sqlite.Open(path+"?_busy_timeout=5000&_journal_mode=WAL&_foreign_keys=on&_txlock=immediate&_secure_delete=on"), &gorm.Config{Logger: dbLogger})
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxOpenConns(1)
	if err := os.Chmod(path, 0o600); err != nil {
		return nil, err
	}
	hadNodeContent := db.Migrator().HasColumn(&domain.Node{}, "Content")
	if err := db.AutoMigrate(
		&domain.Workspace{}, &domain.Node{}, &domain.Edge{}, &domain.Asset{},
		&domain.NodeVersion{}, &domain.GenerationSession{}, &domain.GenerationInput{}, &domain.GenerationTask{},
		&domain.ModelDefinition{}, &domain.SystemSetting{},
	); err != nil {
		return nil, fmt.Errorf("migrate sqlite: %w", err)
	}
	if !hadNodeContent {
		if err := db.Transaction(func(tx *gorm.DB) error {
			if err := tx.Exec("UPDATE nodes SET content = prompt, prompt = '' WHERE node_type = 'text'").Error; err != nil {
				return err
			}
			if err := tx.Exec("UPDATE node_versions SET content = prompt, prompt = '' WHERE node_id IN (SELECT id FROM nodes WHERE node_type = 'text')").Error; err != nil {
				return err
			}
			return tx.Exec("UPDATE generation_sessions SET node_content = node_prompt, node_prompt = '' WHERE task_type = 'text'").Error
		}); err != nil {
			return nil, fmt.Errorf("migrate text node content: %w", err)
		}
	}
	if err := db.Exec(seedModelsSQL).Error; err != nil {
		return nil, fmt.Errorf("seed models: %w", err)
	}
	return db, nil
}
