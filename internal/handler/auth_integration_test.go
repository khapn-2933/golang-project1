package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/glebarez/sqlite"
	govalidator "github.com/go-playground/validator/v10"
	"github.com/kha/foods-drinks/internal/config"
	"github.com/kha/foods-drinks/internal/middleware"
	"github.com/kha/foods-drinks/internal/models"
	"github.com/kha/foods-drinks/internal/repository"
	"github.com/kha/foods-drinks/internal/service"
	customvalidator "github.com/kha/foods-drinks/pkg/validator"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// TestMain sets up the test binary: register custom validators once for the whole handler package.
func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)
	if v, ok := binding.Validator.Engine().(*govalidator.Validate); ok {
		customvalidator.RegisterCustomValidators(v)
	}
	os.Exit(m.Run())
}

func newAuthTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Fatalf("open auth test db: %v", err)
	}
	if err := db.AutoMigrate(
		&models.User{},
		&models.Cart{},
		&models.CartItem{},
		&models.Product{},
		&models.Category{},
	); err != nil {
		t.Fatalf("auth test migrate: %v", err)
	}
	return db
}

func newAuthHandlerRouter(t *testing.T, db *gorm.DB) (*gin.Engine, *service.AuthService) {
	t.Helper()
	jwtCfg := &config.JWTConfig{Secret: "auth-handler-test-secret", Expiration: 1 * time.Hour}
	userRepo := repository.NewUserRepository(db)
	cartRepo := repository.NewCartRepository(db)
	productRepo := repository.NewProductRepository(db)
	cartSvc := service.NewCartService(cartRepo, productRepo)
	authSvc := service.NewAuthService(userRepo, cartSvc, jwtCfg)
	h := NewAuthHandler(authSvc)
	authMW := middleware.NewAuthMiddleware(authSvc)

	r := gin.New()
	r.POST("/auth/register", h.Register)
	r.POST("/auth/login", h.Login)
	r.GET("/profile", authMW.RequireAuth(), h.GetProfile)
	return r, authSvc
}

func TestAuthHandler_Register_Success(t *testing.T) {
	t.Parallel()
	db := newAuthTestDB(t)
	r, _ := newAuthHandlerRouter(t, db)

	body := `{"email":"success@example.com","password":"Test@1234","full_name":"Test User"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/auth/register", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201: %s", w.Code, w.Body)
	}
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["access_token"] == nil {
		t.Error("expected access_token in response")
	}
	if userObj, ok := resp["user"].(map[string]interface{}); ok {
		if userObj["email"] != "success@example.com" {
			t.Errorf("email = %v, want success@example.com", userObj["email"])
		}
	} else {
		t.Error("expected user object in response")
	}
}

func TestAuthHandler_Register_DuplicateEmail(t *testing.T) {
	t.Parallel()
	db := newAuthTestDB(t)
	r, _ := newAuthHandlerRouter(t, db)

	body := `{"email":"dup@example.com","password":"Test@1234","full_name":"User"}`
	for i := 0; i < 2; i++ {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/auth/register", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		if i == 1 && w.Code != http.StatusConflict {
			t.Fatalf("second register: status = %d, want 409", w.Code)
		}
	}
}

func TestAuthHandler_Register_WeakPassword(t *testing.T) {
	t.Parallel()
	db := newAuthTestDB(t)
	r, _ := newAuthHandlerRouter(t, db)

	// password with no uppercase and no special char → fails password_strength
	body := `{"email":"weak@example.com","password":"simplepassword123","full_name":"User"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/auth/register", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400: %s", w.Code, w.Body)
	}
}

