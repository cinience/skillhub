package model

import (
	"time"

	"github.com/google/uuid"
)

type Notification struct {
	ID        uuid.UUID `gorm:"column:id;type:text;primaryKey" json:"id"`
	UserID    uuid.UUID `gorm:"column:user_id;type:text;not null;index" json:"userId"`
	Category  string    `gorm:"column:category;type:varchar(32);not null" json:"category"` // review, member, system
	Title     string    `gorm:"column:title;type:varchar(256);not null" json:"title"`
	Body      *string   `gorm:"column:body;type:text" json:"body,omitempty"`
	Link      *string   `gorm:"column:link;type:text" json:"link,omitempty"`
	IsRead    bool      `gorm:"column:is_read;not null;default:false" json:"isRead"`
	CreatedAt time.Time `gorm:"column:created_at;autoCreateTime;index" json:"createdAt"`
}

func (Notification) TableName() string { return "notifications" }
