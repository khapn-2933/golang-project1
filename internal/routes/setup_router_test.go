package routes

import (
	"html/template"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/kha/foods-drinks/internal/config"
	"github.com/kha/foods-drinks/internal/handler"
	"github.com/kha/foods-drinks/internal/middleware"
	"github.com/kha/foods-drinks/internal/service"
)

func chdirRepoRoot(t *testing.T) {
	t.Helper()
	old, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir("../.."); err != nil {
		t.Fatalf("chdir repo root: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(old)
	})
}

func testTemplateFuncMap() template.FuncMap {
	return template.FuncMap{
		"inc": func(i int) int { return i + 1 },
		"dec": func(i int) int { return i - 1 },
		"formatVND": func(amount float64) string {
			value := int64(math.Round(amount))
			sign := ""
			if value < 0 {
				sign = "-"
				value = -value
			}
			raw := strconv.FormatInt(value, 10)
			n := len(raw)
			if n <= 3 {
				return sign + raw + "đ"
			}
			sepCount := (n - 1) / 3
			buf := make([]byte, n+sepCount)
			read := n - 1
			write := len(buf) - 1
			digitCount := 0
			for read >= 0 {
				buf[write] = raw[read]
				read--
				write--
				digitCount++
				if digitCount%3 == 0 && read >= 0 {
					buf[write] = '.'
					write--
				}
			}
			return sign + string(buf) + "đ"
		},
		"deref": func(s *string) string {
			if s == nil {
				return ""
			}
			return *s
		},
	}
}

func TestSetupRouter_BasicPublicEndpoints(t *testing.T) {
	chdirRepoRoot(t)

	gin.SetMode(gin.TestMode)
	funcMap := testTemplateFuncMap()
	authSvc := service.NewAuthServiceWithConfig(&config.JWTConfig{Secret: "router-test-secret", Expiration: time.Hour})
	authMW := middleware.NewAuthMiddleware(authSvc)

	deps := &RouterDependencies{
		HealthHandler:          handler.NewHealthHandler(),
		AuthHandler:            handler.NewAuthHandler(nil),
		OAuthHandler:           handler.NewOAuthHandler(nil),
		ProfileHandler:         handler.NewProfileHandler(nil),
		AdminCategoryHandler:   handler.NewAdminCategoryHandler(nil, funcMap),
		ProductHandler:         handler.NewProductHandler(nil),
		AdminProductHandler:    handler.NewAdminProductHandler(nil, nil, funcMap),
		AdminOrderHandler:      handler.NewAdminOrderHandler(nil, funcMap),
		AdminOrderStatsHandler: handler.NewAdminOrderStatisticsHandler(nil, funcMap),
		AdminSuggestionHandler: handler.NewAdminSuggestionHandler(nil, funcMap),
		AdminUserHandler:       handler.NewAdminUserHandler(nil, funcMap),
		CartHandler:            handler.NewCartHandler(nil),
		OrderHandler:           handler.NewOrderHandler(nil),
		RatingHandler:          handler.NewRatingHandler(nil),
		SuggestionHandler:      handler.NewSuggestionHandler(nil),
		CorsMiddleware:         middleware.CORSConfig(),
		AuthMiddleware:         authMW,
		UploadPath:             "",
	}

	r := SetupRouter(deps)

	wHealth := httptest.NewRecorder()
	reqHealth := httptest.NewRequest(http.MethodGet, "/health", nil)
	r.ServeHTTP(wHealth, reqHealth)
	if wHealth.Code != http.StatusOK {
		t.Fatalf("GET /health status = %d, want 200", wHealth.Code)
	}

	wRoot := httptest.NewRecorder()
	reqRoot := httptest.NewRequest(http.MethodGet, "/", nil)
	r.ServeHTTP(wRoot, reqRoot)
	if wRoot.Code != http.StatusOK {
		t.Fatalf("GET / status = %d, want 200", wRoot.Code)
	}
}

func TestSetupRouter_UploadPathSafe(t *testing.T) {
	chdirRepoRoot(t)

	gin.SetMode(gin.TestMode)
	funcMap := testTemplateFuncMap()
	authSvc := service.NewAuthServiceWithConfig(&config.JWTConfig{Secret: "router-test-secret", Expiration: time.Hour})
	authMW := middleware.NewAuthMiddleware(authSvc)

	deps := &RouterDependencies{
		HealthHandler:          handler.NewHealthHandler(),
		AuthHandler:            handler.NewAuthHandler(nil),
		OAuthHandler:           handler.NewOAuthHandler(nil),
		ProfileHandler:         handler.NewProfileHandler(nil),
		AdminCategoryHandler:   handler.NewAdminCategoryHandler(nil, funcMap),
		ProductHandler:         handler.NewProductHandler(nil),
		AdminProductHandler:    handler.NewAdminProductHandler(nil, nil, funcMap),
		AdminOrderHandler:      handler.NewAdminOrderHandler(nil, funcMap),
		AdminOrderStatsHandler: handler.NewAdminOrderStatisticsHandler(nil, funcMap),
		AdminSuggestionHandler: handler.NewAdminSuggestionHandler(nil, funcMap),
		AdminUserHandler:       handler.NewAdminUserHandler(nil, funcMap),
		CartHandler:            handler.NewCartHandler(nil),
		OrderHandler:           handler.NewOrderHandler(nil),
		RatingHandler:          handler.NewRatingHandler(nil),
		SuggestionHandler:      handler.NewSuggestionHandler(nil),
		CorsMiddleware:         middleware.CORSConfig(),
		AuthMiddleware:         authMW,
		UploadPath:             "uploads",
	}

	r := SetupRouter(deps)
	if r == nil {
		t.Fatal("expected router to be non-nil")
	}
}
