package provider

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
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
	if req.TaskType == "audio" {
		return v.submitAudio(ctx, req)
	}
	if strings.TrimSpace(v.cfg.APIKey) == "" {
		return Result{}, fmt.Errorf("VOLCENGINE_API_KEY is not configured")
	}
	switch req.TaskType {
	case "text":
		return v.submitText(ctx, req)
	case "image":
		return v.submitImage(ctx, req)
	case "video":
		return v.submitVideo(ctx, req)
	default:
		return Result{}, fmt.Errorf("unsupported task type %q", req.TaskType)
	}
}

func (v *Volcengine) submitAudio(ctx context.Context, req Request) (Result, error) {
	apiKey := strings.TrimSpace(v.cfg.AudioAPIKey)
	if apiKey == "" {
		apiKey = strings.TrimSpace(v.cfg.APIKey)
	}
	if apiKey == "" {
		return Result{}, fmt.Errorf("VOLCENGINE_AUDIO_API_KEY is not configured")
	}
	prompt := strings.TrimSpace(req.Prompt)
	if prompt == "" {
		return Result{}, fmt.Errorf("audio prompt is required")
	}
	format := normalizeAudioFormat(stringParam(req.Params, "responseFormat", "mp3"))
	if format == "" {
		return Result{}, fmt.Errorf("audio response format must be mp3, wav, pcm, or ogg_opus")
	}
	references := make([]map[string]string, 0, 3)
	for _, input := range req.Inputs {
		if input.Type == "audio" && input.DataURI != "" {
			if len(references) == 3 {
				return Result{}, fmt.Errorf("doubao seed audio supports at most 3 reference audios")
			}
			references = append(references, map[string]string{"audio_url": input.DataURI})
		}
	}
	body := map[string]any{
		"model":       stringParam(req.Params, "providerModel", "seed-audio-1.0"),
		"text_prompt": prompt,
		"audio_config": map[string]any{
			"format": format, "sample_rate": intParam(req.Params, "sampleRate", 24000),
			"speech_rate":   clampInt(intParam(req.Params, "speechRate", 0), -50, 100),
			"loudness_rate": clampInt(intParam(req.Params, "loudnessRate", 0), -50, 100),
			"pitch_rate":    clampInt(intParam(req.Params, "pitchRate", 0), -12, 12),
		},
		"watermark": map[string]any{},
	}
	if len(references) > 0 {
		body["references"] = references
	}
	var payload map[string]any
	raw, err := v.doAudio(ctx, body, apiKey, &payload)
	if err != nil {
		return Result{}, err
	}
	if code := numericCode(payload["code"]); code != 0 {
		return Result{Failed: true, Error: v.redact(errorMessage(payload, fmt.Sprintf("豆包音频生成失败，code=%d", code))), Raw: raw}, nil
	}
	if outputURL := deepString(payload, isURL, "audio_url", "audioUrl", "url", "output_url", "file_url", "content"); outputURL != "" {
		return Result{Done: true, URL: outputURL, MIME: audioMIME(format), Raw: raw}, nil
	}
	encoded := deepString(payload, isAudioBase64, "audio", "data", "audio_base64", "audioBase64", "base64", "b64_json")
	if encoded != "" {
		decoded, err := decodeAudio(encoded)
		if err != nil {
			return Result{}, err
		}
		// Do not persist a potentially large Base64 response in the task record.
		return Result{Done: true, Data: decoded, MIME: audioMIME(format)}, nil
	}
	return Result{Failed: true, Error: "豆包音频生成完成但未返回音频", Raw: raw}, nil
}

func (v *Volcengine) doAudio(ctx context.Context, body any, apiKey string, target any) ([]byte, error) {
	encoded, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, v.cfg.AudioEndpoint, bytes.NewReader(encoded))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Api-Key", normalizeCredential(apiKey))
	req.Header.Set("X-Api-Request-Id", randomRequestID())
	resp, err := v.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request volcengine audio: %w", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 256<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return raw, &HTTPError{Status: resp.StatusCode, Message: v.redact(compact(raw))}
	}
	if err := json.Unmarshal(raw, target); err != nil {
		return raw, fmt.Errorf("decode volcengine audio response: %w", err)
	}
	return raw, nil
}

func normalizeAudioFormat(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "mp3", "wav", "pcm", "ogg_opus":
		return strings.ToLower(strings.TrimSpace(raw))
	case "mpeg":
		return "mp3"
	case "wave":
		return "wav"
	case "ogg", "opus":
		return "ogg_opus"
	default:
		return ""
	}
}

func audioMIME(format string) string {
	return map[string]string{"mp3": "audio/mpeg", "wav": "audio/wav", "pcm": "audio/pcm", "ogg_opus": "audio/ogg"}[format]
}

func decodeAudio(raw string) ([]byte, error) {
	if idx := strings.Index(raw, ","); strings.HasPrefix(raw, "data:audio/") && idx >= 0 {
		raw = raw[idx+1:]
	}
	return base64.StdEncoding.DecodeString(raw)
}

