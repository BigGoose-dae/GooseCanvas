package generation

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/BigGoose-dae/GooseCanvas/internal/config"
	"github.com/BigGoose-dae/GooseCanvas/internal/domain"
	"github.com/BigGoose-dae/GooseCanvas/internal/events"
	"github.com/BigGoose-dae/GooseCanvas/internal/provider"
	"github.com/BigGoose-dae/GooseCanvas/internal/runtime"
	"github.com/BigGoose-dae/GooseCanvas/internal/storage"
	"gorm.io/gorm"
)

type Engine struct {
	db       *gorm.DB
	store    storage.Store
	provider provider.Provider
	cfg      config.Config
	http     *http.Client
	snapshot func() runtime.Snapshot
	hub      *events.Hub
	mu       sync.Mutex
	active   map[uint64]bool
	clients  map[uint64]runtime.Snapshot
	wg       sync.WaitGroup
}

func New(db *gorm.DB, store storage.Store, p provider.Provider, cfg config.Config) *Engine {
	return &Engine{db: db, store: store, provider: p, cfg: cfg, http: &http.Client{Timeout: 10 * time.Minute}}
}

func NewManaged(db *gorm.DB, manager *runtime.Manager, hub *events.Hub) *Engine {
	return &Engine{db: db, snapshot: manager.Snapshot, hub: hub, active: map[uint64]bool{}, clients: map[uint64]runtime.Snapshot{}}
}

