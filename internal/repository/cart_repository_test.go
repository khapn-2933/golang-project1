package repository

import (
	"fmt"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/kha/foods-drinks/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func newCartRepoTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Fatalf("open cart repo test db: %v", err)
	}
	if err := db.AutoMigrate(&models.User{}, &models.Category{}, &models.Product{}, &models.ProductImage{}, &models.Cart{}, &models.CartItem{}); err != nil {
		t.Fatalf("migrate cart repo test db: %v", err)
	}
	return db
}

func seedCartRepoUser(t *testing.T, db *gorm.DB, email string) *models.User {
	t.Helper()
	u := &models.User{Email: email, FullName: "Cart Repo User", Role: models.RoleUser, Status: models.UserStatusActive}
	if err := db.Create(u).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	return u
}

func seedCartRepoProduct(t *testing.T, db *gorm.DB, slug string) *models.Product {
	t.Helper()
	cat := &models.Category{Name: "Cart Repo Cat", Slug: "cart-repo-cat-" + slug}
	if err := db.Create(cat).Error; err != nil {
		t.Fatalf("create category: %v", err)
	}
	p := &models.Product{CategoryID: cat.ID, Name: "Cart Repo Product", Slug: slug, Classify: models.ClassifyFood, Price: 10000, Stock: 10, Status: models.ProductStatusActive}
	if err := db.Create(p).Error; err != nil {
		t.Fatalf("create product: %v", err)
	}
	return p
}

func TestCartRepository_GetOrCreateAndFindByUserID(t *testing.T) {
	t.Parallel()
	db := newCartRepoTestDB(t)
	repo := NewCartRepository(db)
	u := seedCartRepoUser(t, db, "cartrepo@example.com")

	cart, err := repo.GetOrCreateByUserID(u.ID)
	if err != nil {
		t.Fatalf("GetOrCreateByUserID: %v", err)
	}
	if cart.ID == 0 {
		t.Fatal("expected cart id")
	}

	found, err := repo.FindByUserID(u.ID)
	if err != nil {
		t.Fatalf("FindByUserID: %v", err)
	}
	if found.UserID != u.ID {
		t.Fatalf("found user_id = %d, want %d", found.UserID, u.ID)
	}

	exists, err := repo.ExistsByUserID(u.ID)
	if err != nil {
		t.Fatalf("ExistsByUserID: %v", err)
	}
	if !exists {
		t.Fatal("cart should exist")
	}
}

func TestCartRepository_CartItemCRUD(t *testing.T) {
	t.Parallel()
	db := newCartRepoTestDB(t)
	repo := NewCartRepository(db)
	u := seedCartRepoUser(t, db, "cartitem@example.com")
	p := seedCartRepoProduct(t, db, "cart-item-product")

	cart, err := repo.GetOrCreateByUserID(u.ID)
	if err != nil {
		t.Fatalf("GetOrCreateByUserID: %v", err)
	}
	item := &models.CartItem{CartID: cart.ID, ProductID: p.ID, Quantity: 2}
	if err := repo.CreateCartItem(item); err != nil {
		t.Fatalf("CreateCartItem: %v", err)
	}

	found, err := repo.FindCartItem(cart.ID, p.ID)
	if err != nil {
		t.Fatalf("FindCartItem: %v", err)
	}
	if found.Quantity != 2 {
		t.Fatalf("quantity = %d, want 2", found.Quantity)
	}

	found.Quantity = 5
	if err := repo.UpdateCartItem(found); err != nil {
		t.Fatalf("UpdateCartItem: %v", err)
	}
	found, _ = repo.FindCartItem(cart.ID, p.ID)
	if found.Quantity != 5 {
		t.Fatalf("updated quantity = %d, want 5", found.Quantity)
	}

	if err := repo.DeleteCartItem(cart.ID, p.ID); err != nil {
		t.Fatalf("DeleteCartItem: %v", err)
	}
	if _, err := repo.FindCartItem(cart.ID, p.ID); err == nil {
		t.Fatal("expected item not found after delete")
	}

	// Recreate and then clear all items.
	_ = repo.CreateCartItem(&models.CartItem{CartID: cart.ID, ProductID: p.ID, Quantity: 1})
	if err := repo.ClearCartItems(cart.ID); err != nil {
		t.Fatalf("ClearCartItems: %v", err)
	}
	var count int64
	db.Model(&models.CartItem{}).Where("cart_id = ?", cart.ID).Count(&count)
	if count != 0 {
		t.Fatalf("expected 0 cart items after clear, got %d", count)
	}
}
