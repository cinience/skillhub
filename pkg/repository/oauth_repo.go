package repository

import (
	"context"

	"github.com/cinience/skillhub/pkg/model"
	"gorm.io/gorm"
)

type OAuthRepo struct {
	db *gorm.DB
}

func NewOAuthRepo(db *gorm.DB) *OAuthRepo {
	return &OAuthRepo{db: db}
}

func (r *OAuthRepo) Create(ctx context.Context, identity *model.OAuthIdentity) error {
	return r.db.WithContext(ctx).Create(identity).Error
}

func (r *OAuthRepo) GetByProviderAndExternalID(ctx context.Context, provider, externalID string) (*model.OAuthIdentity, error) {
	var identity model.OAuthIdentity
	err := r.db.WithContext(ctx).
		Where("provider = ? AND external_id = ?", provider, externalID).
		First(&identity).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	return &identity, err
}

func (r *OAuthRepo) GetByUserID(ctx context.Context, userID string) ([]model.OAuthIdentity, error) {
	var identities []model.OAuthIdentity
	err := r.db.WithContext(ctx).Where("user_id = ?", userID).Find(&identities).Error
	return identities, err
}
