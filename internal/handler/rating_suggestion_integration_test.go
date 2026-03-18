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

func newRatingSuggestionTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Fatalf("open rating/suggestion test db: %v", err)
	}
	if err := db.AutoMigrate(
		&models.User{},
		&models.Category{},
		&models.Product{},
		&models.ProductImage{},
		&models.Order{},
		&models.OrderItem{},
		&models.Rating{},
		&models.Suggestion{},
	); err != nil {
		t.Fatalf("migrate rating/suggestion test db: %v", err)
	}
	return db
}

func setupRatingSuggestionRouter(t *testing.T) (*gin.Engine, *gorm.DB, string) {
	t.Helper()
	db := newRatingSuggestionTestDB(t)

	jwtCfg := &config.JWTConfig{Secret: "rating-suggest-secret", Expiration: time.Hour}
	authSvc := service.NewAuthServiceWithConfig(jwtCfg)
	authMW := middleware.NewAuthMiddleware(authSvc)

	ratingRepo := repository.NewRatingRepository(db)
	productRepo := repository.NewProductRepository(db)
	ratingSvc := service.NewRatingService(ratingRepo, productRepo)
	ratingHandler := NewRatingHandler(ratingSvc)

	suggestionRepo := repository.NewSuggestionRepository(db)
	categoryRepo := repository.NewCategoryRepository(db)
	suggestionSvc := service.NewSuggestionService(suggestionRepo, categoryRepo)
	suggestionHandler := NewSuggestionHandler(suggestionSvc)

	r := gin.New()
	r.GET("/products/:slug/ratings", ratingHandler.ListByProduct)
	protected := r.Group("")
	protected.Use(authMW.RequireAuth())
	protected.POST("/products/:slug/ratings", ratingHandler.Create)
	protected.PUT("/products/:slug/ratings", ratingHandler.Update)
	protected.POST("/suggestions", suggestionHandler.Create)

	user := &models.User{ID: 1001, Email: "rs-user@example.com", FullName: "RS User", Role: models.RoleUser, Status: models.UserStatusActive}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	token, _, err := authSvc.GenerateToken(user)
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}

	return r, db, token
}

func seedPurchasableProduct(t *testing.T, db *gorm.DB, userID uint, slug string) *models.Product {
	t.Helper()
	cat := &models.Category{Name: "Rating Cat " + slug, Slug: "rating-cat-" + slug}
	if err := db.Create(cat).Error; err != nil {
		t.Fatalf("create category: %v", err)
	}
	p := &models.Product{CategoryID: cat.ID, Name: "Rating Product", Slug: slug, Classify: models.ClassifyFood, Price: 20000, Stock: 20, Status: models.ProductStatusActive}
	if err := db.Create(p).Error; err != nil {
		t.Fatalf("create product: %v", err)
	}
	order := &models.Order{UserID: userID, OrderNumber: "ORD-RS-001", TotalAmount: 20000, Status: models.OrderStatusDelivered, ShippingAddress: "HN", ShippingPhone: "0900"}
	if err := db.Create(order).Error; err != nil {
		t.Fatalf("create order: %v", err)
	}
	item := &models.OrderItem{OrderID: order.ID, ProductID: p.ID, ProductName: p.Name, ProductPrice: p.Price, Quantity: 1, Subtotal: p.Price}
	if err := db.Create(item).Error; err != nil {
		t.Fatalf("create order item: %v", err)
	}
	return p
}

func TestRatingHandler_CreateUpdateAndList(t *testing.T) {
	t.Parallel()
	r, db, token := setupRatingSuggestionRouter(t)
	product := seedPurchasableProduct(t, db, 1001, "rating-handler-slug")

	createBody := `{"rating":5,"comment":"great"}`
	wCreate := httptest.NewRecorder()
	reqCreate := httptest.NewRequest(http.MethodPost, "/products/"+product.Slug+"/ratings", bytes.NewBufferString(createBody))
	reqCreate.Header.Set("Authorization", "Bearer "+token)
	reqCreate.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(wCreate, reqCreate)
	if wCreate.Code != http.StatusCreated {
		t.Fatalf("create status = %d, want 201: %s", wCreate.Code, wCreate.Body)
	}

	updateBody := `{"rating":4,"comment":"still good"}`
	wUpdate := httptest.NewRecorder()
	reqUpdate := httptest.NewRequest(http.MethodPut, "/products/"+product.Slug+"/ratings", bytes.NewBufferString(updateBody))
	reqUpdate.Header.Set("Authorization", "Bearer "+token)
	reqUpdate.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(wUpdate, reqUpdate)
	if wUpdate.Code != http.StatusOK {
		t.Fatalf("update status = %d, want 200: %s", wUpdate.Code, wUpdate.Body)
	}

	wList := httptest.NewRecorder()
	reqList := httptest.NewRequest(http.MethodGet, "/products/"+product.Slug+"/ratings", nil)
	r.ServeHTTP(wList, reqList)
	if wList.Code != http.StatusOK {
		t.Fatalf("list status = %d, want 200: %s", wList.Code, wList.Body)
	}
	var resp map[string]interface{}
	json.Unmarshal(wList.Body.Bytes(), &resp)
	items, _ := resp["items"].([]interface{})
	if len(items) != 1 {
		t.Fatalf("len(items) = %d, want 1", len(items))
	}
}

func TestRatingHandler_UnauthorizedAndValidation(t *testing.T) {
	t.Parallel()
	r, db, _ := setupRatingSuggestionRouter(t)
	product := seedPurchasableProduct(t, db, 1001, "rating-handler-slug-2")

	wUnauthorized := httptest.NewRecorder()
	reqUnauthorized := httptest.NewRequest(http.MethodPost, "/products/"+product.Slug+"/ratings", bytes.NewBufferString(`{"rating":5}`))
	reqUnauthorized.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(wUnauthorized, reqUnauthorized)
	if wUnauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized create status = %d, want 401", wUnauthorized.Code)
	}

	wInvalid := httptest.NewRecorder()
	reqInvalid := httptest.NewRequest(http.MethodGet, "/products//ratings", nil)
	r.ServeHTTP(wInvalid, reqInvalid)
	if wInvalid.Code != http.StatusMovedPermanently && wInvalid.Code != http.StatusBadRequest {
		// Gin may normalize // path and redirect depending on engine config.
		t.Fatalf("invalid slug status = %d, want 301 or 400", wInvalid.Code)
	}
}

func TestSuggestionHandler_Create(t *testing.T) {
	t.Parallel()
	r, db, token := setupRatingSuggestionRouter(t)

	cat := &models.Category{Name: "Suggest Cat", Slug: "suggest-cat"}
	if err := db.Create(cat).Error; err != nil {
		t.Fatalf("create category: %v", err)
	}

	body := fmt.Sprintf(`{"name":"New Dish","classify":"food","category_id":%d}`, cat.ID)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/suggestions", bytes.NewBufferString(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("create suggestion status = %d, want 201: %s", w.Code, w.Body)
	}
}

func TestSuggestionHandler_CategoryNotFound(t *testing.T) {
	t.Parallel()
	r, _, token := setupRatingSuggestionRouter(t)

	body := `{"name":"Bad Cat","classify":"food","category_id":99999}`
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/suggestions", bytes.NewBufferString(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("category not found status = %d, want 400", w.Code)
	}
}
