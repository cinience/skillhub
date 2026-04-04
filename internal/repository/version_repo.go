package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/cinience/skillhub/internal/model"
	"gorm.io/gorm"
)

type VersionRepo struct {
	db *gorm.DB
}

func NewVersionRepo(db *gorm.DB) *VersionRepo {
	return &VersionRepo{db: db}
}

func (r *VersionRepo) Create(ctx context.Context, v *model.SkillVersion) error {
	return r.db.WithContext(ctx).Create(v).Error
}

func (r *VersionRepo) GetBySkillAndVersion(ctx context.Context, skillID uuid.UUID, version string) (*model.SkillVersion, error) {
	var v model.SkillVersion
	err := r.db.WithContext(ctx).
		Where("skill_id = ? AND version = ? AND soft_deleted_at IS NULL", skillID, version).
		First(&v).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	return &v, err
}

func (r *VersionRepo) GetByID(ctx context.Context, id uuid.UUID) (*model.SkillVersion, error) {
	var v model.SkillVersion
	err := r.db.WithContext(ctx).
		Where("id = ? AND soft_deleted_at IS NULL", id).
		First(&v).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	return &v, err
}

func (r *VersionRepo) GetLatest(ctx context.Context, skillID uuid.UUID) (*model.SkillVersion, error) {
	var v model.SkillVersion
	err := r.db.WithContext(ctx).
		Where("skill_id = ? AND soft_deleted_at IS NULL", skillID).
		Order("created_at DESC").
		First(&v).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	return &v, err
}

func (r *VersionRepo) ListBySkill(ctx context.Context, skillID uuid.UUID) ([]model.SkillVersion, error) {
	var versions []model.SkillVersion
	err := r.db.WithContext(ctx).
		Where("skill_id = ? AND soft_deleted_at IS NULL", skillID).
		Order("created_at DESC").
		Find(&versions).Error
	return versions, err
}

func (r *VersionRepo) GetByFingerprint(ctx context.Context, fingerprint string) (*model.SkillVersion, error) {
	var v model.SkillVersion
	err := r.db.WithContext(ctx).
		Table("skill_versions").
		Joins("JOIN skills ON skill_versions.skill_id = skills.id").
		Where("skill_versions.fingerprint = ? AND skill_versions.soft_deleted_at IS NULL AND skills.soft_deleted_at IS NULL", fingerprint).
		First(&v).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	return &v, err
}

func (r *VersionRepo) GetBySHA256(ctx context.Context, hash string) (*model.SkillVersion, error) {
	var v model.SkillVersion
	err := r.db.WithContext(ctx).
		Where("sha256_hash = ? AND soft_deleted_at IS NULL", hash).
		First(&v).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	return &v, err
}
