package services

import (
	"context"

	"academyprometheus/backend/models"

	"gorm.io/gorm"
)

func NotificationInbox(ctx context.Context, db *gorm.DB, userID uint, limit int) ([]models.Notification, int64, error) {
	if limit <= 0 {
		limit = 20
	}
	rows := make([]models.Notification, 0)
	if err := db.WithContext(ctx).Where("user_id = ?", userID).Order("created_at desc").Limit(limit).Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	var unread int64
	if err := db.WithContext(ctx).Model(&models.Notification{}).Where("user_id = ? AND is_read = ?", userID, false).Count(&unread).Error; err != nil {
		return nil, 0, err
	}
	return rows, unread, nil
}

func MarkAllNotificationsRead(ctx context.Context, db *gorm.DB, userID uint) error {
	return db.WithContext(ctx).Model(&models.Notification{}).Where("user_id = ?", userID).Update("is_read", true).Error
}
