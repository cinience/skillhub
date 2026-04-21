package service

import (
	"context"
	"fmt"
	"regexp"

	"github.com/google/uuid"
	"github.com/cinience/skillhub/pkg/model"
	"github.com/cinience/skillhub/pkg/repository"
)

var nsSlugRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,62}[a-z0-9]$`)

type NamespaceService struct {
	nsRepo   *repository.NamespaceRepo
	userRepo *repository.UserRepo
}

func NewNamespaceService(nsRepo *repository.NamespaceRepo, userRepo *repository.UserRepo) *NamespaceService {
	return &NamespaceService{nsRepo: nsRepo, userRepo: userRepo}
}

// Create creates a new namespace and adds the creator as owner.
func (s *NamespaceService) Create(ctx context.Context, user *model.User, slug, displayName, description, nsType string) (*model.Namespace, error) {
	if !nsSlugRe.MatchString(slug) {
		return nil, fmt.Errorf("invalid namespace slug: must be 3-64 lowercase alphanumeric characters or hyphens")
	}

	existing, err := s.nsRepo.GetBySlug(ctx, slug)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, fmt.Errorf("namespace slug '%s' already taken", slug)
	}

	if nsType == "" {
		nsType = "team"
	}
	if nsType != "personal" && nsType != "team" {
		return nil, fmt.Errorf("type must be 'personal' or 'team'")
	}

	ns := &model.Namespace{
		ID:      uuid.New(),
		Slug:    slug,
		OwnerID: user.ID,
		Type:    nsType,
		Status:  "active",
	}
	if displayName != "" {
		ns.DisplayName = &displayName
	}
	if description != "" {
		ns.Description = &description
	}

	if err := s.nsRepo.Create(ctx, ns); err != nil {
		return nil, err
	}

	// Add creator as owner member
	member := &model.NamespaceMember{
		ID:          uuid.New(),
		NamespaceID: ns.ID,
		UserID:      user.ID,
		Role:        "owner",
	}
	if err := s.nsRepo.AddMember(ctx, member); err != nil {
		return nil, err
	}

	return ns, nil
}

// GetBySlug returns a namespace by slug.
func (s *NamespaceService) GetBySlug(ctx context.Context, slug string) (*model.Namespace, error) {
	return s.nsRepo.GetBySlug(ctx, slug)
}

// ListByUser returns all namespaces the user belongs to.
func (s *NamespaceService) ListByUser(ctx context.Context, userID uuid.UUID) ([]model.Namespace, error) {
	return s.nsRepo.ListByUser(ctx, userID)
}

// Update updates a namespace's display name and description.
func (s *NamespaceService) Update(ctx context.Context, user *model.User, slug, displayName, description string) (*model.Namespace, error) {
	ns, err := s.nsRepo.GetBySlug(ctx, slug)
	if err != nil || ns == nil {
		return nil, fmt.Errorf("namespace not found")
	}

	role, err := s.nsRepo.GetMemberRole(ctx, ns.ID, user.ID)
	if err != nil {
		return nil, err
	}
	if role != "owner" && role != "admin" && !user.IsAdmin() {
		return nil, fmt.Errorf("forbidden")
	}

	ns.DisplayName = &displayName
	ns.Description = &description

	if err := s.nsRepo.Update(ctx, ns); err != nil {
		return nil, err
	}
	return ns, nil
}

// AddMember adds a user to a namespace.
func (s *NamespaceService) AddMember(ctx context.Context, actor *model.User, nsSlug, handle, role string) error {
	ns, err := s.nsRepo.GetBySlug(ctx, nsSlug)
	if err != nil || ns == nil {
		return fmt.Errorf("namespace not found")
	}

	actorRole, err := s.nsRepo.GetMemberRole(ctx, ns.ID, actor.ID)
	if err != nil {
		return err
	}
	if actorRole != "owner" && actorRole != "admin" && !actor.IsAdmin() {
		return fmt.Errorf("forbidden: only owner or admin can manage members")
	}

	if role == "" {
		role = "member"
	}
	if role != "owner" && role != "admin" && role != "member" {
		return fmt.Errorf("role must be 'owner', 'admin', or 'member'")
	}

	targetUser, err := s.userRepo.GetByHandle(ctx, handle)
	if err != nil || targetUser == nil {
		return fmt.Errorf("user not found: %s", handle)
	}

	// Check if already a member
	existingRole, _ := s.nsRepo.GetMemberRole(ctx, ns.ID, targetUser.ID)
	if existingRole != "" {
		return fmt.Errorf("user is already a member")
	}

	member := &model.NamespaceMember{
		ID:          uuid.New(),
		NamespaceID: ns.ID,
		UserID:      targetUser.ID,
		Role:        role,
	}
	return s.nsRepo.AddMember(ctx, member)
}

// RemoveMember removes a user from a namespace.
func (s *NamespaceService) RemoveMember(ctx context.Context, actor *model.User, nsSlug, handle string) error {
	ns, err := s.nsRepo.GetBySlug(ctx, nsSlug)
	if err != nil || ns == nil {
		return fmt.Errorf("namespace not found")
	}

	actorRole, err := s.nsRepo.GetMemberRole(ctx, ns.ID, actor.ID)
	if err != nil {
		return err
	}
	if actorRole != "owner" && actorRole != "admin" && !actor.IsAdmin() {
		return fmt.Errorf("forbidden: only owner or admin can manage members")
	}

	targetUser, err := s.userRepo.GetByHandle(ctx, handle)
	if err != nil || targetUser == nil {
		return fmt.Errorf("user not found: %s", handle)
	}

	// Cannot remove the owner
	targetRole, _ := s.nsRepo.GetMemberRole(ctx, ns.ID, targetUser.ID)
	if targetRole == "owner" {
		return fmt.Errorf("cannot remove the owner from the namespace")
	}

	return s.nsRepo.RemoveMember(ctx, ns.ID, targetUser.ID)
}

// ListMembers returns all members of a namespace.
func (s *NamespaceService) ListMembers(ctx context.Context, nsSlug string) ([]model.NamespaceMemberWithUser, error) {
	ns, err := s.nsRepo.GetBySlug(ctx, nsSlug)
	if err != nil || ns == nil {
		return nil, fmt.Errorf("namespace not found")
	}
	return s.nsRepo.ListMembers(ctx, ns.ID)
}

// IsMemberOrAdmin checks if a user is a member of the namespace or a system admin.
func (s *NamespaceService) IsMemberOrAdmin(ctx context.Context, nsID uuid.UUID, user *model.User) bool {
	if user.IsAdmin() {
		return true
	}
	role, err := s.nsRepo.GetMemberRole(ctx, nsID, user.ID)
	if err != nil {
		return false
	}
	return role != ""
}

// CanPublish checks if a user can publish to a namespace.
func (s *NamespaceService) CanPublish(ctx context.Context, nsSlug string, userID uuid.UUID) (bool, error) {
	ns, err := s.nsRepo.GetBySlug(ctx, nsSlug)
	if err != nil || ns == nil {
		return false, nil
	}
	role, err := s.nsRepo.GetMemberRole(ctx, ns.ID, userID)
	if err != nil {
		return false, err
	}
	// Any member can publish
	return role != "", nil
}

// Delete deletes a namespace (owner only).
func (s *NamespaceService) Delete(ctx context.Context, user *model.User, nsSlug string) error {
	ns, err := s.nsRepo.GetBySlug(ctx, nsSlug)
	if err != nil || ns == nil {
		return fmt.Errorf("namespace not found")
	}

	role, err := s.nsRepo.GetMemberRole(ctx, ns.ID, user.ID)
	if err != nil {
		return err
	}
	if role != "owner" && !user.IsAdmin() {
		return fmt.Errorf("forbidden: only the owner can delete a namespace")
	}

	return s.nsRepo.Delete(ctx, ns.ID)
}
