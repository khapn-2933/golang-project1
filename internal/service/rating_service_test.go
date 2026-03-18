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

func newRatingServiceTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Fatalf("open rating test db: %v", err)
	}
	if err := db.AutoMigrate(
		&models.User{},
		&models.Category{},
		&models.Product{},
		&models.ProductImage{},
		&models.Order{},
		&models.OrderItem{},
		&models.Rating{},
	); err != nil {
		t.Fatalf("migrate rating test db: %v", err)
	}
	return db
}

func newRatingServiceForTest(db *gorm.DB) *RatingService {
	ratingRepo := repository.NewRatingRepository(db)
	productRepo := repository.NewProductRepository(db)
	return NewRatingService(ratingRepo, productRepo)
}

func TestNormalizeComment(t *testing.T) {
	t.Parallel()

	if got := normalizeComment(nil); got != nil {
		t.Fatal("normalizeComment(nil) should return nil")
	}

	empty := "   "
	if got := normalizeComment(&empty); got != nil {
		t.Fatal("normalizeComment(whitespace) should return nil")
	}

	val := "  nice drink  "
	got := normalizeComment(&val)
	if got == nil || *got != "nice drink" {
		t.Fatalf("normalizeComment trimmed = %v", got)
	}
}

func TestIsDuplicateRatingError(t *testing.T) {
	t.Parallel()

	if isDuplicateRatingError(nil) {
		t.Fatal("nil error should not be duplicate")
	}
	if !isDuplicateRatingError(gorm.ErrDuplicatedKey) {
		t.Fatal("gorm duplicated key should be duplicate")
	}
	if !isDuplicateRatingError(errors.New("Error 1062: Duplicate entry")) {
		t.Fatal("mysql duplicate message should be duplicate")
	}
	if isDuplicateRatingError(errors.New("random failure")) {
		t.Fatal("random error should not be duplicate")
	}
}

func TestRatingServiceFindProductBySlug(t *testing.T) {
	t.Parallel()

	db := newRatingServiceTestDB(t)
	productRepo := repository.NewProductRepository(db)
	svc := NewRatingService(nil, productRepo)

	cat := &models.Category{Name: "Rating Category", Slug: "rating-cat"}
	if err := db.Create(cat).Error; err != nil {
		t.Fatalf("create category: %v", err)
	}

	active := &models.Product{CategoryID: cat.ID, Name: "Active", Slug: "active-rating-product", Classify: models.ClassifyFood, Price: 10000, Stock: 10, Status: models.ProductStatusActive}
	inactive := &models.Product{CategoryID: cat.ID, Name: "Inactive", Slug: "inactive-rating-product", Classify: models.ClassifyFood, Price: 10000, Stock: 10, Status: models.ProductStatusInactive}
	if err := db.Create(active).Error; err != nil {
		t.Fatalf("create active: %v", err)
	}
	if err := db.Create(inactive).Error; err != nil {
		t.Fatalf("create inactive: %v", err)
	}

	p, err := svc.findProductBySlug(" active-rating-product ")
	if err != nil {
		t.Fatalf("find active product: %v", err)
	}
	if p.ID != active.ID {
		t.Fatalf("product id = %d, want %d", p.ID, active.ID)
	}

	if _, err := svc.findProductBySlug("inactive-rating-product"); !errors.Is(err, ErrProductNotFound) {
		t.Fatalf("inactive product error = %v, want ErrProductNotFound", err)
	}

	if _, err := svc.findProductBySlug("not-found-product"); !errors.Is(err, ErrProductNotFound) {
		t.Fatalf("not found product error = %v, want ErrProductNotFound", err)
	}
}

func TestRatingService_CreateUpdateList(t *testing.T) {
	t.Parallel()

	db := newRatingServiceTestDB(t)
	svc := newRatingServiceForTest(db)

	user := &models.User{Email: "rating-flow@example.com", FullName: "Rating User", Role: models.RoleUser, Status: models.UserStatusActive}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	cat := &models.Category{Name: "Rating", Slug: "rating-flow-cat"}
	if err := db.Create(cat).Error; err != nil {
		t.Fatalf("create category: %v", err)
	}
	product := &models.Product{CategoryID: cat.ID, Name: "Pho", Slug: "pho-rating-flow", Classify: models.ClassifyFood, Price: 45000, Stock: 10, Status: models.ProductStatusActive}
	if err := db.Create(product).Error; err != nil {
		t.Fatalf("create product: %v", err)
	}

	order := &models.Order{UserID: user.ID, OrderNumber: "ORD-RATING-001", TotalAmount: 45000, Status: models.OrderStatusDelivered, ShippingAddress: "HN", ShippingPhone: "0123"}
	if err := db.Create(order).Error; err != nil {
		t.Fatalf("create order: %v", err)
	}
	item := &models.OrderItem{OrderID: order.ID, ProductID: product.ID, ProductName: product.Name, ProductPrice: product.Price, Quantity: 1, Subtotal: product.Price}
	if err := db.Create(item).Error; err != nil {
		t.Fatalf("create order item: %v", err)
	}

	comment := "  good taste  "
	created, err := svc.CreateByProductSlug(user.ID, "pho-rating-flow", &dto.CreateRatingRequest{Rating: 5, Comment: &comment})
	if err != nil {
		t.Fatalf("CreateByProductSlug: %v", err)
	}
	if created.Comment == nil || *created.Comment != "good taste" {
		t.Fatalf("created comment = %v, want trimmed", created.Comment)
	}

	updatedComment := "  still good  "
	updated, err := svc.UpdateByProductSlug(user.ID, "pho-rating-flow", &dto.UpdateRatingRequest{Rating: 4, Comment: &updatedComment})
	if err != nil {
		t.Fatalf("UpdateByProductSlug: %v", err)
	}
	if updated.Rating != 4 {
		t.Fatalf("updated rating = %d, want 4", updated.Rating)
	}

	list, err := svc.ListByProductSlug("pho-rating-flow", &dto.RatingListRequest{Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("ListByProductSlug: %v", err)
	}
	items, ok := list.Items.([]dto.RatingResponse)
	if !ok {
		t.Fatalf("items type = %T, want []dto.RatingResponse", list.Items)
	}
	if len(items) != 1 {
		t.Fatalf("len(items) = %d, want 1", len(items))
	}
	if items[0].Rating != 4 {
		t.Fatalf("listed rating = %d, want 4", items[0].Rating)
	}
}

func TestRatingService_CreateByProductSlug_NotPurchased(t *testing.T) {
	t.Parallel()

	db := newRatingServiceTestDB(t)
	svc := newRatingServiceForTest(db)

	user := &models.User{Email: "rating-nopurchase@example.com", FullName: "No Purchase", Role: models.RoleUser, Status: models.UserStatusActive}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	cat := &models.Category{Name: "NoPurchase", Slug: "no-purchase-cat"}
	if err := db.Create(cat).Error; err != nil {
		t.Fatalf("create category: %v", err)
	}
	product := &models.Product{CategoryID: cat.ID, Name: "Tea", Slug: "tea-no-purchase", Classify: models.ClassifyDrink, Price: 20000, Stock: 10, Status: models.ProductStatusActive}
	if err := db.Create(product).Error; err != nil {
		t.Fatalf("create product: %v", err)
	}

	_, err := svc.CreateByProductSlug(user.ID, "tea-no-purchase", &dto.CreateRatingRequest{Rating: 5})
	if !errors.Is(err, ErrProductNotPurchased) {
		t.Fatalf("error = %v, want ErrProductNotPurchased", err)
	}
}
