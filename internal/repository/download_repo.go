package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/cinience/skillhub/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type DownloadRepo struct {
	db *gorm.DB
}

func NewDownloadRepo(db *gorm.DB) *DownloadRepo {
	return &DownloadRepo{db: db}
}

// RecordDownload records a download, returning true if it's new (not a duplicate).
func (r *DownloadRepo) RecordDownload(ctx context.Context, skillID, versionID uuid.UUID, identityHash string) (bool, error) {
	dedup := model.DownloadDedup{
		ID:           uuid.New(),
		SkillID:      skillID,
		VersionID:    versionID,
		IdentityHash: identityHash,
		DownloadedAt: time.Now(),
	}
	result := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{DoNothing: true}).
		Create(&dedup)
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected > 0, nil
}

// AuditLogEntry is kept as a convenience alias for creating audit logs.
type AuditLogEntry struct {
	ActorID      *uuid.UUID
	Action       string
	ResourceType string
	ResourceID   *uuid.UUID
	Details      interface{}
	IPAddress    *string
}

func (r *DownloadRepo) WriteAuditLog(ctx context.Context, entry AuditLogEntry) error {
	var details *string
	if entry.Details != nil {
		if s, ok := entry.Details.(string); ok {
			details = &s
		}
	}
	log := model.AuditLog{
		ID:           uuid.New(),
		ActorID:      entry.ActorID,
		Action:       entry.Action,
		ResourceType: entry.ResourceType,
		ResourceID:   entry.ResourceID,
		Details:      details,
		IPAddress:    entry.IPAddress,
	}
	return r.db.WithContext(ctx).Create(&log).Error
}
