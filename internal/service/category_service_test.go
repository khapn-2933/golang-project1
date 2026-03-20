package service

import (
	"fmt"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/kha/foods-drinks/internal/dto"
	"github.com/kha/foods-drinks/internal/models"
	"github.com/kha/foods-drinks/internal/repository"
	"gorm.io/gorm"
)

func setupCategoryServiceTest(t *testing.T) (*CategoryService, *gorm.DB) {
	t.Helper()

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.Category{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	repo := repository.NewCategoryRepository(db)
	return NewCategoryService(repo), db
}

func TestCategoryService_Create(t *testing.T) {
	svc, _ := setupCategoryServiceTest(t)

	req := &dto.CreateCategoryRequest{Name: "Vietnamese Food"}
	resp, err := svc.Create(req)
	if err != nil {
		t.Fatalf("Create() error: %v", err)
	}
	if resp.ID == 0 {
		t.Fatal("expected non-zero ID")
	}
	if resp.Slug != "vietnamese-food" {
		t.Fatalf("slug = %q, want vietnamese-food", resp.Slug)
	}
	if resp.Name != "Vietnamese Food" {
		t.Fatalf("name = %q, want Vietnamese Food", resp.Name)
	}
}

func TestCategoryService_Create_SlugUniqueness(t *testing.T) {
	svc, _ := setupCategoryServiceTest(t)

	svc.Create(&dto.CreateCategoryRequest{Name: "Drinks"})
	resp, err := svc.Create(&dto.CreateCategoryRequest{Name: "Drinks"})
	if err != nil {
		t.Fatalf("second Create() error: %v", err)
	}
	if resp.Slug == "drinks" {
		t.Fatalf("expected unique slug, got %q (same as first)", resp.Slug)
	}
}

func TestCategoryService_GetByID(t *testing.T) {
	svc, _ := setupCategoryServiceTest(t)

	created, _ := svc.Create(&dto.CreateCategoryRequest{Name: "Soups"})

	found, err := svc.GetByID(created.ID)
	if err != nil {
		t.Fatalf("GetByID() error: %v", err)
	}
	if found.Name != "Soups" {
		t.Fatalf("name = %q, want Soups", found.Name)
	}

	_, err = svc.GetByID(99999)
	if err == nil {
		t.Fatal("expected error for non-existent ID")
	}
}

func TestCategoryService_Update(t *testing.T) {
	svc, _ := setupCategoryServiceTest(t)

	created, _ := svc.Create(&dto.CreateCategoryRequest{Name: "Original"})

	newName := "Updated Name"
	updated, err := svc.Update(created.ID, &dto.UpdateCategoryRequest{Name: &newName})
	if err != nil {
		t.Fatalf("Update() error: %v", err)
	}
	if updated.Name != "Updated Name" {
		t.Fatalf("name = %q, want Updated Name", updated.Name)
	}
}

func TestCategoryService_Delete(t *testing.T) {
	svc, db := setupCategoryServiceTest(t)

	created, _ := svc.Create(&dto.CreateCategoryRequest{Name: "ToDelete"})

	if err := svc.Delete(created.ID); err != nil {
		t.Fatalf("Delete() error: %v", err)
	}

	// Soft delete: should not be found normally
	_, err := svc.GetByID(created.ID)
	if err == nil {
		t.Fatal("expected ErrCategoryNotFound after delete")
	}

	// But still exists in DB (soft delete)
	var count int64
	db.Unscoped().Model(&models.Category{}).Where("id = ?", created.ID).Count(&count)
	if count != 1 {
		t.Fatal("record should exist as soft-deleted")
	}
}

func TestCategoryService_List(t *testing.T) {
	svc, _ := setupCategoryServiceTest(t)

	for i := 1; i <= 5; i++ {
		svc.Create(&dto.CreateCategoryRequest{Name: fmt.Sprintf("Category %d", i)})
	}

	result, err := svc.List(&dto.CategoryListRequest{Page: 1, PageSize: 3})
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}
	if result.Total != 5 {
		t.Fatalf("total = %d, want 5", result.Total)
	}
	items, ok := result.Items.([]dto.CategoryResponse)
	if !ok {
		t.Fatalf("items is not []dto.CategoryResponse")
	}
	if len(items) != 3 {
		t.Fatalf("items count = %d, want 3", len(items))
	}
}

func TestCategoryService_Create_EmptyName(t *testing.T) {
	svc, _ := setupCategoryServiceTest(t)

	_, err := svc.Create(&dto.CreateCategoryRequest{Name: "---"})
	if err == nil {
		t.Fatal("expected error for slug that becomes empty")
	}
}
