package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	AppName  string
	LogoURL  string
	RepoURL  string
	Addr     string
	DataDir  string
	Database string
	Volc     Volcengine
	TOS      TOS
	Worker   Worker
}

type Volcengine struct {
	APIKey  string
	BaseURL string
}

type TOS struct {
	Endpoint   string
	Region     string
	Bucket     string
	AccessKey  string
	SecretKey  string
	Prefix     string
	PresignTTL time.Duration
}

type Worker struct {
	PollInterval time.Duration
	TaskTimeout  time.Duration
	Concurrency  int
}

func Load() (Config, error) {
	_ = godotenv.Load()
	dataDir := env("DATA_DIR", "./data")
	presignTTL, err := duration("TOS_PRESIGN_TTL", 24*time.Hour)
	if err != nil {
		return Config{}, err
	}
	pollInterval, err := duration("WORKER_POLL_INTERVAL", 3*time.Second)
	if err != nil {
		return Config{}, err
	}
	taskTimeout, err := duration("WORKER_TASK_TIMEOUT", 30*time.Minute)
	if err != nil {
		return Config{}, err
	}

	cfg := Config{
		AppName:  env("APP_NAME", "Goose Canvas"),
		LogoURL:  strings.TrimSpace(os.Getenv("APP_LOGO_URL")),
		RepoURL:  env("APP_REPOSITORY_URL", "https://github.com/BigGoose-dae/GooseCanvas"),
		Addr:     env("APP_ADDR", "127.0.0.1:8080"),
		DataDir:  dataDir,
		Database: filepath.Join(dataDir, "goose-canvas.db"),
		Volc: Volcengine{
			APIKey:  strings.TrimSpace(os.Getenv("VOLCENGINE_API_KEY")),
			BaseURL: strings.TrimRight(env("VOLCENGINE_BASE_URL", "https://ark.cn-beijing.volces.com"), "/"),
		},
		TOS: TOS{
			Endpoint:   env("TOS_ENDPOINT", "https://tos-cn-beijing.volces.com"),
			Region:     env("TOS_REGION", "cn-beijing"),
			Bucket:     strings.TrimSpace(os.Getenv("TOS_BUCKET")),
			AccessKey:  strings.TrimSpace(os.Getenv("TOS_ACCESS_KEY")),
			SecretKey:  strings.TrimSpace(os.Getenv("TOS_SECRET_KEY")),
			Prefix:     strings.Trim(env("TOS_PREFIX", "goose-canvas"), "/"),
			PresignTTL: presignTTL,
		},
		Worker: Worker{PollInterval: pollInterval, TaskTimeout: taskTimeout, Concurrency: 4},
	}
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return Config{}, fmt.Errorf("create data directory: %w", err)
	}
	return cfg, nil
}

func (c Config) Missing() []string {
	return append(c.ModelMissing(), c.StorageMissing()...)
}

func (c Config) ModelMissing() []string {
	if c.Volc.APIKey == "" {
		return []string{"VOLCENGINE_API_KEY"}
	}
	return nil
}

func (c Config) StorageMissing() []string {
	checks := []struct {
		key   string
		value string
	}{
		{"TOS_BUCKET", c.TOS.Bucket},
		{"TOS_ACCESS_KEY", c.TOS.AccessKey},
		{"TOS_SECRET_KEY", c.TOS.SecretKey},
	}
	result := make([]string, 0, len(checks))
	for _, check := range checks {
		if strings.TrimSpace(check.value) == "" {
			result = append(result, check.key)
		}
	}
	return result
}

func env(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func duration(key string, fallback time.Duration) (time.Duration, error) {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback, nil
	}
	value, err := time.ParseDuration(raw)
	if err != nil || value <= 0 {
		return 0, fmt.Errorf("invalid %s: must be a positive duration", key)
	}
	return value, nil
}
