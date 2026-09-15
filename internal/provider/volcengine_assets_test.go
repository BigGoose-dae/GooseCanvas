package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/BigGoose-dae/GooseCanvas/internal/config"
)

func TestAssetsClientListSignsAndParsesRemoteCount(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("Action") != "ListAssets" || r.URL.Query().Get("Version") != "2024-01-01" {
			t.Errorf("unexpected query: %s", r.URL.RawQuery)
		}
		if !strings.Contains(r.Header.Get("Authorization"), "Credential=test-ak/") || !strings.Contains(r.Header.Get("Authorization"), "Signature=") {
			t.Errorf("request was not signed: %q", r.Header.Get("Authorization"))
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body["ProjectName"] != "demo" {
			t.Errorf("project = %v", body["ProjectName"])
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ResponseMetadata":{"RequestId":"req-1"},"Result":{"Items":[{"Id":"asset-1","Name":"actor","AssetType":"Image","Status":"Active","ProjectName":"demo"}],"TotalCount":17,"PageNumber":1,"PageSize":100}}`))
	}))
	defer server.Close()
	client := NewAssetsClient(config.Volcengine{AssetsAccessKey: "test-ak", AssetsSecretKey: "test-sk", AssetsProjectName: "demo", AssetsBaseURL: server.URL})
	page, err := client.List(context.Background(), 1, 100)
	if err != nil {
		t.Fatal(err)
	}
	if page.TotalCount != 17 || len(page.Items) != 1 || page.Items[0].ID != "asset-1" {
		t.Fatalf("unexpected page: %+v", page)
	}
}
