package service

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/kha/foods-drinks/internal/dto"
	"github.com/kha/foods-drinks/internal/models"
	"github.com/kha/foods-drinks/internal/repository"
	"gorm.io/gorm"
)

type orderTestNotifier struct {
	called bool
	order  *dto.OrderResponse
}

func (n *orderTestNotifier) NotifyNewOrderAsync(order *dto.OrderResponse) {
	n.called = true
	n.order = order
}

func setupOrderServiceTest(t *testing.T) (*OrderService, *gorm.DB, *orderTestNotifier) {
	t.Helper()

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}

	if err := db.AutoMigrate(
		&models.User{},
		&models.Category{},
		&models.Product{},
		&models.ProductImage{},
		&models.Cart{},
		&models.CartItem{},
		&models.Order{},
		&models.OrderItem{},
	); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	user := models.User{Email: "order-test@example.com", FullName: "Order Test User", Role: models.RoleUser, Status: models.UserStatusActive}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}

	category := models.Category{Name: "Foods", Slug: "foods", Status: models.CategoryStatusActive}
	if err := db.Create(&category).Error; err != nil {
		t.Fatalf("seed category: %v", err)
	}

	product := models.Product{
		CategoryID: category.ID,
		Name:       "Pho Bo",
		Slug:       "pho-bo",
		Classify:   models.ClassifyFood,
		Price:      50000,
		Stock:      10,
		Status:     models.ProductStatusActive,
	}
	if err := db.Create(&product).Error; err != nil {
		t.Fatalf("seed product: %v", err)
	}

	cart := models.Cart{UserID: user.ID}
	if err := db.Create(&cart).Error; err != nil {
		t.Fatalf("seed cart: %v", err)
	}

	cartItem := models.CartItem{CartID: cart.ID, ProductID: product.ID, Quantity: 2}
	if err := db.Create(&cartItem).Error; err != nil {
		t.Fatalf("seed cart item: %v", err)
	}

	orderRepo := repository.NewOrderRepository(db)
	cartRepo := repository.NewCartRepository(db)
	productRepo := repository.NewProductRepository(db)
	notifier := &orderTestNotifier{}

	return NewOrderService(orderRepo, cartRepo, productRepo, notifier), db, notifier
}

// ─── generateOrderNumber ────────────────────────────────────────────────────

func TestGenerateOrderNumber_Format(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 3, 18, 0, 0, 0, 0, time.UTC)
	number := generateOrderNumber(now)

	if !strings.HasPrefix(number, "ORDER-20260318-") {
		t.Fatalf("generateOrderNumber = %q, expected prefix ORDER-20260318-", number)
	}
	// suffix is 4 digits in range 1000-9999
	parts := strings.Split(number, "-")
	if len(parts) != 3 {
		t.Fatalf("generateOrderNumber = %q, expected 3 dash-separated parts", number)
	}
	suffix := parts[2]
	if len(suffix) != 4 {
		t.Fatalf("order number suffix %q should be 4 digits", suffix)
	}
}

func TestGenerateOrderNumber_IsUnique(t *testing.T) {
	t.Parallel()

	seen := make(map[string]bool)
	now := time.Now()
	for i := 0; i < 50; i++ {
		n := generateOrderNumber(now)
		seen[n] = true
	}
	// With random 4-digit suffix, collisions are possible but unlikely in 50 iterations
	if len(seen) == 1 {
		t.Fatal("generateOrderNumber returned the same value 50 times in a row")
	}
}

// ─── isDuplicateKeyError ─────────────────────────────────────────────────────

