package config

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/BigGoose-dae/GooseCanvas/internal/features"
	"github.com/joho/godotenv"
)

func (v Volcengine) AssetCredentialScope() string {
	digest := sha256.Sum256([]byte(strings.TrimSpace(v.APIKey) + "\x00" + strings.TrimSpace(v.AssetsAccessKey) + "\x00" + strings.TrimSpace(v.AssetsProjectName)))
	return fmt.Sprintf("volc-%x", digest[:8])
}

const DefaultAudioEndpoint = "https://openspeech.bytedance.com/api/v3/tts/create"
const DefaultAssetsBaseURL = "https://ark.cn-beijing.volcengineapi.com"

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
	APIKey            string
	BaseURL           string
	AudioAPIKey       string
	AudioAppID        string
	AudioAccessKey    string
	AudioEndpoint     string
	AssetsAccessKey   string
	AssetsSecretKey   string
	AssetsProjectName string
	AssetsBaseURL     string
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
			APIKey:            strings.TrimSpace(os.Getenv("VOLCENGINE_API_KEY")),
			BaseURL:           strings.TrimRight(env("VOLCENGINE_BASE_URL", "https://ark.cn-beijing.volces.com"), "/"),
			AudioAPIKey:       strings.TrimSpace(os.Getenv("VOLCENGINE_AUDIO_API_KEY")),
			AudioAppID:        strings.TrimSpace(os.Getenv("VOLCENGINE_AUDIO_APP_ID")),
			AudioAccessKey:    strings.TrimSpace(os.Getenv("VOLCENGINE_AUDIO_ACCESS_KEY")),
			AudioEndpoint:     env("VOLCENGINE_AUDIO_ENDPOINT", DefaultAudioEndpoint),
			AssetsAccessKey:   strings.TrimSpace(os.Getenv("VOLCENGINE_ASSETS_ACCESS_KEY")),
			AssetsSecretKey:   strings.TrimSpace(os.Getenv("VOLCENGINE_ASSETS_SECRET_KEY")),
			AssetsProjectName: env("VOLCENGINE_ASSETS_PROJECT_NAME", "default"),
			AssetsBaseURL:     strings.TrimRight(env("VOLCENGINE_ASSETS_BASE_URL", DefaultAssetsBaseURL), "/"),
		},
		Worker: Worker{PollInterval: pollInterval, TaskTimeout: taskTimeout, Concurrency: 4},
	}
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return Config{}, fmt.Errorf("create data directory: %w", err)
	}
	return cfg, nil
}

func (c Config) Missing() []string {
	if features.AudioGeneration && c.Volc.AudioConfigured() {
		return nil
	}
	return c.ModelMissing()
}

func (v Volcengine) AudioConfigured() bool {
	return strings.TrimSpace(v.AudioAPIKey) != "" ||
		(strings.TrimSpace(v.AudioAppID) != "" && strings.TrimSpace(v.AudioAccessKey) != "")
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
	for _, secret := range []string{c.Volc.APIKey, c.Volc.AudioAPIKey, c.Volc.AudioAppID, c.Volc.AudioAccessKey, c.Volc.AssetsAccessKey, c.Volc.AssetsSecretKey} {
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
