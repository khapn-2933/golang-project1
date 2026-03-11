package service

import (
	crand "crypto/rand"
	"errors"
	"fmt"
	"math"
	"math/big"
	"sort"
	"strings"
	"time"

	"github.com/kha/foods-drinks/internal/dto"
	"github.com/kha/foods-drinks/internal/models"
	"github.com/kha/foods-drinks/internal/repository"
	"gorm.io/gorm"
)

var (
	ErrCartEmpty          = errors.New("cart is empty")
	ErrOrderNotFound      = errors.New("order not found")
	ErrInvalidDateFilter  = errors.New("invalid date filter")
	ErrInvalidOrderInput  = errors.New("invalid order input")
	ErrInvalidOrderStatus = errors.New("invalid order status transition")
)

type OrderService struct {
	orderRepo   *repository.OrderRepository
	cartRepo    *repository.CartRepository
	productRepo *repository.ProductRepository
	notifier    OrderNotifier
}

type OrderNotifier interface {
	NotifyNewOrderAsync(order *dto.OrderResponse)
}

func NewOrderService(orderRepo *repository.OrderRepository, cartRepo *repository.CartRepository, productRepo *repository.ProductRepository, notifier OrderNotifier) *OrderService {
	return &OrderService{
		orderRepo:   orderRepo,
		cartRepo:    cartRepo,
		productRepo: productRepo,
		notifier:    notifier,
	}
}

func (s *OrderService) CreateOrderFromCart(userID uint, req *dto.CreateOrderRequest) (*dto.OrderResponse, error) {
	shippingAddress := strings.TrimSpace(req.ShippingAddress)
	shippingPhone := strings.TrimSpace(req.ShippingPhone)
	if shippingAddress == "" || shippingPhone == "" {
		return nil, ErrInvalidOrderInput
	}

	var createdOrderID uint
	err := s.cartRepo.GetDB().Transaction(func(tx *gorm.DB) error {
		cartRepoTx := s.cartRepo.WithTx(tx)
		productRepoTx := s.productRepo.WithTx(tx)
		orderRepoTx := s.orderRepo.WithTx(tx)

		cart, err := cartRepoTx.FindByUserID(userID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrCartEmpty
			}
			return fmt.Errorf("failed to find cart: %w", err)
		}
		if len(cart.Items) == 0 {
			return ErrCartEmpty
		}

		orderItems := make([]models.OrderItem, 0, len(cart.Items))
		totalAmount := 0.0
		cartItems := make([]models.CartItem, len(cart.Items))
		copy(cartItems, cart.Items)
		sort.Slice(cartItems, func(i, j int) bool {
			return cartItems[i].ProductID < cartItems[j].ProductID
		})

		for _, item := range cartItems {
			product, err := productRepoTx.FindByIDForUpdate(item.ProductID)
			if err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return ErrProductNotFound
				}
				return fmt.Errorf("failed to find product: %w", err)
			}

			if product.Status != models.ProductStatusActive {
				return ErrProductNotFound
			}
			if product.Stock < item.Quantity {
				return fmt.Errorf("%w: available %d, requested %d", ErrInsufficientStock, product.Stock, item.Quantity)
			}

			subtotal := product.Price * float64(item.Quantity)
			totalAmount += subtotal
			orderItems = append(orderItems, models.OrderItem{
				ProductID:    product.ID,
				ProductName:  product.Name,
				ProductPrice: product.Price,
				Quantity:     item.Quantity,
				Subtotal:     subtotal,
			})
		}

		order := &models.Order{
			UserID:          userID,
			TotalAmount:     totalAmount,
			Status:          models.OrderStatusPending,
			ShippingAddress: shippingAddress,
			ShippingPhone:   shippingPhone,
			Notes:           req.Notes,
		}

		var createErr error
		for attempt := 0; attempt < 8; attempt++ {
			order.OrderNumber = generateOrderNumber(time.Now())
			createErr = orderRepoTx.Create(order)
			if createErr == nil {
				break
			}
			if !isDuplicateKeyError(createErr) {
				return fmt.Errorf("failed to create order: %w", createErr)
			}
		}
		if createErr != nil {
			return fmt.Errorf("failed to create order: %w", createErr)
		}

		for i := range orderItems {
			orderItems[i].OrderID = order.ID
		}
		if err := orderRepoTx.CreateItems(orderItems); err != nil {
			return fmt.Errorf("failed to create order items: %w", err)
		}

		for _, item := range cartItems {
			updated, err := productRepoTx.DecreaseStock(item.ProductID, item.Quantity)
			if err != nil {
				return fmt.Errorf("failed to update stock: %w", err)
			}
			if !updated {
				return fmt.Errorf("%w: product %d", ErrInsufficientStock, item.ProductID)
			}
		}

		if err := cartRepoTx.ClearCartItems(cart.ID); err != nil {
			return fmt.Errorf("failed to clear cart: %w", err)
		}

		createdOrderID = order.ID
		return nil
	})
	if err != nil {
		return nil, err
	}

	order, err := s.GetOrderDetail(userID, createdOrderID)
	if err != nil {
		return nil, err
	}

	if s.notifier != nil {
		s.notifier.NotifyNewOrderAsync(order)
	}

	return order, nil
}

