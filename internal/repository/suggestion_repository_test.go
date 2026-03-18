package repository

import (
	"fmt"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/kha/foods-drinks/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func newSuggestionRepoTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Fatalf("open suggestion repo test db: %v", err)
	}
	if err := db.AutoMigrate(&models.User{}, &models.Category{}, &models.Suggestion{}); err != nil {
		t.Fatalf("migrate suggestion repo test db: %v", err)
	}
	return db
}

func TestSuggestionRepository_CRUDAndList(t *testing.T) {
	t.Parallel()
	db := newSuggestionRepoTestDB(t)
	repo := NewSuggestionRepository(db)

	u := &models.User{Email: "suggestrepo@example.com", FullName: "Suggest Repo User", Role: models.RoleUser, Status: models.UserStatusActive}
	db.Create(u)
	cat := &models.Category{Name: "Suggest Cat", Slug: "suggest-cat-repo"}
	db.Create(cat)

	s := &models.Suggestion{UserID: u.ID, Name: "Matcha", Classify: models.ClassifyDrink, CategoryID: &cat.ID, Status: models.SuggestionStatusPending}
	if err := repo.Create(s); err != nil {
		t.Fatalf("Create: %v", err)
	}

	found, err := repo.FindByID(s.ID)
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if found.Name != "Matcha" {
		t.Fatalf("name=%s want Matcha", found.Name)
	}

	locked, err := repo.FindByIDForUpdate(s.ID)
	if err != nil {
		t.Fatalf("FindByIDForUpdate: %v", err)
	}
	locked.Status = models.SuggestionStatusApproved
	if err := repo.Update(locked); err != nil {
		t.Fatalf("Update: %v", err)
	}

	list, total, err := repo.ListForAdmin(SuggestionListParams{Offset: 0, Limit: 10, Search: "Mat", SortBy: "name", SortDir: "asc"})
	if err != nil {
		t.Fatalf("ListForAdmin: %v", err)
	}
	if total != 1 || len(list) != 1 {
		t.Fatalf("expected 1 suggestion, total=%d len=%d", total, len(list))
	}
	if list[0].Status != models.SuggestionStatusApproved {
		t.Fatalf("status=%s want approved", list[0].Status)
	}
}
