package service

import (
	"bytes"
	"fmt"
	"html/template"
	"log"
	"net/smtp"
	"os"
	"strings"
	"time"

	"github.com/kha/foods-drinks/internal/config"
	"github.com/kha/foods-drinks/internal/dto"
	"github.com/kha/foods-drinks/internal/models"
	"github.com/kha/foods-drinks/internal/repository"
)

const defaultOrderTemplatePath = "templates/email/new_order.html"

type EmailNotificationService struct {
	cfg              *config.EmailConfig
	notificationRepo repository.OrderNotificationRepositoryInterface
	orderTemplate    *template.Template
	maxRetries       int
	retryDelay       time.Duration
	jobs             chan emailNotificationJob
}

type emailNotificationJob struct {
	notificationID uint
	order          *dto.OrderResponse
}

func NewEmailNotificationService(cfg *config.EmailConfig, notificationRepo repository.OrderNotificationRepositoryInterface) *EmailNotificationService {
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
		maxWorkers = 4
	}

	queueSize := cfg.QueueSize
	if queueSize <= 0 {
		queueSize = 100
	}

	tpl := parseOrderTemplate(cfg.OrderTemplatePath)

	svc := &EmailNotificationService{
		cfg:              cfg,
		notificationRepo: notificationRepo,
		orderTemplate:    tpl,
		maxRetries:       maxRetries,
		retryDelay:       retryDelay,
		jobs:             make(chan emailNotificationJob, queueSize),
	}

	for i := 0; i < maxWorkers; i++ {
		go svc.worker()
	}

	return svc
}

func (s *EmailNotificationService) NotifyNewOrderAsync(order *dto.OrderResponse) {
	if s == nil || order == nil || !s.cfg.Enabled || s.notificationRepo == nil {
		return
	}

	recipient := strings.TrimSpace(s.cfg.AdminRecipient)
	if recipient == "" {
		log.Printf("[notification] email notification disabled because admin_recipient is empty")
		return
	}

	notification := &models.OrderNotification{
		OrderID:   order.ID,
		Type:      models.NotificationTypeEmail,
		Status:    models.NotificationStatusPending,
		Recipient: recipient,
	}
	if err := s.notificationRepo.Create(notification); err != nil {
		log.Printf("[notification] failed to create order notification log for order %d: %v", order.ID, err)
		return
	}

	orderCopy := cloneOrderResponse(order)
	job := emailNotificationJob{notificationID: notification.ID, order: orderCopy}

	select {
	case s.jobs <- job:
	default:
		s.markFailed(notification.ID, fmt.Errorf("notification queue is full"))
		log.Printf("[notification] queue full, dropping notification %d", notification.ID)
	}
}

func (s *EmailNotificationService) worker() {
	for job := range s.jobs {
		s.sendWithRetry(job.notificationID, job.order)
	}
}

