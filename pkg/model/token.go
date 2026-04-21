package model

import (
	"time"

	"github.com/google/uuid"
)

type APIToken struct {
	ID         uuid.UUID  `gorm:"column:id;type:text;primaryKey" json:"id"`
	UserID     uuid.UUID  `gorm:"column:user_id;type:text;not null;index" json:"userId"`
	Label      *string    `gorm:"column:label;type:varchar(256)" json:"label,omitempty"`
	Prefix     string     `gorm:"column:prefix;type:varchar(20);not null;index" json:"prefix"`
	TokenHash  string     `gorm:"column:token_hash;type:varchar(128);uniqueIndex;not null" json:"-"`
	Scope      string     `gorm:"column:scope;type:varchar(32);not null;default:'full'" json:"scope"`
	LastUsedAt *time.Time `gorm:"column:last_used_at" json:"lastUsedAt,omitempty"`
	ExpiresAt  *time.Time `gorm:"column:expires_at" json:"expiresAt,omitempty"`
	CreatedAt  time.Time  `gorm:"column:created_at;autoCreateTime" json:"createdAt"`
	RevokedAt  *time.Time `gorm:"column:revoked_at" json:"revokedAt,omitempty"`
}

func (APIToken) TableName() string { return "api_tokens" }
