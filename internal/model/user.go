package model

import (
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID           uuid.UUID  `gorm:"column:id;type:text;primaryKey" json:"id"`
	Handle       string     `gorm:"column:handle;type:varchar(64);uniqueIndex;not null" json:"handle"`
	DisplayName  *string    `gorm:"column:display_name;type:varchar(256)" json:"displayName,omitempty"`
	AvatarURL    *string    `gorm:"column:avatar_url;type:text" json:"avatarUrl,omitempty"`
	Email        *string    `gorm:"column:email;type:varchar(320)" json:"email,omitempty"`
	PasswordHash *string    `gorm:"column:password_hash;type:varchar(256)" json:"-"`
	Role         string     `gorm:"column:role;type:varchar(20);not null;default:'user'" json:"role"`
	IsBanned     bool       `gorm:"column:is_banned;not null;default:false" json:"isBanned"`
	BanReason    *string    `gorm:"column:ban_reason;type:text" json:"banReason,omitempty"`
	CreatedAt    time.Time  `gorm:"column:created_at;autoCreateTime" json:"createdAt"`
	UpdatedAt    time.Time  `gorm:"column:updated_at;autoUpdateTime" json:"updatedAt"`
}

func (User) TableName() string { return "users" }

func (u *User) IsAdmin() bool {
	return u.Role == "admin"
}

func (u *User) IsModerator() bool {
	return u.Role == "moderator" || u.Role == "admin"
}
