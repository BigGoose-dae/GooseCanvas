package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/BigGoose-dae/GooseCanvas/internal/domain"
	"github.com/gin-gonic/gin"
)

type modelView struct {
	ID          uint64         `json:"id"`
	Key         string         `json:"key"`
	Name        string         `json:"name"`
	TaskType    string         `json:"taskType"`
	Provider    string         `json:"provider"`
	Protocol    string         `json:"protocol"`
	Description string         `json:"description"`
	Inputs      []string       `json:"inputs"`
	Defaults    map[string]any `json:"defaults"`
	Options     map[string]any `json:"options"`
	Enabled     bool           `json:"enabled"`
	Builtin     bool           `json:"builtin"`
	CreatedAt   time.Time      `json:"createdAt"`
	UpdatedAt   time.Time      `json:"updatedAt"`
}

type modelRequest struct {
	Key         string         `json:"key"`
	Name        string         `json:"name"`
	TaskType    string         `json:"taskType"`
	Provider    string         `json:"provider"`
	Protocol    string         `json:"protocol"`
	Description string         `json:"description"`
	Inputs      []string       `json:"inputs"`
	Defaults    map[string]any `json:"defaults"`
	Options     map[string]any `json:"options"`
	Enabled     *bool          `json:"enabled"`
}

func (a *API) models(c *gin.Context) {
	query := a.DB.Where("deleted_at IS NULL")
	if taskType := strings.TrimSpace(c.Query("type")); taskType != "" {
		query = query.Where("task_type=?", taskType)
	}
	var rows []domain.ModelDefinition
	if err := query.Order("enabled desc, builtin desc, id asc").Find(&rows).Error; err != nil {
		a.fail(c, 500, err)
		return
	}
	items := make([]modelView, 0, len(rows))
	for _, row := range rows {
		items = append(items, toModelView(row))
	}
	c.JSON(http.StatusOK, items)
}

func (a *API) createModel(c *gin.Context) {
	var req modelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		a.fail(c, 400, err)
		return
	}
	row, err := modelFromRequest(req)
	if err != nil {
		a.fail(c, 400, err)
		return
	}
	if err := a.DB.Create(&row).Error; err != nil {
		a.fail(c, 409, fmt.Errorf("模型 Key 已存在或数据无效: %w", err))
		return
	}
	c.JSON(http.StatusCreated, toModelView(row))
}

func (a *API) updateModel(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	var current domain.ModelDefinition
	if err := a.DB.Where("id=? AND deleted_at IS NULL", id).First(&current).Error; err != nil {
		a.fail(c, 404, fmt.Errorf("模型不存在"))
		return
	}
	var req modelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		a.fail(c, 400, err)
		return
	}
	req.Key = current.ModelKey
	row, err := modelFromRequest(req)
	if err != nil {
		a.fail(c, 400, err)
		return
	}
	updates := map[string]any{
		"name": row.Name, "task_type": row.TaskType, "provider": row.Provider,
		"protocol": row.Protocol, "description": row.Description, "inputs": row.Inputs,
		"defaults": row.Defaults, "options": row.Options, "enabled": row.Enabled,
	}
	if err := a.DB.Model(&current).Updates(updates).Error; err != nil {
		a.fail(c, 500, err)
		return
	}
	if err := a.DB.First(&current, id).Error; err != nil {
		a.fail(c, 500, err)
		return
	}
	c.JSON(http.StatusOK, toModelView(current))
}

func (a *API) deleteModel(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	now := time.Now()
	result := a.DB.Model(&domain.ModelDefinition{}).Where("id=? AND deleted_at IS NULL", id).Update("deleted_at", now)
	if result.Error != nil {
		a.fail(c, 500, result.Error)
		return
	}
	if result.RowsAffected == 0 {
		a.fail(c, 404, fmt.Errorf("模型不存在"))
		return
	}
	c.Status(http.StatusNoContent)
}

func modelFromRequest(req modelRequest) (domain.ModelDefinition, error) {
	req.Key = strings.TrimSpace(req.Key)
	req.Name = strings.TrimSpace(req.Name)
	req.TaskType = strings.ToLower(strings.TrimSpace(req.TaskType))
	req.Provider = strings.ToLower(strings.TrimSpace(req.Provider))
	if req.Key == "" || req.Name == "" {
		return domain.ModelDefinition{}, fmt.Errorf("模型名称和 Key 不能为空")
	}
	if req.TaskType != "text" && req.TaskType != "image" && req.TaskType != "audio" && req.TaskType != "video" {
		return domain.ModelDefinition{}, fmt.Errorf("任务类型仅支持 text、image、audio 或 video")
	}
	if req.Provider == "" {
		req.Provider = "volcengine"
	}
	if req.Protocol == "" {
		req.Protocol = map[string]string{"text": "ark-chat-v3", "image": "ark-image-v3", "audio": "doubao-audio-v3", "video": "ark-video-v3"}[req.TaskType]
	}
	expectedProtocol := map[string]string{"text": "ark-chat-v3", "image": "ark-image-v3", "audio": "doubao-audio-v3", "video": "ark-video-v3"}[req.TaskType]
	if req.Protocol != expectedProtocol {
		return domain.ModelDefinition{}, fmt.Errorf("%s 模型需要使用 %s 协议", req.TaskType, expectedProtocol)
	}
	if req.Provider != "volcengine" {
		return domain.ModelDefinition{}, fmt.Errorf("当前基础版仅内置 volcengine 适配器")
	}
	if req.Inputs == nil {
		req.Inputs = map[string][]string{"text": []string{"text"}, "image": []string{"image"}, "audio": []string{"audio"}, "video": []string{"image"}}[req.TaskType]
	}
	if req.Defaults == nil {
		req.Defaults = map[string]any{}
	}
	if req.Options == nil {
		req.Options = map[string]any{}
	}
	inputs, _ := json.Marshal(req.Inputs)
	defaults, _ := json.Marshal(req.Defaults)
	options, _ := json.Marshal(req.Options)
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	return domain.ModelDefinition{ModelKey: req.Key, Name: req.Name, TaskType: req.TaskType, Provider: req.Provider, Protocol: req.Protocol, Description: strings.TrimSpace(req.Description), Inputs: string(inputs), Defaults: string(defaults), Options: string(options), Enabled: enabled}, nil
}

func toModelView(row domain.ModelDefinition) modelView {
	view := modelView{ID: row.ID, Key: row.ModelKey, Name: row.Name, TaskType: row.TaskType, Provider: row.Provider, Protocol: row.Protocol, Description: row.Description, Enabled: row.Enabled, Builtin: row.Builtin, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt}
	_ = json.Unmarshal([]byte(row.Inputs), &view.Inputs)
	_ = json.Unmarshal([]byte(row.Defaults), &view.Defaults)
	_ = json.Unmarshal([]byte(row.Options), &view.Options)
	if view.Inputs == nil {
		view.Inputs = []string{}
	}
	if view.Defaults == nil {
		view.Defaults = map[string]any{}
	}
	if view.Options == nil {
		view.Options = map[string]any{}
	}
	return view
}
