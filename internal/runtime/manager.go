package runtime

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/BigGoose-dae/GooseCanvas/internal/config"
	"github.com/BigGoose-dae/GooseCanvas/internal/domain"
	"github.com/BigGoose-dae/GooseCanvas/internal/provider"
	"github.com/BigGoose-dae/GooseCanvas/internal/storage"
	"gorm.io/gorm"
)

type Snapshot struct {
	Config   config.Config
	Store    storage.Store
	TOS      *storage.TOSStore
	Provider provider.Provider
}

// Snapshots are immutable. Each dispatched operation retains its own clients.
type Manager struct {
	mu      sync.RWMutex
	db      *gorm.DB
	current Snapshot
}

func New(db *gorm.DB, cfg config.Config) (*Manager, error) {
	var row domain.SystemSetting
	err := db.First(&row, 1).Error
	if err == nil {
		// Server bind address and data directory remain process bootstrap settings.
		boot := cfg
		if err := json.Unmarshal([]byte(row.Value), &cfg); err != nil {
			return nil, err
		}
		cfg.Addr, cfg.DataDir, cfg.Database = boot.Addr, boot.DataDir, boot.Database
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	snap, err := build(cfg)
	if err != nil {
		return nil, err
	}
	return &Manager{db: db, current: snap}, nil
}

func (m *Manager) Snapshot() Snapshot { m.mu.RLock(); defer m.mu.RUnlock(); return m.current }

func (m *Manager) Update(change func(*config.Config) error) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	cfg := m.current.Config
	if err := change(&cfg); err != nil {
		return err
	}
	snap, err := build(cfg)
	if err != nil {
		return err
	}
	raw, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	if err := m.db.Save(&domain.SystemSetting{ID: 1, Value: string(raw)}).Error; err != nil {
		return err
	}
	m.current = snap
	return nil
}

func build(cfg config.Config) (Snapshot, error) {
	if cfg.Worker.Concurrency < 1 || cfg.Worker.Concurrency > 32 {
		return Snapshot{}, fmt.Errorf("并发数应为 1–32")
	}
	if cfg.Worker.PollInterval <= 0 || cfg.Worker.TaskTimeout < time.Minute {
		return Snapshot{}, fmt.Errorf("轮询间隔必须为正数，任务超时至少 1 分钟")
	}
	for _, value := range []string{cfg.Volc.BaseURL, cfg.TOS.Endpoint} {
		parsed, err := url.Parse(value)
		if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
			return Snapshot{}, fmt.Errorf("服务地址必须是有效的 HTTP 或 HTTPS 地址")
		}
	}
	if strings.TrimSpace(cfg.AppName) == "" {
		return Snapshot{}, fmt.Errorf("应用名称不能为空")
	}
	snap := Snapshot{Config: cfg, Provider: provider.NewVolcengine(cfg.Volc)}
	if len(cfg.StorageMissing()) == 0 {
		store, err := storage.NewTOS(cfg.TOS)
		if err != nil {
			return Snapshot{}, err
		}
		snap.Store, snap.TOS = store, store
	}
	return snap, nil
}
