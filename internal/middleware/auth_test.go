package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/kha/foods-drinks/internal/config"
	"github.com/kha/foods-drinks/internal/models"
	"github.com/kha/foods-drinks/internal/service"
)

func setupAuthMiddlewareTest() (*AuthMiddleware, *config.JWTConfig) {
	jwtCfg := &config.JWTConfig{
		Secret:     "middleware-test-secret",
		Expiration: 24 * time.Hour,
	}
	authSvc := service.NewAuthServiceWithConfig(jwtCfg)
	m := NewAuthMiddleware(authSvc)
	return m, jwtCfg
}

func makeTokenForTest(jwtCfg *config.JWTConfig, user *models.User) string {
	svc := service.NewAuthServiceWithConfig(jwtCfg)
	token, _, _ := svc.GenerateToken(user)
	return token
}

func TestRequireAuth_MissingToken(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	m, _ := setupAuthMiddlewareTest()

	r := gin.New()
	r.Use(m.RequireAuth())
	r.GET("/protected", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestRequireAuth_InvalidToken(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	m, _ := setupAuthMiddlewareTest()

	r := gin.New()
	r.Use(m.RequireAuth())
	r.GET("/protected", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer invalid.token.here")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestRequireAuth_ValidToken(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	m, jwtCfg := setupAuthMiddlewareTest()

	user := &models.User{ID: 7, Email: "test@example.com", Role: models.RoleUser}
	token := makeTokenForTest(jwtCfg, user)

	r := gin.New()
	r.Use(m.RequireAuth())
	r.GET("/protected", func(c *gin.Context) {
		userID, _ := c.Get(ContextKeyUserID)
		c.JSON(http.StatusOK, gin.H{"user_id": userID})
	})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (body: %s)", w.Code, http.StatusOK, w.Body.String())
	}
}

func TestRequireAdmin_ForbiddenForUser(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	m, jwtCfg := setupAuthMiddlewareTest()

	user := &models.User{ID: 1, Email: "user@example.com", Role: models.RoleUser}
	token := makeTokenForTest(jwtCfg, user)

	r := gin.New()
	r.Use(m.RequireAuth())
	r.GET("/admin", m.RequireAdmin(), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusForbidden)
	}
}

func TestRequireAdmin_AllowedForAdmin(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	m, jwtCfg := setupAuthMiddlewareTest()

	admin := &models.User{ID: 99, Email: "admin@example.com", Role: models.RoleAdmin}
	token := makeTokenForTest(jwtCfg, admin)

	r := gin.New()
	r.Use(m.RequireAuth())
	r.GET("/admin", m.RequireAdmin(), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestRequireAuth_WrongTokenType(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	m, jwtCfg := setupAuthMiddlewareTest()

	user := &models.User{ID: 1, Email: "u@u.com", Role: models.RoleUser}
	token := makeTokenForTest(jwtCfg, user)

	r := gin.New()
	r.Use(m.RequireAuth())
	r.GET("/protected", func(c *gin.Context) { c.JSON(http.StatusOK, nil) })

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Basic "+token) // wrong type
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}
