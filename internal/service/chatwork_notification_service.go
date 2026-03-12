package service

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/kha/foods-drinks/internal/config"
	"github.com/kha/foods-drinks/internal/dto"
	"github.com/kha/foods-drinks/internal/models"
	"github.com/kha/foods-drinks/internal/repository"
)

const defaultChatworkBaseURL = "https://api.chatwork.com"

var chatworkTextSanitizer = strings.NewReplacer(
	"[", "&#91;",
	"]", "&#93;",
)

var chatworkLineBreakNormalizer = regexp.MustCompile(`\r\n|\r|\n`)

type ChatworkNotificationService struct {
	cfg              *config.ChatworkConfig
	notificationRepo repository.OrderNotificationRepositoryInterface
	httpClient       *http.Client
	maxRetries       int
	retryDelay       time.Duration
	jobs             chan chatworkNotificationJob
}

type chatworkNotificationJob struct {
	notificationID uint
	order          *dto.OrderResponse
}

func NewChatworkNotificationService(cfg *config.ChatworkConfig, notificationRepo repository.OrderNotificationRepositoryInterface) *ChatworkNotificationService {
	if cfg == nil {
		return nil
	}

	maxRetries := cfg.MaxRetries
	if maxRetries <= 0 {
		maxRetries = 3
	}

	retryDelay := time.Duration(cfg.RetryDelaySeconds) * time.Second
	if retryDelay <= 0 {
		retryDelay = 3 * time.Second
	}

	maxWorkers := cfg.MaxWorkers
	if maxWorkers <= 0 {
		maxWorkers = 2
	}

	queueSize := cfg.QueueSize
	if queueSize <= 0 {
		queueSize = 100
	}

	timeout := time.Duration(cfg.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 5 * time.Second
	}

	svc := &ChatworkNotificationService{
		cfg:              cfg,
		notificationRepo: notificationRepo,
		httpClient:       &http.Client{Timeout: timeout},
		maxRetries:       maxRetries,
		retryDelay:       retryDelay,
		jobs:             make(chan chatworkNotificationJob, queueSize),
	}

	for i := 0; i < maxWorkers; i++ {
		go svc.worker()
	}

	return svc
}

func (s *ChatworkNotificationService) NotifyNewOrderAsync(order *dto.OrderResponse) {
	if s == nil || order == nil || !s.cfg.Enabled || s.notificationRepo == nil {
		return
	}

	roomID := strings.TrimSpace(s.cfg.RoomID)
	if roomID == "" {
		log.Printf("[notification] chatwork notification disabled because room_id is empty")
		return
	}

	token := strings.TrimSpace(s.cfg.APIToken)
	if token == "" {
		log.Printf("[notification] chatwork notification disabled because api_token is empty")
		return
	}

	notification := &models.OrderNotification{
		OrderID:   order.ID,
		Type:      models.NotificationTypeChatwork,
		Status:    models.NotificationStatusPending,
		Recipient: roomID,
	}
	if err := s.notificationRepo.Create(notification); err != nil {
		log.Printf("[notification] failed to create chatwork notification log for order %d: %v", order.ID, err)
		return
	}

	job := chatworkNotificationJob{notificationID: notification.ID, order: cloneOrderResponse(order)}
	select {
	case s.jobs <- job:
	default:
		s.markFailed(notification.ID, fmt.Errorf("notification queue is full"))
		log.Printf("[notification] queue full, dropping chatwork notification %d", notification.ID)
	}
}

func (s *ChatworkNotificationService) worker() {
	for job := range s.jobs {
		s.sendWithRetry(job.notificationID, job.order)
	}
}

func (s *ChatworkNotificationService) sendWithRetry(notificationID uint, order *dto.OrderResponse) {
	message := s.formatMessage(order)
	var lastErr error

	for attempt := 1; attempt <= s.maxRetries; attempt++ {
		if err := s.sendChatworkMessage(message); err == nil {
			metadata := fmt.Sprintf("room_id=%s; order_number=%s; total_amount=%.2f; item_count=%d",
				strings.TrimSpace(s.cfg.RoomID),
				order.OrderNumber,
				order.TotalAmount,
				len(order.Items),
			)
			if err := s.notificationRepo.MarkSent(notificationID, metadata, time.Now()); err != nil {
				log.Printf("[notification] failed to mark chatwork notification %d as sent: %v", notificationID, err)
			}
			return
		} else {
			lastErr = err
		}

		if attempt < s.maxRetries {
			time.Sleep(time.Duration(attempt) * s.retryDelay)
		}
	}

	s.markFailed(notificationID, lastErr)
}

func (s *ChatworkNotificationService) sendChatworkMessage(message string) error {
	baseURL := strings.TrimRight(strings.TrimSpace(s.cfg.BaseURL), "/")
	if baseURL == "" {
		baseURL = defaultChatworkBaseURL
	}

	roomID := strings.TrimSpace(s.cfg.RoomID)
	if roomID == "" {
		return fmt.Errorf("room_id is required")
	}

	token := strings.TrimSpace(s.cfg.APIToken)
	if token == "" {
		return fmt.Errorf("api_token is required")
	}

	apiURL := fmt.Sprintf("%s/v2/rooms/%s/messages", baseURL, url.PathEscape(roomID))
	payload := url.Values{}
	payload.Set("body", message)

	req, err := http.NewRequest(http.MethodPost, apiURL, strings.NewReader(payload.Encode()))
	if err != nil {
		return fmt.Errorf("failed to create chatwork request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("X-ChatWorkToken", token)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to call chatwork api: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("chatwork api returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	return nil
}

func (s *ChatworkNotificationService) markFailed(notificationID uint, err error) {
	if err == nil {
		err = fmt.Errorf("failed to send chatwork message")
	}
	if updateErr := s.notificationRepo.MarkFailed(notificationID, err.Error()); updateErr != nil {
		log.Printf("[notification] failed to mark chatwork notification %d as failed: %v", notificationID, updateErr)
	}
}

func (s *ChatworkNotificationService) formatMessage(order *dto.OrderResponse) string {
	lines := []string{}
	prefix := sanitizeChatworkText(s.cfg.MessagePrefix)
	if prefix != "" {
		lines = append(lines, prefix)
	}

	lines = append(lines,
		"[info][title]New Order[/title]",
		fmt.Sprintf("Order number: %s", sanitizeChatworkText(order.OrderNumber)),
		fmt.Sprintf("Status: %s", sanitizeChatworkText(order.Status)),
		fmt.Sprintf("Total amount: %.2f", order.TotalAmount),
		fmt.Sprintf("Shipping phone: %s", sanitizeChatworkText(order.ShippingPhone)),
		fmt.Sprintf("Shipping address: %s", sanitizeChatworkText(order.ShippingAddress)),
	)

	if order.Notes != nil && strings.TrimSpace(*order.Notes) != "" {
		lines = append(lines, fmt.Sprintf("Notes: %s", sanitizeChatworkText(*order.Notes)))
	}

	if len(order.Items) > 0 {
		lines = append(lines, "", "Items:")
		for idx, item := range order.Items {
			lines = append(lines,
				fmt.Sprintf("%d. %s x%d - %.2f", idx+1, sanitizeChatworkText(item.ProductName), item.Quantity, item.Subtotal),
			)
		}
	}

	lines = append(lines, "[/info]")
	return strings.Join(lines, "\n")
}

func sanitizeChatworkText(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}

	// Keep message readable in line-oriented Chatwork body while neutralizing markup control chars.
	normalized := chatworkLineBreakNormalizer.ReplaceAllString(trimmed, " ")
	return chatworkTextSanitizer.Replace(normalized)
}