func (s *EmailNotificationService) sendWithRetry(notificationID uint, order *dto.OrderResponse) {
	body, err := s.renderOrderTemplate(order)
	if err != nil {
		s.markFailed(notificationID, err)
		return
	}

	subject := s.buildSubject(order.OrderNumber)
	var lastErr error

	for attempt := 1; attempt <= s.maxRetries; attempt++ {
		if err := s.sendHTMLEmail(subject, body); err == nil {
			metadata := fmt.Sprintf("subject=%q; order_number=%s; total_amount=%.2f; item_count=%d",
				subject,
				order.OrderNumber,
				order.TotalAmount,
				len(order.Items),
			)
			if err := s.notificationRepo.MarkSent(notificationID, metadata, time.Now()); err != nil {
				log.Printf("[notification] failed to mark notification %d as sent: %v", notificationID, err)
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

func (s *EmailNotificationService) markFailed(notificationID uint, err error) {
	if err == nil {
		err = fmt.Errorf("failed to send email")
	}
	if updateErr := s.notificationRepo.MarkFailed(notificationID, err.Error()); updateErr != nil {
		log.Printf("[notification] failed to mark notification %d as failed: %v", notificationID, updateErr)
	}
}

func (s *EmailNotificationService) sendHTMLEmail(subject, htmlBody string) error {
	host := strings.TrimSpace(s.cfg.SMTPHost)
	port := s.cfg.SMTPPort
	if host == "" || port <= 0 {
		return fmt.Errorf("invalid email smtp config")
	}

	fromEmail := strings.TrimSpace(s.cfg.FromEmail)
	if fromEmail == "" {
		return fmt.Errorf("from_email is required")
	}

	toEmail := strings.TrimSpace(s.cfg.AdminRecipient)
	if toEmail == "" {
		return fmt.Errorf("admin_recipient is required")
	}

	fromHeader := fromEmail
	fromName := strings.TrimSpace(s.cfg.FromName)
	if fromName != "" {
		fromHeader = fmt.Sprintf("%s <%s>", fromName, fromEmail)
	}

	headers := []string{
		"MIME-Version: 1.0",
		"Content-Type: text/html; charset=UTF-8",
		"From: " + fromHeader,
		"To: " + toEmail,
		"Subject: " + subject,
	}
	message := strings.Join(headers, "\r\n") + "\r\n\r\n" + htmlBody

	username := strings.TrimSpace(s.cfg.Username)
	password := strings.TrimSpace(s.cfg.Password)
	var auth smtp.Auth
	if username != "" || password != "" {
		if username == "" || password == "" {
			return fmt.Errorf("both smtp username and password are required when smtp auth is enabled")
		}
		auth = smtp.PlainAuth("", username, password, host)
	}

	addr := fmt.Sprintf("%s:%d", host, port)
	return smtp.SendMail(addr, auth, fromEmail, []string{toEmail}, []byte(message))
}

func (s *EmailNotificationService) renderOrderTemplate(order *dto.OrderResponse) (string, error) {
	data := map[string]any{
		"Order": order,
	}

	var buf bytes.Buffer
	if err := s.orderTemplate.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to render order template: %w", err)
	}
	return buf.String(), nil
}

func (s *EmailNotificationService) buildSubject(orderNumber string) string {
	prefix := strings.TrimSpace(s.cfg.SubjectPrefix)
	if prefix == "" {
		return fmt.Sprintf("New order %s", orderNumber)
	}
	return fmt.Sprintf("%s New order %s", prefix, orderNumber)
}

func parseOrderTemplate(path string) *template.Template {
	funcMap := template.FuncMap{
		"formatPrice": func(price float64) string {
			return fmt.Sprintf("%.2f", price)
		},
	}

	content, err := os.ReadFile(resolveTemplatePath(path))
	if err != nil {
		content = []byte(defaultOrderEmailTemplate)
	}

	tpl, err := template.New("order-email").Funcs(funcMap).Parse(string(content))
	if err == nil {
		return tpl
	}

	fallbackTpl, fallbackErr := template.New("order-email").Funcs(funcMap).Parse(defaultOrderEmailTemplate)
	if fallbackErr != nil {
		panic(fallbackErr)
	}
	return fallbackTpl
}

func resolveTemplatePath(path string) string {
	trimmed := strings.TrimSpace(path)
	if trimmed != "" {
		return trimmed
	}
	return defaultOrderTemplatePath
}

func cloneOrderResponse(order *dto.OrderResponse) *dto.OrderResponse {
	if order == nil {
		return nil
	}

	copyOrder := *order
	if len(order.Items) > 0 {
		copyOrder.Items = append([]dto.OrderItemResponse(nil), order.Items...)
	}
	return &copyOrder
}

const defaultOrderEmailTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8" />
  <title>New Order</title>
</head>
<body style="font-family: Arial, sans-serif; line-height: 1.5; color: #222;">
  <h2>New order received: {{ .Order.OrderNumber }}</h2>
  <p><strong>Status:</strong> {{ .Order.Status }}</p>
  <p><strong>Total amount:</strong> {{ formatPrice .Order.TotalAmount }}</p>
  <p><strong>Shipping phone:</strong> {{ .Order.ShippingPhone }}</p>
  <p><strong>Shipping address:</strong> {{ .Order.ShippingAddress }}</p>

  <h3>Order items</h3>
  <table border="1" cellpadding="8" cellspacing="0" style="border-collapse: collapse; width: 100%;">
    <thead>
      <tr>
        <th align="left">Product</th>
        <th align="right">Price</th>
        <th align="right">Qty</th>
        <th align="right">Subtotal</th>
      </tr>
    </thead>
    <tbody>
      {{ range .Order.Items }}
      <tr>
        <td>{{ .ProductName }}</td>
        <td align="right">{{ formatPrice .ProductPrice }}</td>
        <td align="right">{{ .Quantity }}</td>
        <td align="right">{{ formatPrice .Subtotal }}</td>
      </tr>
      {{ end }}
    </tbody>
  </table>
</body>
</html>`
