package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/saker-ai/skillhub/pkg/model"
	"gorm.io/gorm"
)

type PluginRepo struct {
	db *gorm.DB
}

func NewPluginRepo(db *gorm.DB) *PluginRepo {
	return &PluginRepo{db: db}
}

func (r *PluginRepo) Create(ctx context.Context, p *model.Plugin) error {
	return r.db.WithContext(ctx).Create(p).Error
}

func (r *PluginRepo) GetBySlug(ctx context.Context, slug string) (*model.Plugin, error) {
	var p model.Plugin
	err := r.db.WithContext(ctx).
		Where("slug = ? AND soft_deleted_at IS NULL", slug).
		First(&p).Error
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *PluginRepo) GetByID(ctx context.Context, id uuid.UUID) (*model.Plugin, error) {
	var p model.Plugin
	err := r.db.WithContext(ctx).
		Where("id = ? AND soft_deleted_at IS NULL", id).
		First(&p).Error
	if err != nil {
		return nil, err
	}
	return &p, nil
}

const pluginWithOwnerSelect = "plugins.*, users.handle as owner_handle, users.avatar_url as owner_avatar_url, namespaces.slug as namespace_slug"

func (r *PluginRepo) GetWithOwner(ctx context.Context, slug string) (*model.PluginWithOwner, error) {
	var p model.PluginWithOwner
	err := r.db.WithContext(ctx).
		Table("plugins").
		Select(pluginWithOwnerSelect).
		Joins("LEFT JOIN users ON users.id = plugins.owner_id").
		Joins("LEFT JOIN namespaces ON namespaces.id = plugins.namespace_id").
		Where("plugins.slug = ? AND plugins.soft_deleted_at IS NULL", slug).
		First(&p).Error
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// GetByNSAndSlug looks up a plugin by namespace ID and slug.
func (r *PluginRepo) GetByNSAndSlug(ctx context.Context, namespaceID uuid.UUID, slug string) (*model.PluginWithOwner, error) {
	var p model.PluginWithOwner
	err := r.db.WithContext(ctx).
		Table("plugins").
		Select(pluginWithOwnerSelect).
		Joins("LEFT JOIN users ON users.id = plugins.owner_id").
		Joins("LEFT JOIN namespaces ON namespaces.id = plugins.namespace_id").
		Where("plugins.namespace_id = ? AND plugins.slug = ? AND plugins.soft_deleted_at IS NULL", namespaceID, slug).
		First(&p).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &p, err
}

func (r *PluginRepo) GetByNSAndSlugIncludeDeleted(ctx context.Context, namespaceID uuid.UUID, slug string) (*model.PluginWithOwner, error) {
	var p model.PluginWithOwner
	err := r.db.WithContext(ctx).
		Table("plugins").
		Select(pluginWithOwnerSelect).
		Joins("LEFT JOIN users ON users.id = plugins.owner_id").
		Joins("LEFT JOIN namespaces ON namespaces.id = plugins.namespace_id").
		Where("plugins.namespace_id = ? AND plugins.slug = ?", namespaceID, slug).
		First(&p).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &p, err
}

// GetBySlugGlobal returns all non-deleted plugins matching a slug across namespaces.
func (r *PluginRepo) GetBySlugGlobal(ctx context.Context, slug string) ([]model.PluginWithOwner, error) {
	var plugins []model.PluginWithOwner
	err := r.db.WithContext(ctx).
		Table("plugins").
		Select(pluginWithOwnerSelect).
		Joins("LEFT JOIN users ON users.id = plugins.owner_id").
		Joins("LEFT JOIN namespaces ON namespaces.id = plugins.namespace_id").
		Where("plugins.slug = ? AND plugins.soft_deleted_at IS NULL", slug).
		Find(&plugins).Error
	return plugins, err
}

func (r *PluginRepo) GetBySlugGlobalIncludeDeleted(ctx context.Context, slug string) ([]model.PluginWithOwner, error) {
	var plugins []model.PluginWithOwner
	err := r.db.WithContext(ctx).
		Table("plugins").
		Select(pluginWithOwnerSelect).
		Joins("LEFT JOIN users ON users.id = plugins.owner_id").
		Joins("LEFT JOIN namespaces ON namespaces.id = plugins.namespace_id").
		Where("plugins.slug = ?", slug).
		Find(&plugins).Error
	return plugins, err
}

type PluginListOptions struct {
	Category string
	Sort     string
	Cursor   string
	Limit    int
	OwnerID  *uuid.UUID
}

func (r *PluginRepo) List(ctx context.Context, opts PluginListOptions) ([]model.PluginWithOwner, string, error) {
	if opts.Limit <= 0 || opts.Limit > 100 {
		opts.Limit = 20
	}

	orderClause := "plugins.created_at DESC, plugins.id"
	switch opts.Sort {
	case "downloads":
		orderClause = "plugins.downloads DESC, plugins.id"
	case "stars":
		orderClause = "plugins.stars_count DESC, plugins.id"
	case "name":
		orderClause = "plugins.slug ASC, plugins.id"
	}

	q := r.db.WithContext(ctx).
		Table("plugins").
		Select(pluginWithOwnerSelect).
		Joins("LEFT JOIN users ON users.id = plugins.owner_id").
		Joins("LEFT JOIN namespaces ON namespaces.id = plugins.namespace_id").
		Where("plugins.soft_deleted_at IS NULL").
		Where("plugins.visibility = 'public'").
		Where("plugins.moderation_status = 'approved'")

	if opts.Category != "" {
		q = q.Where("plugins.category = ?", opts.Category)
	}
	if opts.OwnerID != nil {
		q = q.Where("plugins.owner_id = ?", *opts.OwnerID)
	}
	if opts.Cursor != "" {
		switch opts.Sort {
		case "downloads":
			q = q.Where("(plugins.downloads, plugins.id) < (SELECT downloads, id FROM plugins WHERE id = ?)", opts.Cursor)
		case "stars":
			q = q.Where("(plugins.stars_count, plugins.id) < (SELECT stars_count, id FROM plugins WHERE id = ?)", opts.Cursor)
		case "name":
			q = q.Where("(plugins.slug, plugins.id) > (SELECT slug, id FROM plugins WHERE id = ?)", opts.Cursor)
		default:
			q = q.Where("(plugins.created_at, plugins.id) < (SELECT created_at, id FROM plugins WHERE id = ?)", opts.Cursor)
		}
	}

	q = q.Order(orderClause).Limit(opts.Limit + 1)

	var results []model.PluginWithOwner
	if err := q.Find(&results).Error; err != nil {
		return nil, "", err
	}

	var nextCursor string
	if len(results) > opts.Limit {
		nextCursor = results[opts.Limit].ID.String()
		results = results[:opts.Limit]
	}
	return results, nextCursor, nil
}

func (r *PluginRepo) Update(ctx context.Context, p *model.Plugin) error {
	return r.db.WithContext(ctx).Save(p).Error
}

func (r *PluginRepo) IncrementDownloads(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).
		Model(&model.Plugin{}).
		Where("id = ?", id).
		UpdateColumn("downloads", gorm.Expr("downloads + 1")).Error
}

