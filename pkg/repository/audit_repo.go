package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/cinience/skillhub/pkg/model"
	"gorm.io/gorm"
)

type AuditRepo struct {
	db *gorm.DB
}

func NewAuditRepo(db *gorm.DB) *AuditRepo {
	return &AuditRepo{db: db}
}

func (r *AuditRepo) Create(ctx context.Context, log *model.AuditLog) error {
	return r.db.WithContext(ctx).Create(log).Error
}

// AuditFilter controls filtering for audit log queries.
type AuditFilter struct {
	Action       string
	ResourceType string
	ActorID      *uuid.UUID
}

func (r *AuditRepo) List(ctx context.Context, limit int, cursor string, filter AuditFilter) ([]model.AuditLog, string, error) {
	q := r.db.WithContext(ctx).Model(&model.AuditLog{})

	if filter.Action != "" {
		q = q.Where("action = ?", filter.Action)
	}
	if filter.ResourceType != "" {
		q = q.Where("resource_type = ?", filter.ResourceType)
	}
	if filter.ActorID != nil {
		q = q.Where("actor_id = ?", *filter.ActorID)
	}
	if cursor != "" {
		q = q.Where("id < ?", cursor)
	}

	var logs []model.AuditLog
	err := q.Order("created_at DESC, id DESC").Limit(limit + 1).Find(&logs).Error
	if err != nil {
		return nil, "", err
	}

	var nextCursor string
	if len(logs) > limit {
		nextCursor = logs[limit].ID.String()
		logs = logs[:limit]
	}
	return logs, nextCursor, nil
}
