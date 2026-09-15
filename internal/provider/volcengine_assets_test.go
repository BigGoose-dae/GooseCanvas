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

func TestAssetsClientListsGroupsAndGetsFreshDownloadURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Query().Get("Action") {
		case "ListAssetGroups":
			_, _ = w.Write([]byte(`{"Result":{"Items":[{"Id":"group-1","Name":"Characters","Description":"approved people","GroupType":"AIGC","ProjectName":"default"}],"TotalCount":1,"PageNumber":1,"PageSize":100}}`))
		case "GetAsset":
			_, _ = w.Write([]byte(`{"Result":{"Id":"asset-1","Name":"Actor","GroupId":"group-1","AssetType":"Image","Status":"Active","URL":"https://cdn.example/temporary.jpg","ProjectName":"default"}}`))
		default:
			t.Fatalf("unexpected action %q", r.URL.Query().Get("Action"))
		}
	}))
	defer server.Close()
	client := NewAssetsClient(config.Volcengine{AssetsAccessKey: "ak", AssetsSecretKey: "sk", AssetsProjectName: "default", AssetsBaseURL: server.URL})
	groups, err := client.ListGroups(context.Background(), 1, 100)
	if err != nil {
		t.Fatal(err)
	}
	if groups.TotalCount != 1 || groups.Items[0].Name != "Characters" {
		t.Fatalf("groups = %+v", groups)
	}
	asset, err := client.Get(context.Background(), "asset-1")
	if err != nil {
		t.Fatal(err)
	}
	if asset.URL != "https://cdn.example/temporary.jpg" || asset.GroupID != "group-1" {
		t.Fatalf("asset = %+v", asset)
	}
}