// --- PluginVersion ---

func (r *PluginRepo) CreateVersion(ctx context.Context, v *model.PluginVersion) error {
	return r.db.WithContext(ctx).Create(v).Error
}

func (r *PluginRepo) GetVersion(ctx context.Context, pluginID uuid.UUID, version string) (*model.PluginVersion, error) {
	var v model.PluginVersion
	err := r.db.WithContext(ctx).
		Where("plugin_id = ? AND version = ? AND status = ? AND soft_deleted_at IS NULL", pluginID, version, "ready").
		First(&v).Error
	if err != nil {
		return nil, err
	}
	return &v, nil
}

func (r *PluginRepo) GetLatestVersion(ctx context.Context, pluginID uuid.UUID) (*model.PluginVersion, error) {
	var v model.PluginVersion
	err := r.db.WithContext(ctx).
		Where("plugin_id = ? AND status = ? AND soft_deleted_at IS NULL AND yanked_at IS NULL", pluginID, "ready").
		Order("created_at DESC").
		First(&v).Error
	if err != nil {
		return nil, err
	}
	return &v, nil
}

func (r *PluginRepo) ListVersions(ctx context.Context, pluginID uuid.UUID) ([]model.PluginVersion, error) {
	var versions []model.PluginVersion
	err := r.db.WithContext(ctx).
		Where("plugin_id = ? AND status = ? AND soft_deleted_at IS NULL", pluginID, "ready").
		Order("created_at DESC").
		Limit(100).
		Find(&versions).Error
	return versions, err
}