func (s *OrderService) ListOrders(userID uint, req *dto.OrderListRequest) (*dto.PaginatedResponse, error) {
	if req.Page == 0 {
		req.Page = 1
	}
	if req.PageSize == 0 {
		req.PageSize = 20
	}

	fromDate, toDate, err := parseOrderDateRange(req.FromDate, req.ToDate)
	if err != nil {
		return nil, err
	}

	offset := (req.Page - 1) * req.PageSize
	orders, total, err := s.orderRepo.ListByUserID(repository.OrderListParams{
		Offset:   offset,
		Limit:    req.PageSize,
		UserID:   userID,
		Status:   req.Status,
		FromDate: fromDate,
		ToDate:   toDate,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list orders: %w", err)
	}

	orderIDs := make([]uint, 0, len(orders))
	for _, order := range orders {
		orderIDs = append(orderIDs, order.ID)
	}
	itemCounts, err := s.orderRepo.CountItemsByOrderIDs(orderIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to count order items: %w", err)
	}

	items := make([]dto.OrderResponse, len(orders))
	for i, order := range orders {
		items[i] = *s.toResponse(&order, false)
		items[i].ItemCount = itemCounts[order.ID]
	}

	totalPages := int(math.Ceil(float64(total) / float64(req.PageSize)))
	if totalPages == 0 {
		totalPages = 1
	}

	return &dto.PaginatedResponse{
		Items:      items,
		Total:      total,
		Page:       req.Page,
		PageSize:   req.PageSize,
		TotalPages: totalPages,
	}, nil
}

func (s *OrderService) GetOrderDetail(userID, orderID uint) (*dto.OrderResponse, error) {
	order, err := s.orderRepo.FindByIDAndUserID(orderID, userID)
	if err != nil {
		if errors.Is(err, repository.ErrOrderNotFound) {
			return nil, ErrOrderNotFound
		}
		return nil, fmt.Errorf("failed to find order: %w", err)
	}
	return s.toResponse(order, true), nil
}

func (s *OrderService) ListOrdersForAdmin(req *dto.AdminOrderListRequest) (*dto.PaginatedResponse, error) {
	if req.Page == 0 {
		req.Page = 1
	}
	if req.PageSize == 0 {
		req.PageSize = 15
	}
	if req.SortBy == "" {
		req.SortBy = "created_at"
	}
	if req.SortDir == "" {
		req.SortDir = "desc"
	}

	fromDate, toDate, err := parseOrderDateRange(req.FromDate, req.ToDate)
	if err != nil {
		return nil, err
	}

	offset := (req.Page - 1) * req.PageSize
	orders, total, err := s.orderRepo.ListForAdmin(repository.AdminOrderListParams{
		Offset:   offset,
		Limit:    req.PageSize,
		Status:   req.Status,
		FromDate: fromDate,
		ToDate:   toDate,
		SortBy:   req.SortBy,
		SortDir:  req.SortDir,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list orders: %w", err)
	}

	items := make([]dto.OrderResponse, len(orders))
	for i, order := range orders {
		items[i] = *s.toResponse(&order, true)
	}

	totalPages := int(math.Ceil(float64(total) / float64(req.PageSize)))
	if totalPages == 0 {
		totalPages = 1
	}

	return &dto.PaginatedResponse{
		Items:      items,
		Total:      total,
		Page:       req.Page,
		PageSize:   req.PageSize,
		TotalPages: totalPages,
	}, nil
}

func (s *OrderService) GetOrderDetailForAdmin(orderID uint) (*dto.OrderResponse, error) {
	order, err := s.orderRepo.FindByID(orderID)
	if err != nil {
		if errors.Is(err, repository.ErrOrderNotFound) {
			return nil, ErrOrderNotFound
		}
		return nil, fmt.Errorf("failed to find order: %w", err)
	}
	return s.toResponse(order, true), nil
}

func (s *OrderService) UpdateOrderStatusForAdmin(orderID uint, status string) error {
	status = strings.TrimSpace(status)
	if !isValidOrderStatus(status) {
		return ErrInvalidOrderStatus
	}

	err := s.orderRepo.GetDB().Transaction(func(tx *gorm.DB) error {
		orderRepoTx := s.orderRepo.WithTx(tx)

		order, err := orderRepoTx.FindByIDForUpdate(orderID)
		if err != nil {
			if errors.Is(err, repository.ErrOrderNotFound) {
				return ErrOrderNotFound
			}
			return fmt.Errorf("failed to find order: %w", err)
		}

		if !canTransitionOrderStatus(order.Status, status) {
			return ErrInvalidOrderStatus
		}

		order.Status = status
		if err := orderRepoTx.Update(order); err != nil {
			return fmt.Errorf("failed to update order status: %w", err)
		}

		return nil
	})
	if err != nil {
		return err
	}
	return nil
}

func (s *OrderService) toResponse(order *models.Order, includeItems bool) *dto.OrderResponse {
	resp := &dto.OrderResponse{
		ID:              order.ID,
		UserID:          order.UserID,
		OrderNumber:     order.OrderNumber,
		TotalAmount:     order.TotalAmount,
		Status:          order.Status,
		ShippingAddress: order.ShippingAddress,
		ShippingPhone:   order.ShippingPhone,
		Notes:           order.Notes,
		CreatedAt:       order.CreatedAt,
		UpdatedAt:       order.UpdatedAt,
	}

	if order.User.ID > 0 {
		resp.UserName = order.User.FullName
		resp.UserEmail = order.User.Email
	}

	if includeItems {
		resp.Items = make([]dto.OrderItemResponse, 0, len(order.Items))
		resp.ItemCount = len(order.Items)
		for _, item := range order.Items {
			resp.Items = append(resp.Items, dto.OrderItemResponse{
				ID:           item.ID,
				ProductID:    item.ProductID,
				ProductName:  item.ProductName,
				ProductPrice: item.ProductPrice,
				Quantity:     item.Quantity,
				Subtotal:     item.Subtotal,
			})
		}
	}

	return resp
}

func parseOrderDateRange(fromDate, toDate string) (*time.Time, *time.Time, error) {
	fromDate = strings.TrimSpace(fromDate)
	toDate = strings.TrimSpace(toDate)
	if fromDate == "" && toDate == "" {
		return nil, nil, nil
	}

	layout := "2006-01-02"
	var fromPtr *time.Time
	var toPtr *time.Time

	if fromDate != "" {
		from, err := time.Parse(layout, fromDate)
		if err != nil {
			return nil, nil, ErrInvalidDateFilter
		}
		fromPtr = &from
	}

	if toDate != "" {
		to, err := time.Parse(layout, toDate)
		if err != nil {
			return nil, nil, ErrInvalidDateFilter
		}
		to = to.Add(24*time.Hour - time.Nanosecond)
		toPtr = &to
	}

	if fromPtr != nil && toPtr != nil && fromPtr.After(*toPtr) {
		return nil, nil, ErrInvalidDateFilter
	}

	return fromPtr, toPtr, nil
}

func generateOrderNumber(now time.Time) string {
	randPart := 1000
	random, err := crand.Int(crand.Reader, big.NewInt(9000))
	if err == nil {
		randPart = int(random.Int64()) + 1000
	}
	return fmt.Sprintf("ORDER-%s-%04d", now.Format("20060102"), randPart)
}

func isDuplicateKeyError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return true
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "duplicate") || strings.Contains(message, "1062")
}

func isValidOrderStatus(status string) bool {
	switch status {
	case models.OrderStatusPending,
		models.OrderStatusConfirmed,
		models.OrderStatusProcessing,
		models.OrderStatusShipping,
		models.OrderStatusDelivered,
		models.OrderStatusCancelled:
		return true
	default:
		return false
	}
}

func canTransitionOrderStatus(from, to string) bool {
	if from == to {
		return true
	}

	switch from {
	case models.OrderStatusPending:
		return to == models.OrderStatusConfirmed || to == models.OrderStatusCancelled
	case models.OrderStatusConfirmed:
		return to == models.OrderStatusProcessing || to == models.OrderStatusCancelled
	case models.OrderStatusProcessing:
		return to == models.OrderStatusShipping || to == models.OrderStatusCancelled
	case models.OrderStatusShipping:
		return to == models.OrderStatusDelivered
	case models.OrderStatusDelivered, models.OrderStatusCancelled:
		return false
	default:
		return false
	}
}
