package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/BigGoose-dae/GooseCanvas/internal/config"
	"github.com/BigGoose-dae/GooseCanvas/internal/domain"
	"github.com/BigGoose-dae/GooseCanvas/internal/events"
	"github.com/BigGoose-dae/GooseCanvas/internal/provider"
	"github.com/BigGoose-dae/GooseCanvas/internal/runtime"
	"github.com/BigGoose-dae/GooseCanvas/internal/storage"
	"gorm.io/gorm"
)

type API struct {
	DB           *gorm.DB
	Runtime      *runtime.Manager
	Events       *events.Hub
	Config       config.Config
	Store        storage.Store
	TOS          *storage.TOSStore
	Volc         *provider.Volcengine
	StorageError error
}

func (a *API) Register(r *gin.Engine) {
	v1 := r.Group("/api/v1")
	v1.Use(func(c *gin.Context) {
		if c.Request.Method != http.MethodGet && c.Request.Method != http.MethodHead && c.Request.Method != http.MethodOptions {
			if origin := c.GetHeader("Origin"); origin != "" {
				parsed, err := url.Parse(origin)
				sameOrigin := err == nil && parsed.Host == c.Request.Host
				devOrigin := err == nil && (parsed.Host == "localhost:5173" || parsed.Host == "127.0.0.1:5173")
				if !sameOrigin && !devOrigin {
					c.AbortWithStatusJSON(403, gin.H{"error": "不允许跨站修改本机数据"})
					return
				}
			}
		}
		c.Next()
	})
	v1.GET("/system/status", a.status)
	v1.GET("/settings", a.getSettings)
	v1.PUT("/settings", a.saveSettings)
	v1.POST("/settings/test", a.testSettings)
	v1.GET("/workspaces/:id/events", a.stream)
	v1.GET("/workspaces/:id/tasks", a.listTasks)
	v1.POST("/nodes/:id/duplicate", a.duplicateNode)
	v1.GET("/assets/:id/download", a.downloadAsset)
	v1.GET("/models", a.models)
	v1.POST("/models", a.createModel)
	v1.PUT("/models/:id", a.updateModel)
	v1.DELETE("/models/:id", a.deleteModel)
	v1.GET("/workspaces", a.listWorkspaces)
	v1.POST("/workspaces", a.createWorkspace)
	v1.PUT("/workspaces/:id", a.updateWorkspace)
	v1.DELETE("/workspaces/:id", a.deleteWorkspace)
	v1.GET("/workspaces/:id/graph", a.graph)
	v1.POST("/workspaces/:id/nodes", a.createNode)
	v1.PUT("/nodes/:id", a.updateNode)
	v1.DELETE("/nodes/:id", a.deleteNode)
	v1.GET("/nodes/:id/history", a.nodeHistory)
	v1.POST("/workspaces/:id/edges", a.createEdge)
	v1.DELETE("/edges/:id", a.deleteEdge)
	v1.POST("/assets/upload", a.uploadAsset)
	v1.GET("/assets/:id/content", a.assetContent)
	v1.POST("/nodes/:id/run", a.runNode)
	v1.GET("/generations/:id", a.generation)
	v1.POST("/generations/:id/retry", a.retryGeneration)
}

func (a *API) status(c *gin.Context) {
	a = a.current()
	missing := a.Config.Missing()
	storageStatus := "not_configured"
	storageMessage := ""
	if a.Store != nil && a.TOS != nil {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 8*time.Second)
		defer cancel()
		if err := a.Store.Ready(ctx); err != nil {
			storageStatus = "error"
			storageMessage = err.Error()
		} else {
			storageStatus = "ok"
		}
	} else if a.StorageError != nil {
		storageMessage = a.StorageError.Error()
	}
	c.JSON(http.StatusOK, gin.H{
		"ready":    len(missing) == 0 && storageStatus == "ok",
		"app":      gin.H{"name": a.Config.AppName, "logoUrl": a.Config.LogoURL, "repositoryUrl": a.Config.RepoURL},
		"database": gin.H{"status": "ok", "driver": "sqlite"},
		"storage":  gin.H{"status": storageStatus, "provider": "tos", "bucket": a.Config.TOS.Bucket, "message": storageMessage},
		"model":    gin.H{"status": map[bool]string{true: "configured", false: "not_configured"}[a.Config.Volc.APIKey != ""], "provider": "volcengine"},
		"missing":  missing,
	})
}

