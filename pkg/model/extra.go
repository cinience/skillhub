package model

import (
	"time"

	"github.com/google/uuid"
)

// Star represents a user starring a skill.
type Star struct {
	ID        uuid.UUID `gorm:"column:id;type:text;primaryKey" json:"id"`
	UserID    uuid.UUID `gorm:"column:user_id;type:text;not null;uniqueIndex:idx_stars_user_skill" json:"userId"`
	SkillID   uuid.UUID `gorm:"column:skill_id;type:text;not null;uniqueIndex:idx_stars_user_skill;index" json:"skillId"`
	CreatedAt time.Time `gorm:"column:created_at;autoCreateTime" json:"createdAt"`
}

func (Star) TableName() string { return "stars" }

// DownloadDedup tracks unique downloads per skill+version+identity.
type DownloadDedup struct {
	ID           uuid.UUID `gorm:"column:id;type:text;primaryKey" json:"id"`
	SkillID      uuid.UUID `gorm:"column:skill_id;type:text;not null;uniqueIndex:idx_dedup_skill_ver_identity" json:"skillId"`
	VersionID    uuid.UUID `gorm:"column:version_id;type:text;not null;uniqueIndex:idx_dedup_skill_ver_identity" json:"versionId"`
	IdentityHash string    `gorm:"column:identity_hash;type:varchar(128);not null;uniqueIndex:idx_dedup_skill_ver_identity" json:"identityHash"`
	DownloadedAt time.Time `gorm:"column:downloaded_at;not null" json:"downloadedAt"`
}

func (DownloadDedup) TableName() string { return "download_dedupes" }

// AuditLog records actions for auditing.
type AuditLog struct {
	ID           uuid.UUID  `gorm:"column:id;type:text;primaryKey" json:"id"`
	ActorID      *uuid.UUID `gorm:"column:actor_id;type:text;index" json:"actorId,omitempty"`
	Action       string     `gorm:"column:action;type:varchar(64);not null" json:"action"`
	ResourceType string     `gorm:"column:resource_type;type:varchar(64);not null" json:"resourceType"`
	ResourceID   *uuid.UUID `gorm:"column:resource_id;type:text" json:"resourceId,omitempty"`
	Details      *string    `gorm:"column:details;type:text" json:"details,omitempty"`
	IPAddress    *string    `gorm:"column:ip_address;type:text" json:"ipAddress,omitempty"`
	CreatedAt    time.Time  `gorm:"column:created_at;autoCreateTime;index" json:"createdAt"`
}

func (AuditLog) TableName() string { return "audit_logs" }

// SkillDailyStats tracks daily skill metrics.
type SkillDailyStats struct {
	ID         uuid.UUID `gorm:"column:id;type:text;primaryKey" json:"id"`
	SkillID    uuid.UUID `gorm:"column:skill_id;type:text;not null;uniqueIndex:idx_daily_stats_skill_date" json:"skillId"`
	Date       string    `gorm:"column:date;type:varchar(10);not null;uniqueIndex:idx_daily_stats_skill_date" json:"date"`
	Downloads  int       `gorm:"column:downloads;not null;default:0" json:"downloads"`
	Installs   int       `gorm:"column:installs;not null;default:0" json:"installs"`
	StarsDelta int       `gorm:"column:stars_delta;not null;default:0" json:"starsDelta"`
}

func (SkillDailyStats) TableName() string { return "skill_daily_stats" }

// ReservedSlug prevents certain slugs from being used.
type ReservedSlug struct {
	Slug      string    `gorm:"column:slug;type:varchar(128);primaryKey" json:"slug"`
	Reason    *string   `gorm:"column:reason;type:text" json:"reason,omitempty"`
	CreatedAt time.Time `gorm:"column:created_at;autoCreateTime" json:"createdAt"`
}

func (ReservedSlug) TableName() string { return "reserved_slugs" }

// SkillOwnershipTransfer tracks skill ownership transfers.
type SkillOwnershipTransfer struct {
	ID         uuid.UUID  `gorm:"column:id;type:text;primaryKey" json:"id"`
	SkillID    uuid.UUID  `gorm:"column:skill_id;type:text;not null" json:"skillId"`
	FromUserID uuid.UUID  `gorm:"column:from_user_id;type:text;not null" json:"fromUserId"`
	ToUserID   uuid.UUID  `gorm:"column:to_user_id;type:text;not null" json:"toUserId"`
	Status     string     `gorm:"column:status;type:varchar(20);not null;default:'pending'" json:"status"`
	CreatedAt  time.Time  `gorm:"column:created_at;autoCreateTime" json:"createdAt"`
	ResolvedAt *time.Time `gorm:"column:resolved_at" json:"resolvedAt,omitempty"`
}

func (SkillOwnershipTransfer) TableName() string { return "skill_ownership_transfers" }

// Comment represents a user comment on a skill.
type Comment struct {
	ID            uuid.UUID  `gorm:"column:id;type:text;primaryKey" json:"id"`
	SkillID       uuid.UUID  `gorm:"column:skill_id;type:text;not null;index" json:"skillId"`
	UserID        uuid.UUID  `gorm:"column:user_id;type:text;not null" json:"userId"`
	Body          string     `gorm:"column:body;type:text;not null" json:"body"`
	CreatedAt     time.Time  `gorm:"column:created_at;autoCreateTime" json:"createdAt"`
	UpdatedAt     time.Time  `gorm:"column:updated_at;autoUpdateTime" json:"updatedAt"`
	SoftDeletedAt *time.Time `gorm:"column:soft_deleted_at" json:"softDeletedAt,omitempty"`
}

func (Comment) TableName() string { return "comments" }
