package httpapi

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"github.com/BigGoose-dae/GooseCanvas/internal/config"
	"github.com/BigGoose-dae/GooseCanvas/internal/domain"
	"github.com/gin-gonic/gin"
)

func (a *API) current() *API {
	if a.Runtime == nil {
		return a
	}
	copy := *a
	snap := a.Runtime.Snapshot()
	copy.Config, copy.Store = snap.Config, snap.Store
	return &copy
}

type settingsRequest struct {
	AppName           string `json:"appName"`
	BaseURL           string `json:"baseUrl"`
	APIKey            string `json:"apiKey"`
	AudioEndpoint     string `json:"audioEndpoint"`
	AudioAPIKey       string `json:"audioApiKey"`
	AudioAppID        string `json:"audioAppId"`
	AudioAccessKey    string `json:"audioAccessKey"`
	AssetsAccessKey   string `json:"assetsAccessKey"`
	AssetsSecretKey   string `json:"assetsSecretKey"`
	AssetsProjectName string `json:"assetsProjectName"`
	AssetsBaseURL     string `json:"assetsBaseUrl"`
	Concurrency       int    `json:"concurrency"`
	TimeoutMinutes    int    `json:"timeoutMinutes"`
}

func (a *API) getSettings(c *gin.Context) {
	cfg := a.current().Config
	assetDirectory, _ := filepath.Abs(filepath.Join(cfg.DataDir, "assets"))
	c.Header("Cache-Control", "no-store")
	c.JSON(200, gin.H{
		"appName": cfg.AppName, "baseUrl": cfg.Volc.BaseURL,
		"apiKeyConfigured": cfg.Volc.APIKey != "", "assetDirectory": assetDirectory,
		"audioEndpoint": cfg.Volc.AudioEndpoint, "audioApiKeyConfigured": cfg.Volc.AudioAPIKey != "",
		"audioAppIdConfigured": cfg.Volc.AudioAppID != "", "audioAccessKeyConfigured": cfg.Volc.AudioAccessKey != "",
		"assetsAccessKeyConfigured": cfg.Volc.AssetsAccessKey != "", "assetsSecretKeyConfigured": cfg.Volc.AssetsSecretKey != "",
		"assetsProjectName": cfg.Volc.AssetsProjectName, "assetsBaseUrl": cfg.Volc.AssetsBaseURL,
		"concurrency": cfg.Worker.Concurrency, "timeoutMinutes": int(cfg.Worker.TaskTimeout.Minutes()),
	})
}

func (a *API) saveSettings(c *gin.Context) {
	if a.Runtime == nil {
		a.fail(c, 503, fmt.Errorf("配置服务未启动"))
		return
	}
	var req settingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		a.fail(c, 400, err)
		return
	}
	err := a.Runtime.Update(func(cfg *config.Config) error {
		req.BaseURL = strings.TrimRight(strings.TrimSpace(req.BaseURL), "/")
		req.AudioEndpoint = strings.TrimSpace(req.AudioEndpoint)
		req.AssetsBaseURL = strings.TrimRight(strings.TrimSpace(req.AssetsBaseURL), "/")
		req.AssetsProjectName = strings.TrimSpace(req.AssetsProjectName)
		if req.AssetsBaseURL == "" {
			req.AssetsBaseURL = cfg.Volc.AssetsBaseURL
		}
		if req.AssetsProjectName == "" {
			req.AssetsProjectName = cfg.Volc.AssetsProjectName
		}
		if req.AudioEndpoint == "" {
			req.AudioEndpoint = cfg.Volc.AudioEndpoint
		}
		if cfg.Volc.BaseURL != req.BaseURL || cfg.Volc.AudioEndpoint != req.AudioEndpoint {
			var count int64
			if err := a.DB.Model(&domain.GenerationSession{}).Where("status IN ?", domain.ActiveStatuses()).Count(&count).Error; err != nil {
				return err
			}
			if count > 0 {
				return fmt.Errorf("请等待当前任务结束后再更换服务地址")
			}
		}
		cfg.AppName, cfg.Volc.BaseURL, cfg.Volc.AudioEndpoint = strings.TrimSpace(req.AppName), req.BaseURL, req.AudioEndpoint
		cfg.Volc.AssetsBaseURL, cfg.Volc.AssetsProjectName = req.AssetsBaseURL, req.AssetsProjectName
		// Empty secret fields explicitly mean keep the existing value.
		if strings.TrimSpace(req.APIKey) != "" {
			cfg.Volc.APIKey = strings.TrimSpace(req.APIKey)
		}
		if strings.TrimSpace(req.AudioAPIKey) != "" {
			cfg.Volc.AudioAPIKey = strings.TrimSpace(req.AudioAPIKey)
		}
		if strings.TrimSpace(req.AudioAppID) != "" {
			cfg.Volc.AudioAppID = strings.TrimSpace(req.AudioAppID)
		}
		if strings.TrimSpace(req.AudioAccessKey) != "" {
			cfg.Volc.AudioAccessKey = strings.TrimSpace(req.AudioAccessKey)
		}
		if strings.TrimSpace(req.AssetsAccessKey) != "" {
			cfg.Volc.AssetsAccessKey = strings.TrimSpace(req.AssetsAccessKey)
		}
		if strings.TrimSpace(req.AssetsSecretKey) != "" {
			cfg.Volc.AssetsSecretKey = strings.TrimSpace(req.AssetsSecretKey)
		}
		cfg.Worker.Concurrency = req.Concurrency
		cfg.Worker.TaskTimeout = time.Duration(req.TimeoutMinutes) * time.Minute
		return nil
	})
	if err != nil {
		a.fail(c, 400, err)
		return
	}
	a.Events.Notify(0)
	a.getSettings(c)
}

func testInferenceConnection(ctx context.Context, cfg config.Volcengine) error {
	if strings.TrimSpace(cfg.APIKey) == "" {
		return fmt.Errorf("方舟 API Key 未配置")
	}
	endpoint, err := url.Parse(strings.TrimRight(cfg.BaseURL, "/"))
	if err != nil {
		return err
	}
	path := strings.TrimSuffix(endpoint.Path, "/api/v3")
	endpoint.Path = strings.TrimRight(path, "/") + "/ping"
	endpoint.RawQuery = ""
	endpoint.Fragment = ""
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(cfg.APIKey))
	resp, err := (&http.Client{Timeout: 12 * time.Second}).Do(req)
	if err != nil {
		return fmt.Errorf("无法连接方舟推理服务: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		message := strings.TrimSpace(string(body))
		if message == "" {
			message = http.StatusText(resp.StatusCode)
		}
		return fmt.Errorf("方舟推理服务返回 HTTP %d: %s", resp.StatusCode, message)
	}
	return nil
}

func (a *API) fail(c *gin.Context, status int, err error) {
	c.JSON(status, gin.H{"error": a.current().Config.Redact(err.Error())})
}
