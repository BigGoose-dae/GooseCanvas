package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"path/filepath"
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
	cfg := a.current().Config.Volc
	client := provider.NewAssetsClient(cfg)
	if !client.Configured() {
		a.fail(c, http.StatusServiceUnavailable, fmt.Errorf("请先在设置中配置火山素材库 IAM AK/SK"))
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Minute)
	defer cancel()
	remote, err := client.Get(ctx, req.ID)
	if err != nil {
		a.fail(c, http.StatusBadGateway, fmt.Errorf("读取可信素材详情失败: %w", err))
		return
	}
	if !strings.EqualFold(strings.TrimSpace(remote.Status), "active") {
		a.fail(c, http.StatusConflict, fmt.Errorf("可信素材当前状态为 %s，尚不能添加到画布", remote.Status))
		return
	}
	if strings.TrimSpace(remote.URL) == "" {
		a.fail(c, http.StatusBadGateway, fmt.Errorf("火山素材详情没有返回可下载地址"))
		return
	}
	typeName := strings.ToLower(strings.TrimSpace(remote.AssetType))
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
	key, contentType, size, etag, err := a.downloadTrustedAsset(ctx, wid, remote, typeName)
	if err != nil {
		a.fail(c, http.StatusBadGateway, err)
		return
	}
	meta, _ := json.Marshal(gin.H{"credentialScope": cfg.AssetCredentialScope(), "projectName": cfg.AssetsProjectName, "groupId": remote.GroupID, "providerAssetId": remote.ID})
	asset := domain.Asset{WorkspaceID: wid, AssetType: typeName, StorageProvider: "local", ObjectKey: key, ContentType: contentType, Size: size, ETag: etag, Meta: string(meta), Source: "trusted_library"}
	title := strings.TrimSpace(remote.Name)
	if title == "" {
		title = req.ID
	}
	var placeholder domain.Asset
	if err := a.DB.Where("workspace_id=? AND storage_provider=? AND object_key=? AND deleted_at IS NULL", wid, "volcengine", req.ID).Order("id").First(&placeholder).Error; err == nil {
		var existingNode domain.Node
		err = a.DB.Transaction(func(tx *gorm.DB) error {
			updates := map[string]any{"asset_type": typeName, "storage_provider": "local", "object_key": key, "content_type": contentType, "size": size, "e_tag": etag, "meta": string(meta), "source": "trusted_library"}
			if err := tx.Model(&placeholder).Updates(updates).Error; err != nil {
				return err
			}
			asset = placeholder
			asset.AssetType = typeName
			asset.StorageProvider = "local"
			asset.ObjectKey = key
			asset.ContentType = contentType
			asset.Size = size
			asset.ETag = etag
			asset.Meta = string(meta)
			asset.Source = "trusted_library"
			if err := tx.Where("workspace_id=? AND current_asset_id=? AND deleted_at IS NULL", wid, placeholder.ID).Order("id").First(&existingNode).Error; err != nil {
				return err
			}
			if strings.TrimSpace(existingNode.Title) == "" || existingNode.Title == req.ID {
				if err := tx.Model(&existingNode).Update("title", title).Error; err != nil {
					return err
				}
				existingNode.Title = title
			}
			return nil
		})
		if err != nil {
			_ = a.current().Store.Delete(ctx, key)
			a.fail(c, 500, fmt.Errorf("修复已有可信素材节点失败: %w", err))
			return
		}
		if a.Events != nil {
			a.Events.Notify(wid)
		}
		c.JSON(http.StatusOK, nodeView{Node: existingNode, Asset: &assetView{Asset: asset, URL: fmt.Sprintf("/api/v1/assets/%d/content", asset.ID)}})
		return
	} else if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		_ = a.current().Store.Delete(ctx, key)
		a.fail(c, 500, err)
		return
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
		_ = a.current().Store.Delete(ctx, key)
		a.fail(c, 500, err)
		return
	}
	if a.Events != nil {
		a.Events.Notify(wid)
	}
	c.JSON(http.StatusCreated, nodeView{Node: node, Asset: &assetView{Asset: asset, URL: fmt.Sprintf("/api/v1/assets/%d/content", asset.ID)}})
}

