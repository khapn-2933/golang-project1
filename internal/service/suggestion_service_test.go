package service

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/kha/foods-drinks/internal/dto"
	"github.com/kha/foods-drinks/internal/models"
	"github.com/kha/foods-drinks/internal/repository"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func newSuggestionServiceTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Fatalf("open suggestion test db: %v", err)
	}
	if err := db.AutoMigrate(&models.User{}, &models.Category{}, &models.Suggestion{}); err != nil {
		t.Fatalf("migrate suggestion test db: %v", err)
	}
	return db
}

func newSuggestionServiceForTest(db *gorm.DB) *SuggestionService {
	suggestionRepo := repository.NewSuggestionRepository(db)
	categoryRepo := repository.NewCategoryRepository(db)
	return NewSuggestionService(suggestionRepo, categoryRepo)
}

func TestNormalizeSuggestionNote(t *testing.T) {
	t.Parallel()

	if got := normalizeSuggestionNote(nil); got != nil {
		t.Fatal("normalizeSuggestionNote(nil) should return nil")
	}

	empty := "   "
	if got := normalizeSuggestionNote(&empty); got != nil {
		t.Fatal("normalizeSuggestionNote(whitespace) should return nil")
	}

	note := "  should be approved  "
	got := normalizeSuggestionNote(&note)
	if got == nil || *got != "should be approved" {
		t.Fatalf("normalized note = %v", got)
	}
}

func TestToSuggestionResponse(t *testing.T) {
	t.Parallel()

	desc := "new drink idea"
	adminNote := "looks good"
	now := time.Now()

	s := &models.Suggestion{
		ID:          10,
		UserID:      5,
		Name:        "Matcha Latte",
		Description: &desc,
		Classify:    models.ClassifyDrink,
		Status:      models.SuggestionStatusPending,
		AdminNote:   &adminNote,
		CreatedAt:   now,
		UpdatedAt:   now,
		User:        &models.User{FullName: "Tester"},
		Category:    &models.Category{Name: "Tea"},
	}

	resp := toSuggestionResponse(s)
	if resp.ID != 10 {
		t.Fatalf("id = %d, want 10", resp.ID)
	}
	if resp.UserName != "Tester" {
		t.Fatalf("user_name = %s, want Tester", resp.UserName)
	}
	if resp.CategoryName != "Tea" {
		t.Fatalf("category_name = %s, want Tea", resp.CategoryName)
	}
}

func TestSuggestionServiceCreate(t *testing.T) {
	t.Parallel()

	db := newSuggestionServiceTestDB(t)
	svc := newSuggestionServiceForTest(db)

	u := &models.User{Email: "suggest-create@example.com", FullName: "Suggest User", Role: models.RoleUser, Status: models.UserStatusActive}
	if err := db.Create(u).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	cat := &models.Category{Name: "Dessert", Slug: "dessert-suggest"}
	if err := db.Create(cat).Error; err != nil {
		t.Fatalf("create category: %v", err)
	}

	desc := "  add this item please  "
	resp, err := svc.Create(u.ID, &dto.CreateSuggestionRequest{
		Name:        "Mochi",
		Description: &desc,
		Classify:    models.ClassifyFood,
		CategoryID:  &cat.ID,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if resp.Status != models.SuggestionStatusPending {
		t.Fatalf("status = %s, want pending", resp.Status)
	}
	if resp.Description == nil || *resp.Description != "add this item please" {
		t.Fatalf("description = %v, want trimmed text", resp.Description)
	}
}

func TestSuggestionServiceCreate_CategoryNotFound(t *testing.T) {
	t.Parallel()

	db := newSuggestionServiceTestDB(t)
	svc := newSuggestionServiceForTest(db)
	u := &models.User{Email: "suggest-cat404@example.com", FullName: "Suggest User", Role: models.RoleUser, Status: models.UserStatusActive}
	if err := db.Create(u).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	missingID := uint(99999)
	_, err := svc.Create(u.ID, &dto.CreateSuggestionRequest{
		Name:       "No Cat",
		Classify:   models.ClassifyFood,
		CategoryID: &missingID,
	})
	if !errors.Is(err, ErrCategoryNotFound) {
		t.Fatalf("error = %v, want ErrCategoryNotFound", err)
	}
}

func TestSuggestionServiceListForAdmin(t *testing.T) {
	t.Parallel()

	db := newSuggestionServiceTestDB(t)
	svc := newSuggestionServiceForTest(db)

	u := &models.User{Email: "suggest-list@example.com", FullName: "Admin View User", Role: models.RoleUser, Status: models.UserStatusActive}
	if err := db.Create(u).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	if err := db.Create(&models.Suggestion{UserID: u.ID, Name: "Apple Pie", Classify: models.ClassifyFood, Status: models.SuggestionStatusPending}).Error; err != nil {
		t.Fatalf("seed suggestion 1: %v", err)
	}
	if err := db.Create(&models.Suggestion{UserID: u.ID, Name: "Lemon Tea", Classify: models.ClassifyDrink, Status: models.SuggestionStatusApproved}).Error; err != nil {
		t.Fatalf("seed suggestion 2: %v", err)
	}

	resp, err := svc.ListForAdmin(&dto.AdminSuggestionListRequest{Search: "Apple"})
	if err != nil {
		t.Fatalf("ListForAdmin: %v", err)
	}
	items, ok := resp.Items.([]dto.SuggestionResponse)
	if !ok {
		t.Fatalf("items type = %T, want []dto.SuggestionResponse", resp.Items)
	}
	if len(items) != 1 {
		t.Fatalf("len(items) = %d, want 1", len(items))
	}
	if items[0].Name != "Apple Pie" {
		t.Fatalf("item name = %s, want Apple Pie", items[0].Name)
	}
}

func TestSuggestionServiceUpdateStatusForAdmin(t *testing.T) {
	t.Parallel()

	db := newSuggestionServiceTestDB(t)
	svc := newSuggestionServiceForTest(db)

	u := &models.User{Email: "suggest-update@example.com", FullName: "Updater", Role: models.RoleUser, Status: models.UserStatusActive}
	if err := db.Create(u).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	s := &models.Suggestion{UserID: u.ID, Name: "Banh Mi", Classify: models.ClassifyFood, Status: models.SuggestionStatusPending}
	if err := db.Create(s).Error; err != nil {
		t.Fatalf("create suggestion: %v", err)
	}

	note := "  approved for menu  "
	err := svc.UpdateStatusForAdmin(s.ID, &dto.AdminUpdateSuggestionStatusRequest{
		Status:    models.SuggestionStatusApproved,
		AdminNote: &note,
	})
	if err != nil {
		t.Fatalf("UpdateStatusForAdmin: %v", err)
	}

	var got models.Suggestion
	if err := db.First(&got, s.ID).Error; err != nil {
		t.Fatalf("reload suggestion: %v", err)
	}
	if got.Status != models.SuggestionStatusApproved {
		t.Fatalf("status = %s, want approved", got.Status)
	}
	if got.AdminNote == nil || *got.AdminNote != "approved for menu" {
		t.Fatalf("admin note = %v, want trimmed note", got.AdminNote)
	}

	// Cannot update again once not pending.
	err = svc.UpdateStatusForAdmin(s.ID, &dto.AdminUpdateSuggestionStatusRequest{Status: models.SuggestionStatusRejected})
	if !errors.Is(err, ErrInvalidSuggestionState) {
		t.Fatalf("second update error = %v, want ErrInvalidSuggestionState", err)
	}
}
