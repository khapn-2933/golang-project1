package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/kha/foods-drinks/internal/models"
	"github.com/kha/foods-drinks/internal/repository"
	"github.com/kha/foods-drinks/internal/service"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func newProductTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Fatalf("open product test db: %v", err)
	}
	if err := db.AutoMigrate(
		&models.Category{},
		&models.Product{},
		&models.ProductImage{},
	); err != nil {
		t.Fatalf("product test migrate: %v", err)
	}
	return db
}

func newProductHandlerRouter(db *gorm.DB) *gin.Engine {
	productRepo := repository.NewProductRepository(db)
	categoryRepo := repository.NewCategoryRepository(db)
	svc := service.NewProductService(productRepo, categoryRepo, "http://test.local")
	h := NewProductHandler(svc)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/products", h.List)
	r.GET("/products/:slug", h.GetBySlug)
	return r
}

func TestProductHandler_List_Empty(t *testing.T) {
	t.Parallel()
	db := newProductTestDB(t)
	r := newProductHandlerRouter(db)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/products", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", w.Code, w.Body)
	}
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	items, _ := resp["items"].([]interface{})
	if len(items) != 0 {
		t.Errorf("expected empty items, got %d", len(items))
	}
}

func TestProductHandler_List_OnlyActiveShown(t *testing.T) {
	t.Parallel()
	db := newProductTestDB(t)

	cat := &models.Category{Name: "Drinks", Slug: "drinks-active-test"}
	db.Create(cat)
	db.Create(&models.Product{CategoryID: cat.ID, Name: "Iced Tea", Slug: "iced-tea-active-test", Classify: "drink", Price: 25000, Stock: 10, Status: "active"})
	db.Create(&models.Product{CategoryID: cat.ID, Name: "Hot Coffee", Slug: "hot-coffee-active-test", Classify: "drink", Price: 30000, Stock: 5, Status: "active"})
	db.Create(&models.Product{CategoryID: cat.ID, Name: "Old Drink", Slug: "old-drink-inactive-test", Classify: "drink", Price: 5000, Stock: 0, Status: "inactive"})

	r := newProductHandlerRouter(db)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/products", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", w.Code, w.Body)
	}
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	items, _ := resp["items"].([]interface{})
	if len(items) != 2 {
		t.Errorf("expected 2 active items, got %d", len(items))
	}
}

func TestProductHandler_List_Pagination(t *testing.T) {
	t.Parallel()
	db := newProductTestDB(t)

	cat := &models.Category{Name: "Food", Slug: "food-paginate-test"}
	db.Create(cat)
	for i := 0; i < 5; i++ {
		db.Create(&models.Product{
			CategoryID: cat.ID,
			Name:       fmt.Sprintf("Food Item %d", i),
			Slug:       fmt.Sprintf("food-item-paginate-test-%d", i),
			Classify:   "food",
			Price:      float64(i+1) * 10000,
			Stock:      10,
			Status:     "active",
		})
	}

	r := newProductHandlerRouter(db)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/products?page=1&page_size=2", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", w.Code, w.Body)
	}
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	items, _ := resp["items"].([]interface{})
	if len(items) != 2 {
		t.Errorf("expected 2 items with page_size=2, got %d", len(items))
	}
	if total, _ := resp["total"].(float64); total != 5 {
		t.Errorf("expected total=5, got %v", resp["total"])
	}
}

func TestProductHandler_List_FilterByClassify(t *testing.T) {
	t.Parallel()
	db := newProductTestDB(t)

	cat := &models.Category{Name: "Mixed", Slug: "mixed-classify-test"}
	db.Create(cat)
	db.Create(&models.Product{CategoryID: cat.ID, Name: "Burger", Slug: "burger-classify-test", Classify: "food", Price: 50000, Stock: 5, Status: "active"})
	db.Create(&models.Product{CategoryID: cat.ID, Name: "Juice", Slug: "juice-classify-test", Classify: "drink", Price: 20000, Stock: 10, Status: "active"})

	r := newProductHandlerRouter(db)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/products?classify=food", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	items, _ := resp["items"].([]interface{})
	if len(items) != 1 {
		t.Errorf("expected 1 food item, got %d", len(items))
	}
}

func TestProductHandler_List_SortByPrice(t *testing.T) {
	t.Parallel()
	db := newProductTestDB(t)

	cat := &models.Category{Name: "Sortable", Slug: "sortable-price-test"}
	db.Create(cat)
	db.Create(&models.Product{CategoryID: cat.ID, Name: "Cheap", Slug: "cheap-sort-test", Classify: "food", Price: 10000, Stock: 5, Status: "active"})
	db.Create(&models.Product{CategoryID: cat.ID, Name: "Expensive", Slug: "expensive-sort-test", Classify: "food", Price: 100000, Stock: 5, Status: "active"})

	r := newProductHandlerRouter(db)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/products?sort_by=price&sort_dir=asc", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	items, _ := resp["items"].([]interface{})
	if len(items) != 2 {
		t.Errorf("expected 2 items, got %d", len(items))
	}
	// First item should be cheapest
	first := items[0].(map[string]interface{})
	if first["slug"] != "cheap-sort-test" {
		t.Errorf("expected cheapest first, got slug=%v", first["slug"])
	}
}

func TestProductHandler_GetBySlug_NotFound(t *testing.T) {
	t.Parallel()
	db := newProductTestDB(t)
	r := newProductHandlerRouter(db)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/products/non-existent-slug", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", w.Code)
	}
}

func TestProductHandler_GetBySlug_InactiveReturns404(t *testing.T) {
	t.Parallel()
	db := newProductTestDB(t)

	cat := &models.Category{Name: "Food", Slug: "food-inactive-slug-test"}
	db.Create(cat)
	db.Create(&models.Product{CategoryID: cat.ID, Name: "Old Dish", Slug: "old-dish-inactive-slug-test", Classify: "food", Price: 50000, Stock: 0, Status: "inactive"})

	r := newProductHandlerRouter(db)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/products/old-dish-inactive-slug-test", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404 for inactive product, got %d", w.Code)
	}
}

func TestProductHandler_GetBySlug_ActiveProduct(t *testing.T) {
	t.Parallel()
	db := newProductTestDB(t)

	cat := &models.Category{Name: "Food", Slug: "food-active-slug-test"}
	db.Create(cat)
	db.Create(&models.Product{CategoryID: cat.ID, Name: "Spring Roll", Slug: "spring-roll-active-test", Classify: "food", Price: 45000, Stock: 20, Status: "active"})

	r := newProductHandlerRouter(db)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/products/spring-roll-active-test", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", w.Code, w.Body)
	}
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["slug"] != "spring-roll-active-test" {
		t.Errorf("slug = %v, want spring-roll-active-test", resp["slug"])
	}
}

func TestProductHandler_GetBySlug_Returns200WithImages(t *testing.T) {
	t.Parallel()
	db := newProductTestDB(t)

	cat := &models.Category{Name: "Beverage", Slug: "beverage-images-test"}
	db.Create(cat)
	p := &models.Product{CategoryID: cat.ID, Name: "Smoothie", Slug: "smoothie-images-test", Classify: "drink", Price: 35000, Stock: 15, Status: "active"}
	db.Create(p)
	db.Create(&models.ProductImage{ProductID: p.ID, ImageURL: "http://test.local/smoothie.jpg", SortOrder: 0, IsPrimary: true})

	r := newProductHandlerRouter(db)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/products/smoothie-images-test", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", w.Code, w.Body)
	}
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	images, _ := resp["images"].([]interface{})
	if len(images) != 1 {
		t.Errorf("expected 1 image, got %d", len(images))
	}
}
