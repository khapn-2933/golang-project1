package repository

import (
	"errors"
	"time"

	"github.com/kha/foods-drinks/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ErrOrderNotFound = errors.New("order not found")

type OrderRepository struct {
	db *gorm.DB
}

func NewOrderRepository(db *gorm.DB) *OrderRepository {
	return &OrderRepository{db: db}
}

func (r *OrderRepository) GetDB() *gorm.DB {
	return r.db
}

func (r *OrderRepository) WithTx(tx *gorm.DB) *OrderRepository {
	return &OrderRepository{db: tx}
}

func (r *OrderRepository) Create(order *models.Order) error {
	return r.db.Create(order).Error
}

func (r *OrderRepository) CreateItems(items []models.OrderItem) error {
	if len(items) == 0 {
		return nil
	}
	return r.db.Create(&items).Error
}

func (r *OrderRepository) FindByIDAndUserID(orderID uint, userID uint) (*models.Order, error) {
	var order models.Order
	err := r.db.Where("id = ? AND user_id = ?", orderID, userID).
		Preload("Items", func(db *gorm.DB) *gorm.DB {
			return db.Order("order_items.id ASC")
		}).
		First(&order).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrOrderNotFound
		}
		return nil, err
	}
	return &order, nil
}

type OrderListParams struct {
	Offset   int
	Limit    int
	UserID   uint
	Status   string
	FromDate *time.Time
	ToDate   *time.Time
}

type AdminOrderListParams struct {
	Offset   int
	Limit    int
	Status   string
	FromDate *time.Time
	ToDate   *time.Time
	SortBy   string
	SortDir  string
}

type OrderStatisticsParams struct {
	Status   string
	FromDate *time.Time
	ToDate   *time.Time
	GroupBy  string
}

type OrderStatisticsSummaryRow struct {
	OrdersCount    int64
	RevenueAmount  float64
	AverageOrder   float64
	DeliveredCount int64
	CancelledCount int64
}

type OrderStatisticsSeriesRow struct {
	PeriodLabel   string
	OrdersCount   int64
	RevenueAmount float64
}

func (r *OrderRepository) ListByUserID(params OrderListParams) ([]models.Order, int64, error) {
	var orders []models.Order
	var total int64

	query := r.db.Model(&models.Order{}).Where("user_id = ?", params.UserID)
	if params.Status != "" {
		query = query.Where("status = ?", params.Status)
	}
	if params.FromDate != nil {
		query = query.Where("created_at >= ?", *params.FromDate)
	}
	if params.ToDate != nil {
		query = query.Where("created_at <= ?", *params.ToDate)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	err := query.
		Order("created_at DESC").
		Offset(params.Offset).
		Limit(params.Limit).
		Find(&orders).Error
	if err != nil {
		return nil, 0, err
	}

	return orders, total, nil
}

func (r *OrderRepository) CountItemsByOrderIDs(orderIDs []uint) (map[uint]int, error) {
	counts := make(map[uint]int)
	if len(orderIDs) == 0 {
		return counts, nil
	}

	type row struct {
		OrderID uint
		Count   int
	}

	rows := make([]row, 0, len(orderIDs))
	err := r.db.Model(&models.OrderItem{}).
		Select("order_id, COUNT(*) as count").
		Where("order_id IN ?", orderIDs).
		Group("order_id").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	for _, row := range rows {
		counts[row.OrderID] = row.Count
	}

	return counts, nil
}

func (r *OrderRepository) ListForAdmin(params AdminOrderListParams) ([]models.Order, int64, error) {
	var orders []models.Order
	var total int64

	query := r.db.Model(&models.Order{})
	if params.Status != "" {
		query = query.Where("status = ?", params.Status)
	}
	if params.FromDate != nil {
		query = query.Where("created_at >= ?", *params.FromDate)
	}
	if params.ToDate != nil {
		query = query.Where("created_at <= ?", *params.ToDate)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	sortBy := "created_at"
	if params.SortBy == "total_amount" || params.SortBy == "status" {
		sortBy = params.SortBy
	}
	sortDir := "desc"
	if params.SortDir == "asc" {
		sortDir = "asc"
	}

	err := query.
		Preload("User").
		Preload("Items", func(db *gorm.DB) *gorm.DB {
			return db.Order("order_items.id ASC")
		}).
		Order(sortBy + " " + sortDir).
		Offset(params.Offset).
		Limit(params.Limit).
		Find(&orders).Error
	if err != nil {
		return nil, 0, err
	}

	return orders, total, nil
}

func (r *OrderRepository) FindByID(id uint) (*models.Order, error) {
	var order models.Order
	err := r.db.Where("id = ?", id).
		Preload("User").
		Preload("Items", func(db *gorm.DB) *gorm.DB {
			return db.Order("order_items.id ASC")
		}).
		First(&order).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrOrderNotFound
		}
		return nil, err
	}
	return &order, nil
}

func (r *OrderRepository) FindByIDForUpdate(id uint) (*models.Order, error) {
	var order models.Order
	err := r.db.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id = ?", id).
		First(&order).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrOrderNotFound
		}
		return nil, err
	}
	return &order, nil
}

func (r *OrderRepository) Update(order *models.Order) error {
	return r.db.Save(order).Error
}

func (r *OrderRepository) GetStatisticsSummary(params OrderStatisticsParams) (OrderStatisticsSummaryRow, error) {
	row := OrderStatisticsSummaryRow{}

	query := r.db.Model(&models.Order{})
	if params.Status != "" {
		query = query.Where("status = ?", params.Status)
	}
	if params.FromDate != nil {
		query = query.Where("created_at >= ?", *params.FromDate)
	}
	if params.ToDate != nil {
		query = query.Where("created_at <= ?", *params.ToDate)
	}

	err := query.Select(
		"COUNT(*) AS orders_count, " +
			"COALESCE(SUM(total_amount), 0) AS revenue_amount, " +
			"COALESCE(AVG(total_amount), 0) AS average_order, " +
			"SUM(CASE WHEN status = 'delivered' THEN 1 ELSE 0 END) AS delivered_count, " +
			"SUM(CASE WHEN status = 'cancelled' THEN 1 ELSE 0 END) AS cancelled_count",
	).Scan(&row).Error

	return row, err
}

func (r *OrderRepository) GetStatisticsSeries(params OrderStatisticsParams) ([]OrderStatisticsSeriesRow, error) {
	rows := []OrderStatisticsSeriesRow{}

	query := r.db.Model(&models.Order{})
	if params.Status != "" {
		query = query.Where("status = ?", params.Status)
	}
	if params.FromDate != nil {
		query = query.Where("created_at >= ?", *params.FromDate)
	}
	if params.ToDate != nil {
		query = query.Where("created_at <= ?", *params.ToDate)
	}

	periodExpr := "DATE_FORMAT(created_at, '%Y-%m')"

	switch params.GroupBy {
	case "day":
		periodExpr = "DATE_FORMAT(created_at, '%Y-%m-%d')"
	case "week":
		periodExpr = "DATE_FORMAT(DATE_SUB(DATE(created_at), INTERVAL WEEKDAY(created_at) DAY), '%Y-%m-%d')"
	case "month":
		periodExpr = "DATE_FORMAT(created_at, '%Y-%m')"
	}

	base := query.Select(periodExpr + " AS period_value, total_amount")

	err := r.db.Table("(?) AS order_periods", base).
		Select("period_value AS period_label, COUNT(*) AS orders_count, COALESCE(SUM(total_amount), 0) AS revenue_amount").
		Group("period_value").
		Order("period_value ASC").
		Scan(&rows).Error

	return rows, err
}
