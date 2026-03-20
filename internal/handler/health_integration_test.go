package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func setupHealthIntegrationRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	h := NewHealthHandler()

	r.GET("/health", h.HealthCheck)
	r.GET("/", h.Welcome)

	return r
}

func TestHealthCheckEndpoint(t *testing.T) {
	t.Parallel()

	r := setupHealthIntegrationRouter()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d", w.Code, http.StatusOK)
	}

	var body map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if body["status"] != "ok" {
		t.Fatalf("status field = %q, want ok", body["status"])
	}
	if body["message"] != "Server is running" {
		t.Fatalf("message field = %q, want %q", body["message"], "Server is running")
	}
}

func TestWelcomeEndpoint(t *testing.T) {
	t.Parallel()

	r := setupHealthIntegrationRouter()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d", w.Code, http.StatusOK)
	}

	var body map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if body["message"] != "Welcome to Foods & Drinks API" {
		t.Fatalf("message field = %q, want %q", body["message"], "Welcome to Foods & Drinks API")
	}
	if body["version"] != "v1" {
		t.Fatalf("version field = %q, want v1", body["version"])
	}
}