func (e *Engine) Start(ctx context.Context) {
	// A submission interrupted before receiving its external ID has an uncertain
	// outcome. Never automatically submit it again and potentially charge twice.
	var interrupted []domain.GenerationTask
	if err := e.db.Where("status = ?", domain.StatusSubmitting).Find(&interrupted).Error; err != nil {
		log.Printf("recover tasks: %v", err)
	}
	for i := range interrupted {
		e.fail(&interrupted[i], fmt.Errorf("上次提交中断，第三方可能已接收。请核实第三方记录后再重新生成"))
	}
	if err := e.db.Model(&domain.GenerationTask{}).Where("status IN ?", []string{domain.StatusPending, domain.StatusProcessing, domain.StatusArchiving}).Update("lease_until", nil).Error; err != nil {
		log.Printf("recover leases: %v", err)
	}
	e.wg.Add(1)
	go func() {
		defer e.wg.Done()
		ticker := time.NewTicker(250 * time.Millisecond)
		defer ticker.Stop()
		for {
			if ctx.Err() != nil {
				return
			}
			e.dispatch(ctx)
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
}

func (e *Engine) Wait() { e.wg.Wait() }

func (e *Engine) dispatch(ctx context.Context) {
	snap := e.snapshot()
	if snap.Store == nil || (strings.TrimSpace(snap.Config.Volc.APIKey) == "" && !snap.Config.Volc.AudioConfigured()) {
		return
	}
	e.mu.Lock()
	slots := snap.Config.Worker.Concurrency - len(e.active)
	e.mu.Unlock()
	if slots <= 0 {
		return
	}
	now := time.Now()
	var tasks []domain.GenerationTask
	err := e.db.Where("status IN ? AND (next_poll_at IS NULL OR next_poll_at <= ?) AND (lease_until IS NULL OR lease_until < ?)", []string{domain.StatusPending, domain.StatusProcessing, domain.StatusArchiving}, now, now).Order("updated_at asc, id asc").Limit(slots).Find(&tasks).Error
	if err != nil {
		log.Printf("schedule tasks: %v", err)
		return
	}
	for _, row := range tasks {
		task := row
		e.mu.Lock()
		if e.active[task.ID] {
			e.mu.Unlock()
			continue
		}
		e.active[task.ID] = true
		taskSnap, exists := e.clients[task.ID]
		if !exists {
			taskSnap = snap
			e.clients[task.ID] = taskSnap
		}
		e.mu.Unlock()
		lease := now.Add(15 * time.Minute)
		claim := e.db.Model(&domain.GenerationTask{}).Where("id=? AND status=? AND (lease_until IS NULL OR lease_until < ?)", task.ID, task.Status, now).Update("lease_until", lease)
		if claim.Error != nil || claim.RowsAffected == 0 {
			e.mu.Lock()
			delete(e.active, task.ID)
			e.mu.Unlock()
			continue
		}
		e.wg.Add(1)
		go func() {
			defer e.wg.Done()
			worker := New(e.db, taskSnap.Store, taskSnap.Provider, taskSnap.Config)
			worker.hub = e.hub
			defer func() {
				if recovered := recover(); recovered != nil {
					worker.fail(&task, fmt.Errorf("任务执行异常，请检查服务日志"))
					log.Printf("task %d panic: %s", task.ID, worker.cfg.Redact(fmt.Sprint(recovered)))
				}
				if err := e.db.Model(&domain.GenerationTask{}).Where("id=?", task.ID).Update("lease_until", nil).Error; err != nil {
					log.Printf("release task %d: %v", task.ID, err)
				}
				var current domain.GenerationTask
				e.db.First(&current, task.ID)
				e.mu.Lock()
				delete(e.active, task.ID)
				if current.Status == domain.StatusSucceeded || current.Status == domain.StatusFailed {
					delete(e.clients, task.ID)
				}
				e.mu.Unlock()
			}()
			opCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)
			defer cancel()
			switch task.Status {
			case domain.StatusPending:
				worker.submit(opCtx, &task)
			case domain.StatusProcessing:
				worker.poll(opCtx, &task)
			case domain.StatusArchiving:
				worker.archive(opCtx, &task)
			}
		}()
	}
}

func (e *Engine) submit(ctx context.Context, task *domain.GenerationTask) {
	var session domain.GenerationSession
	if err := e.db.First(&session, task.SessionID).Error; err != nil {
		e.fail(task, err)
		return
	}
	request, err := e.buildRequest(ctx, session)
	if err != nil {
		e.fail(task, err)
		return
	}
	payload, _ := json.Marshal(request)
	now := time.Now()
	err = e.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(task).Updates(map[string]any{"status": domain.StatusSubmitting, "attempt": gorm.Expr("attempt + 1"), "request_payload": string(payload)}).Error; err != nil {
			return err
		}
		return tx.Model(&session).Updates(map[string]any{"status": domain.StatusSubmitting, "started_at": now}).Error
	})
	if err != nil {
		e.fail(task, err)
		return
	}
	e.hub.Notify(session.WorkspaceID)
	result, err := e.provider.Submit(ctx, request)
	if err != nil {
		var rejected *provider.HTTPError
		if errors.As(err, &rejected) {
			e.fail(task, rejected)
		} else {
			e.fail(task, fmt.Errorf("提交失败（请求中断时第三方可能已接收，请核实后再重试）: %w", err))
		}
		return
	}
	e.applyResult(ctx, task, &session, result)
}

func (e *Engine) poll(ctx context.Context, task *domain.GenerationTask) {
	var session domain.GenerationSession
	if err := e.db.First(&session, task.SessionID).Error; err != nil {
		e.fail(task, err)
		return
	}
	started := session.CreatedAt
	if session.StartedAt != nil {
		started = *session.StartedAt
	}
	if time.Since(started) > e.cfg.Worker.TaskTimeout {
		e.db.Model(task).Update("retry_mode", "query")
		e.fail(task, fmt.Errorf("状态查询超时，第三方任务可能仍在执行；可继续查询原任务"))
		return
	}
	pollCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	result, err := e.provider.Poll(pollCtx, task.ExternalTaskID)
	if err != nil {
		next := time.Now().Add(10 * time.Second)
		if err := e.db.Model(task).Updates(map[string]any{"next_poll_at": next, "error_message": e.cfg.Redact(err.Error())}).Error; err != nil {
			log.Printf("poll retry: %v", err)
		}
		e.hub.Notify(session.WorkspaceID)
		return
	}
	e.applyResult(ctx, task, &session, result)
}

