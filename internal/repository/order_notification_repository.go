package repository

import (
	"time"

	"github.com/kha/foods-drinks/internal/models"
	"gorm.io/gorm"
)

type OrderNotificationRepository struct {
	db *gorm.DB
}

type OrderNotificationRepositoryInterface interface {
	Create(notification *models.OrderNotification) error
	MarkSent(id uint, message string, sentAt time.Time) error
	MarkFailed(id uint, errMessage string) error
}

func NewOrderNotificationRepository(db *gorm.DB) *OrderNotificationRepository {
	return &OrderNotificationRepository{db: db}
}

func (r *OrderNotificationRepository) Create(notification *models.OrderNotification) error {
	return r.db.Create(notification).Error
}

func (r *OrderNotificationRepository) MarkSent(id uint, message string, sentAt time.Time) error {
	return r.db.Model(&models.OrderNotification{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"status":        models.NotificationStatusSent,
			"message":       message,
			"error_message": nil,
			"sent_at":       sentAt,
		}).Error
}

func (r *OrderNotificationRepository) MarkFailed(id uint, errMessage string) error {
	return r.db.Model(&models.OrderNotification{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"status":        models.NotificationStatusFailed,
			"error_message": errMessage,
		}).Error
}
