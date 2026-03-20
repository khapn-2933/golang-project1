package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/kha/foods-drinks/internal/config"
	"github.com/kha/foods-drinks/internal/middleware"
	"github.com/kha/foods-drinks/internal/models"
	"github.com/kha/foods-drinks/internal/repository"
	"github.com/kha/foods-drinks/internal/service"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func newCartHandlerTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Fatalf("open cart handler test db: %v", err)
	}
	if err := db.AutoMigrate(
		&models.User{},
		&models.Category{},
		&models.Product{},
		&models.ProductImage{},
		&models.Cart{},
		&models.CartItem{},
	); err != nil {
		t.Fatalf("cart handler migrate: %v", err)
	}
	return db
}

func setupCartHandlerRouter(t *testing.T) (*gin.Engine, *gorm.DB, *service.AuthService) {
	t.Helper()
	db := newCartHandlerTestDB(t)

	userRepo := repository.NewUserRepository(db)
	productRepo := repository.NewProductRepository(db)
	cartRepo := repository.NewCartRepository(db)
	cartSvc := service.NewCartService(cartRepo, productRepo)
	authSvc := service.NewAuthService(userRepo, cartSvc, &config.JWTConfig{Secret: "cart-handler-secret", Expiration: time.Hour})
	authMW := middleware.NewAuthMiddleware(authSvc)

	cartHandler := NewCartHandler(cartSvc)

	r := gin.New()
	group := r.Group("")
	group.Use(authMW.RequireAuth())
	group.GET("/cart", cartHandler.Get)
	group.POST("/cart/items", cartHandler.Add)
	group.PUT("/cart/items/:product_id", cartHandler.Update)
	group.DELETE("/cart/items/:product_id", cartHandler.Remove)
	group.DELETE("/cart", cartHandler.Clear)
	return r, db, authSvc
}

func seedCartUserAndToken(t *testing.T, db *gorm.DB, authSvc *service.AuthService, email string) (uint, string) {
	t.Helper()
	hash, err := authSvc.HashPassword("Test@1234")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	u := &models.User{Email: email, PasswordHash: &hash, FullName: "Cart User", Role: models.RoleUser, Status: models.UserStatusActive}
	if err := db.Create(u).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	token, _, err := authSvc.GenerateToken(u)
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}
	return u.ID, token
}

func seedCartProduct(t *testing.T, db *gorm.DB, slug string, stock int) *models.Product {
	t.Helper()
	cat := &models.Category{Name: "Cart Category " + slug, Slug: "cart-handler-cat-" + slug}
	if err := db.Create(cat).Error; err != nil {
		t.Fatalf("create category: %v", err)
	}
	p := &models.Product{CategoryID: cat.ID, Name: "Cart Product " + slug, Slug: slug, Classify: models.ClassifyFood, Price: 20000, Stock: stock, Status: models.ProductStatusActive}
	if err := db.Create(p).Error; err != nil {
		t.Fatalf("create product: %v", err)
	}
	return p
}

func TestCartHandler_Unauthorized(t *testing.T) {
	t.Parallel()
	r, _, _ := setupCartHandlerRouter(t)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/cart", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", w.Code)
	}
}