func (e *Engine) archive(ctx context.Context, task *domain.GenerationTask) {
	var session domain.GenerationSession
	if err := e.db.First(&session, task.SessionID).Error; err != nil {
		e.fail(task, err)
		return
	}
	var result provider.Result
	if err := json.Unmarshal([]byte(task.ResultPayload), &result); err != nil {
		e.fail(task, err)
		return
	}
	e.applyResult(ctx, task, &session, result)
}

func (e *Engine) buildRequest(ctx context.Context, session domain.GenerationSession) (provider.Request, error) {
	var params map[string]any
	if strings.TrimSpace(session.Params) != "" {
		_ = json.Unmarshal([]byte(session.Params), &params)
	}
	if params == nil {
		params = map[string]any{}
	}
	var rows []domain.GenerationInput
	if err := e.db.Where("session_id = ?", session.ID).Order("sort_order asc").Find(&rows).Error; err != nil {
		return provider.Request{}, err
	}
	inputs := make([]provider.Input, 0, len(rows))
	var totalSize int64
	for _, row := range rows {
		var asset domain.Asset
		if err := e.db.First(&asset, row.AssetID).Error; err != nil {
			return provider.Request{}, err
		}
		if asset.StorageProvider != "local" {
			return provider.Request{}, fmt.Errorf("素材 %d 使用旧版 TOS 存储，请重新上传后再生成", asset.ID)
		}
		file, info, err := e.store.Open(ctx, asset.ObjectKey)
		if err != nil {
			return provider.Request{}, fmt.Errorf("读取本地素材 %d: %w", asset.ID, err)
		}
		if info.Size() > 50<<20 {
			file.Close()
			return provider.Request{}, fmt.Errorf("素材 %d 超过 Base64 输入的 50MB 限制", asset.ID)
		}
		totalSize += info.Size()
		if totalSize > 50<<20 {
			file.Close()
			return provider.Request{}, fmt.Errorf("输入素材总大小超过 50MB 限制")
		}
		var encoded strings.Builder
		encoded.Grow(base64.StdEncoding.EncodedLen(int(info.Size())))
		encoder := base64.NewEncoder(base64.StdEncoding, &encoded)
		readSize, readErr := io.Copy(encoder, file)
		closeErr := encoder.Close()
		file.Close()
		if readErr != nil {
			return provider.Request{}, fmt.Errorf("读取本地素材 %d: %w", asset.ID, readErr)
		}
		if closeErr != nil {
			return provider.Request{}, fmt.Errorf("编码本地素材 %d: %w", asset.ID, closeErr)
		}
		if readSize != info.Size() {
			return provider.Request{}, fmt.Errorf("本地素材 %d 在读取时发生变化，请重试", asset.ID)
		}
		mimeType := strings.TrimSpace(asset.ContentType)
		if mimeType == "" {
			mimeType = "application/octet-stream"
		}
		inputs = append(inputs, provider.Input{Type: row.InputType, Role: row.InputRole, MIME: mimeType, DataURI: "data:" + mimeType + ";base64," + encoded.String()})
	}
	return provider.Request{TaskType: session.TaskType, Model: session.ModelKey, Prompt: session.Prompt, Params: params, Inputs: inputs}, nil
}

