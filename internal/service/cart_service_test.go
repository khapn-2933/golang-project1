package service

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/kha/foods-drinks/internal/dto"
	"github.com/kha/foods-drinks/internal/models"
	"github.com/kha/foods-drinks/internal/repository"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func newCartServiceTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Fatalf("open cart service test db: %v", err)
	}
	if err := db.AutoMigrate(
		&models.User{},
		&models.Category{},
		&models.Product{},
		&models.ProductImage{},
		&models.Cart{},
		&models.CartItem{},
	); err != nil {
		t.Fatalf("cart service migrate: %v", err)
	}
	return db
}

func newCartServiceForTest(db *gorm.DB) *CartService {
	cartRepo := repository.NewCartRepository(db)
	productRepo := repository.NewProductRepository(db)
	return NewCartService(cartRepo, productRepo)
}

func seedUserForCartTest(t *testing.T, db *gorm.DB, email string) *models.User {
	t.Helper()
	u := &models.User{Email: email, FullName: "Cart User", Role: models.RoleUser, Status: models.UserStatusActive}
	if err := db.Create(u).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}
	return u
}

func seedProductForCartTest(t *testing.T, db *gorm.DB, slug string, stock int) *models.Product {
	t.Helper()
	cat := &models.Category{Name: "Cart Category " + slug, Slug: "cart-cat-" + slug}
	if err := db.Create(cat).Error; err != nil {
		t.Fatalf("seed category: %v", err)
	}
	p := &models.Product{
		CategoryID: cat.ID,
		Name:       "Product " + slug,
		Slug:       slug,
		Classify:   models.ClassifyFood,
		Price:      10000,
		Stock:      stock,
		Status:     models.ProductStatusActive,
	}
	if err := db.Create(p).Error; err != nil {
		t.Fatalf("seed product: %v", err)
	}
	return p
}

func TestCartService_EnsureCartForUserAndGetCart(t *testing.T) {
	t.Parallel()
	db := newCartServiceTestDB(t)
	svc := newCartServiceForTest(db)
	u := seedUserForCartTest(t, db, "ensure-cart@example.com")

	if err := svc.EnsureCartForUser(u.ID); err != nil {
		t.Fatalf("EnsureCartForUser: %v", err)
	}

	cart, err := svc.GetCart(u.ID)
	if err != nil {
		t.Fatalf("GetCart: %v", err)
	}
	if cart.ID == 0 {
		t.Error("expected cart id to be non-zero")
	}
	if len(cart.Items) != 0 {
		t.Errorf("expected empty cart items, got %d", len(cart.Items))
	}
}

func TestCartService_AddItem_SuccessAndMergeQuantity(t *testing.T) {
	t.Parallel()
	db := newCartServiceTestDB(t)
	svc := newCartServiceForTest(db)
	u := seedUserForCartTest(t, db, "add-item@example.com")
	p := seedProductForCartTest(t, db, "add-item-slug", 10)

	resp, err := svc.AddItem(u.ID, &dto.AddCartItemRequest{ProductID: p.ID, Quantity: 2})
	if err != nil {
		t.Fatalf("AddItem first: %v", err)
	}
	if len(resp.Items) != 1 || resp.Items[0].Quantity != 2 {
		t.Fatalf("expected 1 item qty=2, got items=%d qty=%d", len(resp.Items), resp.Items[0].Quantity)
	}

	resp, err = svc.AddItem(u.ID, &dto.AddCartItemRequest{ProductID: p.ID, Quantity: 3})
	if err != nil {
		t.Fatalf("AddItem second: %v", err)
	}
	if len(resp.Items) != 1 || resp.Items[0].Quantity != 5 {
		t.Fatalf("expected merged qty=5, got items=%d qty=%d", len(resp.Items), resp.Items[0].Quantity)
	}
}

