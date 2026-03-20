package repository

import (
	"fmt"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/kha/foods-drinks/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func newUserRepoTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Fatalf("open user repo test db: %v", err)
	}
	if err := db.AutoMigrate(&models.User{}); err != nil {
		t.Fatalf("migrate user: %v", err)
	}
	return db
}

func TestUserRepositoryCreateAndFindByID(t *testing.T) {
	t.Parallel()
	db := newUserRepoTestDB(t)
	repo := NewUserRepository(db)

	hash := "hashed-password"
	u := &models.User{
		Email:        "user1@example.com",
		PasswordHash: &hash,
		FullName:     "User One",
		Role:         models.RoleUser,
		Status:       models.UserStatusActive,
	}
	if err := repo.Create(u); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := repo.FindByID(u.ID)
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if got.Email != "user1@example.com" {
		t.Errorf("email = %s, want user1@example.com", got.Email)
	}
}

func TestUserRepositoryFindByEmail(t *testing.T) {
	t.Parallel()
	db := newUserRepoTestDB(t)
	repo := NewUserRepository(db)

	u := &models.User{Email: "findme@example.com", FullName: "Find Me", Role: models.RoleUser, Status: models.UserStatusActive}
	if err := repo.Create(u); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := repo.FindByEmail("findme@example.com")
	if err != nil {
		t.Fatalf("FindByEmail: %v", err)
	}
	if got.ID != u.ID {
		t.Errorf("id = %d, want %d", got.ID, u.ID)
	}
}

func TestUserRepositoryExistsByEmail(t *testing.T) {
	t.Parallel()
	db := newUserRepoTestDB(t)
	repo := NewUserRepository(db)

	exists, err := repo.ExistsByEmail("none@example.com")
	if err != nil {
		t.Fatalf("ExistsByEmail(none): %v", err)
	}
	if exists {
		t.Fatal("expected not exists for none@example.com")
	}

	u := &models.User{Email: "exists@example.com", FullName: "Exists", Role: models.RoleUser, Status: models.UserStatusActive}
	if err := repo.Create(u); err != nil {
		t.Fatalf("Create: %v", err)
	}

	exists, err = repo.ExistsByEmail("exists@example.com")
	if err != nil {
		t.Fatalf("ExistsByEmail(exists): %v", err)
	}
	if !exists {
		t.Fatal("expected exists for exists@example.com")
	}
}

func TestUserRepositoryUpdate(t *testing.T) {
	t.Parallel()
	db := newUserRepoTestDB(t)
	repo := NewUserRepository(db)

	u := &models.User{Email: "update@example.com", FullName: "Before", Role: models.RoleUser, Status: models.UserStatusActive}
	if err := repo.Create(u); err != nil {
		t.Fatalf("Create: %v", err)
	}

	u.FullName = "After"
	u.Status = models.UserStatusInactive
	if err := repo.Update(u); err != nil {
		t.Fatalf("Update: %v", err)
	}

	got, err := repo.FindByID(u.ID)
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if got.FullName != "After" {
		t.Errorf("full_name = %s, want After", got.FullName)
	}
	if got.Status != models.UserStatusInactive {
		t.Errorf("status = %s, want %s", got.Status, models.UserStatusInactive)
	}
}

func TestUserRepositoryDelete(t *testing.T) {
	t.Parallel()
	db := newUserRepoTestDB(t)
	repo := NewUserRepository(db)

	u := &models.User{Email: "delete@example.com", FullName: "Delete Me", Role: models.RoleUser, Status: models.UserStatusActive}
	if err := repo.Create(u); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := repo.Delete(u.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	if _, err := repo.FindByID(u.ID); err == nil {
		t.Fatal("expected FindByID to fail after delete")
	}
}
