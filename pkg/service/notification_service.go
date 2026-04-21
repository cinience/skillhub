package service

import (
	"context"
	"log"

	"github.com/google/uuid"
	"github.com/cinience/skillhub/pkg/model"
	"github.com/cinience/skillhub/pkg/repository"
)

type NotificationService struct {
	repo *repository.NotificationRepo
}

func NewNotificationService(repo *repository.NotificationRepo) *NotificationService {
	return &NotificationService{repo: repo}
}

// Notify creates a notification for a user. Errors are logged but not returned.
func (s *NotificationService) Notify(ctx context.Context, userID uuid.UUID, category, title, body, link string) {
	n := &model.Notification{
		ID:       uuid.New(),
		UserID:   userID,
		Category: category,
		Title:    title,
	}
	if body != "" {
		n.Body = &body
	}
	if link != "" {
		n.Link = &link
	}
	if err := s.repo.Create(ctx, n); err != nil {
		log.Printf("notification: failed to create for user %s: %v", userID, err)
	}
}

// List returns paginated notifications for a user.
func (s *NotificationService) List(ctx context.Context, userID uuid.UUID, limit int, cursor string) ([]model.Notification, string, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	return s.repo.ListByUser(ctx, userID, limit, cursor)
}

// CountUnread returns the unread notification count.
func (s *NotificationService) CountUnread(ctx context.Context, userID uuid.UUID) (int64, error) {
	return s.repo.CountUnread(ctx, userID)
}

// MarkRead marks a single notification as read.
func (s *NotificationService) MarkRead(ctx context.Context, id, userID uuid.UUID) error {
	return s.repo.MarkRead(ctx, id, userID)
}

// MarkAllRead marks all notifications as read for a user.
func (s *NotificationService) MarkAllRead(ctx context.Context, userID uuid.UUID) error {
	return s.repo.MarkAllRead(ctx, userID)
}
