package provider

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/BigGoose-dae/GooseCanvas/internal/config"
)

func TestSubmitImage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v3/images/generations" || r.Method != http.MethodPost {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Fatalf("authorization header was not set")
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		images, ok := body["image"].([]any)
		if !ok || len(images) != 1 || images[0] != "data:image/png;base64,aW1hZ2U=" {
			t.Fatalf("image input = %#v", body["image"])
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"url":"https://provider.example/result.png"}]}`))
	}))
	defer server.Close()

	client := NewVolcengine(config.Volcengine{APIKey: "test-key", BaseURL: server.URL})
	result, err := client.Submit(context.Background(), Request{TaskType: "image", Model: "image-model", Prompt: "a goose", Inputs: []Input{{Type: "image", DataURI: "data:image/png;base64,aW1hZ2U="}}})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Done || result.URL != "https://provider.example/result.png" {
		t.Fatalf("result = %#v", result)
	}
}

func TestSubmitText(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v3/chat/completions" || r.Method != http.MethodPost {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body["model"] != "doubao-seed-2-1-pro-260628" || body["stream"] != false {
			t.Fatalf("request body = %#v", body)
		}
		messages, _ := body["messages"].([]any)
		message, _ := messages[0].(map[string]any)
		thinking, _ := body["thinking"].(map[string]any)
		if message["role"] != "user" || message["content"] != "rewrite this" || thinking["type"] != "disabled" {
			t.Fatalf("text request = %#v", body)

		}
		if body["temperature"] != 0.7 || body["max_tokens"] != float64(4096) {
			t.Fatalf("text parameters = %#v", body)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"Rewritten text"}}]}`))
	}))
	defer server.Close()

	client := NewVolcengine(config.Volcengine{APIKey: "test-key", BaseURL: server.URL})
	result, err := client.Submit(context.Background(), Request{
		TaskType: "text", Model: "doubao-seed-2-1-pro-260628", Prompt: "rewrite this",
		Params: map[string]any{"temperature": 0.7, "maxTokens": 4096, "thinking": "disabled"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Done || result.Text != "Rewritten text" {
		t.Fatalf("result = %#v", result)
	}
}

func TestSubmitAudioWithLocalReference(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/v3/tts/create" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("X-Api-Key") != "audio-key" || r.Header.Get("X-Api-Request-Id") == "" {
			t.Fatalf("audio headers = %#v", r.Header)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body["model"] != "seed-audio-1.0" || body["text_prompt"] != "warm narration" {
			t.Fatalf("audio request = %#v", body)
		}
		references, _ := body["references"].([]any)
		if len(references) != 1 {
			t.Fatalf("audio references = %#v", body["references"])
		}
		reference, _ := references[0].(map[string]any)
		if reference["audio_url"] != "data:audio/mpeg;base64,cmVmZXJlbmNl" {
			t.Fatalf("audio references = %#v", body["references"])
		}
		config, _ := body["audio_config"].(map[string]any)
		if config["format"] != "mp3" || config["sample_rate"] != float64(24000) || config["speech_rate"] != float64(25) {
			t.Fatalf("audio config = %#v", config)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":0,"data":{"audio":"data:audio/mpeg;base64,Z2VuZXJhdGVkLWF1ZGlv"}}`))
	}))
	defer server.Close()

	client := NewVolcengine(config.Volcengine{AudioAPIKey: "bearer audio-key", AudioEndpoint: server.URL + "/api/v3/tts/create"})
	result, err := client.Submit(context.Background(), Request{
		TaskType: "audio", Prompt: "warm narration",
		Params: map[string]any{"providerModel": "seed-audio-1.0", "responseFormat": "mp3", "sampleRate": 24000, "speechRate": 25},
		Inputs: []Input{{Type: "audio", DataURI: "data:audio/mpeg;base64,cmVmZXJlbmNl"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Done || string(result.Data) != "generated-audio" || result.MIME != "audio/mpeg" {
		t.Fatalf("audio result = %#v", result)
	}
	if len(result.Raw) != 0 {
		t.Fatal("base64 audio response must not be retained in task payload")
	}
}

func TestSubmitAndPollVideo(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v3/contents/generations/tasks":
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			content, ok := body["content"].([]any)
			if !ok || len(content) != 2 {
				t.Fatalf("content = %#v", body["content"])
			}
			input, _ := content[1].(map[string]any)
			imageURL, _ := input["image_url"].(map[string]any)
			if imageURL["url"] != "data:image/png;base64,aW1hZ2U=" {
				t.Fatalf("video image input = %#v", input)
			}
			_, _ = w.Write([]byte(`{"id":"video-task-1"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v3/contents/generations/tasks/video-task-1":
			_, _ = w.Write([]byte(`{"status":"succeeded","content":{"video_url":"https://provider.example/result.mp4"}}`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	client := NewVolcengine(config.Volcengine{APIKey: "test-key", BaseURL: server.URL})
	submitted, err := client.Submit(context.Background(), Request{TaskType: "video", Model: "video-model", Prompt: "take flight", Inputs: []Input{{Type: "image", Role: "reference", DataURI: "data:image/png;base64,aW1hZ2U="}}})
	if err != nil {
		t.Fatal(err)
	}
	if submitted.ExternalID != "video-task-1" {
		t.Fatalf("external id = %q", submitted.ExternalID)
	}
	result, err := client.Poll(context.Background(), submitted.ExternalID)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Done || result.URL != "https://provider.example/result.mp4" {
		t.Fatalf("result = %#v", result)
	}
}

func TestProviderErrorRedactsCredential(t *testing.T) {
	secret := "fixture-secret-that-must-not-be-logged"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
		json.NewEncoder(w).Encode(map[string]any{"error": map[string]any{"message": "Rejected " + secret}})
	}))
	defer server.Close()
	client := NewVolcengine(config.Volcengine{APIKey: secret, BaseURL: server.URL})
	_, err := client.Submit(context.Background(), Request{TaskType: "image", Model: "mock"})
	if err == nil || strings.Contains(err.Error(), secret) || !strings.Contains(err.Error(), "[REDACTED]") {
		t.Fatal("provider error leaked credential")
	}
	var response *HTTPError
	if !errors.As(err, &response) || response.Status != 401 {
		t.Fatal("provider rejection was not classified")
	}
}

func TestAudioProviderErrorRedactsBearerCredential(t *testing.T) {
	secret := "fixture-audio-secret-that-must-not-be-logged"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"message":"Rejected ` + secret + `"}`))
	}))
	defer server.Close()

	client := NewVolcengine(config.Volcengine{AudioAPIKey: "Bearer " + secret, AudioEndpoint: server.URL})
	_, err := client.Submit(context.Background(), Request{TaskType: "audio", Prompt: "test"})
	if err == nil || strings.Contains(err.Error(), secret) || !strings.Contains(err.Error(), "[REDACTED]") {
		t.Fatal("audio provider error leaked credential")
	}
}
