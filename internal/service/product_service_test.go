package service

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/kha/foods-drinks/internal/dto"
	"github.com/kha/foods-drinks/internal/models"
	"github.com/kha/foods-drinks/internal/repository"
	"gorm.io/gorm"
)

func setupProductServiceTest(t *testing.T) (*ProductService, *repository.ProductRepository, *gorm.DB) {
	t.Helper()

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}

	if err := db.AutoMigrate(&models.Category{}, &models.Product{}, &models.ProductImage{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	category := models.Category{
		Name:   "Foods",
		Slug:   "foods",
		Status: models.CategoryStatusActive,
	}
	if err := db.Create(&category).Error; err != nil {
		t.Fatalf("seed category: %v", err)
	}

	productRepo := repository.NewProductRepository(db)
	categoryRepo := repository.NewCategoryRepository(db)
	service := NewProductService(productRepo, categoryRepo, "https://foods.example.com/")

	return service, productRepo, db
}

func TestGenerateSlug(t *testing.T) {
	t.Parallel()

	svc := &ProductService{}

	cases := []struct {
		name string
		in   string
		want string
	}{
		{name: "trim and lowercase", in: "  Bun Bo Hue  ", want: "bun-bo-hue"},
		{name: "replace underscore", in: "Bun_Bo_Hue", want: "bun-bo-hue"},
		{name: "collapse repeated dashes", in: "Bun---Bo___Hue", want: "bun-bo-hue"},
		{name: "remove non alnum", in: "Pho @ #1", want: "pho-1"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := svc.generateSlug(tc.in)
			if got != tc.want {
				t.Fatalf("generateSlug(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestBuildProductURL(t *testing.T) {
	t.Parallel()

	t.Run("uses normalized base url and path escapes slug", func(t *testing.T) {
		t.Parallel()
		svc := NewProductService(nil, nil, "https://foods.example.com/")
		got := svc.buildProductURL("tra sua dac biet")
		want := "https://foods.example.com/products/tra%20sua%20dac%20biet"
		if got != want {
			t.Fatalf("buildProductURL mismatch: got %q, want %q", got, want)
		}
	})

	t.Run("falls back to localhost when base url is empty", func(t *testing.T) {
		t.Parallel()
		svc := NewProductService(nil, nil, "")
		got := svc.buildProductURL("pho")
		want := "http://localhost:8000/products/pho"
		if got != want {
			t.Fatalf("buildProductURL fallback mismatch: got %q, want %q", got, want)
		}
	})

	t.Run("returns base url when slug is blank", func(t *testing.T) {
		t.Parallel()
		svc := NewProductService(nil, nil, "https://foods.example.com")
		got := svc.buildProductURL("   ")
		want := "https://foods.example.com"
		if got != want {
			t.Fatalf("blank slug mismatch: got %q, want %q", got, want)
		}
	})
}

func TestBuildSocialShare(t *testing.T) {
	t.Parallel()

	svc := NewProductService(nil, nil, "https://foods.example.com")
	product := &models.Product{Name: "Pho Bo", Slug: "pho-bo"}

	share := svc.buildSocialShare(product)

	fbURL, err := url.Parse(share.Facebook)
	if err != nil {
		t.Fatalf("parse facebook share url: %v", err)
	}
	twURL, err := url.Parse(share.Twitter)
	if err != nil {
		t.Fatalf("parse twitter share url: %v", err)
	}

	wantProductURL := "https://foods.example.com/products/pho-bo"
	if got := fbURL.Query().Get("u"); got != wantProductURL {
		t.Fatalf("facebook shared url = %q, want %q", got, wantProductURL)
	}
	if got := twURL.Query().Get("url"); got != wantProductURL {
		t.Fatalf("twitter shared url = %q, want %q", got, wantProductURL)
	}

	wantText := "Khám phá Pho Bo tại Foods & Drinks"
	if got := twURL.Query().Get("text"); got != wantText {
		t.Fatalf("twitter share text = %q, want %q", got, wantText)
	}
}

func TestEnsureUniqueSlug(t *testing.T) {
	t.Parallel()

	svc, _, db := setupProductServiceTest(t)

	existing := models.Product{
		CategoryID: 1,
		Name:       "Pho Bo",
		Slug:       "pho-bo",
		Classify:   models.ClassifyFood,
		Price:      39000,
		Stock:      10,
		Status:     models.ProductStatusActive,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
	if err := db.Create(&existing).Error; err != nil {
		t.Fatalf("seed existing product: %v", err)
	}

	got, err := svc.ensureUniqueSlug("pho-bo", 0)
	if err != nil {
		t.Fatalf("ensureUniqueSlug returned error: %v", err)
	}
	if got != "pho-bo-2" {
		t.Fatalf("ensureUniqueSlug = %q, want %q", got, "pho-bo-2")
	}

	got, err = svc.ensureUniqueSlug("pho-bo", existing.ID)
	if err != nil {
		t.Fatalf("ensureUniqueSlug with exclude id returned error: %v", err)
	}
	if strings.TrimSpace(got) != "pho-bo" {
		t.Fatalf("ensureUniqueSlug with exclude id = %q, want pho-bo", got)
	}
}

func TestProductService_CreateGetUpdateDeleteList(t *testing.T) {
	t.Parallel()

	svc, _, db := setupProductServiceTest(t)

	createReq := &dto.CreateProductRequest{
		CategoryID: 1,
		Name:       "Orange Juice",
		Classify:   models.ClassifyDrink,
		Price:      30000,
		Stock:      20,
		Status:     models.ProductStatusActive,
	}
	created, err := svc.Create(createReq, []string{"https://img.example.com/oj-1.jpg", "https://img.example.com/oj-2.jpg"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if created.ID == 0 {
		t.Fatal("created product id should be non-zero")
	}
	if created.PrimaryImage == nil {
		t.Fatal("expected primary image after create")
	}

	byID, err := svc.GetByID(created.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if byID.Slug == "" {
		t.Fatal("slug should not be empty")
	}

	bySlug, err := svc.GetBySlug(byID.Slug)
	if err != nil {
		t.Fatalf("GetBySlug: %v", err)
	}
	if bySlug.ID != byID.ID {
		t.Fatalf("id by slug = %d, want %d", bySlug.ID, byID.ID)
	}

	newName := "Orange Juice Premium"
	newPrice := 35000.0
	newStock := 15
	newDesc := "  fresh and cold  "
	updated, err := svc.Update(created.ID, &dto.UpdateProductRequest{
		Name:        &newName,
		Price:       &newPrice,
		Stock:       &newStock,
		Description: &newDesc,
	}, []string{"https://img.example.com/oj-new.jpg"}, true)
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.Name != newName {
		t.Fatalf("updated name = %s, want %s", updated.Name, newName)
	}
	if len(updated.Images) != 1 {
		t.Fatalf("updated images len = %d, want 1", len(updated.Images))
	}

	list, err := svc.List(&dto.ProductListRequest{Page: 1, PageSize: 10, Status: models.ProductStatusActive, SortBy: "created_at", SortDir: "desc"})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	items, ok := list.Items.([]dto.ProductResponse)
	if !ok {
		t.Fatalf("list items type = %T", list.Items)
	}
	if len(items) == 0 {
		t.Fatal("expected at least one product in list")
	}

	if err := svc.Delete(created.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := svc.GetByID(created.ID); !errors.Is(err, ErrProductNotFound) {
		t.Fatalf("GetByID after delete err = %v, want ErrProductNotFound", err)
	}

	// Ensure DB row is soft-deleted.
	var count int64
	db.Unscoped().Model(&models.Product{}).Where("id = ?", created.ID).Count(&count)
	if count != 1 {
		t.Fatalf("soft-deleted row count = %d, want 1", count)
	}
}

func TestProductService_CreateInvalidSlugAndUpdateDuplicateSlug(t *testing.T) {
	t.Parallel()

	svc, _, _ := setupProductServiceTest(t)

	_, err := svc.Create(&dto.CreateProductRequest{
		CategoryID: 1,
		Name:       "abc",
		Slug:       "---",
		Classify:   models.ClassifyDrink,
		Price:      10000,
		Stock:      1,
		Status:     models.ProductStatusActive,
	}, nil)
	if !errors.Is(err, ErrProductEmptySlug) {
		t.Fatalf("create invalid slug err = %v, want ErrProductEmptySlug", err)
	}

	p1, err := svc.Create(&dto.CreateProductRequest{CategoryID: 1, Name: "A", Slug: "dup-slug", Classify: models.ClassifyFood, Price: 10000, Stock: 1, Status: models.ProductStatusActive}, nil)
	if err != nil {
		t.Fatalf("create p1: %v", err)
	}
	p2, err := svc.Create(&dto.CreateProductRequest{CategoryID: 1, Name: "B", Slug: "other-slug", Classify: models.ClassifyFood, Price: 10000, Stock: 1, Status: models.ProductStatusActive}, nil)
	if err != nil {
		t.Fatalf("create p2: %v", err)
	}

	dup := "dup-slug"
	_, err = svc.Update(p2.ID, &dto.UpdateProductRequest{Slug: &dup}, nil, false)
	if !errors.Is(err, ErrProductSlugExists) {
		t.Fatalf("update duplicate slug err = %v, want ErrProductSlugExists", err)
	}

	// keep same slug on same product should pass
	_, err = svc.Update(p1.ID, &dto.UpdateProductRequest{Slug: &dup}, nil, false)
	if err != nil {
		t.Fatalf("update same slug on same product should pass, got %v", err)
	}
}
