package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/goosecanvas/goosecanvas/internal/config"
	"github.com/goosecanvas/goosecanvas/internal/storage"
)

func TestStatusHandlesTypedNilStore(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var typedNil *storage.TOSStore
	api := &API{Config: config.Config{}, Store: typedNil, TOS: typedNil}
	router := gin.New()
	router.GET("/status", api.status)

	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/status", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("status code = %d, body = %s", response.Code, response.Body.String())
	}
	var body struct {
		Ready   bool `json:"ready"`
		Storage struct {
			Status string `json:"status"`
		} `json:"storage"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Ready || body.Storage.Status != "not_configured" {
		t.Fatalf("unexpected response: %s", response.Body.String())
	}
}
