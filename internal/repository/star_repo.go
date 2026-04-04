package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/cinience/skillhub/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type StarRepo struct {
	db *gorm.DB
}

func NewStarRepo(db *gorm.DB) *StarRepo {
	return &StarRepo{db: db}
}

func (r *StarRepo) Star(ctx context.Context, userID, skillID uuid.UUID) error {
	star := model.Star{
		ID:      uuid.New(),
		UserID:  userID,
		SkillID: skillID,
	}
	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{DoNothing: true}).
		Create(&star).Error
}

func (r *StarRepo) Unstar(ctx context.Context, userID, skillID uuid.UUID) error {
	return r.db.WithContext(ctx).
		Where("user_id = ? AND skill_id = ?", userID, skillID).
		Delete(&model.Star{}).Error
}

func (r *StarRepo) IsStarred(ctx context.Context, userID, skillID uuid.UUID) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&model.Star{}).
		Where("user_id = ? AND skill_id = ?", userID, skillID).
		Count(&count).Error
	return count > 0, err
}

func (r *StarRepo) ListByUser(ctx context.Context, userID uuid.UUID) ([]model.Skill, error) {
	var skills []model.Skill
	err := r.db.WithContext(ctx).
		Table("skills").
		Joins("JOIN stars ON skills.id = stars.skill_id").
		Where("stars.user_id = ? AND skills.soft_deleted_at IS NULL", userID).
		Order("stars.created_at DESC").
		Find(&skills).Error
	return skills, err
}
