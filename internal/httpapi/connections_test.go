package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/BigGoose-dae/GooseCanvas/internal/config"
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