func TestCartService_AddItem_InvalidQuantity(t *testing.T) {
	t.Parallel()
	db := newCartServiceTestDB(t)
	svc := newCartServiceForTest(db)
	u := seedUserForCartTest(t, db, "invalid-qty@example.com")
	p := seedProductForCartTest(t, db, "invalid-qty-slug", 10)

	_, err := svc.AddItem(u.ID, &dto.AddCartItemRequest{ProductID: p.ID, Quantity: 0})
	if err == nil {
		t.Fatal("expected error for quantity=0")
	}
	if err != ErrInvalidQuantity {
		t.Fatalf("error = %v, want %v", err, ErrInvalidQuantity)
	}
}

func TestCartService_AddItem_InsufficientStock(t *testing.T) {
	t.Parallel()
	db := newCartServiceTestDB(t)
	svc := newCartServiceForTest(db)
	u := seedUserForCartTest(t, db, "stock@example.com")
	p := seedProductForCartTest(t, db, "stock-slug", 2)

	_, err := svc.AddItem(u.ID, &dto.AddCartItemRequest{ProductID: p.ID, Quantity: 5})
	if err == nil {
		t.Fatal("expected insufficient stock error")
	}
	if !errors.Is(err, ErrInsufficientStock) {
		t.Fatalf("error = %v, want %v", err, ErrInsufficientStock)
	}
}

func TestCartService_UpdateItem(t *testing.T) {
	t.Parallel()
	db := newCartServiceTestDB(t)
	svc := newCartServiceForTest(db)
	u := seedUserForCartTest(t, db, "update-item@example.com")
	p := seedProductForCartTest(t, db, "update-item-slug", 10)

	_, err := svc.AddItem(u.ID, &dto.AddCartItemRequest{ProductID: p.ID, Quantity: 1})
	if err != nil {
		t.Fatalf("AddItem: %v", err)
	}

	resp, err := svc.UpdateItem(u.ID, p.ID, 4)
	if err != nil {
		t.Fatalf("UpdateItem: %v", err)
	}
	if len(resp.Items) != 1 || resp.Items[0].Quantity != 4 {
		t.Fatalf("expected qty=4 after update, got %d", resp.Items[0].Quantity)
	}
}

func TestCartService_RemoveAndClear(t *testing.T) {
	t.Parallel()
	db := newCartServiceTestDB(t)
	svc := newCartServiceForTest(db)
	u := seedUserForCartTest(t, db, "remove-clear@example.com")
	p1 := seedProductForCartTest(t, db, "remove-1-slug", 10)
	p2 := seedProductForCartTest(t, db, "remove-2-slug", 10)

	resp, err := svc.AddItem(u.ID, &dto.AddCartItemRequest{ProductID: p1.ID, Quantity: 1})
	if err != nil {
		t.Fatalf("AddItem p1: %v", err)
	}
	if _, err := svc.AddItem(u.ID, &dto.AddCartItemRequest{ProductID: p2.ID, Quantity: 2}); err != nil {
		t.Fatalf("AddItem p2: %v", err)
	}
	_ = resp

	resp, err = svc.RemoveItem(u.ID, p1.ID)
	if err != nil {
		t.Fatalf("RemoveItem: %v", err)
	}
	if len(resp.Items) != 1 {
		t.Fatalf("expected 1 item after remove, got %d", len(resp.Items))
	}

	resp, err = svc.ClearCart(u.ID)
	if err != nil {
		t.Fatalf("ClearCart: %v", err)
	}
	if len(resp.Items) != 0 {
		t.Fatalf("expected 0 items after clear, got %d", len(resp.Items))
	}
}

func TestCartService_UpdateAndRemove_NotFound(t *testing.T) {
	t.Parallel()
	db := newCartServiceTestDB(t)
	svc := newCartServiceForTest(db)
	u := seedUserForCartTest(t, db, "notfound-item@example.com")

	_, err := svc.UpdateItem(u.ID, 9999, 2)
	if err == nil {
		t.Fatal("expected UpdateItem not found error")
	}
	if err != ErrProductNotFound {
		t.Fatalf("error = %v, want %v", err, ErrProductNotFound)
	}

	// RemoveItem deletes by product_id and does not fail when item does not exist.
	if _, err = svc.RemoveItem(u.ID, 9999); err != nil {
		t.Fatalf("RemoveItem should ignore missing item, got error: %v", err)
	}
}
