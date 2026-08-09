package model

import (
	"time"

	"github.com/google/uuid"
)

// Plugin represents an Agent Plugins package containing skills and MCP servers.
type Plugin struct {
	ID               uuid.UUID   `gorm:"column:id;type:text;primaryKey" json:"id"`
	Slug             string      `gorm:"column:slug;type:varchar(128);not null" json:"slug"`
	DisplayName      *string     `gorm:"column:display_name;type:varchar(256)" json:"displayName,omitempty"`
	Summary          *string     `gorm:"column:summary;type:text" json:"summary,omitempty"`
	OwnerID          uuid.UUID   `gorm:"column:owner_id;type:text;not null;index" json:"ownerId"`
	NamespaceID      *uuid.UUID  `gorm:"column:namespace_id;type:text;index" json:"namespaceId,omitempty"`
	LatestVersionID  *uuid.UUID  `gorm:"column:latest_version_id;type:text" json:"latestVersionId,omitempty"`
	Visibility       string      `gorm:"column:visibility;type:varchar(20);not null;default:'private'" json:"visibility"`
	ModerationStatus string      `gorm:"column:moderation_status;type:varchar(20);not null;default:'approved'" json:"moderationStatus"`
	Category         string      `gorm:"column:category;type:varchar(64);not null;default:'general';index" json:"category"`
	Tags             StringArray `gorm:"column:tags;type:text;not null;default:'[]'" json:"tags"`
	Downloads        int64       `gorm:"column:downloads;not null;default:0;index" json:"downloads"`
	StarsCount       int         `gorm:"column:stars_count;not null;default:0;index" json:"starsCount"`
	VersionsCount    int         `gorm:"column:versions_count;not null;default:0" json:"versionsCount"`
	CreatedAt        time.Time   `gorm:"column:created_at;autoCreateTime;index" json:"createdAt"`
	UpdatedAt        time.Time   `gorm:"column:updated_at;autoUpdateTime;index" json:"updatedAt"`
	SoftDeletedAt    *time.Time  `gorm:"column:soft_deleted_at;index" json:"softDeletedAt,omitempty"`
}

func (Plugin) TableName() string { return "plugins" }

// PluginWithOwner adds owner info for API responses.
type PluginWithOwner struct {
	Plugin
	OwnerHandle    string  `gorm:"column:owner_handle" json:"ownerHandle"`
	OwnerAvatarURL *string `gorm:"column:owner_avatar_url" json:"ownerAvatarUrl,omitempty"`
	NamespaceSlug  string  `gorm:"column:namespace_slug" json:"namespaceSlug"`
}

// PluginVersion represents a specific version of a plugin.
type PluginVersion struct {
	ID            uuid.UUID  `gorm:"column:id;type:text;primaryKey" json:"id"`
	PluginID      uuid.UUID  `gorm:"column:plugin_id;type:text;not null;uniqueIndex:idx_plugin_version;index:idx_pluginver_created,priority:1" json:"pluginId"`
	Version       string     `gorm:"column:version;type:varchar(64);not null;uniqueIndex:idx_plugin_version" json:"version"`
	Fingerprint   string     `gorm:"column:fingerprint;type:varchar(128);not null;index" json:"fingerprint"`
	Manifest      JSONRaw    `gorm:"column:manifest;type:text;not null;default:'{}'" json:"manifest"`
	Files         JSONRaw    `gorm:"column:files;type:text;not null;default:'[]'" json:"files"`
	Changelog     *string    `gorm:"column:changelog;type:text" json:"changelog,omitempty"`
	CreatedBy     uuid.UUID  `gorm:"column:created_by;type:text;not null" json:"createdBy"`
	SHA256Hash    string     `gorm:"column:sha256_hash;type:varchar(128);not null;index" json:"sha256Hash"`
	Status        string     `gorm:"column:status;type:varchar(20);not null;default:'ready';index" json:"-"`
	CreatedAt     time.Time  `gorm:"column:created_at;autoCreateTime;index:idx_pluginver_created,priority:2,sort:desc" json:"createdAt"`
	SoftDeletedAt *time.Time `gorm:"column:soft_deleted_at" json:"softDeletedAt,omitempty"`
	YankedAt      *time.Time `gorm:"column:yanked_at" json:"yankedAt,omitempty"`
	YankReason    *string    `gorm:"column:yank_reason;type:text" json:"yankReason,omitempty"`
}

func (PluginVersion) TableName() string { return "plugin_versions" }