func TestCartHandler_Flow_AddUpdateRemoveClear(t *testing.T) {
	t.Parallel()
	r, db, authSvc := setupCartHandlerRouter(t)
	_, token := seedCartUserAndToken(t, db, authSvc, "cartflow@example.com")
	product := seedCartProduct(t, db, "cart-flow-product", 20)

	authHeader := "Bearer " + token

	addBody := fmt.Sprintf(`{"product_id":%d,"quantity":2}`, product.ID)
	wAdd := httptest.NewRecorder()
	reqAdd := httptest.NewRequest(http.MethodPost, "/cart/items", bytes.NewBufferString(addBody))
	reqAdd.Header.Set("Authorization", authHeader)
	reqAdd.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(wAdd, reqAdd)
	if wAdd.Code != http.StatusOK {
		t.Fatalf("add status = %d, want 200: %s", wAdd.Code, wAdd.Body)
	}

	updateBody := `{"quantity":5}`
	wUpdate := httptest.NewRecorder()
	reqUpdate := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/cart/items/%d", product.ID), bytes.NewBufferString(updateBody))
	reqUpdate.Header.Set("Authorization", authHeader)
	reqUpdate.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(wUpdate, reqUpdate)
	if wUpdate.Code != http.StatusOK {
		t.Fatalf("update status = %d, want 200: %s", wUpdate.Code, wUpdate.Body)
	}

	wGet := httptest.NewRecorder()
	reqGet := httptest.NewRequest(http.MethodGet, "/cart", nil)
	reqGet.Header.Set("Authorization", authHeader)
	r.ServeHTTP(wGet, reqGet)
	if wGet.Code != http.StatusOK {
		t.Fatalf("get status = %d, want 200: %s", wGet.Code, wGet.Body)
	}
	var getResp map[string]interface{}
	json.Unmarshal(wGet.Body.Bytes(), &getResp)
	if totalItems, ok := getResp["total_items"].(float64); !ok || int(totalItems) != 5 {
		t.Fatalf("total_items = %v, want 5", getResp["total_items"])
	}

	wRemove := httptest.NewRecorder()
	reqRemove := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/cart/items/%d", product.ID), nil)
	reqRemove.Header.Set("Authorization", authHeader)
	r.ServeHTTP(wRemove, reqRemove)
	if wRemove.Code != http.StatusOK {
		t.Fatalf("remove status = %d, want 200: %s", wRemove.Code, wRemove.Body)
	}

	wClear := httptest.NewRecorder()
	reqClear := httptest.NewRequest(http.MethodDelete, "/cart", nil)
	reqClear.Header.Set("Authorization", authHeader)
	r.ServeHTTP(wClear, reqClear)
	if wClear.Code != http.StatusOK {
		t.Fatalf("clear status = %d, want 200: %s", wClear.Code, wClear.Body)
	}
}

func TestCartHandler_AddValidationAndInvalidPath(t *testing.T) {
	t.Parallel()
	r, db, authSvc := setupCartHandlerRouter(t)
	_, token := seedCartUserAndToken(t, db, authSvc, "cart-validate@example.com")
	authHeader := "Bearer " + token

	wAdd := httptest.NewRecorder()
	reqAdd := httptest.NewRequest(http.MethodPost, "/cart/items", bytes.NewBufferString(`{"product_id":1,"quantity":0}`))
	reqAdd.Header.Set("Authorization", authHeader)
	reqAdd.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(wAdd, reqAdd)
	if wAdd.Code != http.StatusBadRequest {
		t.Fatalf("add invalid qty status = %d, want 400", wAdd.Code)
	}

	wUpdate := httptest.NewRecorder()
	reqUpdate := httptest.NewRequest(http.MethodPut, "/cart/items/abc", bytes.NewBufferString(`{"quantity":1}`))
	reqUpdate.Header.Set("Authorization", authHeader)
	reqUpdate.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(wUpdate, reqUpdate)
	if wUpdate.Code != http.StatusBadRequest {
		t.Fatalf("update invalid product_id status = %d, want 400", wUpdate.Code)
	}
}

func TestParsePositiveUintParam(t *testing.T) {
	t.Parallel()

	cases := []struct {
		in   string
		ok   bool
		want uint64
	}{
		{in: "1", ok: true, want: 1},
		{in: " 42 ", ok: true, want: 42},
		{in: "0", ok: false, want: 0},
		{in: "", ok: false, want: 0},
		{in: "-1", ok: false, want: 0},
		{in: "abc", ok: false, want: 0},
	}

	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			got, ok := parsePositiveUintParam(tc.in)
			if ok != tc.ok || got != tc.want {
				t.Fatalf("parsePositiveUintParam(%q) = (%d, %v), want (%d, %v)", tc.in, got, ok, tc.want, tc.ok)
			}
		})
	}
}
