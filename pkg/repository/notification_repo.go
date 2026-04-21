package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/cinience/skillhub/pkg/model"
	"gorm.io/gorm"
)

type NotificationRepo struct {
	db *gorm.DB
}

func NewNotificationRepo(db *gorm.DB) *NotificationRepo {
	return &NotificationRepo{db: db}
}

func (r *NotificationRepo) Create(ctx context.Context, n *model.Notification) error {
	return r.db.WithContext(ctx).Create(n).Error
}

func (r *NotificationRepo) ListByUser(ctx context.Context, userID uuid.UUID, limit int, cursor string) ([]model.Notification, string, error) {
	q := r.db.WithContext(ctx).Where("user_id = ?", userID)
	if cursor != "" {
		q = q.Where("id < ?", cursor)
	}

	var notifications []model.Notification
	err := q.Order("created_at DESC, id DESC").Limit(limit + 1).Find(&notifications).Error
	if err != nil {
		return nil, "", err
	}

	var nextCursor string
	if len(notifications) > limit {
		nextCursor = notifications[limit].ID.String()
		notifications = notifications[:limit]
	}
	return notifications, nextCursor, nil
}

func (r *NotificationRepo) CountUnread(ctx context.Context, userID uuid.UUID) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&model.Notification{}).
		Where("user_id = ? AND is_read = ?", userID, false).
		Count(&count).Error
	return count, err
}

func (r *NotificationRepo) MarkRead(ctx context.Context, id, userID uuid.UUID) error {
	return r.db.WithContext(ctx).
		Model(&model.Notification{}).
		Where("id = ? AND user_id = ?", id, userID).
		Update("is_read", true).Error
}

func (r *NotificationRepo) MarkAllRead(ctx context.Context, userID uuid.UUID) error {
	return r.db.WithContext(ctx).
		Model(&model.Notification{}).
		Where("user_id = ? AND is_read = ?", userID, false).
		Updates(map[string]interface{}{
			"is_read":    true,
			"created_at": gorm.Expr("created_at"), // prevent autoUpdateTime
		}).Error
}

// CleanOld removes notifications older than the given duration.
func (r *NotificationRepo) CleanOld(ctx context.Context, before time.Time) (int64, error) {
	result := r.db.WithContext(ctx).
		Where("created_at < ?", before).
		Delete(&model.Notification{})
	return result.RowsAffected, result.Error
}
