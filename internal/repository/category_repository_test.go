package repository

import (
	"fmt"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/kha/foods-drinks/internal/models"
	"gorm.io/gorm"
)

func setupCategoryRepoTest(t *testing.T) (*CategoryRepository, *gorm.DB) {
	t.Helper()

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	if err := db.AutoMigrate(&models.Category{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	return NewCategoryRepository(db), db
}

func TestCategoryRepositoryCreate(t *testing.T) {
	repo, db := setupCategoryRepoTest(t)

	cat := &models.Category{
		Name:   "Foods",
		Slug:   "foods",
		Status: models.CategoryStatusActive,
	}
	if err := repo.Create(cat); err != nil {
		t.Fatalf("Create() error: %v", err)
	}
	if cat.ID == 0 {
		t.Fatal("Create() did not assign ID")
	}

	var saved models.Category
	if err := db.First(&saved, cat.ID).Error; err != nil {
		t.Fatalf("read saved category: %v", err)
	}
	if saved.Name != "Foods" {
		t.Fatalf("Name = %q, want Foods", saved.Name)
	}
	if saved.Slug != "foods" {
		t.Fatalf("Slug = %q, want foods", saved.Slug)
	}
}

func TestCategoryRepositoryFindByID(t *testing.T) {
	repo, db := setupCategoryRepoTest(t)

	cat := &models.Category{Name: "Drinks", Slug: "drinks", Status: models.CategoryStatusActive}
	db.Create(cat)

	found, err := repo.FindByID(cat.ID)
	if err != nil {
		t.Fatalf("FindByID error: %v", err)
	}
	if found.Slug != "drinks" {
		t.Fatalf("Slug = %q, want drinks", found.Slug)
	}

	_, err = repo.FindByID(99999)
	if err == nil {
		t.Fatal("expected error for non-existent ID")
	}
}

func TestCategoryRepositoryFindBySlug(t *testing.T) {
	repo, db := setupCategoryRepoTest(t)

	cat := &models.Category{Name: "Snacks", Slug: "snacks", Status: models.CategoryStatusActive}
	db.Create(cat)

	found, err := repo.FindBySlug("snacks")
	if err != nil {
		t.Fatalf("FindBySlug error: %v", err)
	}
	if found.Name != "Snacks" {
		t.Fatalf("Name = %q, want Snacks", found.Name)
	}

	_, err = repo.FindBySlug("non-existent")
	if err == nil {
		t.Fatal("expected error for non-existent slug")
	}
}

func TestCategoryRepositoryExistsBySlug(t *testing.T) {
	repo, db := setupCategoryRepoTest(t)

	cat := &models.Category{Name: "Beverages", Slug: "beverages", Status: models.CategoryStatusActive}
	db.Create(cat)

	exists, err := repo.ExistsBySlug("beverages")
	if err != nil {
		t.Fatalf("ExistsBySlug error: %v", err)
	}
	if !exists {
		t.Fatal("ExistsBySlug = false, want true")
	}

	// Excluding the same ID should return false
	exists, err = repo.ExistsBySlug("beverages", cat.ID)
	if err != nil {
		t.Fatalf("ExistsBySlug with exclusion error: %v", err)
	}
	if exists {
		t.Fatal("ExistsBySlug with own ID exclusion = true, want false")
	}

	exists, err = repo.ExistsBySlug("non-existent")
	if err != nil {
		t.Fatalf("ExistsBySlug(non-existent) error: %v", err)
	}
	if exists {
		t.Fatal("ExistsBySlug(non-existent) = true, want false")
	}
}

func TestCategoryRepositoryUpdate(t *testing.T) {
	repo, _ := setupCategoryRepoTest(t)

	cat := &models.Category{Name: "Old", Slug: "old-slug", Status: models.CategoryStatusActive}
	repo.Create(cat)

	cat.Name = "Updated"
	if err := repo.Update(cat); err != nil {
		t.Fatalf("Update() error: %v", err)
	}

	found, _ := repo.FindByID(cat.ID)
	if found.Name != "Updated" {
		t.Fatalf("after update Name = %q, want Updated", found.Name)
	}
}

func TestCategoryRepositoryDelete(t *testing.T) {
	repo, db := setupCategoryRepoTest(t)

	cat := &models.Category{Name: "Temp", Slug: "temp", Status: models.CategoryStatusActive}
	db.Create(cat)

	if err := repo.Delete(cat.ID); err != nil {
		t.Fatalf("Delete() error: %v", err)
	}

	var count int64
	db.Unscoped().Model(&models.Category{}).Where("id = ?", cat.ID).Count(&count)
	if count == 0 {
		t.Fatal("record should still exist as soft-deleted")
	}

	// Regular FindByID should not return deleted record
	_, err := repo.FindByID(cat.ID)
	if err == nil {
		t.Fatal("expected error for soft-deleted record")
	}
}

func TestCategoryRepositoryList(t *testing.T) {
	repo, _ := setupCategoryRepoTest(t)

	for i := 1; i <= 5; i++ {
		repo.Create(&models.Category{
			Name:   fmt.Sprintf("Category %d", i),
			Slug:   fmt.Sprintf("category-%d", i),
			Status: models.CategoryStatusActive,
		})
	}

	cats, total, err := repo.List(CategoryListParams{Offset: 0, Limit: 3})
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}
	if total != 5 {
		t.Fatalf("total = %d, want 5", total)
	}
	if len(cats) != 3 {
		t.Fatalf("returned %d items, want 3", len(cats))
	}
}
