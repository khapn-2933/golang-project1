package repository

import (
	"fmt"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/kha/foods-drinks/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func newRatingRepoTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Fatalf("open rating repo test db: %v", err)
	}
	if err := db.AutoMigrate(&models.User{}, &models.Category{}, &models.Product{}, &models.Order{}, &models.OrderItem{}, &models.Rating{}); err != nil {
		t.Fatalf("migrate rating repo test db: %v", err)
	}
	return db
}

func TestRatingRepository_CRUDAndStats(t *testing.T) {
	t.Parallel()
	db := newRatingRepoTestDB(t)
	repo := NewRatingRepository(db)

	u := &models.User{Email: "ratingrepo@example.com", FullName: "Rating Repo User", Role: models.RoleUser, Status: models.UserStatusActive}
	db.Create(u)
	cat := &models.Category{Name: "Rating Repo Cat", Slug: "rating-repo-cat"}
	db.Create(cat)
	p := &models.Product{CategoryID: cat.ID, Name: "Repo Product", Slug: "repo-product", Classify: models.ClassifyFood, Price: 10000, Stock: 10, Status: models.ProductStatusActive}
	db.Create(p)
	order := &models.Order{UserID: u.ID, OrderNumber: "ORD-RR-1", TotalAmount: 10000, Status: models.OrderStatusDelivered, ShippingAddress: "HN", ShippingPhone: "0900"}
	db.Create(order)
	db.Create(&models.OrderItem{OrderID: order.ID, ProductID: p.ID, ProductName: p.Name, ProductPrice: p.Price, Quantity: 1, Subtotal: p.Price})

	r := &models.Rating{UserID: u.ID, ProductID: p.ID, OrderID: &order.ID, Rating: 5}
	if err := repo.Create(r); err != nil {
		t.Fatalf("Create: %v", err)
	}

	found, err := repo.FindByUserAndProduct(u.ID, p.ID)
	if err != nil {
		t.Fatalf("FindByUserAndProduct: %v", err)
	}
	if found.Rating != 5 {
		t.Fatalf("rating = %d, want 5", found.Rating)
	}

	found.Rating = 4
	if err := repo.Update(found); err != nil {
		t.Fatalf("Update: %v", err)
	}

	list, total, err := repo.ListByProductID(p.ID, 0, 10)
	if err != nil {
		t.Fatalf("ListByProductID: %v", err)
	}
	if total != 1 || len(list) != 1 {
		t.Fatalf("expected one rating, total=%d len=%d", total, len(list))
	}

	purchasedOrderID, err := repo.FindPurchasedOrderID(u.ID, p.ID)
	if err != nil {
		t.Fatalf("FindPurchasedOrderID: %v", err)
	}
	if purchasedOrderID == nil || *purchasedOrderID != order.ID {
		t.Fatalf("purchased order id = %v, want %d", purchasedOrderID, order.ID)
	}

	avg, count, err := repo.CalcProductRatingStats(p.ID)
	if err != nil {
		t.Fatalf("CalcProductRatingStats: %v", err)
	}
	if count != 1 || avg != 4 {
		t.Fatalf("stats avg=%v count=%d, want avg=4 count=1", avg, count)
	}

	if err := repo.UpdateProductRatingStats(p.ID, avg, count); err != nil {
		t.Fatalf("UpdateProductRatingStats: %v", err)
	}
	var updated models.Product
	if err := db.First(&updated, p.ID).Error; err != nil {
		t.Fatalf("reload product: %v", err)
	}
	if updated.RatingCount != int(count) {
		t.Fatalf("rating_count=%d want=%d", updated.RatingCount, count)
	}
}