func randomRequestID() string {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return fmt.Sprint(time.Now().UnixNano())
	}
	return hex.EncodeToString(value[:])
}

func clampInt(value, minimum, maximum int) int {
	if value < minimum {
		return minimum
	}
	if value > maximum {
		return maximum
	}
	return value
}

func numericCode(value any) int {
	switch typed := value.(type) {
	case float64:
		return int(typed)
	case int:
		return typed
	case json.Number:
		parsed, _ := strconv.Atoi(typed.String())
		return parsed
	case string:
		parsed, _ := strconv.Atoi(strings.TrimSpace(typed))
		return parsed
	default:
		return 0
	}
}

func isURL(value string) bool {
	return strings.HasPrefix(value, "https://") || strings.HasPrefix(value, "http://")
}

func isAudioBase64(value string) bool {
	return strings.HasPrefix(value, "data:audio/") || (!isURL(value) && len(value) > 64)
}

func deepString(value any, accept func(string) bool, keys ...string) string {
	wanted := map[string]bool{}
	for _, key := range keys {
		wanted[key] = true
	}
	var visit func(any) string
	visit = func(current any) string {
		switch typed := current.(type) {
		case map[string]any:
			for key, child := range typed {
				if wanted[key] {
					if text, ok := child.(string); ok && accept(strings.TrimSpace(text)) {
						return strings.TrimSpace(text)
					}
				}
			}
			for _, child := range typed {
				if found := visit(child); found != "" {
					return found
				}
			}
		case []any:
			for _, child := range typed {
				if found := visit(child); found != "" {
					return found
				}
			}
		}
		return ""
	}
	return visit(value)
}

func (v *Volcengine) submitText(ctx context.Context, req Request) (Result, error) {
	if strings.TrimSpace(req.Model) == "" {
		return Result{}, fmt.Errorf("model key is required")
	}
	prompt := strings.TrimSpace(req.Prompt)
	if prompt == "" {
		return Result{}, fmt.Errorf("prompt is required")
	}
	thinking := strings.ToLower(stringParam(req.Params, "thinking", "disabled"))
	if thinking != "enabled" && thinking != "disabled" && thinking != "auto" {
		return Result{}, fmt.Errorf("thinking must be enabled, disabled, or auto")
	}
	body := map[string]any{
		"model":    req.Model,
		"messages": []map[string]string{{"role": "user", "content": prompt}},
		"thinking": map[string]string{"type": thinking},
		"stream":   false,
	}
	if temperature, ok := numberParam(req.Params, "temperature"); ok {
		if temperature < 0 || temperature > 2 {
			return Result{}, fmt.Errorf("temperature must be between 0 and 2")
		}
		body["temperature"] = temperature
	}
	if maxTokens := intParam(req.Params, "maxTokens", 0); maxTokens > 0 {
		body["max_tokens"] = maxTokens
	}
	var payload map[string]any
	raw, err := v.do(ctx, http.MethodPost, v.cfg.BaseURL+"/api/v3/chat/completions", body, &payload)
	if err != nil {
		return Result{}, err
	}
	if message := apiError(payload); message != "" {
		return Result{Failed: true, Error: v.redact(message), Raw: raw}, nil
	}
	choices, _ := payload["choices"].([]any)
	if len(choices) == 0 {
		return Result{Failed: true, Error: "文本生成完成但未返回文本", Raw: raw}, nil
	}
	choice, _ := choices[0].(map[string]any)
	message, _ := choice["message"].(map[string]any)
	text := strings.TrimSpace(firstString(message, "content"))
	if text == "" {
		return Result{Failed: true, Error: "文本生成完成但未返回文本", Raw: raw}, nil
	}
	return Result{Done: true, Text: text, Raw: raw}, nil
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
	if v, ok := m[key].(json.Number); ok {
		if parsed, err := v.Int64(); err == nil {
			return int(parsed)
		}
	}
	return fallback
}
func numberParam(m map[string]any, key string) (float64, bool) {
	switch value := m[key].(type) {
	case float64:
		return value, true
	case float32:
		return float64(value), true
	case int:
		return float64(value), true
	case json.Number:
		parsed, err := value.Float64()
		return parsed, err == nil
	default:
		return 0, false
	}
}
func compact(raw []byte) string {
	s := strings.TrimSpace(string(raw))
	if len(s) > 500 {
		return s[:500]
	}
	return s
}

func normalizeCredential(secret string) string {
	secret = strings.TrimSpace(secret)
	if len(secret) >= 7 && strings.EqualFold(secret[:7], "Bearer ") {
		return strings.TrimSpace(secret[7:])
	}
	return secret
}

func (v *Volcengine) redact(message string) string {
	for _, secret := range []string{v.cfg.APIKey, v.cfg.AudioAPIKey} {
		for _, variant := range []string{strings.TrimSpace(secret), normalizeCredential(secret)} {
			if variant != "" {
				message = strings.ReplaceAll(message, variant, "[REDACTED]")
			}
		}
	}
	return message
}
