package generation

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/BigGoose-dae/GooseCanvas/internal/config"
	"github.com/BigGoose-dae/GooseCanvas/internal/database"
	"github.com/BigGoose-dae/GooseCanvas/internal/domain"
	"github.com/BigGoose-dae/GooseCanvas/internal/events"
	"github.com/BigGoose-dae/GooseCanvas/internal/provider"
	"github.com/BigGoose-dae/GooseCanvas/internal/runtime"
	"github.com/BigGoose-dae/GooseCanvas/internal/storage"
	"gorm.io/gorm"
)

type fakeStore struct{ fail atomic.Bool }

func (s *fakeStore) Ready(context.Context) error { return nil }
func (s *fakeStore) Put(_ context.Context, _ string, _ string, _ int64, r io.Reader) (storage.PutResult, error) {
	if s.fail.Load() {
		return storage.PutResult{}, fmt.Errorf("storage temporarily unavailable")
	}
	_, err := io.Copy(io.Discard, r)
	return storage.PutResult{}, err
}
func (s *fakeStore) PresignGet(context.Context, string, time.Duration) (string, error) {
	return "https://example.invalid/input", nil
}
func (s *fakeStore) Delete(context.Context, string) error { return nil }

type fakeProvider struct {
	submit func(context.Context, provider.Request) (provider.Result, error)
	poll   func(context.Context, string) (provider.Result, error)
	calls  atomic.Int32
}

func (p *fakeProvider) Submit(ctx context.Context, req provider.Request) (provider.Result, error) {
	p.calls.Add(1)
	return p.submit(ctx, req)
}
func (p *fakeProvider) Poll(ctx context.Context, id string) (provider.Result, error) {
	return p.poll(ctx, id)
}

func testEngine(t *testing.T, p provider.Provider, concurrency int) (*Engine, *gorm.DB, *fakeStore) {
	t.Helper()
	dir := t.TempDir()
	db, err := database.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { sql, _ := db.DB(); sql.Close() })
	cfg := config.Config{DataDir: dir, Volc: config.Volcengine{APIKey: "test"}, TOS: config.TOS{Bucket: "test", Region: "cn-beijing", Endpoint: "https://tos-cn-beijing.volces.com", AccessKey: "test", SecretKey: "test", Prefix: "review", PresignTTL: time.Hour}, Worker: config.Worker{Concurrency: concurrency, PollInterval: time.Hour, TaskTimeout: time.Minute}}
	tos, err := storage.NewTOS(cfg.TOS)
	if err != nil {
		t.Fatal(err)
	}
	store := &fakeStore{}
	snap := runtime.Snapshot{Config: cfg, Store: store, TOS: tos, Provider: p}
	e := &Engine{db: db, snapshot: func() runtime.Snapshot { return snap }, hub: events.New(), active: map[uint64]bool{}, clients: map[uint64]runtime.Snapshot{}}
	return e, db, store
}

func addTask(t *testing.T, db *gorm.DB, prompt string) domain.GenerationTask {
	t.Helper()
	node := domain.Node{WorkspaceID: 1, NodeType: "image", Title: prompt, Params: "{}"}
	if err := db.Create(&node).Error; err != nil {
		t.Fatal(err)
	}
	session := domain.GenerationSession{WorkspaceID: 1, NodeID: node.ID, TaskType: "image", ModelKey: "mock", Prompt: prompt, Params: "{}", Status: domain.StatusPending}
	if err := db.Create(&session).Error; err != nil {
		t.Fatal(err)
	}
	task := domain.GenerationTask{SessionID: session.ID, Provider: "mock", Status: domain.StatusPending}
	if err := db.Create(&task).Error; err != nil {
		t.Fatal(err)
	}
	return task
}

func awaitStatus(t *testing.T, db *gorm.DB, id uint64, want string) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		var task domain.GenerationTask
		db.First(&task, id)
		if task.Status == want {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	var task domain.GenerationTask
	db.First(&task, id)
	t.Fatalf("task %d: got %s, want %s; %s", id, task.Status, want, task.ErrorMessage)
}

