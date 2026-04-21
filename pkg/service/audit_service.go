package service

import (
	"context"
	"log"

	"github.com/google/uuid"
	"github.com/cinience/skillhub/pkg/model"
	"github.com/cinience/skillhub/pkg/repository"
)

type AuditService struct {
	repo *repository.AuditRepo
}

func NewAuditService(repo *repository.AuditRepo) *AuditService {
	return &AuditService{repo: repo}
}

// Log records an audit event. Errors are logged but not returned to avoid
// breaking the caller's flow for a non-critical side effect.
func (s *AuditService) Log(ctx context.Context, actorID *uuid.UUID, action, resourceType string, resourceID *uuid.UUID, details, ipAddress string) {
	entry := &model.AuditLog{
		ID:           uuid.New(),
		ActorID:      actorID,
		Action:       action,
		ResourceType: resourceType,
		ResourceID:   resourceID,
	}
	if details != "" {
		entry.Details = &details
	}
	if ipAddress != "" {
		entry.IPAddress = &ipAddress
	}
	if err := s.repo.Create(ctx, entry); err != nil {
		log.Printf("audit: failed to log %s %s: %v", action, resourceType, err)
	}
}

// List returns paginated audit logs with optional filters.
func (s *AuditService) List(ctx context.Context, limit int, cursor string, filter repository.AuditFilter) ([]model.AuditLog, string, error) {
	return s.repo.List(ctx, limit, cursor, filter)
}
