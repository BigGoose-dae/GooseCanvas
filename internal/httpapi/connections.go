package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/BigGoose-dae/GooseCanvas/internal/config"
	"github.com/BigGoose-dae/GooseCanvas/internal/domain"
	"github.com/BigGoose-dae/GooseCanvas/internal/provider"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func applyConnectionRequest(cfg config.Volcengine, req settingsRequest) config.Volcengine {
	if value := strings.TrimSpace(req.APIKey); value != "" {
		cfg.APIKey = value
	}
	if value := strings.TrimSpace(req.BaseURL); value != "" {
		cfg.BaseURL = strings.TrimRight(value, "/")
	}
	if value := strings.TrimSpace(req.AssetsAccessKey); value != "" {
		cfg.AssetsAccessKey = value
	}
	if value := strings.TrimSpace(req.AssetsSecretKey); value != "" {
		cfg.AssetsSecretKey = value
	}
	if value := strings.TrimSpace(req.AssetsProjectName); value != "" {
		cfg.AssetsProjectName = value
	}
	if value := strings.TrimSpace(req.AssetsBaseURL); value != "" {
		cfg.AssetsBaseURL = strings.TrimRight(value, "/")
	}
	return cfg
}

func credentialScope(cfg config.Volcengine) string {
	return cfg.AssetCredentialScope()
}

func (a *API) testConnection(c *gin.Context) {
	var req settingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		a.fail(c, http.StatusBadRequest, err)
		return
	}
	cfg := applyConnectionRequest(a.current().Config.Volc, req)
	ctx, cancel := context.WithTimeout(c.Request.Context(), 20*time.Second)
	defer cancel()
	response := gin.H{"inference": gin.H{"ok": false}, "assetLibrary": gin.H{"configured": false, "ok": false}}
	if err := testInferenceConnection(ctx, cfg); err != nil {
		response["inference"] = gin.H{"ok": false, "message": (config.Config{Volc: cfg}).Redact(err.Error())}
	} else {
		response["inference"] = gin.H{"ok": true, "message": "方舟推理服务连接成功"}
	}
	assets := provider.NewAssetsClient(cfg)
	if assets.Configured() {
		page, err := assets.List(ctx, 1, 1)
		if err != nil {
			response["assetLibrary"] = gin.H{"configured": true, "ok": false, "message": (config.Config{Volc: cfg}).Redact(err.Error())}
		} else {
			response["assetLibrary"] = gin.H{"configured": true, "ok": true, "message": "火山可信素材库连接成功", "remoteCount": page.TotalCount, "credentialScope": credentialScope(cfg)}
		}
	}
	c.JSON(http.StatusOK, response)
}

type importAssetRequest struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	AssetType string  `json:"assetType"`
	GroupID   string  `json:"groupId"`
	PosX      float64 `json:"posX"`
	PosY      float64 `json:"posY"`
}

func (a *API) importLibraryAsset(c *gin.Context) {
	wid, ok := idParam(c)
	if !ok {
		return
	}
	var workspace struct{ ID uint64 }
	if err := a.DB.Raw("SELECT id FROM workspaces WHERE id=? AND deleted_at IS NULL", wid).Scan(&workspace).Error; err != nil || workspace.ID == 0 {
		a.fail(c, 404, fmt.Errorf("项目不存在"))
		return
	}
	var req importAssetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		a.fail(c, 400, err)
		return
	}
	req.ID = strings.TrimSpace(req.ID)
	if req.ID == "" || !strings.HasPrefix(req.ID, "asset-") {
		a.fail(c, 400, fmt.Errorf("无效的火山素材 ID"))
		return
	}
	typeName := strings.ToLower(strings.TrimSpace(req.AssetType))
	switch {
	case strings.Contains(typeName, "image"):
		typeName = "image"
	case strings.Contains(typeName, "video"):
		typeName = "video"
	case strings.Contains(typeName, "audio"):
		typeName = "audio"
	default:
		a.fail(c, 400, fmt.Errorf("该素材类型暂不支持添加到画布"))
		return
	}
	meta, _ := json.Marshal(gin.H{"credentialScope": a.current().Config.Volc.AssetCredentialScope(), "projectName": a.current().Config.Volc.AssetsProjectName, "groupId": req.GroupID})
	asset := domain.Asset{WorkspaceID: wid, AssetType: typeName, StorageProvider: "volcengine", ObjectKey: req.ID, ContentType: map[string]string{"image": "image/*", "video": "video/*", "audio": "audio/*"}[typeName], Meta: string(meta), Source: "trusted_library"}
	title := strings.TrimSpace(req.Name)
	if title == "" {
		title = req.ID
	}
	node := domain.Node{WorkspaceID: wid, NodeType: typeName, Title: title, Params: "{}", PosX: req.PosX, PosY: req.PosY, Width: 320, Height: 360, Version: 1}
	if err := a.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&asset).Error; err != nil {
			return err
		}
		node.CurrentAssetID = &asset.ID
		if err := tx.Create(&node).Error; err != nil {
			return err
		}
		return tx.Create(&domain.NodeVersion{NodeID: node.ID, Version: 1, AssetID: &asset.ID, Params: "{}"}).Error
	}); err != nil {
		a.fail(c, 500, err)
		return
	}
	c.JSON(http.StatusCreated, nodeView{Node: node, Asset: &assetView{Asset: asset}})
}

func (a *API) assetLibrary(c *gin.Context) {
	cfg := a.current().Config.Volc
	client := provider.NewAssetsClient(cfg)
	if !client.Configured() {
		a.fail(c, http.StatusServiceUnavailable, fmt.Errorf("请先在设置中配置火山素材库 IAM AK/SK"))
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "100"))
	ctx, cancel := context.WithTimeout(c.Request.Context(), 35*time.Second)
	defer cancel()
	result, err := client.List(ctx, page, pageSize)
	if err != nil {
		a.fail(c, http.StatusBadGateway, err)
		return
	}
	active, processing, failed := 0, 0, 0
	for _, item := range result.Items {
		switch strings.ToLower(strings.TrimSpace(item.Status)) {
		case "active":
			active++
		case "submitting", "processing", "pending":
			processing++
		default:
			failed++
		}
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, gin.H{
		"items":   result.Items,
		"summary": gin.H{"remoteTotal": result.TotalCount, "active": active, "processing": processing, "failed": failed},
		"page":    result.PageNumber, "pageSize": result.PageSize,
		"credentialScope": credentialScope(cfg), "projectName": cfg.AssetsProjectName, "syncedAt": time.Now(),
	})
}
