package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/cinience/skillhub/pkg/model"
	"gorm.io/gorm"
)

type NamespaceRepo struct {
	db *gorm.DB
}

func NewNamespaceRepo(db *gorm.DB) *NamespaceRepo {
	return &NamespaceRepo{db: db}
}

func (r *NamespaceRepo) Create(ctx context.Context, ns *model.Namespace) error {
	return r.db.WithContext(ctx).Create(ns).Error
}

func (r *NamespaceRepo) GetBySlug(ctx context.Context, slug string) (*model.Namespace, error) {
	var ns model.Namespace
	err := r.db.WithContext(ctx).Where("slug = ?", slug).First(&ns).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	return &ns, err
}

func (r *NamespaceRepo) GetByID(ctx context.Context, id uuid.UUID) (*model.Namespace, error) {
	var ns model.Namespace
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&ns).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	return &ns, err
}

func (r *NamespaceRepo) Update(ctx context.Context, ns *model.Namespace) error {
	return r.db.WithContext(ctx).Save(ns).Error
}

// ListByUser returns all namespaces where the user is a member.
func (r *NamespaceRepo) ListByUser(ctx context.Context, userID uuid.UUID) ([]model.Namespace, error) {
	var namespaces []model.Namespace
	err := r.db.WithContext(ctx).
		Joins("JOIN namespace_members ON namespace_members.namespace_id = namespaces.id").
		Where("namespace_members.user_id = ?", userID).
		Order("namespaces.created_at DESC").
		Find(&namespaces).Error
	return namespaces, err
}

// AddMember adds a user to a namespace with the given role.
func (r *NamespaceRepo) AddMember(ctx context.Context, member *model.NamespaceMember) error {
	return r.db.WithContext(ctx).Create(member).Error
}

// RemoveMember removes a user from a namespace.
func (r *NamespaceRepo) RemoveMember(ctx context.Context, namespaceID, userID uuid.UUID) error {
	return r.db.WithContext(ctx).
		Where("namespace_id = ? AND user_id = ?", namespaceID, userID).
		Delete(&model.NamespaceMember{}).Error
}

// GetMemberRole returns the member's role in a namespace, or "" if not a member.
func (r *NamespaceRepo) GetMemberRole(ctx context.Context, namespaceID, userID uuid.UUID) (string, error) {
	var member model.NamespaceMember
	err := r.db.WithContext(ctx).
		Where("namespace_id = ? AND user_id = ?", namespaceID, userID).
		First(&member).Error
	if err == gorm.ErrRecordNotFound {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return member.Role, nil
}

// ListMembers returns all members of a namespace with user info.
func (r *NamespaceRepo) ListMembers(ctx context.Context, namespaceID uuid.UUID) ([]model.NamespaceMemberWithUser, error) {
	var members []model.NamespaceMemberWithUser
	err := r.db.WithContext(ctx).
		Table("namespace_members").
		Select("namespace_members.*, users.handle, users.display_name").
		Joins("JOIN users ON users.id = namespace_members.user_id").
		Where("namespace_members.namespace_id = ?", namespaceID).
		Order("namespace_members.created_at ASC").
		Find(&members).Error
	return members, err
}

// Delete removes a namespace.
func (r *NamespaceRepo) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("namespace_id = ?", id).Delete(&model.NamespaceMember{}).Error; err != nil {
			return err
		}
		return tx.Where("id = ?", id).Delete(&model.Namespace{}).Error
	})
}
