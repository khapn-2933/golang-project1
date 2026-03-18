package repository

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/kha/foods-drinks/internal/models"
	"gorm.io/gorm"
)

func setupOrderRepositoryTest(t *testing.T) (*OrderRepository, *gorm.DB, uint, uint) {
	t.Helper()

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}

	if err := db.AutoMigrate(&models.User{}, &models.Order{}, &models.OrderItem{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	u1 := models.User{Email: "u1@example.com", FullName: "User One", Role: models.RoleUser, Status: models.UserStatusActive}
	u2 := models.User{Email: "u2@example.com", FullName: "User Two", Role: models.RoleUser, Status: models.UserStatusActive}
	if err := db.Create(&u1).Error; err != nil {
		t.Fatalf("seed user1: %v", err)
	}
	if err := db.Create(&u2).Error; err != nil {
		t.Fatalf("seed user2: %v", err)
	}

	return NewOrderRepository(db), db, u1.ID, u2.ID
}

func seedOrder(t *testing.T, db *gorm.DB, userID uint, number, status string, total float64, createdAt time.Time) uint {
	t.Helper()

	o := models.Order{
		UserID:          userID,
		OrderNumber:     number,
		Status:          status,
		TotalAmount:     total,
		ShippingAddress: "HN",
		ShippingPhone:   "0901234567",
		CreatedAt:       createdAt,
		UpdatedAt:       createdAt,
	}
	if err := db.Create(&o).Error; err != nil {
		t.Fatalf("seed order: %v", err)
	}
	return o.ID
}

func TestOrderRepository_CreateFindUpdate(t *testing.T) {
	t.Parallel()

	repo, db, userID, _ := setupOrderRepositoryTest(t)

	order := &models.Order{
		UserID:          userID,
		OrderNumber:     "ORDER-1",
		Status:          models.OrderStatusPending,
		TotalAmount:     123000,
		ShippingAddress: "123 Le Loi",
		ShippingPhone:   "0901234567",
	}
	if err := repo.Create(order); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if order.ID == 0 {
		t.Fatal("expected created order id")
	}

	items := []models.OrderItem{
		{OrderID: order.ID, ProductID: 1, ProductName: "Pho", ProductPrice: 50000, Quantity: 2, Subtotal: 100000},
		{OrderID: order.ID, ProductID: 2, ProductName: "Tea", ProductPrice: 23000, Quantity: 1, Subtotal: 23000},
	}
	if err := repo.CreateItems(items); err != nil {
		t.Fatalf("CreateItems: %v", err)
	}

	foundByUser, err := repo.FindByIDAndUserID(order.ID, userID)
	if err != nil {
		t.Fatalf("FindByIDAndUserID: %v", err)
	}
	if len(foundByUser.Items) != 2 {
		t.Fatalf("items len = %d, want 2", len(foundByUser.Items))
	}

	found, err := repo.FindByID(order.ID)
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if found.UserID != userID {
		t.Fatalf("FindByID user id = %d, want %d", found.UserID, userID)
	}

	found.Status = models.OrderStatusConfirmed
	if err := repo.Update(found); err != nil {
		t.Fatalf("Update: %v", err)
	}

	locked, err := repo.FindByIDForUpdate(order.ID)
	if err != nil {
		t.Fatalf("FindByIDForUpdate: %v", err)
	}
	if locked.Status != models.OrderStatusConfirmed {
		t.Fatalf("status after update = %s, want %s", locked.Status, models.OrderStatusConfirmed)
	}

	counts, err := repo.CountItemsByOrderIDs([]uint{order.ID})
	if err != nil {
		t.Fatalf("CountItemsByOrderIDs: %v", err)
	}
	if counts[order.ID] != 2 {
		t.Fatalf("item count = %d, want 2", counts[order.ID])
	}

	emptyCounts, err := repo.CountItemsByOrderIDs(nil)
	if err != nil {
		t.Fatalf("CountItemsByOrderIDs empty: %v", err)
	}
	if len(emptyCounts) != 0 {
		t.Fatalf("expected empty map, got %+v", emptyCounts)
	}

	var totalItems int64
	if err := db.Model(&models.OrderItem{}).Where("order_id = ?", order.ID).Count(&totalItems).Error; err != nil {
		t.Fatalf("count item rows: %v", err)
	}
	if totalItems != 2 {
		t.Fatalf("item rows = %d, want 2", totalItems)
	}
}

func TestOrderRepository_ListAndStatistics(t *testing.T) {
	t.Parallel()

	repo, _, user1ID, user2ID := setupOrderRepositoryTest(t)
	now := time.Now().UTC()

	o1 := seedOrder(t, repo.GetDB(), user1ID, "ORDER-A", models.OrderStatusPending, 100000, now.Add(-48*time.Hour))
	_ = seedOrder(t, repo.GetDB(), user1ID, "ORDER-B", models.OrderStatusDelivered, 200000, now.Add(-24*time.Hour))
	_ = seedOrder(t, repo.GetDB(), user2ID, "ORDER-C", models.OrderStatusCancelled, 50000, now.Add(-12*time.Hour))

	if err := repo.CreateItems([]models.OrderItem{{OrderID: o1, ProductID: 1, ProductName: "A", ProductPrice: 100000, Quantity: 1, Subtotal: 100000}}); err != nil {
		t.Fatalf("seed CreateItems: %v", err)
	}

	from := now.Add(-36 * time.Hour)
	to := now
	orders, total, err := repo.ListByUserID(OrderListParams{
		UserID:   user1ID,
		Offset:   0,
		Limit:    10,
		FromDate: &from,
		ToDate:   &to,
	})
	if err != nil {
		t.Fatalf("ListByUserID: %v", err)
	}
	if total != 1 || len(orders) != 1 {
		t.Fatalf("ListByUserID total=%d len=%d, want 1/1", total, len(orders))
	}

	adminOrders, adminTotal, err := repo.ListForAdmin(AdminOrderListParams{
		Status:  models.OrderStatusDelivered,
		Offset:  0,
		Limit:   10,
		SortBy:  "total_amount",
		SortDir: "asc",
	})
	if err != nil {
		t.Fatalf("ListForAdmin: %v", err)
	}
	if adminTotal != 1 || len(adminOrders) != 1 {
		t.Fatalf("ListForAdmin total=%d len=%d, want 1/1", adminTotal, len(adminOrders))
	}

	summary, err := repo.GetStatisticsSummary(OrderStatisticsParams{})
	if err != nil {
		t.Fatalf("GetStatisticsSummary: %v", err)
	}
	if summary.OrdersCount != 3 {
		t.Fatalf("summary orders count = %d, want 3", summary.OrdersCount)
	}
	if summary.DeliveredCount != 1 || summary.CancelledCount != 1 {
		t.Fatalf("summary delivered/cancelled = %d/%d, want 1/1", summary.DeliveredCount, summary.CancelledCount)
	}

	if _, err := repo.GetStatisticsSeries(OrderStatisticsParams{GroupBy: "month"}); err == nil {
		t.Fatal("expected GetStatisticsSeries to fail on sqlite DATE_FORMAT")
	}
}

func TestOrderRepository_NotFoundPaths(t *testing.T) {
	t.Parallel()

	repo, _, _, _ := setupOrderRepositoryTest(t)

	if _, err := repo.FindByIDAndUserID(999, 1); !errors.Is(err, ErrOrderNotFound) {
		t.Fatalf("FindByIDAndUserID expected ErrOrderNotFound, got %v", err)
	}
	if _, err := repo.FindByID(999); !errors.Is(err, ErrOrderNotFound) {
		t.Fatalf("FindByID expected ErrOrderNotFound, got %v", err)
	}
	if _, err := repo.FindByIDForUpdate(999); !errors.Is(err, ErrOrderNotFound) {
		t.Fatalf("FindByIDForUpdate expected ErrOrderNotFound, got %v", err)
	}
}