func TestAuthHandler_Register_MissingFields(t *testing.T) {
	t.Parallel()
	db := newAuthTestDB(t)
	r, _ := newAuthHandlerRouter(t, db)

	body := `{"email":"noname@example.com"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/auth/register", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}

func TestAuthHandler_Register_InvalidJSON(t *testing.T) {
	t.Parallel()
	db := newAuthTestDB(t)
	r, _ := newAuthHandlerRouter(t, db)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/auth/register", bytes.NewBufferString(`not json`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}

func TestAuthHandler_Login_Success(t *testing.T) {
	t.Parallel()
	db := newAuthTestDB(t)
	r, _ := newAuthHandlerRouter(t, db)

	// Register first
	regBody := `{"email":"loginok@example.com","password":"Test@1234","full_name":"Login User"}`
	regReq, _ := http.NewRequest(http.MethodPost, "/auth/register", bytes.NewBufferString(regBody))
	regReq.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(httptest.NewRecorder(), regReq)

	// Login
	loginBody := `{"email":"loginok@example.com","password":"Test@1234"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/auth/login", bytes.NewBufferString(loginBody))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", w.Code, w.Body)
	}
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["access_token"] == nil {
		t.Error("expected access_token")
	}
}

func TestAuthHandler_Login_WrongPassword(t *testing.T) {
	t.Parallel()
	db := newAuthTestDB(t)
	r, _ := newAuthHandlerRouter(t, db)

	// Register
	regBody := `{"email":"wrongpwd@example.com","password":"Test@1234","full_name":"Wrong Pwd"}`
	regReq, _ := http.NewRequest(http.MethodPost, "/auth/register", bytes.NewBufferString(regBody))
	regReq.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(httptest.NewRecorder(), regReq)

	// Login with wrong password
	loginBody := `{"email":"wrongpwd@example.com","password":"WrongPass@99"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/auth/login", bytes.NewBufferString(loginBody))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401: %s", w.Code, w.Body)
	}
}

func TestAuthHandler_Login_NotRegistered(t *testing.T) {
	t.Parallel()
	db := newAuthTestDB(t)
	r, _ := newAuthHandlerRouter(t, db)

	loginBody := `{"email":"noone@example.com","password":"Test@1234"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/auth/login", bytes.NewBufferString(loginBody))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", w.Code)
	}
}

func TestAuthHandler_Login_InvalidJSON(t *testing.T) {
	t.Parallel()
	db := newAuthTestDB(t)
	r, _ := newAuthHandlerRouter(t, db)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/auth/login", bytes.NewBufferString(`not json`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}

func TestAuthHandler_GetProfile_Unauthorized(t *testing.T) {
	t.Parallel()
	db := newAuthTestDB(t)
	r, _ := newAuthHandlerRouter(t, db)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/profile", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", w.Code)
	}
}

func TestAuthHandler_GetProfile_WithToken(t *testing.T) {
	t.Parallel()
	db := newAuthTestDB(t)
	r, _ := newAuthHandlerRouter(t, db)

	// Register to get a token
	regBody := `{"email":"profile@example.com","password":"Test@1234","full_name":"Profile User"}`
	regReq, _ := http.NewRequest(http.MethodPost, "/auth/register", bytes.NewBufferString(regBody))
	regReq.Header.Set("Content-Type", "application/json")
	regW := httptest.NewRecorder()
	r.ServeHTTP(regW, regReq)

	var regResp map[string]interface{}
	json.Unmarshal(regW.Body.Bytes(), &regResp)
	token, _ := regResp["access_token"].(string)
	if token == "" {
		t.Fatalf("register returned no token, body: %s", regW.Body)
	}

	// Get profile with token
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/profile", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", w.Code, w.Body)
	}
	var profResp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &profResp)
	if profResp["email"] != "profile@example.com" {
		t.Errorf("email = %v, want profile@example.com", profResp["email"])
	}
}

func TestAuthHandler_Register_EmailNormalization(t *testing.T) {
	t.Parallel()
	db := newAuthTestDB(t)
	r, _ := newAuthHandlerRouter(t, db)

	// Register with uppercase email → should be normalized to lowercase
	body := `{"email":"UPPER@Example.COM","password":"Test@1234","full_name":"Upper User"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/auth/register", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201: %s", w.Code, w.Body)
	}

	// Login with lowercase version
	loginBody := `{"email":"upper@example.com","password":"Test@1234"}`
	wLogin := httptest.NewRecorder()
	loginReq, _ := http.NewRequest(http.MethodPost, "/auth/login", bytes.NewBufferString(loginBody))
	loginReq.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(wLogin, loginReq)

	if wLogin.Code != http.StatusOK {
		t.Errorf("login with normalized email: status = %d, want 200: %s", wLogin.Code, wLogin.Body)
	}
}
