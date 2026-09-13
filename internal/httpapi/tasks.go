package httpapi

import (
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"path/filepath"
	"strconv"
	"time"

	"github.com/BigGoose-dae/GooseCanvas/internal/domain"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type generationView struct {
	domain.GenerationSession
	NodeTitle      string     `json:"nodeTitle"`
	Asset          *assetView `json:"asset,omitempty"`
	ExternalTaskID string     `json:"externalTaskId,omitempty"`
	TaskError      string     `json:"taskError,omitempty"`
	RetryMode      string     `json:"retryMode,omitempty"`
}

func (a *API) generationViews(sessions []domain.GenerationSession) ([]generationView, error) {
	views := make([]generationView, 0, len(sessions))
	if len(sessions) == 0 {
		return views, nil
	}
	ids, nodeIDs, assetIDs := []uint64{}, []uint64{}, []uint64{}
	for _, session := range sessions {
		ids = append(ids, session.ID)
		nodeIDs = append(nodeIDs, session.NodeID)
		if session.ResultAssetID != nil {
			assetIDs = append(assetIDs, *session.ResultAssetID)
		}
	}
	var tasks []domain.GenerationTask
	var nodes []domain.Node
	var assets []domain.Asset
	if err := a.DB.Where("session_id IN ?", ids).Find(&tasks).Error; err != nil {
		return nil, err
	}
	if err := a.DB.Where("id IN ?", nodeIDs).Find(&nodes).Error; err != nil {
		return nil, err
	}
	if len(assetIDs) > 0 {
		if err := a.DB.Where("id IN ? AND deleted_at IS NULL", assetIDs).Find(&assets).Error; err != nil {
			return nil, err
		}
	}
	taskMap := map[uint64]domain.GenerationTask{}
	nodeMap := map[uint64]string{}
	assetMap := map[uint64]assetView{}
	for _, task := range tasks {
		taskMap[task.SessionID] = task
	}
	for _, node := range nodes {
		nodeMap[node.ID] = node.Title
	}
	for _, asset := range assets {
		assetMap[asset.ID] = assetView{Asset: asset, URL: fmt.Sprintf("/api/v1/assets/%d/content", asset.ID)}
	}
	for _, session := range sessions {
		task := taskMap[session.ID]
		view := generationView{GenerationSession: session, NodeTitle: nodeMap[session.NodeID], ExternalTaskID: task.ExternalTaskID, TaskError: task.ErrorMessage, RetryMode: task.RetryMode}
		if session.ResultAssetID != nil {
			if asset, ok := assetMap[*session.ResultAssetID]; ok {
				view.Asset = &asset
			}
		}
		views = append(views, view)
	}
	return views, nil
}

func (a *API) listTasks(c *gin.Context) {
	wid, ok := idParam(c)
	if !ok {
		return
	}
	views, err := a.queue(wid)
	if err != nil {
		a.fail(c, 500, err)
		return
	}
	c.JSON(200, views)
}

func (a *API) queue(wid uint64) ([]generationView, error) {
	var sessions []domain.GenerationSession
	recent := a.DB.Model(&domain.GenerationSession{}).Select("id").Where("workspace_id=?", wid).Order("id desc").Limit(100)
	if err := a.DB.Where("workspace_id=? AND (status IN ? OR id IN (?))", wid, domain.ActiveStatuses(), recent).Order("id desc").Find(&sessions).Error; err != nil {
		return nil, err
	}
	return a.generationViews(sessions)
}

func (a *API) nodeHistory(c *gin.Context) {
	nid, ok := idParam(c)
	if !ok {
		return
	}
	query := a.DB.Where("node_id=?", nid)
	if before, err := strconv.ParseUint(c.Query("before"), 10, 64); err == nil && before > 0 {
		query = query.Where("id < ?", before)
	}
	var sessions []domain.GenerationSession
	if err := query.Order("id desc").Limit(21).Find(&sessions).Error; err != nil {
		a.fail(c, 500, err)
		return
	}
	more := len(sessions) > 20
	if more {
		sessions = sessions[:20]
	}
	views, err := a.generationViews(sessions)
	if err != nil {
		a.fail(c, 500, err)
		return
	}
	c.JSON(200, gin.H{"items": views, "hasMore": more})
}

// Only server-owned result fields are emitted. Draft text, layout and selection
// never travel back through SSE and therefore cannot overwrite local edits.
func (a *API) snapshot(wid uint64) (any, error) {
	queue, err := a.queue(wid)
	if err != nil {
		return nil, err
	}
	var nodes []domain.Node
	if err := a.DB.Where("workspace_id=? AND deleted_at IS NULL", wid).Find(&nodes).Error; err != nil {
		return nil, err
	}
	latestIDs := a.DB.Model(&domain.GenerationSession{}).Select("MAX(id)").Where("workspace_id=?", wid).Group("node_id")
	var latest []domain.GenerationSession
	if err := a.DB.Where("id IN (?)", latestIDs).Find(&latest).Error; err != nil {
		return nil, err
	}
	sessions := map[uint64]domain.GenerationSession{}
	for _, session := range latest {
		sessions[session.NodeID] = session
	}
	results := make([]gin.H, 0, len(nodes))
	for _, node := range nodes {
		row := gin.H{"id": node.ID, "version": node.Version, "currentAssetId": node.CurrentAssetID, "asset": nil, "generation": nil}
		if session, ok := sessions[node.ID]; ok {
			row["generation"] = session
		}
		if node.CurrentAssetID != nil {
			row["asset"] = gin.H{"id": *node.CurrentAssetID, "url": fmt.Sprintf("/api/v1/assets/%d/content", *node.CurrentAssetID)}
		}
		results = append(results, row)
	}
	cfg := a.current().Config
	return gin.H{"tasks": queue, "nodes": results, "concurrency": cfg.Worker.Concurrency, "configured": len(cfg.Missing()) == 0}, nil
}

func (a *API) stream(c *gin.Context) {
	wid, ok := idParam(c)
	if !ok {
		return
	}
	var ws domain.Workspace
	if a.DB.Where("id=? AND deleted_at IS NULL", wid).First(&ws).Error != nil {
		a.fail(c, 404, fmt.Errorf("项目不存在"))
		return
	}
	if a.Events == nil {
		a.fail(c, 503, fmt.Errorf("事件服务未启动"))
		return
	}
	updates, unsubscribe := a.Events.Subscribe(wid)
	defer unsubscribe()
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache, no-transform")
	c.Header("X-Accel-Buffering", "no")
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	send := func() bool {
		snapshot, err := a.snapshot(wid)
		if err != nil {
			return false
		}
		raw, err := json.Marshal(snapshot)
		if err != nil {
			return false
		}
		if _, err = fmt.Fprintf(c.Writer, "retry: 2000\nevent: snapshot\ndata: %s\n\n", raw); err != nil {
			return false
		}
		c.Writer.Flush()
		return true
	}
	if !send() {
		return
	}
	for {
		select {
		case <-c.Request.Context().Done():
			return
		case <-updates:
			if !send() {
				return
			}
		case <-ticker.C:
			if _, err := io.WriteString(c.Writer, ": heartbeat\n\n"); err != nil {
				return
			}
			c.Writer.Flush()
		}
	}
}

func (a *API) duplicateNode(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	var result domain.Node
	err := a.DB.Transaction(func(tx *gorm.DB) error {
		var original domain.Node
		if err := tx.Where("id=? AND deleted_at IS NULL", id).First(&original).Error; err != nil {
			return err
		}
		result = original
		result.ID = 0
		result.Title += " 副本"
		result.PosX += 48
		result.PosY += 48
		result.Version = 1
		result.CreatedAt = time.Time{}
		result.UpdatedAt = time.Time{}
		if err := tx.Create(&result).Error; err != nil {
			return err
		}
		return tx.Create(&domain.NodeVersion{NodeID: result.ID, Version: 1, AssetID: result.CurrentAssetID, Prompt: result.Prompt, Content: result.Content, ModelKey: result.ModelKey, Params: result.Params}).Error
	})
	if err != nil {
		a.fail(c, 400, err)
		return
	}
	view := nodeView{Node: result}
	if result.CurrentAssetID != nil {
		asset, err := a.loadAsset(c.Request.Context(), *result.CurrentAssetID)
		if err == nil {
			view.Asset = &asset
		}
	}
	c.JSON(201, view)
}

func (a *API) retryGeneration(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	var result domain.GenerationSession
	err := a.DB.Transaction(func(tx *gorm.DB) error {
		var original domain.GenerationSession
		if err := tx.First(&original, id).Error; err != nil {
			return err
		}
		if original.Status != domain.StatusFailed {
			return fmt.Errorf("只有失败任务可以重试")
		}
		var node domain.Node
		if err := tx.Where("id=? AND deleted_at IS NULL", original.NodeID).First(&node).Error; err != nil {
			return fmt.Errorf("节点已删除")
		}
		var count int64
		if err := tx.Model(&domain.GenerationSession{}).Where("node_id=? AND status IN ?", original.NodeID, domain.ActiveStatuses()).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return fmt.Errorf("该节点已有进行中的任务")
		}
		var task domain.GenerationTask
		if err := tx.Where("session_id=?", id).First(&task).Error; err != nil {
			return err
		}
		if task.RetryMode == "archive" || task.RetryMode == "query" {
			state := domain.StatusArchiving
			if task.RetryMode == "query" {
				state = domain.StatusProcessing
			}
			now := time.Now()
			if err := tx.Model(&original).Updates(map[string]any{"status": state, "error_message": "", "finished_at": nil, "started_at": now}).Error; err != nil {
				return err
			}
			if err := tx.Model(&task).Updates(map[string]any{"status": state, "error_message": "", "next_poll_at": nil, "lease_until": nil, "retry_mode": ""}).Error; err != nil {
				return err
			}
			result = original
			return nil
		}
		result = original
		result.ID = 0
		result.CreatedAt = time.Time{}
		result.UpdatedAt = time.Time{}
		result.StartedAt = nil
		result.FinishedAt = nil
		result.ErrorMessage = ""
		result.ResultAssetID = nil
		result.ResultText = ""
		result.Status = domain.StatusPending
		if err := tx.Create(&result).Error; err != nil {
			return err
		}
		var inputs []domain.GenerationInput
		if err := tx.Where("session_id=?", id).Find(&inputs).Error; err != nil {
			return err
		}
		for _, input := range inputs {
			input.ID = 0
			input.SessionID = result.ID
			input.CreatedAt = time.Time{}
			if err := tx.Create(&input).Error; err != nil {
				return err
			}
		}
		return tx.Create(&domain.GenerationTask{SessionID: result.ID, Provider: task.Provider, Status: domain.StatusPending}).Error
	})
	if err != nil {
		a.fail(c, 409, err)
		return
	}
	a.Events.Notify(result.WorkspaceID)
	c.JSON(201, result)
}

func (a *API) downloadAsset(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	view, err := a.loadAsset(c.Request.Context(), id)
	if err != nil {
		a.fail(c, 404, err)
		return
	}
	file, info, err := a.current().Store.Open(c.Request.Context(), view.ObjectKey)
	if err != nil {
		a.fail(c, 404, err)
		return
	}
	defer file.Close()
	name := fmt.Sprintf("goose-canvas-%d%s", view.ID, filepath.Ext(view.ObjectKey))
	c.Header("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": name}))
	c.Header("Content-Type", view.ContentType)
	c.Header("X-Content-Type-Options", "nosniff")
	http.ServeContent(c.Writer, c.Request, name, info.ModTime(), file)
}