func TestIsDuplicateKeyError(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil", err: nil, want: false},
		{name: "random error", err: errors.New("something went wrong"), want: false},
		{name: "gorm duplicate key", err: gorm.ErrDuplicatedKey, want: true},
		{name: "mysql 1062", err: errors.New("Error 1062: duplicate entry"), want: true},
		{name: "duplicate word", err: errors.New("duplicate key value violates unique constraint"), want: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := isDuplicateKeyError(tc.err)
			if got != tc.want {
				t.Fatalf("isDuplicateKeyError(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}

// ─── isValidOrderStatus ──────────────────────────────────────────────────────

func TestIsValidOrderStatus(t *testing.T) {
	t.Parallel()

	valid := []string{"pending", "confirmed", "processing", "shipping", "delivered", "cancelled"}
	for _, s := range valid {
		if !isValidOrderStatus(s) {
			t.Errorf("isValidOrderStatus(%q) = false, want true", s)
		}
	}

	invalid := []string{"", "unknown", "PENDING", "done"}
	for _, s := range invalid {
		if isValidOrderStatus(s) {
			t.Errorf("isValidOrderStatus(%q) = true, want false", s)
		}
	}
}

// ─── canTransitionOrderStatus ────────────────────────────────────────────────

func TestCanTransitionOrderStatus(t *testing.T) {
	t.Parallel()

	cases := []struct {
		from string
		to   string
		want bool
	}{
		// Same status → always allowed
		{"pending", "pending", true},
		{"delivered", "delivered", true},
		// Valid transitions
		{"pending", "confirmed", true},
		{"pending", "cancelled", true},
		{"confirmed", "processing", true},
		{"confirmed", "cancelled", true},
		{"processing", "shipping", true},
		{"processing", "cancelled", true},
		{"shipping", "delivered", true},
		// Invalid transitions
		{"pending", "delivered", false},
		{"pending", "shipping", false},
		{"delivered", "pending", false},
		{"delivered", "cancelled", false},
		{"cancelled", "pending", false},
		{"shipping", "cancelled", false},
	}

	for _, tc := range cases {
		t.Run(tc.from+"->"+tc.to, func(t *testing.T) {
			t.Parallel()
			got := canTransitionOrderStatus(tc.from, tc.to)
			if got != tc.want {
				t.Fatalf("canTransitionOrderStatus(%q, %q) = %v, want %v", tc.from, tc.to, got, tc.want)
			}
		})
	}
}

// ─── parseOrderDateRange ─────────────────────────────────────────────────────

func TestParseOrderDateRange(t *testing.T) {
	t.Parallel()

	t.Run("both empty", func(t *testing.T) {
		t.Parallel()
		from, to, err := parseOrderDateRange("", "")
		if err != nil || from != nil || to != nil {
			t.Fatalf("empty strings: got from=%v to=%v err=%v", from, to, err)
		}
	})

	t.Run("valid range", func(t *testing.T) {
		t.Parallel()
		from, to, err := parseOrderDateRange("2026-01-01", "2026-01-31")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if from == nil || to == nil {
			t.Fatal("expected non-nil from and to")
		}
		if from.Year() != 2026 || from.Month() != 1 || from.Day() != 1 {
			t.Fatalf("from date mismatch: %v", from)
		}
	})

	t.Run("invalid from date", func(t *testing.T) {
		t.Parallel()
		_, _, err := parseOrderDateRange("not-a-date", "")
		if !errors.Is(err, ErrInvalidDateFilter) {
			t.Fatalf("expected ErrInvalidDateFilter, got %v", err)
		}
	})

	t.Run("invalid to date", func(t *testing.T) {
		t.Parallel()
		_, _, err := parseOrderDateRange("", "not-a-date")
		if !errors.Is(err, ErrInvalidDateFilter) {
			t.Fatalf("expected ErrInvalidDateFilter, got %v", err)
		}
	})

	t.Run("from after to", func(t *testing.T) {
		t.Parallel()
		_, _, err := parseOrderDateRange("2026-12-31", "2026-01-01")
		if !errors.Is(err, ErrInvalidDateFilter) {
			t.Fatalf("expected ErrInvalidDateFilter, got %v", err)
		}
	})
}

func TestOrderService_CreateOrderFromCart(t *testing.T) {
	t.Parallel()

	svc, db, notifier := setupOrderServiceTest(t)

	order, err := svc.CreateOrderFromCart(1, &dto.CreateOrderRequest{
		ShippingAddress: "123 Le Loi",
		ShippingPhone:   "0901234567",
	})
	if err != nil {
		t.Fatalf("CreateOrderFromCart returned error: %v", err)
	}
	if order == nil || order.ID == 0 {
		t.Fatal("expected created order with non-zero id")
	}
	if order.TotalAmount != 100000 {
		t.Fatalf("total amount = %v, want 100000", order.TotalAmount)
	}
	if order.ItemCount != 1 {
		t.Fatalf("item count = %d, want 1", order.ItemCount)
	}

	if !notifier.called || notifier.order == nil || notifier.order.ID != order.ID {
		t.Fatal("expected notifier to be called with created order")
	}

	var product models.Product
	if err := db.First(&product, 1).Error; err != nil {
		t.Fatalf("query product: %v", err)
	}
	if product.Stock != 8 {
		t.Fatalf("product stock = %d, want 8", product.Stock)
	}

	var remaining int64
	if err := db.Model(&models.CartItem{}).Where("cart_id = ?", 1).Count(&remaining).Error; err != nil {
		t.Fatalf("count cart items: %v", err)
	}
	if remaining != 0 {
		t.Fatalf("remaining cart items = %d, want 0", remaining)
	}
}

func TestOrderService_CreateOrderFromCart_InvalidInputAndEmptyCart(t *testing.T) {
	t.Parallel()

	svc, db, _ := setupOrderServiceTest(t)

	_, err := svc.CreateOrderFromCart(1, &dto.CreateOrderRequest{ShippingAddress: " ", ShippingPhone: " "})
	if !errors.Is(err, ErrInvalidOrderInput) {
		t.Fatalf("expected ErrInvalidOrderInput, got %v", err)
	}

	if err := db.Where("1 = 1").Delete(&models.CartItem{}).Error; err != nil {
		t.Fatalf("clear cart items: %v", err)
	}

	_, err = svc.CreateOrderFromCart(1, &dto.CreateOrderRequest{ShippingAddress: "A", ShippingPhone: "0901234567"})
	if !errors.Is(err, ErrCartEmpty) {
		t.Fatalf("expected ErrCartEmpty, got %v", err)
	}
}

func TestOrderService_ValidationBranches(t *testing.T) {
	t.Parallel()

	svc := &OrderService{}

	listReq := &dto.OrderListRequest{FromDate: "bad-date"}
	if _, err := svc.ListOrders(1, listReq); !errors.Is(err, ErrInvalidDateFilter) {
		t.Fatalf("ListOrders expected ErrInvalidDateFilter, got %v", err)
	}
	if listReq.Page != 1 || listReq.PageSize != 20 {
		t.Fatalf("ListOrders defaults not applied, got page=%d pageSize=%d", listReq.Page, listReq.PageSize)
	}

	adminListReq := &dto.AdminOrderListRequest{FromDate: "bad-date"}
	if _, err := svc.ListOrdersForAdmin(adminListReq); !errors.Is(err, ErrInvalidDateFilter) {
		t.Fatalf("ListOrdersForAdmin expected ErrInvalidDateFilter, got %v", err)
	}
	if adminListReq.Page != 1 || adminListReq.PageSize != 15 || adminListReq.SortBy != "created_at" || adminListReq.SortDir != "desc" {
		t.Fatalf("ListOrdersForAdmin defaults not applied, got %+v", *adminListReq)
	}

	if err := svc.UpdateOrderStatusForAdmin(1, "not-valid"); !errors.Is(err, ErrInvalidOrderStatus) {
		t.Fatalf("UpdateOrderStatusForAdmin expected ErrInvalidOrderStatus, got %v", err)
	}

	if _, err := svc.GetStatisticsForAdmin(&dto.AdminOrderStatisticsRequest{GroupBy: "year"}); !errors.Is(err, ErrInvalidOrderInput) {
		t.Fatalf("GetStatisticsForAdmin expected ErrInvalidOrderInput, got %v", err)
	}

	if _, err := svc.GetStatisticsForAdmin(&dto.AdminOrderStatisticsRequest{Status: "not-valid"}); !errors.Is(err, ErrInvalidStatusFilter) {
		t.Fatalf("GetStatisticsForAdmin expected ErrInvalidStatusFilter, got %v", err)
	}

	if _, err := svc.GetStatisticsForAdmin(&dto.AdminOrderStatisticsRequest{FromDate: "bad-date"}); !errors.Is(err, ErrInvalidDateFilter) {
		t.Fatalf("GetStatisticsForAdmin expected ErrInvalidDateFilter, got %v", err)
	}
}
