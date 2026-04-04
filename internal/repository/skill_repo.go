package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/cinience/skillhub/internal/model"
	"gorm.io/gorm"
)

type SkillRepo struct {
	db *gorm.DB
}

func NewSkillRepo(db *gorm.DB) *SkillRepo {
	return &SkillRepo{db: db}
}

func (r *SkillRepo) Create(ctx context.Context, skill *model.Skill) error {
	return r.db.WithContext(ctx).Create(skill).Error
}

func (r *SkillRepo) GetBySlug(ctx context.Context, slug string) (*model.SkillWithOwner, error) {
	var skill model.SkillWithOwner
	err := r.db.WithContext(ctx).
		Table("skills").
		Select("skills.*, users.handle AS owner_handle, users.display_name AS owner_display_name, users.avatar_url AS owner_avatar_url").
		Joins("JOIN users ON skills.owner_id = users.id").
		Where("skills.slug = ? AND skills.soft_deleted_at IS NULL", slug).
		First(&skill).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	return &skill, err
}

func (r *SkillRepo) GetBySlugOrAlias(ctx context.Context, slug string) (*model.SkillWithOwner, error) {
	skill, err := r.GetBySlug(ctx, slug)
	if err != nil {
		return nil, err
	}
	if skill != nil {
		return skill, nil
	}
	// Check aliases
	var alias model.SkillSlugAlias
	err = r.db.WithContext(ctx).Where("old_slug = ?", slug).First(&alias).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	// Found alias, get actual skill
	var s model.SkillWithOwner
	err = r.db.WithContext(ctx).
		Table("skills").
		Select("skills.*, users.handle AS owner_handle, users.display_name AS owner_display_name, users.avatar_url AS owner_avatar_url").
		Joins("JOIN users ON skills.owner_id = users.id").
		Where("skills.id = ? AND skills.soft_deleted_at IS NULL", alias.SkillID).
		First(&s).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	return &s, err
}

func (r *SkillRepo) GetByID(ctx context.Context, id uuid.UUID) (*model.Skill, error) {
	var skill model.Skill
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&skill).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	return &skill, err
}

func (r *SkillRepo) List(ctx context.Context, limit int, cursor string, sort string) ([]model.SkillWithOwner, string, error) {
	orderClause := "skills.created_at DESC, skills.id"
	switch sort {
	case "downloads":
		orderClause = "skills.downloads DESC, skills.id"
	case "stars":
		orderClause = "skills.stars_count DESC, skills.id"
	case "updated":
		orderClause = "skills.updated_at DESC, skills.id"
	case "name":
		orderClause = "skills.slug ASC, skills.id"
	}

	q := r.db.WithContext(ctx).
		Table("skills").
		Select("skills.*, users.handle AS owner_handle, users.display_name AS owner_display_name, users.avatar_url AS owner_avatar_url").
		Joins("JOIN users ON skills.owner_id = users.id").
		Where("skills.soft_deleted_at IS NULL AND skills.moderation_status = ?", "approved")

	if cursor != "" {
		q = q.Where("skills.id > ?", cursor)
	}

	var skills []model.SkillWithOwner
	err := q.Order(orderClause).Limit(limit + 1).Find(&skills).Error
	if err != nil {
		return nil, "", err
	}

	var nextCursor string
	if len(skills) > limit {
		nextCursor = skills[limit].ID.String()
		skills = skills[:limit]
	}
	return skills, nextCursor, nil
}

func (r *SkillRepo) UpdateLatestVersion(ctx context.Context, skillID uuid.UUID, versionID uuid.UUID) error {
	return r.db.WithContext(ctx).
		Model(&model.Skill{}).
		Where("id = ?", skillID).
		Updates(map[string]interface{}{
			"latest_version_id": versionID,
			"versions_count":    gorm.Expr("versions_count + 1"),
			"updated_at":        time.Now(),
		}).Error
}

func (r *SkillRepo) IncrementDownloads(ctx context.Context, skillID uuid.UUID) error {
	return r.db.WithContext(ctx).
		Model(&model.Skill{}).
		Where("id = ?", skillID).
		Update("downloads", gorm.Expr("downloads + 1")).Error
}

func (r *SkillRepo) UpdateStarsCount(ctx context.Context, skillID uuid.UUID, delta int) error {
	return r.db.WithContext(ctx).
		Model(&model.Skill{}).
		Where("id = ?", skillID).
		Update("stars_count", gorm.Expr("stars_count + ?", delta)).Error
}

func (r *SkillRepo) SoftDelete(ctx context.Context, skillID uuid.UUID) error {
	now := time.Now()
	return r.db.WithContext(ctx).
		Model(&model.Skill{}).
		Where("id = ?", skillID).
		Updates(map[string]interface{}{
			"soft_deleted_at": now,
			"updated_at":      now,
		}).Error
}

func (r *SkillRepo) Undelete(ctx context.Context, skillID uuid.UUID) error {
	return r.db.WithContext(ctx).
		Model(&model.Skill{}).
		Where("id = ?", skillID).
		Updates(map[string]interface{}{
			"soft_deleted_at": nil,
			"updated_at":      time.Now(),
		}).Error
}

func (r *SkillRepo) IsSlugReserved(ctx context.Context, slug string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.ReservedSlug{}).Where("slug = ?", slug).Count(&count).Error
	return count > 0, err
}

func (r *SkillRepo) Update(ctx context.Context, skill *model.Skill) error {
	return r.db.WithContext(ctx).
		Model(&model.Skill{}).
		Where("id = ?", skill.ID).
		Updates(map[string]interface{}{
			"display_name": skill.DisplayName,
			"summary":      skill.Summary,
			"tags":         skill.Tags,
			"updated_at":   time.Now(),
		}).Error
}

func (r *SkillRepo) Rename(ctx context.Context, skillID uuid.UUID, oldSlug, newSlug string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.Skill{}).Where("id = ?", skillID).
			Update("slug", newSlug).Error; err != nil {
			return fmt.Errorf("rename skill: %w", err)
		}
		alias := model.SkillSlugAlias{
			ID:      uuid.New(),
			SkillID: skillID,
			OldSlug: oldSlug,
		}
		return tx.Create(&alias).Error
	})
}
