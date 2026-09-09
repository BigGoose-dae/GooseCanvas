package httpapi

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/goosecanvas/goosecanvas/internal/config"
	"github.com/goosecanvas/goosecanvas/internal/database"
	"github.com/goosecanvas/goosecanvas/internal/domain"
	"github.com/goosecanvas/goosecanvas/internal/events"
	"github.com/goosecanvas/goosecanvas/internal/runtime"
)

func featureAPI(t *testing.T) (*API, *gin.Engine) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	dir := t.TempDir()
	db, err := database.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { sql, _ := db.DB(); sql.Close() })
	cfg := config.Config{AppName: "Test", DataDir: dir, Volc: config.Volcengine{BaseURL: "https://ark.example.invalid"}, TOS: config.TOS{Endpoint: "https://tos-cn-beijing.volces.com", Region: "cn-beijing", Prefix: "test"}, Worker: config.Worker{Concurrency: 4, PollInterval: time.Second, TaskTimeout: 30 * time.Minute}}
	manager, err := runtime.New(db, cfg)
	if err != nil {
		t.Fatal(err)
	}
	a := &API{DB: db, Runtime: manager, Events: events.New()}
	r := gin.New()
	a.Register(r)
	db.Create(&domain.Workspace{ID: 1, Name: "test"})
	return a, r
}

