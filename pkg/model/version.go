package model

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type SkillVersion struct {
	ID              uuid.UUID       `gorm:"column:id;type:text;primaryKey" json:"id"`
	SkillID         uuid.UUID       `gorm:"column:skill_id;type:text;not null;index" json:"skillId"`
	Version         string          `gorm:"column:version;type:varchar(64);not null" json:"version"`
	Fingerprint     string          `gorm:"column:fingerprint;type:varchar(128);not null;index" json:"fingerprint"`
	GitCommitHash   *string         `gorm:"column:git_commit_hash;type:varchar(64)" json:"gitCommitHash,omitempty"`
	Changelog       *string         `gorm:"column:changelog;type:text" json:"changelog,omitempty"`
	ChangelogSource *string         `gorm:"column:changelog_source;type:varchar(20)" json:"changelogSource,omitempty"`
	Files           json.RawMessage `gorm:"column:files;type:text;not null;default:'[]'" json:"files"`
	Parsed          json.RawMessage `gorm:"column:parsed;type:text;not null;default:'{}'" json:"parsed"`
	CreatedBy       uuid.UUID       `gorm:"column:created_by;type:text;not null" json:"createdBy"`
	SHA256Hash      string          `gorm:"column:sha256_hash;type:varchar(128);not null;index" json:"sha256Hash"`
	CreatedAt       time.Time       `gorm:"column:created_at;autoCreateTime" json:"createdAt"`
	SoftDeletedAt   *time.Time      `gorm:"column:soft_deleted_at" json:"softDeletedAt,omitempty"`
}

func (SkillVersion) TableName() string { return "skill_versions" }

type VersionFile struct {
	Path        string `json:"path"`
	Size        int64  `json:"size"`
	SHA256      string `json:"sha256"`
	ContentType string `json:"contentType,omitempty"`
}
