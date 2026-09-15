package httpapi

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/BigGoose-dae/GooseCanvas/internal/config"
	"github.com/BigGoose-dae/GooseCanvas/internal/provider"
	"github.com/BigGoose-dae/GooseCanvas/internal/storage"
)

func TestInferenceConnectionUsesPingAndBearerKey(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/ping" {
			t.Errorf("path = %q", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("authorization = %q", r.Header.Get("Authorization"))
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	if err := testInferenceConnection(context.Background(), config.Volcengine{APIKey: "test-key", BaseURL: server.URL + "/api/v3"}); err != nil {
		t.Fatal(err)
	}
}

func TestDownloadTrustedAssetArchivesRemoteFileLocally(t *testing.T) {
	media := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write([]byte("trusted-image"))
	}))
	defer media.Close()
	store, err := storage.NewLocal(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	a := &API{Store: store, Config: config.Config{Volc: config.Volcengine{}}}
	key, contentType, size, _, err := a.downloadTrustedAsset(context.Background(), 3, provider.AssetLibraryItem{ID: "asset-1", URL: media.URL + "/actor.png"}, "image")
	if err != nil {
		t.Fatal(err)
	}
	if contentType != "image/png" || size != int64(len("trusted-image")) {
		t.Fatalf("type=%q size=%d", contentType, size)
	}
	file, _, err := store.Open(context.Background(), key)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	got, _ := io.ReadAll(file)
	if string(got) != "trusted-image" {
		t.Fatalf("downloaded %q", got)
	}
}