func jsonCall(t *testing.T, r http.Handler, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	raw, _ := json.Marshal(body)
	req := httptest.NewRequest(method, path, bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	r.ServeHTTP(recorder, req)
	return recorder
}

func TestSettingsPersistWithoutReturningSecrets(t *testing.T) {
	a, r := featureAPI(t)
	body := settingsRequest{AppName: "Friendly Canvas", BaseURL: "https://ark.example.invalid", APIKey: "private-api-key", Endpoint: "https://tos-cn-beijing.volces.com", Region: "cn-beijing", Bucket: "test", AccessKey: "private-access", SecretKey: "private-secret", Concurrency: 6, TimeoutMinutes: 45}
	response := jsonCall(t, r, "PUT", "/api/v1/settings", body)
	if response.Code != 200 {
		t.Fatalf("settings: %d %s", response.Code, response.Body.String())
	}
	body.APIKey = ""
	body.AccessKey = ""
	body.SecretKey = ""
	body.Concurrency = 3
	if result := jsonCall(t, r, "PUT", "/api/v1/settings", body); result.Code != 200 {
		t.Fatal(result.Body.String())
	}
	read := jsonCall(t, r, "GET", "/api/v1/settings", nil)
	for _, secret := range []string{"private-api-key", "private-access", "private-secret"} {
		if strings.Contains(read.Body.String(), secret) {
			t.Fatal("secret leaked in settings response")
		}
	}
	restarted, err := runtime.New(a.DB, a.Runtime.Snapshot().Config)
	if err != nil {
		t.Fatal(err)
	}
	cfg := restarted.Snapshot().Config
	if cfg.Volc.APIKey != "private-api-key" || cfg.TOS.SecretKey != "private-secret" || cfg.Worker.Concurrency != 3 {
		t.Fatal("settings or retained secrets lost on restart")
	}
	body.Concurrency = 0
	if result := jsonCall(t, r, "PUT", "/api/v1/settings", body); result.Code != 400 {
		t.Fatal("invalid concurrency accepted")
	}
	if a.Runtime.Snapshot().Config.Worker.Concurrency != 3 {
		t.Fatal("invalid setting mutated runtime")
	}
}

func TestNodeCopyAndAutosaveDoNotOverwriteGeneratedAsset(t *testing.T) {
	a, r := featureAPI(t)
	assetID := uint64(99)
	node := domain.Node{WorkspaceID: 1, NodeType: "image", Title: "original", Prompt: "draft", ModelKey: "model", Params: `{"ratio":"16:9"}`, PosX: 12, PosY: 24, Width: 320, Height: 240, CurrentAssetID: &assetID}
	a.DB.Create(&node)
	response := jsonCall(t, r, "POST", "/api/v1/nodes/1/duplicate", nil)
	if response.Code != 201 {
		t.Fatalf("copy: %s", response.Body.String())
	}
	var duplicate domain.Node
	json.Unmarshal(response.Body.Bytes(), &duplicate)
	if duplicate.ID == node.ID || duplicate.Prompt != node.Prompt || duplicate.PosX == node.PosX || duplicate.CurrentAssetID == nil || *duplicate.CurrentAssetID != assetID {
		t.Fatalf("invalid copy: %+v", duplicate)
	}
	saved := jsonCall(t, r, "PUT", "/api/v1/nodes/1", map[string]any{"prompt": "new text", "assetId": 12, "params": map[string]any{}})
	if saved.Code != 204 {
		t.Fatal(saved.Body.String())
	}
	var current domain.Node
	a.DB.First(&current, 1)
	if current.Prompt != "new text" || *current.CurrentAssetID != 99 {
		t.Fatal("autosave overwrote generated result")
	}
	invalid := jsonCall(t, r, "POST", "/api/v1/workspaces/999/nodes", map[string]any{"nodeType": "text"})
	if invalid.Code != 404 {
		t.Fatal("nonexistent workspace accepted")
	}
}

func TestRetryResumesOriginalExternalTaskWithFreshTimeout(t *testing.T) {
	a, r := featureAPI(t)
	node := domain.Node{WorkspaceID: 1, NodeType: "video", Params: "{}"}
	a.DB.Create(&node)
	old := time.Now().Add(-time.Hour)
	session := domain.GenerationSession{WorkspaceID: 1, NodeID: node.ID, TaskType: "video", ModelKey: "model", Prompt: "test", Params: "{}", Status: domain.StatusFailed, StartedAt: &old, CreatedAt: old}
	a.DB.Create(&session)
	task := domain.GenerationTask{SessionID: session.ID, Provider: "volcengine", Status: domain.StatusFailed, ExternalTaskID: "original-id", RetryMode: "query"}
	a.DB.Create(&task)
	response := jsonCall(t, r, "POST", "/api/v1/generations/1/retry", nil)
	if response.Code != 201 {
		t.Fatal(response.Body.String())
	}
	var current domain.GenerationSession
	a.DB.First(&current, session.ID)
	var currentTask domain.GenerationTask
	a.DB.First(&currentTask, task.ID)
	if current.Status != domain.StatusProcessing || current.StartedAt == nil || time.Since(*current.StartedAt) > time.Minute || currentTask.ExternalTaskID != "original-id" {
		t.Fatalf("bad resume: %+v %+v", current, currentTask)
	}
	if duplicate := jsonCall(t, r, "POST", "/api/v1/generations/1/retry", nil); duplicate.Code != 409 {
		t.Fatal("active task was retried")
	}
}

func TestHistoryPagination(t *testing.T) {
	a, r := featureAPI(t)
	for i := 0; i < 25; i++ {
		a.DB.Create(&domain.GenerationSession{WorkspaceID: 1, NodeID: 1, TaskType: "image", ModelKey: "model", Prompt: "test", Params: "{}", Status: domain.StatusSucceeded})
	}
	first := jsonCall(t, r, "GET", "/api/v1/nodes/1/history", nil)
	var page struct {
		Items   []generationView `json:"items"`
		HasMore bool             `json:"hasMore"`
	}
	if err := json.Unmarshal(first.Body.Bytes(), &page); err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 20 || !page.HasMore || page.Items[0].ID != 25 {
		t.Fatal("first history page invalid")
	}
	second := jsonCall(t, r, "GET", "/api/v1/nodes/1/history?before=6", nil)
	json.Unmarshal(second.Body.Bytes(), &page)
	if len(page.Items) != 5 || page.HasMore {
		t.Fatal("older history page invalid")
	}
}

func TestSSEInitialSnapshotUpdatesAndReconnect(t *testing.T) {
	a, r := featureAPI(t)
	node := domain.Node{WorkspaceID: 1, NodeType: "text", Prompt: "do not send draft", Params: "{}"}
	a.DB.Create(&node)
	server := httptest.NewServer(r)
	defer server.Close()
	connect := func() (ioReader *bufio.Scanner, closeStream func()) {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		req, _ := http.NewRequestWithContext(ctx, "GET", server.URL+"/api/v1/workspaces/1/events", nil)
		response, err := http.DefaultClient.Do(req)
		if err != nil {
			cancel()
			t.Fatal(err)
		}
		if response.Header.Get("Content-Type") != "text/event-stream" {
			t.Fatal("SSE header missing")
		}
		return bufio.NewScanner(response.Body), func() { cancel(); response.Body.Close() }
	}
	read := func(scanner *bufio.Scanner) string {
		for scanner.Scan() {
			if strings.HasPrefix(scanner.Text(), "data: ") {
				return strings.TrimPrefix(scanner.Text(), "data: ")
			}
		}
		t.Fatalf("missing snapshot: %v", scanner.Err())
		return ""
	}
	scanner, closeStream := connect()
	initial := read(scanner)
	if strings.Contains(initial, "do not send draft") {
		t.Fatal("SSE sent editable node data")
	}
	session := domain.GenerationSession{WorkspaceID: 1, NodeID: node.ID, TaskType: "image", ModelKey: "mock", Prompt: "task snapshot", Params: "{}", Status: domain.StatusProcessing}
	a.DB.Create(&session)
	a.Events.Notify(1)
	if update := read(scanner); !strings.Contains(update, `"status":"processing"`) {
		t.Fatal("task update missing")
	}
	closeStream()
	a.DB.Model(&session).Update("status", domain.StatusSucceeded)
	scanner, closeStream = connect()
	defer closeStream()
	if update := read(scanner); !strings.Contains(update, `"status":"succeeded"`) {
		t.Fatal("reconnect did not recover latest state")
	}
}
