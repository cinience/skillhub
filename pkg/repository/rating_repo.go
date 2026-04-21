package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/cinience/skillhub/pkg/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type RatingRepo struct {
	db *gorm.DB
}

func NewRatingRepo(db *gorm.DB) *RatingRepo {
	return &RatingRepo{db: db}
}

// Upsert creates or updates a rating (one per user per skill).
func (r *RatingRepo) Upsert(ctx context.Context, rating *model.Rating) error {
	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "skill_id"}, {Name: "user_id"}},
			DoUpdates: clause.AssignmentColumns([]string{"score", "comment", "updated_at"}),
		}).
		Create(rating).Error
}

// ListBySkill returns ratings for a skill with user info.
func (r *RatingRepo) ListBySkill(ctx context.Context, skillID uuid.UUID, limit int, cursor string) ([]model.RatingWithUser, string, error) {
	q := r.db.WithContext(ctx).
		Table("ratings").
		Select("ratings.*, users.handle, users.display_name").
		Joins("JOIN users ON users.id = ratings.user_id").
		Where("ratings.skill_id = ?", skillID)

	if cursor != "" {
		q = q.Where("ratings.id < ?", cursor)
	}

	var ratings []model.RatingWithUser
	err := q.Order("ratings.created_at DESC, ratings.id DESC").Limit(limit + 1).Find(&ratings).Error
	if err != nil {
		return nil, "", err
	}

	var nextCursor string
	if len(ratings) > limit {
		nextCursor = ratings[limit].ID.String()
		ratings = ratings[:limit]
	}
	return ratings, nextCursor, nil
}

// GetByUserAndSkill returns a user's rating for a skill.
func (r *RatingRepo) GetByUserAndSkill(ctx context.Context, userID, skillID uuid.UUID) (*model.Rating, error) {
	var rating model.Rating
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND skill_id = ?", userID, skillID).
		First(&rating).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	return &rating, err
}

// Delete removes a rating.
func (r *RatingRepo) Delete(ctx context.Context, userID, skillID uuid.UUID) error {
	return r.db.WithContext(ctx).
		Where("user_id = ? AND skill_id = ?", userID, skillID).
		Delete(&model.Rating{}).Error
}

// GetAverageAndCount returns the average score and count for a skill.
func (r *RatingRepo) GetAverageAndCount(ctx context.Context, skillID uuid.UUID) (float64, int, error) {
	var result struct {
		Avg   float64
		Count int
	}
	err := r.db.WithContext(ctx).
		Model(&model.Rating{}).
		Select("COALESCE(AVG(score), 0) as avg, COUNT(*) as count").
		Where("skill_id = ?", skillID).
		Scan(&result).Error
	return result.Avg, result.Count, err
}