func (e *Engine) applyResult(ctx context.Context, task *domain.GenerationTask, session *domain.GenerationSession, result provider.Result) {
	if result.Failed {
		e.fail(task, fmt.Errorf("%s", result.Error))
		return
	}
	if !result.Done {
		next := time.Now().Add(e.cfg.Worker.PollInterval)
		err := e.db.Transaction(func(tx *gorm.DB) error {
			if err := tx.Model(task).Updates(map[string]any{"status": domain.StatusProcessing, "external_task_id": result.ExternalID, "next_poll_at": next, "response_payload": string(result.Raw), "error_message": ""}).Error; err != nil {
				return err
			}
			return tx.Model(session).Update("status", domain.StatusProcessing).Error
		})
		if err != nil {
			e.fail(task, err)
		} else {
			e.hub.Notify(session.WorkspaceID)
		}
		return
	}
	if session.TaskType == "text" {
		e.completeText(task, session, result)
		return
	}
	payload, _ := json.Marshal(result)
	if err := e.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(task).Updates(map[string]any{"status": domain.StatusArchiving, "result_payload": string(payload)}).Error; err != nil {
			return err
		}
		return tx.Model(session).Update("status", domain.StatusArchiving).Error
	}); err != nil {
		e.fail(task, err)
		return
	}
	e.hub.Notify(session.WorkspaceID)
	asset, err := e.persistResult(ctx, *session, result)
	if err != nil {
		e.db.Model(task).Update("retry_mode", "archive")
		e.fail(task, err)
		return
	}
	now := time.Now()
	err = e.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&asset).Error; err != nil {
			return err
		}
		if err := tx.Model(&domain.Node{}).Where("id = ?", session.NodeID).Updates(map[string]any{"current_asset_id": asset.ID, "version": gorm.Expr("version + 1")}).Error; err != nil {
			return err
		}
		var node domain.Node
		if err := tx.First(&node, session.NodeID).Error; err != nil {
			return err
		}
		version := domain.NodeVersion{NodeID: node.ID, Version: node.Version, AssetID: &asset.ID, Prompt: session.Prompt, ModelKey: session.ModelKey, Params: session.Params}
		if err := tx.Create(&version).Error; err != nil {
			return err
		}
		if err := tx.Model(session).Updates(map[string]any{"status": domain.StatusSucceeded, "result_asset_id": asset.ID, "error_message": "", "finished_at": now}).Error; err != nil {
			return err
		}
		return tx.Model(task).Updates(map[string]any{"status": domain.StatusSucceeded, "response_payload": string(result.Raw), "error_message": "", "next_poll_at": nil, "result_payload": ""}).Error
	})
	if err != nil {
		e.db.Model(task).Update("retry_mode", "archive")
		e.fail(task, err)
	} else {
		e.hub.Notify(session.WorkspaceID)
	}
}

func (e *Engine) completeText(task *domain.GenerationTask, session *domain.GenerationSession, result provider.Result) {
	text := strings.TrimSpace(result.Text)
	if text == "" {
		e.fail(task, fmt.Errorf("文本生成完成但未返回文本"))
		return
	}
	now := time.Now()
	err := e.db.Transaction(func(tx *gorm.DB) error {
		var node domain.Node
		if err := tx.First(&node, session.NodeID).Error; err != nil {
			return err
		}
		// Keep an edit made while the model was running. The generated text remains
		// available in history and is written back only when the instruction is unchanged.
		if node.Prompt == session.NodePrompt {
			if err := tx.Model(&node).Updates(map[string]any{"prompt": text, "version": gorm.Expr("version + 1")}).Error; err != nil {
				return err
			}
			if err := tx.First(&node, session.NodeID).Error; err != nil {
				return err
			}
			version := domain.NodeVersion{NodeID: node.ID, Version: node.Version, Prompt: text, ModelKey: session.ModelKey, Params: session.Params}
			if err := tx.Create(&version).Error; err != nil {
				return err
			}
		}
		if err := tx.Model(session).Updates(map[string]any{"status": domain.StatusSucceeded, "result_text": text, "error_message": "", "finished_at": now}).Error; err != nil {
			return err
		}
		return tx.Model(task).Updates(map[string]any{"status": domain.StatusSucceeded, "response_payload": string(result.Raw), "error_message": "", "next_poll_at": nil, "result_payload": ""}).Error
	})
	if err != nil {
		e.fail(task, err)
		return
	}
	e.hub.Notify(session.WorkspaceID)
}

