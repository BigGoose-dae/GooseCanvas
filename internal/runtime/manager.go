package runtime

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"path/filepath"
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
	Provider provider.Provider
}

// Snapshots are immutable. Each dispatched operation retains its own clients.
type Manager struct {
	mu      sync.RWMutex
	db      *gorm.DB
	current Snapshot
}

func New(db *gorm.DB, cfg config.Config) (*Manager, error) {
	if strings.TrimSpace(cfg.Volc.AudioEndpoint) == "" {
		cfg.Volc.AudioEndpoint = config.DefaultAudioEndpoint
	}
	var row domain.SystemSetting
	err := db.First(&row, 1).Error
	legacy := err == nil && !strings.HasPrefix(row.Value, vaultPrefix)
	if err == nil {
		// Server bind address and data directory remain process bootstrap settings.
		boot := cfg
		raw := []byte(row.Value)
		if !legacy {
			raw, err = openSettings(boot.DataDir, row.Value)
			if err != nil {
				return nil, err
			}
		}
		if err := json.Unmarshal(raw, &cfg); err != nil {
			return nil, err
		}
		cfg.Addr, cfg.DataDir, cfg.Database = boot.Addr, boot.DataDir, boot.Database
		if strings.TrimSpace(cfg.Volc.AudioEndpoint) == "" {
			cfg.Volc.AudioEndpoint = boot.Volc.AudioEndpoint
		}
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	snap, err := build(cfg)
	if err != nil {
		return nil, err
	}
	manager := &Manager{db: db, current: snap}
	if legacy {
		if err := manager.Update(func(*config.Config) error { return nil }); err != nil {
			return nil, err
		}
		// Flush the secure-delete update to the main database and remove plaintext
		// WAL pages before serving requests. Existing external backups are unaffected.
		var checkpoint struct{ Busy int }
		if err := db.Raw("PRAGMA wal_checkpoint(TRUNCATE)").Scan(&checkpoint).Error; err != nil {
			return nil, err
		}
		if checkpoint.Busy != 0 {
			return nil, fmt.Errorf("旧配置迁移需要独占数据库，请停止其他使用此数据目录的实例")
		}
	}
	return manager, nil
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
	encrypted, err := sealSettings(cfg.DataDir, raw)
	if err != nil {
		return err
	}
	if err := m.db.Save(&domain.SystemSetting{ID: 1, Value: encrypted}).Error; err != nil {
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
	if (strings.TrimSpace(cfg.Volc.AudioAppID) == "") != (strings.TrimSpace(cfg.Volc.AudioAccessKey) == "") {
		return Snapshot{}, fmt.Errorf("豆包语音 App ID 与 Access Key 必须同时配置")
	}
	for _, value := range []string{cfg.Volc.BaseURL, cfg.Volc.AudioEndpoint} {
		parsed, err := url.Parse(value)
		if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
			return Snapshot{}, fmt.Errorf("服务地址必须是有效的 HTTP 或 HTTPS 地址")
		}
	}
	for _, value := range []string{cfg.Volc.BaseURL, cfg.Volc.AudioEndpoint} {
		parsed, _ := url.Parse(value)
		if parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
			return Snapshot{}, fmt.Errorf("服务地址不能包含凭证、查询参数或片段")
		}
		host := parsed.Hostname()
		local := host == "localhost" || net.ParseIP(host).IsLoopback()
		if parsed.Scheme == "http" && !local {
			return Snapshot{}, fmt.Errorf("远程服务请使用 HTTPS，以保护传输中的凭证")
		}
	}
	if strings.TrimSpace(cfg.AppName) == "" {
		return Snapshot{}, fmt.Errorf("应用名称不能为空")
	}
	store, err := storage.NewLocal(filepath.Join(cfg.DataDir, "assets"))
	if err != nil {
		return Snapshot{}, err
	}
	return Snapshot{Config: cfg, Store: store, Provider: provider.NewVolcengine(cfg.Volc)}, nil
}