func TestSlowTaskDoesNotBlockIndependentTask(t *testing.T) {
	blocked := make(chan struct{})
	entered := make(chan struct{})
	p := &fakeProvider{submit: func(ctx context.Context, r provider.Request) (provider.Result, error) {
		if r.Prompt == "slow" {
			close(entered)
			select {
			case <-blocked:
			case <-ctx.Done():
				return provider.Result{}, ctx.Err()
			}
		}
		return provider.Result{Done: true, Data: []byte("image"), MIME: "image/png"}, nil
	}}
	e, db, _ := testEngine(t, p, 2)
	slow := addTask(t, db, "slow")
	fast := addTask(t, db, "fast")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	e.dispatch(ctx)
	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("slow task did not start")
	}
	awaitStatus(t, db, fast.ID, domain.StatusSucceeded)
	var current domain.GenerationTask
	db.First(&current, slow.ID)
	if current.Status != domain.StatusSubmitting {
		t.Fatalf("slow task is not isolated: %s", current.Status)
	}
	close(blocked)
	e.Wait()
	awaitStatus(t, db, slow.ID, domain.StatusSucceeded)
}

func TestWaitingVideoReleasesConcurrencySlot(t *testing.T) {
	p := &fakeProvider{submit: func(_ context.Context, r provider.Request) (provider.Result, error) {
		if r.Prompt == "video" {
			return provider.Result{ExternalID: "external-video"}, nil
		}
		return provider.Result{Done: true, Data: []byte("image"), MIME: "image/png"}, nil
	}}
	e, db, _ := testEngine(t, p, 1)
	video := addTask(t, db, "video")
	fast := addTask(t, db, "fast")
	e.dispatch(context.Background())
	e.Wait()
	awaitStatus(t, db, video.ID, domain.StatusProcessing)
	e.dispatch(context.Background())
	e.Wait()
	awaitStatus(t, db, fast.ID, domain.StatusSucceeded)
}

func TestArchiveRetryDoesNotResubmitProvider(t *testing.T) {
	p := &fakeProvider{submit: func(context.Context, provider.Request) (provider.Result, error) {
		return provider.Result{Done: true, Data: []byte("image"), MIME: "image/png"}, nil
	}}
	e, db, store := testEngine(t, p, 1)
	task := addTask(t, db, "image")
	store.fail.Store(true)
	e.dispatch(context.Background())
	e.Wait()
	var failed domain.GenerationTask
	db.First(&failed, task.ID)
	if failed.Status != domain.StatusFailed || failed.RetryMode != "archive" || failed.ResultPayload == "" {
		t.Fatalf("lost archive recovery: %+v", failed)
	}
	store.fail.Store(false)
	db.Model(&failed).Updates(map[string]any{"status": domain.StatusArchiving, "next_poll_at": nil})
	e.dispatch(context.Background())
	e.Wait()
	awaitStatus(t, db, task.ID, domain.StatusSucceeded)
	if p.calls.Load() != 1 {
		t.Fatalf("provider submitted %d times", p.calls.Load())
	}
}

func TestRestartDoesNotResubmitUncertainSubmission(t *testing.T) {
	p := &fakeProvider{submit: func(context.Context, provider.Request) (provider.Result, error) {
		return provider.Result{}, fmt.Errorf("unexpected submission")
	}}
	e, db, _ := testEngine(t, p, 1)
	task := addTask(t, db, "interrupted")
	db.Model(&task).Update("status", domain.StatusSubmitting)
	ctx, cancel := context.WithCancel(context.Background())
	e.Start(ctx)
	cancel()
	e.Wait()
	awaitStatus(t, db, task.ID, domain.StatusFailed)
	if p.calls.Load() != 0 {
		t.Fatal("uncertain provider submission was repeated")
	}
}
