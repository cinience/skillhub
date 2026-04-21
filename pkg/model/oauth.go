package model

import (
	"time"

	"github.com/google/uuid"
)

type OAuthIdentity struct {
	ID         uuid.UUID `gorm:"column:id;type:text;primaryKey" json:"id"`
	UserID     uuid.UUID `gorm:"column:user_id;type:text;not null;index" json:"userId"`
	Provider   string    `gorm:"column:provider;type:varchar(32);not null;index:idx_oauth_provider_ext,unique" json:"provider"`
	ExternalID string    `gorm:"column:external_id;type:varchar(256);not null;index:idx_oauth_provider_ext,unique" json:"externalId"`
	Email      *string   `gorm:"column:email;type:varchar(256)" json:"email,omitempty"`
	AvatarURL  *string   `gorm:"column:avatar_url;type:text" json:"avatarUrl,omitempty"`
	CreatedAt  time.Time `gorm:"column:created_at;autoCreateTime" json:"createdAt"`
}

func (OAuthIdentity) TableName() string { return "oauth_identities" }