func (a *API) downloadTrustedAsset(ctx context.Context, workspaceID uint64, remote provider.AssetLibraryItem, typeName string) (string, string, int64, string, error) {
	parsed, err := url.Parse(strings.TrimSpace(remote.URL))
	if err != nil || (parsed.Scheme != "https" && parsed.Scheme != "http") || parsed.Host == "" {
		return "", "", 0, "", fmt.Errorf("可信素材下载地址无效")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return "", "", 0, "", err
	}
	resp, err := (&http.Client{Timeout: 10 * time.Minute}).Do(req)
	if err != nil {
		return "", "", 0, "", fmt.Errorf("下载可信素材失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", "", 0, "", fmt.Errorf("下载可信素材返回 HTTP %d", resp.StatusCode)
	}
	const maxSize int64 = 2 << 30
	if resp.ContentLength > maxSize {
		return "", "", 0, "", fmt.Errorf("可信素材超过 2GB 限制")
	}
	contentType := strings.TrimSpace(strings.Split(resp.Header.Get("Content-Type"), ";")[0])
	if contentType == "" || contentType == "application/octet-stream" {
		contentType = map[string]string{"image": "image/jpeg", "video": "video/mp4", "audio": "audio/mpeg"}[typeName]
	}
	if !strings.HasPrefix(contentType, typeName+"/") {
		return "", "", 0, "", fmt.Errorf("可信素材返回类型 %s，与 %s 素材不匹配", contentType, typeName)
	}
	ext := filepath.Ext(parsed.Path)
	if len(ext) > 10 || strings.ContainsAny(ext, "/\\") {
		ext = ""
	}
	if ext == "" {
		if values, _ := mime.ExtensionsByType(contentType); len(values) > 0 {
			ext = values[0]
		}
	}
	if ext == "" {
		ext = map[string]string{"image": ".jpg", "video": ".mp4", "audio": ".mp3"}[typeName]
	}
	fileID := strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_' {
			return r
		}
		return '-'
	}, remote.ID)
	key := a.current().Store.Key("trusted", strconv.FormatUint(workspaceID, 10), time.Now().Format("20060102"), fmt.Sprintf("%d-%s%s", time.Now().UnixNano(), fileID, ext))
	limited := &io.LimitedReader{R: resp.Body, N: maxSize + 1}
	put, err := a.current().Store.Put(ctx, key, contentType, -1, limited)
	if err != nil {
		return "", "", 0, "", fmt.Errorf("保存可信素材到本地失败: %w", err)
	}
	size := maxSize + 1 - limited.N
	if size > maxSize {
		_ = a.current().Store.Delete(ctx, key)
		return "", "", 0, "", fmt.Errorf("可信素材超过 2GB 限制")
	}
	return key, contentType, size, put.ETag, nil
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
	groupID := strings.TrimSpace(c.Query("groupId"))
	result, err := client.List(ctx, page, pageSize, groupID)
	if err != nil {
		a.fail(c, http.StatusBadGateway, err)
		return
	}
	groups, err := client.ListGroups(ctx, 1, 100)
	if err != nil {
		a.fail(c, http.StatusBadGateway, fmt.Errorf("同步素材组失败: %w", err))
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
		"items":  result.Items,
		"groups": groups.Items, "groupTotal": groups.TotalCount,
		"summary": gin.H{"remoteTotal": result.TotalCount, "active": active, "processing": processing, "failed": failed},
		"page":    result.PageNumber, "pageSize": result.PageSize,
		"credentialScope": credentialScope(cfg), "projectName": cfg.AssetsProjectName, "selectedGroupId": groupID, "syncedAt": time.Now(),
	})
}
