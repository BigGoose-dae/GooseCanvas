package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/BigGoose-dae/GooseCanvas/internal/config"
	"github.com/gin-gonic/gin"
)

func TestStatusHandlesMissingStore(t *testing.T) {
	gin.SetMode(gin.TestMode)
	api := &API{Config: config.Config{}}
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
	if body.Ready || body.Storage.Status != "error" {
		t.Fatalf("unexpected response: %s", response.Body.String())
	}
}
