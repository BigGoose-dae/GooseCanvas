package provider

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/BigGoose-dae/GooseCanvas/internal/config"
)

type Volcengine struct {
	cfg  config.Volcengine
	http *http.Client
}

func NewVolcengine(cfg config.Volcengine) *Volcengine {
	return &Volcengine{cfg: cfg, http: &http.Client{Timeout: 5 * time.Minute}}
}

func (v *Volcengine) Submit(ctx context.Context, req Request) (Result, error) {
	if strings.TrimSpace(v.cfg.APIKey) == "" {
		return Result{}, fmt.Errorf("VOLCENGINE_API_KEY is not configured")
	}
	switch req.TaskType {
	case "image":
		return v.submitImage(ctx, req)
	case "video":
		return v.submitVideo(ctx, req)
	default:
		return Result{}, fmt.Errorf("unsupported task type %q", req.TaskType)
	}
}

func (v *Volcengine) Poll(ctx context.Context, externalID string) (Result, error) {
	var payload map[string]any
	raw, err := v.do(ctx, http.MethodGet, v.cfg.BaseURL+"/api/v3/contents/generations/tasks/"+externalID, nil, &payload)
	if err != nil {
		return Result{}, err
	}
	status := strings.ToLower(firstString(payload, "status", "task_status"))
	if status == "failed" || status == "error" || status == "expired" || status == "cancelled" || status == "canceled" {
		return Result{Failed: true, Error: v.redact(errorMessage(payload, "视频生成失败")), Raw: raw}, nil
	}
	url := nestedString(payload, "content", "video_url", "file_url", "url")
	if url == "" {
		url = firstString(payload, "video_url", "file_url", "url")
	}
	if url != "" {
		return Result{Done: true, URL: url, MIME: "video/mp4", Raw: raw}, nil
	}
	if status == "succeeded" || status == "success" || status == "completed" || status == "done" {
		return Result{Failed: true, Error: "视频任务完成但未返回结果地址", Raw: raw}, nil
	}
	return Result{ExternalID: externalID, Raw: raw}, nil
}

func (v *Volcengine) submitImage(ctx context.Context, req Request) (Result, error) {
	if strings.TrimSpace(req.Model) == "" {
		return Result{}, fmt.Errorf("model key is required")
	}
	ratio := stringParam(req.Params, "ratio", "1:1")
	resolution := stringParam(req.Params, "resolution", "2K")
	body := map[string]any{"model": req.Model, "prompt": strings.TrimSpace(req.Prompt) + "\n图片比例：" + ratio, "size": resolution, "response_format": "url", "watermark": false}
	var images []string
	for _, input := range req.Inputs {
		if input.Type == "image" && input.DataURI != "" {
			images = append(images, input.DataURI)
		}
	}
	if len(images) > 0 {
		body["image"] = images
	}
	var payload map[string]any
	raw, err := v.do(ctx, http.MethodPost, v.cfg.BaseURL+"/api/v3/images/generations", body, &payload)
	if err != nil {
		return Result{}, err
	}
	if message := apiError(payload); message != "" {
		return Result{Failed: true, Error: v.redact(message), Raw: raw}, nil
	}
	if data, ok := payload["data"].([]any); ok {
		for _, row := range data {
			item, _ := row.(map[string]any)
			if url := firstString(item, "url", "image_url", "file_url"); url != "" {
				return Result{Done: true, URL: url, MIME: "image/png", Raw: raw}, nil
			}
			if encoded := firstString(item, "b64_json", "base64", "image_base64"); encoded != "" {
				decoded, decodeErr := decodeImage(encoded)
				if decodeErr != nil {
					return Result{}, decodeErr
				}
				return Result{Done: true, Data: decoded, MIME: "image/png", Raw: raw}, nil
			}
		}
	}
	return Result{Failed: true, Error: "图片生成完成但未返回图片", Raw: raw}, nil
}

