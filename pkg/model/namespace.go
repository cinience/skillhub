package model

import (
	"time"

	"github.com/google/uuid"
)

type Namespace struct {
	ID          uuid.UUID `gorm:"column:id;type:text;primaryKey" json:"id"`
	Slug        string    `gorm:"column:slug;type:varchar(128);uniqueIndex;not null" json:"slug"`
	DisplayName *string   `gorm:"column:display_name;type:varchar(256)" json:"displayName,omitempty"`
	Description *string   `gorm:"column:description;type:text" json:"description,omitempty"`
	OwnerID     uuid.UUID `gorm:"column:owner_id;type:text;not null;index" json:"ownerId"`
	Type        string    `gorm:"column:type;type:varchar(20);not null;default:'team'" json:"type"` // personal | team
	Status      string    `gorm:"column:status;type:varchar(20);not null;default:'active'" json:"status"` // active | suspended
	CreatedAt   time.Time `gorm:"column:created_at;autoCreateTime" json:"createdAt"`
	UpdatedAt   time.Time `gorm:"column:updated_at;autoUpdateTime" json:"updatedAt"`
}

func (Namespace) TableName() string { return "namespaces" }

type NamespaceMember struct {
	ID          uuid.UUID `gorm:"column:id;type:text;primaryKey" json:"id"`
	NamespaceID uuid.UUID `gorm:"column:namespace_id;type:text;not null;index:idx_ns_member,unique" json:"namespaceId"`
	UserID      uuid.UUID `gorm:"column:user_id;type:text;not null;index:idx_ns_member,unique" json:"userId"`
	Role        string    `gorm:"column:role;type:varchar(20);not null;default:'member'" json:"role"` // owner | admin | member
	CreatedAt   time.Time `gorm:"column:created_at;autoCreateTime" json:"createdAt"`
}

func (NamespaceMember) TableName() string { return "namespace_members" }

// NamespaceMemberWithUser adds user info for API responses.
type NamespaceMemberWithUser struct {
	NamespaceMember
	Handle      string  `gorm:"column:handle" json:"handle"`
	DisplayName *string `gorm:"column:display_name" json:"displayName,omitempty"`
}