func (a *API) listWorkspaces(c *gin.Context) {
	var items []domain.Workspace
	if err := a.DB.Where("deleted_at IS NULL").Order("updated_at desc").Find(&items).Error; err != nil {
		fail(c, 500, err)
		return
	}
	c.JSON(http.StatusOK, items)
}

type workspaceRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

func (a *API) createWorkspace(c *gin.Context) {
	var req workspaceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, 400, err)
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		fail(c, 400, fmt.Errorf("项目名称不能为空"))
		return
	}
	item := domain.Workspace{Name: req.Name, Description: strings.TrimSpace(req.Description)}
	if err := a.DB.Create(&item).Error; err != nil {
		fail(c, 500, err)
		return
	}
	c.JSON(http.StatusCreated, item)
}
func (a *API) updateWorkspace(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	var req workspaceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, 400, err)
		return
	}
	updates := map[string]any{"updated_at": time.Now()}
	if strings.TrimSpace(req.Name) != "" {
		updates["name"] = strings.TrimSpace(req.Name)
	}
	updates["description"] = strings.TrimSpace(req.Description)
	if a.DB.Model(&domain.Workspace{}).Where("id=? AND deleted_at IS NULL", id).Updates(updates).RowsAffected == 0 {
		fail(c, 404, fmt.Errorf("项目不存在"))
		return
	}
	c.Status(http.StatusNoContent)
}
func (a *API) deleteWorkspace(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	now := time.Now()
	if err := a.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&domain.Edge{}).Where("workspace_id=?", id).Update("deleted_at", now).Error; err != nil {
			return err
		}
		if err := tx.Model(&domain.Node{}).Where("workspace_id=?", id).Update("deleted_at", now).Error; err != nil {
			return err
		}
		return tx.Model(&domain.Workspace{}).Where("id=?", id).Update("deleted_at", now).Error
	}); err != nil {
		fail(c, 500, err)
		return
	}
	c.Status(http.StatusNoContent)
}

type assetView struct {
	domain.Asset
	URL string `json:"url"`
}
type nodeView struct {
	domain.Node
	Asset      *assetView                `json:"asset,omitempty"`
	Generation *domain.GenerationSession `json:"generation,omitempty"`
}

func (a *API) graph(c *gin.Context) {
	wid, ok := idParam(c)
	if !ok {
		return
	}
	var ws domain.Workspace
	if err := a.DB.Where("id=? AND deleted_at IS NULL", wid).First(&ws).Error; err != nil {
		fail(c, 404, fmt.Errorf("项目不存在"))
		return
	}
	var nodes []domain.Node
	a.DB.Where("workspace_id=? AND deleted_at IS NULL", wid).Order("id").Find(&nodes)
	result := make([]nodeView, 0, len(nodes))
	for _, node := range nodes {
		view := nodeView{Node: node}
		if node.CurrentAssetID != nil {
			if asset, err := a.loadAsset(c.Request.Context(), *node.CurrentAssetID); err == nil {
				view.Asset = &assetView{Asset: asset.Asset, URL: asset.URL}
			}
		}
		var session domain.GenerationSession
		if err := a.DB.Where("node_id=?", node.ID).Order("id desc").First(&session).Error; err == nil {
			view.Generation = &session
		}
		result = append(result, view)
	}
	var edges []domain.Edge
	a.DB.Where("workspace_id=? AND deleted_at IS NULL", wid).Order("id").Find(&edges)
	c.JSON(http.StatusOK, gin.H{"workspace": ws, "nodes": result, "edges": edges})
}

type nodeRequest struct {
	NodeType string         `json:"nodeType"`
	Title    string         `json:"title"`
	Prompt   string         `json:"prompt"`
	ModelKey string         `json:"modelKey"`
	Params   map[string]any `json:"params"`
	PosX     float64        `json:"posX"`
	PosY     float64        `json:"posY"`
	Width    float64        `json:"width"`
	Height   float64        `json:"height"`
	AssetID  *uint64        `json:"assetId"`
}

