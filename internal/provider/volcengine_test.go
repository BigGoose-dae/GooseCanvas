package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
		if !ok || len(images) != 1 || images[0] != "https://storage.example/input.png" {
			t.Fatalf("image input = %#v", body["image"])
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"url":"https://provider.example/result.png"}]}`))
	}))
	defer server.Close()

	client := NewVolcengine(config.Volcengine{APIKey: "test-key", BaseURL: server.URL})
	result, err := client.Submit(context.Background(), Request{TaskType: "image", Model: "image-model", Prompt: "a goose", Inputs: []Input{{Type: "image", URL: "https://storage.example/input.png"}}})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Done || result.URL != "https://provider.example/result.png" {
		t.Fatalf("result = %#v", result)
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
			_, _ = w.Write([]byte(`{"id":"video-task-1"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v3/contents/generations/tasks/video-task-1":
			_, _ = w.Write([]byte(`{"status":"succeeded","content":{"video_url":"https://provider.example/result.mp4"}}`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	client := NewVolcengine(config.Volcengine{APIKey: "test-key", BaseURL: server.URL})
	submitted, err := client.Submit(context.Background(), Request{TaskType: "video", Model: "video-model", Prompt: "take flight", Inputs: []Input{{Type: "image", Role: "reference", URL: "https://storage.example/input.png"}}})
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
