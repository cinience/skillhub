package model

import (
	"time"

	"github.com/google/uuid"
)

type Rating struct {
	ID        uuid.UUID `gorm:"column:id;type:text;primaryKey" json:"id"`
	SkillID   uuid.UUID `gorm:"column:skill_id;type:text;not null;index:idx_rating_skill_user,unique" json:"skillId"`
	UserID    uuid.UUID `gorm:"column:user_id;type:text;not null;index:idx_rating_skill_user,unique" json:"userId"`
	Score     int       `gorm:"column:score;not null" json:"score"` // 1-5
	Comment   *string   `gorm:"column:comment;type:text" json:"comment,omitempty"`
	CreatedAt time.Time `gorm:"column:created_at;autoCreateTime" json:"createdAt"`
	UpdatedAt time.Time `gorm:"column:updated_at;autoUpdateTime" json:"updatedAt"`
}

func (Rating) TableName() string { return "ratings" }

// RatingWithUser adds user info for API responses.
type RatingWithUser struct {
	Rating
	Handle      string  `gorm:"column:handle" json:"handle"`
	DisplayName *string `gorm:"column:display_name" json:"displayName,omitempty"`
}