func (a *API) createNode(c *gin.Context) {
	wid, ok := idParam(c)
	if !ok {
		return
	}
	var req nodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, 400, err)
		return
	}
	typeName := strings.ToLower(strings.TrimSpace(req.NodeType))
	if typeName != "text" && typeName != "image" && typeName != "video" {
		fail(c, 400, fmt.Errorf("节点类型仅支持 text/image/video"))
		return
	}
	if req.Params == nil {
		req.Params = map[string]any{}
	}
	params, _ := json.Marshal(req.Params)
	var workspace domain.Workspace
	if err := a.DB.Where("id=? AND deleted_at IS NULL", wid).First(&workspace).Error; err != nil {
		fail(c, 404, fmt.Errorf("项目不存在"))
		return
	}
	if req.AssetID != nil {
		var asset domain.Asset
		if err := a.DB.Where("id=? AND workspace_id=? AND asset_type=? AND deleted_at IS NULL", *req.AssetID, wid, typeName).First(&asset).Error; err != nil {
			fail(c, 400, fmt.Errorf("素材不存在或不属于当前项目/节点类型"))
			return
		}
	}
	node := domain.Node{WorkspaceID: wid, NodeType: typeName, Title: strings.TrimSpace(req.Title), Prompt: req.Prompt, ModelKey: req.ModelKey, Params: string(params), PosX: req.PosX, PosY: req.PosY, Width: req.Width, Height: req.Height, CurrentAssetID: req.AssetID, Version: 1}
	if node.Title == "" {
		node.Title = map[string]string{"text": "文本", "image": "图片", "video": "视频"}[typeName]
	}
	if err := a.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&node).Error; err != nil {
			return err
		}
		return tx.Create(&domain.NodeVersion{NodeID: node.ID, Version: 1, AssetID: req.AssetID, Prompt: node.Prompt, ModelKey: node.ModelKey, Params: node.Params}).Error
	}); err != nil {
		fail(c, 500, err)
		return
	}
	c.JSON(http.StatusCreated, node)
}
func (a *API) updateNode(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	var req nodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, 400, err)
		return
	}
	if req.Params == nil {
		req.Params = map[string]any{}
	}
	params, _ := json.Marshal(req.Params)
	updates := map[string]any{"title": req.Title, "prompt": req.Prompt, "model_key": req.ModelKey, "params": string(params), "pos_x": req.PosX, "pos_y": req.PosY, "width": req.Width, "height": req.Height}

	err := a.DB.Transaction(func(tx *gorm.DB) error {
		var node domain.Node
		if err := tx.Where("id=? AND deleted_at IS NULL", id).First(&node).Error; err != nil {
			return err
		}
		if err := tx.Model(&node).Updates(updates).Error; err != nil {
			return err
		}
		return tx.Model(&domain.Workspace{}).Where("id=?", node.WorkspaceID).Update("updated_at", time.Now()).Error
	})
	if err != nil {
		fail(c, 500, err)
		return
	}
	c.Status(http.StatusNoContent)
}
func (a *API) deleteNode(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	now := time.Now()
	a.DB.Transaction(func(tx *gorm.DB) error {
		tx.Model(&domain.Edge{}).Where("from_node_id=? OR to_node_id=?", id, id).Update("deleted_at", now)
		return tx.Model(&domain.Node{}).Where("id=?", id).Update("deleted_at", now).Error
	})
	c.Status(http.StatusNoContent)
}

type edgeRequest struct {
	FromNodeID uint64 `json:"fromNodeId"`
	ToNodeID   uint64 `json:"toNodeId"`
}

func (a *API) createEdge(c *gin.Context) {
	wid, ok := idParam(c)
	if !ok {
		return
	}
	var req edgeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, 400, err)
		return
	}
	if req.FromNodeID == 0 || req.ToNodeID == 0 || req.FromNodeID == req.ToNodeID {
		fail(c, 400, fmt.Errorf("无效连接"))
		return
	}
	var nodeCount int64
	a.DB.Model(&domain.Node{}).Where("workspace_id=? AND id IN ? AND deleted_at IS NULL", wid, []uint64{req.FromNodeID, req.ToNodeID}).Count(&nodeCount)
	if nodeCount != 2 {
		fail(c, 400, fmt.Errorf("连接节点不属于当前项目或已被删除"))
		return
	}
	var count int64
	a.DB.Model(&domain.Edge{}).Where("workspace_id=? AND from_node_id=? AND to_node_id=? AND deleted_at IS NULL", wid, req.FromNodeID, req.ToNodeID).Count(&count)
	if count > 0 {
		fail(c, 409, fmt.Errorf("连接已存在"))
		return
	}
	item := domain.Edge{WorkspaceID: wid, FromNodeID: req.FromNodeID, ToNodeID: req.ToNodeID}
	if err := a.DB.Create(&item).Error; err != nil {
		fail(c, 500, err)
		return
	}
	c.JSON(201, item)
}
func (a *API) deleteEdge(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	now := time.Now()
	a.DB.Model(&domain.Edge{}).Where("id=?", id).Update("deleted_at", now)
	c.Status(204)
}