func (r *PluginRepo) SetLatestVersion(ctx context.Context, pluginID, versionID uuid.UUID) error {
	return r.db.WithContext(ctx).
		Model(&model.Plugin{}).
		Where("id = ?", pluginID).
		Updates(map[string]any{
			"latest_version_id": versionID,
			"versions_count":    gorm.Expr("versions_count + 1"),
		}).Error
}

func (r *PluginRepo) GetBySlugIncludeDeleted(ctx context.Context, slug string) (*model.Plugin, error) {
	var p model.Plugin
	err := r.db.WithContext(ctx).
		Where("slug = ?", slug).
		First(&p).Error
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *PluginRepo) CountByOwner(ctx context.Context, ownerID uuid.UUID) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&model.Plugin{}).
		Where("owner_id = ? AND soft_deleted_at IS NULL", ownerID).
		Count(&count).Error
	return count, err
}

func (r *PluginRepo) SoftDelete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).
		Model(&model.Plugin{}).
		Where("id = ?", id).
		UpdateColumn("soft_deleted_at", time.Now()).Error
}

func (r *PluginRepo) Undelete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).
		Model(&model.Plugin{}).
		Where("id = ?", id).
		UpdateColumn("soft_deleted_at", nil).Error
}

func (r *PluginRepo) SetYanked(ctx context.Context, versionID uuid.UUID, reason string) error {
	now := time.Now()
	return r.db.WithContext(ctx).
		Model(&model.PluginVersion{}).
		Where("id = ?", versionID).
		Updates(map[string]any{
			"yanked_at":   &now,
			"yank_reason": &reason,
		}).Error
}

func (r *PluginRepo) ClearYanked(ctx context.Context, versionID uuid.UUID) error {
	return r.db.WithContext(ctx).
		Model(&model.PluginVersion{}).
		Where("id = ?", versionID).
		Updates(map[string]any{
			"yanked_at":   nil,
			"yank_reason": nil,
		}).Error
}

func (r *PluginRepo) RepointLatest(ctx context.Context, pluginID uuid.UUID) error {
	var latest model.PluginVersion
	err := r.db.WithContext(ctx).
		Where("plugin_id = ? AND soft_deleted_at IS NULL AND yanked_at IS NULL", pluginID).
		Order("created_at DESC").
		First(&latest).Error
	if err != nil {
		return r.db.WithContext(ctx).
			Model(&model.Plugin{}).
			Where("id = ?", pluginID).
			UpdateColumn("latest_version_id", nil).Error
	}
	return r.db.WithContext(ctx).
		Model(&model.Plugin{}).
		Where("id = ?", pluginID).
		UpdateColumn("latest_version_id", latest.ID).Error
}

func (r *PluginRepo) ListAllForAdmin(ctx context.Context, limit int, cursor, visibility string) ([]model.PluginWithOwner, string, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	q := r.db.WithContext(ctx).
		Table("plugins").
		Select("plugins.*, users.handle as owner_handle, users.avatar_url as owner_avatar_url").
		Joins("LEFT JOIN users ON users.id = plugins.owner_id")

	if visibility != "" {
		q = q.Where("plugins.visibility = ?", visibility)
	}
	if cursor != "" {
		q = q.Where("(plugins.created_at, plugins.id) < (SELECT created_at, id FROM plugins WHERE id = ?)", cursor)
	}

	q = q.Order("plugins.created_at DESC, plugins.id").Limit(limit + 1)

	var results []model.PluginWithOwner
	if err := q.Find(&results).Error; err != nil {
		return nil, "", err
	}

	var nextCursor string
	if len(results) > limit {
		nextCursor = results[limit].ID.String()
		results = results[:limit]
	}
	return results, nextCursor, nil
}

func (r *PluginRepo) SetVisibility(ctx context.Context, id uuid.UUID, visibility, moderationStatus string) error {
	return r.db.WithContext(ctx).
		Model(&model.Plugin{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"visibility":        visibility,
			"moderation_status": moderationStatus,
		}).Error
}

func (r *PluginRepo) Migrate(ctx context.Context) error {
	if err := r.db.WithContext(ctx).AutoMigrate(&model.Plugin{}, &model.PluginVersion{}); err != nil {
		return fmt.Errorf("plugin migration: %w", err)
	}
	return nil
}
