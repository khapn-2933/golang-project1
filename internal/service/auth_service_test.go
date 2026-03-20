package service

import (
	"testing"
	"time"

	"github.com/kha/foods-drinks/internal/config"
	"github.com/kha/foods-drinks/internal/models"
)

func jwtTestConfig() *config.JWTConfig {
	return &config.JWTConfig{
		Secret:     "test-secret-key-12345",
		Expiration: 24 * time.Hour,
	}
}

func testAuthService() *AuthService {
	return NewAuthServiceWithConfig(jwtTestConfig())
}

func TestHashPassword(t *testing.T) {
	t.Parallel()

	svc := testAuthService()
	hash, err := svc.HashPassword("password123")
	if err != nil {
		t.Fatalf("HashPassword error: %v", err)
	}
	if hash == "" {
		t.Fatal("HashPassword returned empty string")
	}
	if hash == "password123" {
		t.Fatal("HashPassword returned plaintext")
	}
}

func TestCheckPassword(t *testing.T) {
	t.Parallel()

	svc := testAuthService()
	hash, _ := svc.HashPassword("correct-password")

	if !svc.CheckPassword("correct-password", hash) {
		t.Fatal("CheckPassword returned false for correct password")
	}
	if svc.CheckPassword("wrong-password", hash) {
		t.Fatal("CheckPassword returned true for wrong password")
	}
}

func TestGenerateAndValidateToken(t *testing.T) {
	t.Parallel()

	svc := testAuthService()
	user := &models.User{
		ID:    42,
		Email: "user@example.com",
		Role:  models.RoleUser,
	}

	token, expiresIn, err := svc.GenerateToken(user)
	if err != nil {
		t.Fatalf("GenerateToken error: %v", err)
	}
	if token == "" {
		t.Fatal("GenerateToken returned empty token")
	}
	if expiresIn <= 0 {
		t.Fatalf("GenerateToken expiresIn = %d, want > 0", expiresIn)
	}

	claims, err := svc.ValidateToken(token)
	if err != nil {
		t.Fatalf("ValidateToken error: %v", err)
	}
	if claims.UserID != user.ID {
		t.Fatalf("claims.UserID = %d, want %d", claims.UserID, user.ID)
	}
	if claims.Email != user.Email {
		t.Fatalf("claims.Email = %q, want %q", claims.Email, user.Email)
	}
	if claims.Role != user.Role {
		t.Fatalf("claims.Role = %q, want %q", claims.Role, user.Role)
	}
}

func TestValidateToken_InvalidToken(t *testing.T) {
	t.Parallel()

	svc := testAuthService()

	t.Run("empty token", func(t *testing.T) {
		t.Parallel()
		_, err := svc.ValidateToken("")
		if err == nil {
			t.Fatal("expected error for empty token, got nil")
		}
	})

	t.Run("malformed token", func(t *testing.T) {
		t.Parallel()
		_, err := svc.ValidateToken("not.a.valid.jwt")
		if err == nil {
			t.Fatal("expected error for malformed token, got nil")
		}
	})

	t.Run("wrong secret", func(t *testing.T) {
		t.Parallel()
		svc2 := &AuthService{jwtConfig: &config.JWTConfig{Secret: "other-secret", Expiration: time.Hour}}
		token, _, _ := svc2.GenerateToken(&models.User{ID: 1, Email: "a@b.com", Role: models.RoleUser})
		_, err := svc.ValidateToken(token)
		if err == nil {
			t.Fatal("expected error when validating token signed with wrong secret, got nil")
		}
	})
}

func TestGenerateToken_AdminRole(t *testing.T) {
	t.Parallel()

	svc := testAuthService()
	admin := &models.User{ID: 99, Email: "admin@example.com", Role: models.RoleAdmin}
	token, _, err := svc.GenerateToken(admin)
	if err != nil {
		t.Fatalf("GenerateToken error: %v", err)
	}
	claims, err := svc.ValidateToken(token)
	if err != nil {
		t.Fatalf("ValidateToken error: %v", err)
	}
	if claims.Role != models.RoleAdmin {
		t.Fatalf("claims.Role = %q, want %q", claims.Role, models.RoleAdmin)
	}
}
