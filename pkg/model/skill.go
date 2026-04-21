package model

import (
	"time"

	"github.com/google/uuid"
)

type Skill struct {
	ID               uuid.UUID    `gorm:"column:id;type:text;primaryKey" json:"id"`
	Slug             string       `gorm:"column:slug;type:varchar(128);uniqueIndex;not null" json:"slug"`
	DisplayName      *string      `gorm:"column:display_name;type:varchar(256)" json:"displayName,omitempty"`
	Summary          *string      `gorm:"column:summary;type:text" json:"summary,omitempty"`
	OwnerID          uuid.UUID    `gorm:"column:owner_id;type:text;not null;index" json:"ownerId"`
	NamespaceID      *uuid.UUID   `gorm:"column:namespace_id;type:text;index" json:"namespaceId,omitempty"`
	LatestVersionID  *uuid.UUID   `gorm:"column:latest_version_id;type:text" json:"latestVersionId,omitempty"`
	Visibility       string       `gorm:"column:visibility;type:varchar(20);not null;default:'private'" json:"visibility"`
	ModerationStatus string       `gorm:"column:moderation_status;type:varchar(20);not null;default:'approved'" json:"moderationStatus"`
	IsSuspicious     bool         `gorm:"column:is_suspicious;not null;default:false" json:"isSuspicious"`
	Category         string       `gorm:"column:category;type:varchar(64);not null;default:'general';index" json:"category"`
	Tags             StringArray  `gorm:"column:tags;type:text;not null;default:'[]'" json:"tags"`
	Downloads        int64        `gorm:"column:downloads;not null;default:0;index" json:"downloads"`
	Installs         int64        `gorm:"column:installs;not null;default:0" json:"installs"`
	StarsCount       int          `gorm:"column:stars_count;not null;default:0;index" json:"starsCount"`
	VersionsCount    int          `gorm:"column:versions_count;not null;default:0" json:"versionsCount"`
	CommentsCount    int          `gorm:"column:comments_count;not null;default:0" json:"commentsCount"`
	AverageRating    float64      `gorm:"column:average_rating;not null;default:0" json:"averageRating"`
	RatingsCount     int          `gorm:"column:ratings_count;not null;default:0" json:"ratingsCount"`
	CreatedAt        time.Time    `gorm:"column:created_at;autoCreateTime;index" json:"createdAt"`
	UpdatedAt        time.Time    `gorm:"column:updated_at;autoUpdateTime;index" json:"updatedAt"`
	SoftDeletedAt    *time.Time   `gorm:"column:soft_deleted_at;index" json:"softDeletedAt,omitempty"`
}

func (Skill) TableName() string { return "skills" }

// SkillWithOwner adds owner info for API responses (not a DB table).
type SkillWithOwner struct {
	Skill
	OwnerHandle      string  `gorm:"column:owner_handle" json:"ownerHandle"`
	OwnerDisplayName *string `gorm:"column:owner_display_name" json:"ownerDisplayName,omitempty"`
	OwnerAvatarURL   *string `gorm:"column:owner_avatar_url" json:"ownerAvatarUrl,omitempty"`
}

// SkillSlugAlias for slug rename redirects.
type SkillSlugAlias struct {
	ID        uuid.UUID `gorm:"column:id;type:text;primaryKey" json:"id"`
	SkillID   uuid.UUID `gorm:"column:skill_id;type:text;not null;index" json:"skillId"`
	OldSlug   string    `gorm:"column:old_slug;type:varchar(128);uniqueIndex;not null" json:"oldSlug"`
	CreatedAt time.Time `gorm:"column:created_at;autoCreateTime" json:"createdAt"`
}

func (SkillSlugAlias) TableName() string { return "skill_slug_aliases" }

// ValidCategories lists the allowed skill categories.
var ValidCategories = []string{
	"devops", "security", "data", "frontend", "backend",
	"infra", "testing", "ai", "general",
}

// IsValidCategory checks if a category string is in the allowed list.
func IsValidCategory(cat string) bool {
	for _, c := range ValidCategories {
		if c == cat {
			return true
		}
	}
	return false
}
