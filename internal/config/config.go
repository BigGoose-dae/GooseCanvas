package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

const DefaultAudioEndpoint = "https://openspeech.bytedance.com/api/v3/tts/create"

type Config struct {
	AppName  string
	LogoURL  string
	RepoURL  string
	Addr     string
	DataDir  string
	Database string
	Volc     Volcengine
	Worker   Worker
}

type Volcengine struct {
	APIKey        string
	BaseURL       string
	AudioAPIKey   string
	AudioEndpoint string
}

type Worker struct {
	PollInterval time.Duration
	TaskTimeout  time.Duration
	Concurrency  int
}

func Load() (Config, error) {
	_ = godotenv.Load()
	if err := os.Chmod(".env", 0o600); err != nil && !os.IsNotExist(err) {
		return Config{}, err
	}
	dataDir := env("DATA_DIR", "./data")
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
			APIKey:        strings.TrimSpace(os.Getenv("VOLCENGINE_API_KEY")),
			BaseURL:       strings.TrimRight(env("VOLCENGINE_BASE_URL", "https://ark.cn-beijing.volces.com"), "/"),
			AudioAPIKey:   strings.TrimSpace(os.Getenv("VOLCENGINE_AUDIO_API_KEY")),
			AudioEndpoint: env("VOLCENGINE_AUDIO_ENDPOINT", DefaultAudioEndpoint),
		},
		Worker: Worker{PollInterval: pollInterval, TaskTimeout: taskTimeout, Concurrency: 4},
	}
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return Config{}, fmt.Errorf("create data directory: %w", err)
	}
	return cfg, nil
}

func (c Config) Missing() []string {
	if strings.TrimSpace(c.Volc.AudioAPIKey) != "" {
		return nil
	}
	return c.ModelMissing()
}

func (c Config) ModelMissing() []string {
	if c.Volc.APIKey == "" {
		return []string{"VOLCENGINE_API_KEY"}
	}
	return nil
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

// Redact keeps configured credentials out of user-facing errors and logs.
func (c Config) Redact(message string) string {
	for _, secret := range []string{c.Volc.APIKey, c.Volc.AudioAPIKey} {
		secret = strings.TrimSpace(secret)
		variants := []string{secret}
		if len(secret) >= 7 && strings.EqualFold(secret[:7], "Bearer ") {
			variants = append(variants, strings.TrimSpace(secret[7:]))
		}
		for _, variant := range variants {
			if variant != "" {
				message = strings.ReplaceAll(message, variant, "[REDACTED]")
			}
		}
	}
	return message
}
