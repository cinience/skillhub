package service

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/cinience/skillhub/pkg/model"
	"github.com/cinience/skillhub/pkg/repository"
)

type RatingService struct {
	ratingRepo *repository.RatingRepo
	skillRepo  *repository.SkillRepo
}

func NewRatingService(ratingRepo *repository.RatingRepo, skillRepo *repository.SkillRepo) *RatingService {
	return &RatingService{ratingRepo: ratingRepo, skillRepo: skillRepo}
}

// Rate creates or updates a rating for a skill.
func (s *RatingService) Rate(ctx context.Context, userID uuid.UUID, slug string, score int, comment string) error {
	if score < 1 || score > 5 {
		return fmt.Errorf("score must be between 1 and 5")
	}

	skill, err := s.skillRepo.GetBySlugOrAlias(ctx, slug)
	if err != nil || skill == nil {
		return fmt.Errorf("skill not found")
	}

	// Owner cannot rate their own skill
	if skill.OwnerID == userID {
		return fmt.Errorf("cannot rate your own skill")
	}

	rating := &model.Rating{
		ID:      uuid.New(),
		SkillID: skill.ID,
		UserID:  userID,
		Score:   score,
	}
	if comment != "" {
		rating.Comment = &comment
	}

	if err := s.ratingRepo.Upsert(ctx, rating); err != nil {
		return err
	}

	return s.updateStats(ctx, skill.ID)
}

// GetRatings returns paginated ratings for a skill.
func (s *RatingService) GetRatings(ctx context.Context, slug string, limit int, cursor string) ([]model.RatingWithUser, string, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	skill, err := s.skillRepo.GetBySlugOrAlias(ctx, slug)
	if err != nil || skill == nil {
		return nil, "", fmt.Errorf("skill not found")
	}

	return s.ratingRepo.ListBySkill(ctx, skill.ID, limit, cursor)
}

// DeleteRating removes a user's rating.
func (s *RatingService) DeleteRating(ctx context.Context, userID uuid.UUID, slug string) error {
	skill, err := s.skillRepo.GetBySlugOrAlias(ctx, slug)
	if err != nil || skill == nil {
		return fmt.Errorf("skill not found")
	}

	existing, err := s.ratingRepo.GetByUserAndSkill(ctx, userID, skill.ID)
	if err != nil {
		return err
	}
	if existing == nil {
		return fmt.Errorf("no rating found")
	}

	if err := s.ratingRepo.Delete(ctx, userID, skill.ID); err != nil {
		return err
	}

	return s.updateStats(ctx, skill.ID)
}

func (s *RatingService) updateStats(ctx context.Context, skillID uuid.UUID) error {
	avg, count, err := s.ratingRepo.GetAverageAndCount(ctx, skillID)
	if err != nil {
		return err
	}
	return s.skillRepo.UpdateRatingStats(ctx, skillID, avg, count)
}
