package repository

import (
	"fmt"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/kha/foods-drinks/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func newProductRepoTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Fatalf("open product repo test db: %v", err)
	}
	if err := db.AutoMigrate(&models.Category{}, &models.Product{}, &models.ProductImage{}); err != nil {
		t.Fatalf("migrate product repo models: %v", err)
	}
	return db
}

func seedCategoryForProductRepo(t *testing.T, db *gorm.DB, slug string) *models.Category {
	t.Helper()
	cat := &models.Category{Name: "Category " + slug, Slug: slug}
	if err := db.Create(cat).Error; err != nil {
		t.Fatalf("seed category: %v", err)
	}
	return cat
}

func TestProductRepositoryCreateFindByIDFindBySlug(t *testing.T) {
	t.Parallel()
	db := newProductRepoTestDB(t)
	repo := NewProductRepository(db)
	cat := seedCategoryForProductRepo(t, db, "product-repo-basic")

	p := &models.Product{
		CategoryID: cat.ID,
		Name:       "Milk Tea",
		Slug:       "milk-tea-repo-test",
		Classify:   models.ClassifyDrink,
		Price:      32000,
		Stock:      50,
		Status:     models.ProductStatusActive,
	}
	if err := repo.Create(p); err != nil {
		t.Fatalf("Create: %v", err)
	}

	gotByID, err := repo.FindByID(p.ID)
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if gotByID.Slug != "milk-tea-repo-test" {
		t.Errorf("slug by id = %s", gotByID.Slug)
	}

	gotBySlug, err := repo.FindBySlug("milk-tea-repo-test")
	if err != nil {
		t.Fatalf("FindBySlug: %v", err)
	}
	if gotBySlug.ID != p.ID {
		t.Errorf("id by slug = %d, want %d", gotBySlug.ID, p.ID)
	}
}

func TestProductRepositoryExistsBySlug(t *testing.T) {
	t.Parallel()
	db := newProductRepoTestDB(t)
	repo := NewProductRepository(db)
	cat := seedCategoryForProductRepo(t, db, "product-repo-exists")

	p := &models.Product{CategoryID: cat.ID, Name: "Cake", Slug: "cake-exists-test", Classify: models.ClassifyFood, Price: 10000, Stock: 10, Status: models.ProductStatusActive}
	repo.Create(p)

	exists, err := repo.ExistsBySlug("cake-exists-test")
	if err != nil {
		t.Fatalf("ExistsBySlug: %v", err)
	}
	if !exists {
		t.Fatal("expected slug to exist")
	}

	exists, err = repo.ExistsBySlug("cake-exists-test", p.ID)
	if err != nil {
		t.Fatalf("ExistsBySlug exclude: %v", err)
	}
	if exists {
		t.Fatal("expected slug to not exist when excluded")
	}
}

func TestProductRepositoryDecreaseStock(t *testing.T) {
	t.Parallel()
	db := newProductRepoTestDB(t)
	repo := NewProductRepository(db)
	cat := seedCategoryForProductRepo(t, db, "product-repo-stock")

	p := &models.Product{CategoryID: cat.ID, Name: "Soda", Slug: "soda-stock-test", Classify: models.ClassifyDrink, Price: 10000, Stock: 5, Status: models.ProductStatusActive}
	repo.Create(p)

	ok, err := repo.DecreaseStock(p.ID, 3)
	if err != nil {
		t.Fatalf("DecreaseStock(3): %v", err)
	}
	if !ok {
		t.Fatal("expected decrease stock to succeed")
	}
	got, _ := repo.FindByID(p.ID)
	if got.Stock != 2 {
		t.Errorf("stock = %d, want 2", got.Stock)
	}

	ok, err = repo.DecreaseStock(p.ID, 5)
	if err != nil {
		t.Fatalf("DecreaseStock(5): %v", err)
	}
	if ok {
		t.Fatal("expected decrease stock to fail when insufficient")
	}
}

func TestProductRepositoryListFiltersAndSort(t *testing.T) {
	t.Parallel()
	db := newProductRepoTestDB(t)
	repo := NewProductRepository(db)
	cat := seedCategoryForProductRepo(t, db, "product-repo-list")

	repo.Create(&models.Product{CategoryID: cat.ID, Name: "A", Slug: "a-list-test", Classify: models.ClassifyFood, Price: 10000, Stock: 10, Status: models.ProductStatusActive})
	repo.Create(&models.Product{CategoryID: cat.ID, Name: "B", Slug: "b-list-test", Classify: models.ClassifyDrink, Price: 30000, Stock: 10, Status: models.ProductStatusActive})
	repo.Create(&models.Product{CategoryID: cat.ID, Name: "C", Slug: "c-list-test", Classify: models.ClassifyFood, Price: 50000, Stock: 10, Status: models.ProductStatusInactive})

	items, total, err := repo.List(ProductListParams{
		Offset:   0,
		Limit:    10,
		Classify: models.ClassifyFood,
		Status:   models.ProductStatusActive,
		SortBy:   "price",
		SortDir:  "asc",
	})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if total != 1 {
		t.Fatalf("total = %d, want 1", total)
	}
	if len(items) != 1 {
		t.Fatalf("len(items) = %d, want 1", len(items))
	}
	if items[0].Slug != "a-list-test" {
		t.Errorf("slug = %s, want a-list-test", items[0].Slug)
	}
}

func TestProductRepositoryUpdateDelete(t *testing.T) {
	t.Parallel()
	db := newProductRepoTestDB(t)
	repo := NewProductRepository(db)
	cat := seedCategoryForProductRepo(t, db, "product-repo-update")

	p := &models.Product{CategoryID: cat.ID, Name: "Before", Slug: "before-update-test", Classify: models.ClassifyFood, Price: 10000, Stock: 5, Status: models.ProductStatusActive}
	repo.Create(p)

	p.Name = "After"
	p.Price = 20000
	if err := repo.Update(p); err != nil {
		t.Fatalf("Update: %v", err)
	}
	got, _ := repo.FindByID(p.ID)
	if got.Name != "After" {
		t.Errorf("name = %s, want After", got.Name)
	}

	if err := repo.Delete(p.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := repo.FindByID(p.ID); err == nil {
		t.Fatal("expected FindByID to fail after delete")
	}
}