func (a *API) uploadAsset(c *gin.Context) {
	a = a.current()
	if a.Store == nil || a.TOS == nil {
		fail(c, 503, fmt.Errorf("TOS 未配置"))
		return
	}
	wid, err := strconv.ParseUint(c.PostForm("workspaceId"), 10, 64)
	if err != nil || wid == 0 {
		fail(c, 400, fmt.Errorf("workspaceId 无效"))
		return
	}
	var workspaceCount int64
	a.DB.Model(&domain.Workspace{}).Where("id=? AND deleted_at IS NULL", wid).Count(&workspaceCount)
	if workspaceCount == 0 {
		fail(c, 404, fmt.Errorf("项目不存在"))
		return
	}
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		fail(c, 400, err)
		return
	}
	defer file.Close()
	if header.Size > 500<<20 {
		fail(c, 413, fmt.Errorf("文件不能超过 500MB"))
		return
	}
	contentType := header.Header.Get("Content-Type")
	kind := assetKind(contentType)
	if kind == "" {
		fail(c, 400, fmt.Errorf("仅支持图片和视频"))
		return
	}
	key := a.TOS.Key("uploads", fmt.Sprint(wid), time.Now().Format("20060102"), fmt.Sprintf("%d%s", time.Now().UnixNano(), safeExt(header)))
	put, err := a.Store.Put(c.Request.Context(), key, contentType, header.Size, file)
	if err != nil {
		fail(c, 502, err)
		return
	}
	asset := domain.Asset{WorkspaceID: wid, AssetType: kind, StorageProvider: "tos", Bucket: a.TOS.Bucket(), ObjectKey: key, ContentType: contentType, Size: header.Size, ETag: put.ETag, Source: "upload"}
	if err := a.DB.Create(&asset).Error; err != nil {
		_ = a.Store.Delete(c.Request.Context(), key)
		fail(c, 500, err)
		return
	}
	c.JSON(201, assetView{Asset: asset, URL: fmt.Sprintf("/api/v1/assets/%d/content", asset.ID)})
}

func (a *API) assetContent(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	view, err := a.loadAsset(c.Request.Context(), id)
	if err != nil {
		fail(c, 404, err)
		return
	}
	url, err := a.current().Store.PresignGet(c.Request.Context(), view.ObjectKey, 30*time.Minute)
	if err != nil {
		fail(c, 502, err)
		return
	}
	c.Header("Cache-Control", "no-store")
	c.Redirect(http.StatusTemporaryRedirect, url)
}
func (a *API) loadAsset(ctx context.Context, id uint64) (assetView, error) {
	a = a.current()
	var asset domain.Asset
	if err := a.DB.Where("id=? AND deleted_at IS NULL", id).First(&asset).Error; err != nil {
		return assetView{}, err
	}
	if a.Store == nil || a.TOS == nil {
		return assetView{}, fmt.Errorf("TOS 未配置")
	}
	return assetView{Asset: asset, URL: fmt.Sprintf("/api/v1/assets/%d/content", asset.ID)}, nil
}

type runRequest struct {
	Prompt       string         `json:"prompt"`
	ModelKey     string         `json:"modelKey"`
	Params       map[string]any `json:"params"`
	InputNodeIDs []uint64       `json:"inputNodeIds"`
}

