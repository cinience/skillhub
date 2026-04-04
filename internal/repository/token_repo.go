package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/cinience/skillhub/internal/model"
	"gorm.io/gorm"
)

type TokenRepo struct {
	db *gorm.DB
}

func NewTokenRepo(db *gorm.DB) *TokenRepo {
	return &TokenRepo{db: db}
}

func (r *TokenRepo) Create(ctx context.Context, token *model.APIToken) error {
	return r.db.WithContext(ctx).Create(token).Error
}

func (r *TokenRepo) GetByPrefix(ctx context.Context, prefix string) ([]model.APIToken, error) {
	var tokens []model.APIToken
	err := r.db.WithContext(ctx).
		Where("prefix = ? AND revoked_at IS NULL", prefix).
		Find(&tokens).Error
	return tokens, err
}

func (r *TokenRepo) GetByUserID(ctx context.Context, userID uuid.UUID) ([]model.APIToken, error) {
	var tokens []model.APIToken
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND revoked_at IS NULL", userID).
		Order("created_at DESC").
		Find(&tokens).Error
	return tokens, err
}

func (r *TokenRepo) UpdateLastUsed(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).
		Model(&model.APIToken{}).
		Where("id = ?", id).
		Update("last_used_at", time.Now()).Error
}

func (r *TokenRepo) Revoke(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).
		Model(&model.APIToken{}).
		Where("id = ?", id).
		Update("revoked_at", time.Now()).Error
}

func (r *TokenRepo) GetByID(ctx context.Context, id uuid.UUID) (*model.APIToken, error) {
	var token model.APIToken
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&token).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	return &token, err
}