func (v *Volcengine) submitVideo(ctx context.Context, req Request) (Result, error) {
	if strings.TrimSpace(req.Model) == "" {
		return Result{}, fmt.Errorf("model key is required")
	}
	content := []map[string]any{}
	if strings.TrimSpace(req.Prompt) != "" {
		content = append(content, map[string]any{"type": "text", "text": strings.TrimSpace(req.Prompt)})
	}
	for _, input := range req.Inputs {
		if input.DataURI == "" || (input.Type != "image" && input.Type != "video") {
			continue
		}
		kind := input.Type + "_url"
		role := input.Role
		if role == "" {
			role = "reference"
		}
		content = append(content, map[string]any{"type": kind, "role": role, kind: map[string]any{"url": input.DataURI}})
	}
	body := map[string]any{
		"model": req.Model, "content": content,
		"ratio": stringParam(req.Params, "ratio", "16:9"), "resolution": stringParam(req.Params, "resolution", "720p"),
		"duration": intParam(req.Params, "duration", 5), "watermark": false,
	}
	var payload map[string]any
	raw, err := v.do(ctx, http.MethodPost, v.cfg.BaseURL+"/api/v3/contents/generations/tasks", body, &payload)
	if err != nil {
		return Result{}, err
	}
	if message := apiError(payload); message != "" {
		return Result{Failed: true, Error: v.redact(message), Raw: raw}, nil
	}
	id := firstString(payload, "id", "task_id", "taskId")
	if id == "" {
		return Result{Failed: true, Error: "视频任务未返回任务 ID", Raw: raw}, nil
	}
	return Result{ExternalID: id, Raw: raw}, nil
}

func (v *Volcengine) do(ctx context.Context, method, url string, body any, target any) ([]byte, error) {
	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reader = bytes.NewReader(encoded)
	}
	req, err := http.NewRequestWithContext(ctx, method, url, reader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+v.cfg.APIKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := v.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request volcengine: %w", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		message := compact(raw)
		var response map[string]any
		if json.Unmarshal(raw, &response) == nil {
			if detail := apiError(response); detail != "" {
				message = detail
			}
		}
		if v.cfg.APIKey != "" {
			message = strings.ReplaceAll(message, v.cfg.APIKey, "[REDACTED]")
		}
		return raw, &HTTPError{Status: resp.StatusCode, Message: message}
	}
	if err := json.Unmarshal(raw, target); err != nil {
		return raw, fmt.Errorf("decode volcengine response: %w", err)
	}
	return raw, nil
}

func decodeImage(raw string) ([]byte, error) {
	if idx := strings.Index(raw, ","); strings.HasPrefix(raw, "data:") && idx >= 0 {
		raw = raw[idx+1:]
	}
	return base64.StdEncoding.DecodeString(raw)
}
func firstString(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if s, ok := m[k].(string); ok && strings.TrimSpace(s) != "" {
			return strings.TrimSpace(s)
		}
	}
	return ""
}
func nestedString(m map[string]any, object string, keys ...string) string {
	child, _ := m[object].(map[string]any)
	return firstString(child, keys...)
}
func apiError(m map[string]any) string {
	if e, ok := m["error"].(map[string]any); ok {
		return firstString(e, "message", "error_message", "code")
	}
	return ""
}
func errorMessage(m map[string]any, fallback string) string {
	if s := apiError(m); s != "" {
		return s
	}
	if s := firstString(m, "message", "error_message"); s != "" {
		return s
	}
	return fallback
}
func stringParam(m map[string]any, key, fallback string) string {
	if v, ok := m[key]; ok && strings.TrimSpace(fmt.Sprint(v)) != "" {
		return strings.TrimSpace(fmt.Sprint(v))
	}
	return fallback
}
func intParam(m map[string]any, key string, fallback int) int {
	if v, ok := m[key].(float64); ok {
		return int(v)
	}
	if v, ok := m[key].(int); ok {
		return v
	}
	return fallback
}
func compact(raw []byte) string {
	s := strings.TrimSpace(string(raw))
	if len(s) > 500 {
		return s[:500]
	}
	return s
}

func (v *Volcengine) redact(message string) string {
	if v.cfg.APIKey != "" {
		return strings.ReplaceAll(message, v.cfg.APIKey, "[REDACTED]")
	}
	return message
}