func (a *API) runNode(c *gin.Context) {
	a = a.current()
	nid, ok := idParam(c)
	if !ok {
		return
	}
	var req runRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, 400, err)
		return
	}
	var node domain.Node
	if err := a.DB.Where("id=? AND deleted_at IS NULL", nid).First(&node).Error; err != nil {
		fail(c, 404, fmt.Errorf("节点不存在"))
		return
	}
	if node.NodeType != "image" && node.NodeType != "video" {
		fail(c, 400, fmt.Errorf("仅图片和视频节点可运行"))
		return
	}
	if a.Store == nil || len(a.Config.StorageMissing()) > 0 {
		fail(c, 503, fmt.Errorf("TOS 未配置，无法生成内容"))
		return
	}
	if len(a.Config.ModelMissing()) > 0 {
		fail(c, 503, fmt.Errorf("VOLCENGINE_API_KEY 未配置，无法生成内容"))
		return
	}
	if req.Params == nil {
		req.Params = map[string]any{}
	}
	var model domain.ModelDefinition
	modelQuery := a.DB.Where("task_type=? AND enabled=? AND deleted_at IS NULL", node.NodeType, true)
	if strings.TrimSpace(req.ModelKey) != "" {
		modelQuery = modelQuery.Where("model_key=?", strings.TrimSpace(req.ModelKey))
	}
	if err := modelQuery.Order("builtin desc, id asc").First(&model).Error; err != nil {
		fail(c, 400, fmt.Errorf("没有找到可用的%s模型，请先在模型管理中添加并启用", map[string]string{"image": "图片", "video": "视频"}[node.NodeType]))
		return
	}
	if model.Provider != "volcengine" {
		fail(c, 400, fmt.Errorf("当前版本尚未安装模型供应商 %s 的适配器", model.Provider))
		return
	}
	defaults := map[string]any{}
	_ = json.Unmarshal([]byte(model.Defaults), &defaults)
	for key, value := range req.Params {
		defaults[key] = value
	}
	params, _ := json.Marshal(defaults)
	var session domain.GenerationSession
	err := a.DB.Transaction(func(tx *gorm.DB) error {
		var active int64
		if err := tx.Model(&domain.GenerationSession{}).Where("node_id=? AND status IN ?", nid, domain.ActiveStatuses()).Count(&active).Error; err != nil {
			return err
		}
		if active > 0 {
			return fmt.Errorf("该节点已有进行中的任务")
		}
		promptParts := []string{strings.TrimSpace(req.Prompt)}
		assetInputs := make([]domain.GenerationInput, 0, len(req.InputNodeIDs))
		for _, inputID := range req.InputNodeIDs {
			var inputNode domain.Node
			if err := tx.Where("id=? AND workspace_id=? AND deleted_at IS NULL", inputID, node.WorkspaceID).First(&inputNode).Error; err != nil {
				return fmt.Errorf("输入节点 %d 不存在", inputID)
			}
			if inputNode.NodeType == "text" {
				if text := strings.TrimSpace(inputNode.Prompt); text != "" {
					promptParts = append(promptParts, text)
				}
				continue
			}
			if inputNode.CurrentAssetID == nil {
				return fmt.Errorf("输入节点 %d 没有素材", inputID)
			}
			assetInputs = append(assetInputs, domain.GenerationInput{AssetID: *inputNode.CurrentAssetID, InputType: inputNode.NodeType, InputRole: "reference", SortOrder: len(assetInputs) + 1})
		}
		combinedPrompt := strings.Join(promptParts, "\n\n")
		if combinedPrompt == "" {
			return fmt.Errorf("提示词不能为空；也可以连接一个有内容的文本节点")
		}
		session = domain.GenerationSession{WorkspaceID: node.WorkspaceID, NodeID: node.ID, TaskType: node.NodeType, ModelKey: model.ModelKey, Prompt: combinedPrompt, Params: string(params), Status: domain.StatusPending}
		if err := tx.Create(&session).Error; err != nil {
			return err
		}
		for _, input := range assetInputs {
			input.SessionID = session.ID
			if err := tx.Create(&input).Error; err != nil {
				return err
			}
		}
		if err := tx.Create(&domain.GenerationTask{SessionID: session.ID, Provider: model.Provider, Status: domain.StatusPending}).Error; err != nil {
			return err
		}
		return tx.Model(&node).Updates(map[string]any{"prompt": req.Prompt, "model_key": model.ModelKey, "params": string(params)}).Error
	})
	if err != nil {
		fail(c, 400, err)
		return
	}
	a.Events.Notify(session.WorkspaceID)
	c.JSON(201, session)
}

func (a *API) generation(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	var session domain.GenerationSession
	if err := a.DB.First(&session, id).Error; err != nil {
		fail(c, 404, fmt.Errorf("任务不存在"))
		return
	}
	response := gin.H{"session": session}
	if session.ResultAssetID != nil {
		if asset, err := a.loadAsset(c.Request.Context(), *session.ResultAssetID); err == nil {
			response["asset"] = asset
		}
	}
	c.JSON(200, response)
}
func idParam(c *gin.Context) (uint64, bool) {
	value, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || value == 0 {
		fail(c, 400, fmt.Errorf("ID 无效"))
		return 0, false
	}
	return value, true
}
func fail(c *gin.Context, status int, err error) { c.JSON(status, gin.H{"error": err.Error()}) }
func assetKind(contentType string) string {
	if strings.HasPrefix(contentType, "image/") {
		return "image"
	}
	if strings.HasPrefix(contentType, "video/") {
		return "video"
	}
	return ""
}
func safeExt(header *multipart.FileHeader) string {
	ext := strings.ToLower(filepath.Ext(filepath.Base(header.Filename)))
	if len(ext) > 10 || ext == "" {
		if strings.HasPrefix(header.Header.Get("Content-Type"), "image/") {
			return ".png"
		}
		return ".mp4"
	}
	return ext
}