func (e *Engine) persistResult(ctx context.Context, session domain.GenerationSession, result provider.Result) (domain.Asset, error) {
	mimeType := result.MIME
	var body io.Reader
	var size int64
	var closeFn func()
	if len(result.Data) > 0 {
		body = bytes.NewReader(result.Data)
		size = int64(len(result.Data))
	} else {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, result.URL, nil)
		if err != nil {
			return domain.Asset{}, err
		}
		resp, err := e.http.Do(req)
		if err != nil {
			return domain.Asset{}, fmt.Errorf("下载模型结果失败: %w", err)
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			resp.Body.Close()
			return domain.Asset{}, fmt.Errorf("下载模型结果返回 HTTP %d", resp.StatusCode)
		}
		tmp, err := os.CreateTemp(e.cfg.DataDir, "result-*")
		if err != nil {
			resp.Body.Close()
			return domain.Asset{}, err
		}
		closeFn = func() { tmp.Close(); os.Remove(tmp.Name()) }
		defer closeFn()
		n, copyErr := io.Copy(tmp, io.LimitReader(resp.Body, 2<<30))
		resp.Body.Close()
		if copyErr != nil {
			return domain.Asset{}, copyErr
		}
		if n >= 2<<30 {
			return domain.Asset{}, fmt.Errorf("模型结果超过 2GB 限制")
		}
		if _, err := tmp.Seek(0, io.SeekStart); err != nil {
			return domain.Asset{}, err
		}
		body = tmp
		size = n
		if mimeType == "" {
			mimeType = resp.Header.Get("Content-Type")
		}
	}
	if mimeType == "" {
		if session.TaskType == "image" {
			mimeType = "image/png"
		} else {
			mimeType = "video/mp4"
		}
	}
	ext := extension(mimeType, session.TaskType)
	key := e.store.Key("generated", fmt.Sprint(session.WorkspaceID), fmt.Sprint(session.ID), "result"+ext)
	put, err := e.store.Put(ctx, key, mimeType, size, body)
	if err != nil {
		return domain.Asset{}, err
	}
	return domain.Asset{WorkspaceID: session.WorkspaceID, AssetType: session.TaskType, StorageProvider: "local", ObjectKey: key, ContentType: mimeType, Size: size, ETag: put.ETag, Source: "generated"}, nil
}

func (e *Engine) fail(task *domain.GenerationTask, failure error) {
	now := time.Now()
	err := e.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(task).Updates(map[string]any{"status": domain.StatusFailed, "error_message": e.cfg.Redact(failure.Error()), "next_poll_at": nil}).Error; err != nil {
			return err
		}
		return tx.Model(&domain.GenerationSession{}).Where("id=?", task.SessionID).Updates(map[string]any{"status": domain.StatusFailed, "error_message": e.cfg.Redact(failure.Error()), "finished_at": now}).Error
	})
	if err != nil {
		log.Printf("fail task %d: %v", task.ID, err)
	}
	var session domain.GenerationSession
	if e.db.First(&session, task.SessionID).Error == nil {
		e.hub.Notify(session.WorkspaceID)
	}
}

func extension(contentType, taskType string) string {
	mediaType := strings.Split(contentType, ";")[0]
	if known := map[string]string{
		"image/png": ".png", "image/jpeg": ".jpg", "image/webp": ".webp",
		"image/gif": ".gif", "video/mp4": ".mp4", "video/webm": ".webm",
		"audio/mpeg": ".mp3", "audio/wav": ".wav", "audio/pcm": ".pcm", "audio/ogg": ".ogg",
	}[mediaType]; known != "" {
		return known
	}
	if values, _ := mime.ExtensionsByType(mediaType); len(values) > 0 {
		return values[0]
	}
	if taskType == "image" {
		return ".png"
	}
	if taskType == "audio" {
		return ".mp3"
	}
	return ".mp4"
}
