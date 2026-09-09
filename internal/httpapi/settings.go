package httpapi

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/goosecanvas/goosecanvas/internal/config"
	"github.com/goosecanvas/goosecanvas/internal/domain"
)

func (a *API) current() *API {
	if a.Runtime == nil {
		return a
	}
	copy := *a
	snap := a.Runtime.Snapshot()
	copy.Config, copy.Store, copy.TOS = snap.Config, snap.Store, snap.TOS
	return &copy
}

type settingsRequest struct {
	AppName        string `json:"appName"`
	BaseURL        string `json:"baseUrl"`
	APIKey         string `json:"apiKey"`
	Endpoint       string `json:"endpoint"`
	Region         string `json:"region"`
	Bucket         string `json:"bucket"`
	AccessKey      string `json:"accessKey"`
	SecretKey      string `json:"secretKey"`
	Prefix         string `json:"prefix"`
	Concurrency    int    `json:"concurrency"`
	TimeoutMinutes int    `json:"timeoutMinutes"`
}

func (a *API) getSettings(c *gin.Context) {
	cfg := a.current().Config
	c.Header("Cache-Control", "no-store")
	c.JSON(200, gin.H{
		"appName": cfg.AppName, "baseUrl": cfg.Volc.BaseURL,
		"endpoint": cfg.TOS.Endpoint, "region": cfg.TOS.Region, "bucket": cfg.TOS.Bucket, "prefix": cfg.TOS.Prefix,
		"apiKeyConfigured": cfg.Volc.APIKey != "", "accessKeyConfigured": cfg.TOS.AccessKey != "", "secretKeyConfigured": cfg.TOS.SecretKey != "",
		"concurrency": cfg.Worker.Concurrency, "timeoutMinutes": int(cfg.Worker.TaskTimeout.Minutes()),
	})
}

func (a *API) saveSettings(c *gin.Context) {
	if a.Runtime == nil {
		fail(c, 503, fmt.Errorf("配置服务未启动"))
		return
	}
	var req settingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, 400, err)
		return
	}
	err := a.Runtime.Update(func(cfg *config.Config) error {
		req.BaseURL = strings.TrimRight(strings.TrimSpace(req.BaseURL), "/")
		req.Endpoint, req.Region, req.Bucket = strings.TrimSpace(req.Endpoint), strings.TrimSpace(req.Region), strings.TrimSpace(req.Bucket)
		// Prevent routing existing assets/tasks to a different account/region.
		storageChanged := cfg.TOS.Bucket != req.Bucket || cfg.TOS.Endpoint != req.Endpoint || cfg.TOS.Region != req.Region
		if storageChanged {
			var count int64
			if err := a.DB.Model(&domain.Asset{}).Count(&count).Error; err != nil {
				return err
			}
			if count > 0 {
				return fmt.Errorf("已有素材，暂不能更换存储桶或区域；可更新访问凭证")
			}
		}
		if storageChanged || cfg.Volc.BaseURL != req.BaseURL {
			var count int64
			if err := a.DB.Model(&domain.GenerationSession{}).Where("status IN ?", domain.ActiveStatuses()).Count(&count).Error; err != nil {
				return err
			}
			if count > 0 {
				return fmt.Errorf("请等待当前任务结束后再更换服务地址或存储桶")
			}
		}
		cfg.AppName, cfg.Volc.BaseURL = strings.TrimSpace(req.AppName), req.BaseURL
		cfg.TOS.Endpoint, cfg.TOS.Region, cfg.TOS.Bucket, cfg.TOS.Prefix = req.Endpoint, req.Region, req.Bucket, strings.Trim(req.Prefix, "/ ")
		// Empty secret fields explicitly mean keep the existing value.
		if strings.TrimSpace(req.APIKey) != "" {
			cfg.Volc.APIKey = strings.TrimSpace(req.APIKey)
		}
		if strings.TrimSpace(req.AccessKey) != "" {
			cfg.TOS.AccessKey = strings.TrimSpace(req.AccessKey)
		}
		if strings.TrimSpace(req.SecretKey) != "" {
			cfg.TOS.SecretKey = strings.TrimSpace(req.SecretKey)
		}
		cfg.Worker.Concurrency = req.Concurrency
		cfg.Worker.TaskTimeout = time.Duration(req.TimeoutMinutes) * time.Minute
		return nil
	})
	if err != nil {
		fail(c, 400, err)
		return
	}
	a.Events.Notify(0)
	a.getSettings(c)
}

func (a *API) testSettings(c *gin.Context) {
	current := a.current()
	if current.Store == nil {
		fail(c, 400, fmt.Errorf("请先保存完整的存储配置"))
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 8*time.Second)
	defer cancel()
	if err := current.Store.Ready(ctx); err != nil {
		fail(c, 502, err)
		return
	}
	c.JSON(200, gin.H{"message": "存储连接成功。模型密钥在实际生成时验证，本次未提交生成任务。"})
}
