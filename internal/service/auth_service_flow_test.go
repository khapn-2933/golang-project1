package service

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/kha/foods-drinks/internal/config"
	"github.com/kha/foods-drinks/internal/dto"
	"github.com/kha/foods-drinks/internal/models"
	"github.com/kha/foods-drinks/internal/repository"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func newAuthServiceTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Fatalf("open auth service test db: %v", err)
	}
	if err := db.AutoMigrate(&models.User{}, &models.Cart{}, &models.CartItem{}, &models.Category{}, &models.Product{}, &models.ProductImage{}); err != nil {
		t.Fatalf("migrate auth service test db: %v", err)
	}
	return db
}

func newAuthServiceForFlowTest(db *gorm.DB) *AuthService {
	userRepo := repository.NewUserRepository(db)
	cartRepo := repository.NewCartRepository(db)
	productRepo := repository.NewProductRepository(db)
	cartSvc := NewCartService(cartRepo, productRepo)
	jwtCfg := &config.JWTConfig{Secret: "auth-service-flow-secret", Expiration: 2 * time.Hour}
	return NewAuthService(userRepo, cartSvc, jwtCfg)
}

func TestAuthService_RegisterLoginProfileFlow(t *testing.T) {
	t.Parallel()

	db := newAuthServiceTestDB(t)
	svc := newAuthServiceForFlowTest(db)

	registerResp, err := svc.Register(&dto.RegisterRequest{
		Email:    "flow@example.com",
		Password: "Test@1234",
		FullName: "Flow User",
	})
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if registerResp.AccessToken == "" {
		t.Fatal("Register should return access token")
	}
	if registerResp.User.Email != "flow@example.com" {
		t.Fatalf("registered email = %s", registerResp.User.Email)
	}

	// Ensure cart was auto-created during register.
	cartRepo := repository.NewCartRepository(db)
	if _, err := cartRepo.FindByUserID(registerResp.User.ID); err != nil {
		t.Fatalf("expected cart for user after register, got err: %v", err)
	}

	loginResp, err := svc.Login(&dto.LoginRequest{Email: "flow@example.com", Password: "Test@1234"})
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if loginResp.AccessToken == "" {
		t.Fatal("Login should return access token")
	}

	phone := "0901234567"
	address := "Hanoi"
	updatedUser, err := svc.UpdateProfile(registerResp.User.ID, &dto.UpdateProfileRequest{FullName: "Flow User Updated", Phone: &phone, Address: &address})
	if err != nil {
		t.Fatalf("UpdateProfile: %v", err)
	}
	if updatedUser.FullName != "Flow User Updated" {
		t.Fatalf("full name = %s, want updated", updatedUser.FullName)
	}

	gotUser, err := svc.GetUserByID(registerResp.User.ID)
	if err != nil {
		t.Fatalf("GetUserByID: %v", err)
	}
	if gotUser.Phone == nil || *gotUser.Phone != phone {
		t.Fatalf("phone = %v, want %s", gotUser.Phone, phone)
	}
}

func TestAuthService_RegisterDuplicateAndLoginFailures(t *testing.T) {
	t.Parallel()

	db := newAuthServiceTestDB(t)
	svc := newAuthServiceForFlowTest(db)

	_, err := svc.Register(&dto.RegisterRequest{Email: "dup@example.com", Password: "Test@1234", FullName: "Dup User"})
	if err != nil {
		t.Fatalf("first register: %v", err)
	}
	_, err = svc.Register(&dto.RegisterRequest{Email: "dup@example.com", Password: "Test@1234", FullName: "Dup User"})
	if !errors.Is(err, ErrEmailAlreadyExists) {
		t.Fatalf("duplicate register err = %v, want ErrEmailAlreadyExists", err)
	}

	_, err = svc.Login(&dto.LoginRequest{Email: "dup@example.com", Password: "Wrong@1234"})
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("wrong password err = %v, want ErrInvalidCredentials", err)
	}

	_, err = svc.Login(&dto.LoginRequest{Email: "notfound@example.com", Password: "Test@1234"})
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("unknown email err = %v, want ErrInvalidCredentials", err)
	}
}

func TestAuthService_LoginInactiveAndBanned(t *testing.T) {
	t.Parallel()

	db := newAuthServiceTestDB(t)
	svc := newAuthServiceForFlowTest(db)
	hash, _ := svc.HashPassword("Test@1234")

	inactive := &models.User{Email: "inactive@example.com", PasswordHash: &hash, FullName: "Inactive", Role: models.RoleUser, Status: models.UserStatusInactive}
	banned := &models.User{Email: "banned@example.com", PasswordHash: &hash, FullName: "Banned", Role: models.RoleUser, Status: models.UserStatusBanned}
	if err := db.Create(inactive).Error; err != nil {
		t.Fatalf("create inactive: %v", err)
	}
	if err := db.Create(banned).Error; err != nil {
		t.Fatalf("create banned: %v", err)
	}

	_, err := svc.Login(&dto.LoginRequest{Email: "inactive@example.com", Password: "Test@1234"})
	if !errors.Is(err, ErrUserInactive) {
		t.Fatalf("inactive err = %v, want ErrUserInactive", err)
	}

	_, err = svc.Login(&dto.LoginRequest{Email: "banned@example.com", Password: "Test@1234"})
	if !errors.Is(err, ErrUserBanned) {
		t.Fatalf("banned err = %v, want ErrUserBanned", err)
	}
}
