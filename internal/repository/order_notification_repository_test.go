package repository

import (
	"fmt"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/kha/foods-drinks/internal/models"
	"gorm.io/gorm"
)

func setupOrderNotificationRepoTest(t *testing.T) (*OrderNotificationRepository, *gorm.DB) {
	t.Helper()

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}

	if err := db.AutoMigrate(&models.OrderNotification{}); err != nil {
		t.Fatalf("auto migrate order_notifications: %v", err)
	}

	return NewOrderNotificationRepository(db), db
}

func TestOrderNotificationRepositoryCreate(t *testing.T) {
	repo, db := setupOrderNotificationRepoTest(t)

	notification := &models.OrderNotification{
		OrderID:   101,
		Type:      models.NotificationTypeEmail,
		Status:    models.NotificationStatusPending,
		Recipient: "admin@example.com",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := repo.Create(notification); err != nil {
		t.Fatalf("Create() error: %v", err)
	}
	if notification.ID == 0 {
		t.Fatalf("Create() did not assign ID")
	}

	var saved models.OrderNotification
	if err := db.First(&saved, notification.ID).Error; err != nil {
		t.Fatalf("read inserted notification: %v", err)
	}
	if saved.Recipient != notification.Recipient {
		t.Fatalf("recipient mismatch: got %q, want %q", saved.Recipient, notification.Recipient)
	}
}

func TestOrderNotificationRepositoryMarkSent(t *testing.T) {
	repo, db := setupOrderNotificationRepoTest(t)

	notification := &models.OrderNotification{
		OrderID:   202,
		Type:      models.NotificationTypeChatwork,
		Status:    models.NotificationStatusPending,
		Recipient: "room-123",
	}
	if err := db.Create(notification).Error; err != nil {
		t.Fatalf("seed notification: %v", err)
	}

	sentAt := time.Now().Truncate(time.Second)
	if err := repo.MarkSent(notification.ID, "sent successfully", sentAt); err != nil {
		t.Fatalf("MarkSent() error: %v", err)
	}

	var updated models.OrderNotification
	if err := db.First(&updated, notification.ID).Error; err != nil {
		t.Fatalf("load updated notification: %v", err)
	}

	if updated.Status != models.NotificationStatusSent {
		t.Fatalf("status = %q, want %q", updated.Status, models.NotificationStatusSent)
	}
	if updated.Message == nil || *updated.Message != "sent successfully" {
		t.Fatalf("message mismatch: %+v", updated.Message)
	}
	if updated.ErrorMessage != nil {
		t.Fatalf("error_message should be nil, got: %v", *updated.ErrorMessage)
	}
	if updated.SentAt == nil || !updated.SentAt.Equal(sentAt) {
		t.Fatalf("sent_at mismatch: got %v, want %v", updated.SentAt, sentAt)
	}
}

func TestOrderNotificationRepositoryMarkFailed(t *testing.T) {
	repo, db := setupOrderNotificationRepoTest(t)

	notification := &models.OrderNotification{
		OrderID:   303,
		Type:      models.NotificationTypeEmail,
		Status:    models.NotificationStatusPending,
		Recipient: "admin@example.com",
	}
	if err := db.Create(notification).Error; err != nil {
		t.Fatalf("seed notification: %v", err)
	}

	if err := repo.MarkFailed(notification.ID, "smtp timeout"); err != nil {
		t.Fatalf("MarkFailed() error: %v", err)
	}

	var updated models.OrderNotification
	if err := db.First(&updated, notification.ID).Error; err != nil {
		t.Fatalf("load updated notification: %v", err)
	}

	if updated.Status != models.NotificationStatusFailed {
		t.Fatalf("status = %q, want %q", updated.Status, models.NotificationStatusFailed)
	}
	if updated.ErrorMessage == nil || *updated.ErrorMessage != "smtp timeout" {
		t.Fatalf("error_message mismatch: %+v", updated.ErrorMessage)
	}
}
