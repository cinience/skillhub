package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/cinience/skillhub/pkg/model"
	"gorm.io/gorm"
)

type UserRepo struct {
	db *gorm.DB
}

func NewUserRepo(db *gorm.DB) *UserRepo {
	return &UserRepo{db: db}
}

func (r *UserRepo) Create(ctx context.Context, user *model.User) error {
	return r.db.WithContext(ctx).Create(user).Error
}

func (r *UserRepo) GetByID(ctx context.Context, id uuid.UUID) (*model.User, error) {
	var user model.User
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&user).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	return &user, err
}

func (r *UserRepo) GetByHandle(ctx context.Context, handle string) (*model.User, error) {
	var user model.User
	err := r.db.WithContext(ctx).Where("handle = ?", handle).First(&user).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	return &user, err
}

func (r *UserRepo) List(ctx context.Context, limit int, cursor string) ([]model.User, string, error) {
	q := r.db.WithContext(ctx).Model(&model.User{})
	if cursor != "" {
		q = q.Where("id > ?", cursor)
	}

	var users []model.User
	err := q.Order("id").Limit(limit + 1).Find(&users).Error
	if err != nil {
		return nil, "", err
	}

	var nextCursor string
	if len(users) > limit {
		nextCursor = users[limit].ID.String()
		users = users[:limit]
	}
	return users, nextCursor, nil
}

func (r *UserRepo) UpdateRole(ctx context.Context, id uuid.UUID, role string) error {
	return r.db.WithContext(ctx).
		Model(&model.User{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"role":       role,
			"updated_at": time.Now(),
		}).Error
}

func (r *UserRepo) Ban(ctx context.Context, id uuid.UUID, reason string) error {
	return r.db.WithContext(ctx).
		Model(&model.User{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"is_banned":  true,
			"ban_reason": reason,
			"updated_at": time.Now(),
		}).Error
}

func (r *UserRepo) Unban(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).
		Model(&model.User{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"is_banned":  false,
			"ban_reason": nil,
			"updated_at": time.Now(),
		}).Error
}

func (r *UserRepo) GetByEmail(ctx context.Context, email string) (*model.User, error) {
	var user model.User
	err := r.db.WithContext(ctx).Where("email = ?", email).First(&user).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	return &user, err
}

func (r *UserRepo) SetPassword(ctx context.Context, id uuid.UUID, passwordHash string) error {
	return r.db.WithContext(ctx).
		Model(&model.User{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"password_hash": passwordHash,
			"updated_at":    time.Now(),
		}).Error
}
