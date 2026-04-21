package auth

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/cinience/skillhub/pkg/model"
	"github.com/cinience/skillhub/pkg/repository"
	"golang.org/x/crypto/bcrypt"
)

type Service struct {
	tokenRepo *repository.TokenRepo
	userRepo  *repository.UserRepo
}

func NewService(tokenRepo *repository.TokenRepo, userRepo *repository.UserRepo) *Service {
	return &Service{tokenRepo: tokenRepo, userRepo: userRepo}
}

// CreateToken generates a new API token for a user.
// scope: "full" (default), "read", or "publish".
// expiresIn: optional duration; if zero, the token never expires.
func (s *Service) CreateToken(ctx context.Context, userID uuid.UUID, label, scope string, expiresIn time.Duration) (string, *model.APIToken, error) {
	rawToken, prefix, tokenHash, err := GenerateToken("")
	if err != nil {
		return "", nil, err
	}

	if scope == "" {
		scope = "full"
	}
	if scope != "full" && scope != "read" && scope != "publish" {
		return "", nil, fmt.Errorf("invalid scope: %s", scope)
	}

	token := &model.APIToken{
		ID:        uuid.New(),
		UserID:    userID,
		Label:     &label,
		Prefix:    prefix,
		TokenHash: tokenHash,
		Scope:     scope,
	}

	if expiresIn > 0 {
		exp := time.Now().Add(expiresIn)
		token.ExpiresAt = &exp
	}

	if err := s.tokenRepo.Create(ctx, token); err != nil {
		return "", nil, err
	}

	return rawToken, token, nil
}

// ValidateToken validates a raw token and returns the associated user and token scope.
func (s *Service) ValidateToken(ctx context.Context, rawToken string) (*model.User, string, error) {
	prefix := ExtractPrefix(rawToken)
	tokens, err := s.tokenRepo.GetByPrefix(ctx, prefix)
	if err != nil {
		return nil, "", err
	}

	tokenHash := HashToken(rawToken)

	for _, t := range tokens {
		if subtle.ConstantTimeCompare([]byte(t.TokenHash), []byte(tokenHash)) == 1 {
			// Check expiration
			if t.ExpiresAt != nil && t.ExpiresAt.Before(time.Now()) {
				continue
			}

			// Update last used (fire and forget)
			go s.tokenRepo.UpdateLastUsed(context.Background(), t.ID)

			user, err := s.userRepo.GetByID(ctx, t.UserID)
			if err != nil {
				return nil, "", err
			}
			if user == nil || user.IsBanned {
				return nil, "", nil
			}
			scope := t.Scope
			if scope == "" {
				scope = "full"
			}
			return user, scope, nil
		}
	}

	return nil, "", nil
}

// HashPassword hashes a plaintext password using bcrypt.
func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// CheckPassword verifies a plaintext password against a bcrypt hash.
func CheckPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

// Login validates a handle+password, and returns a new API token for the session.
func (s *Service) Login(ctx context.Context, handle, password string) (string, *model.User, error) {
	user, err := s.userRepo.GetByHandle(ctx, handle)
	if err != nil {
		return "", nil, err
	}
	if user == nil {
		return "", nil, errors.New("invalid username or password")
	}
	if user.IsBanned {
		return "", nil, errors.New("account is banned")
	}
	if user.PasswordHash == nil || *user.PasswordHash == "" {
		return "", nil, errors.New("password not set, contact an admin")
	}
	if !CheckPassword(*user.PasswordHash, password) {
		return "", nil, errors.New("invalid username or password")
	}

	// Create a session token (30-day expiry)
	label := "web-session"
	rawToken, _, err := s.CreateToken(ctx, user.ID, label, "full", 30*24*time.Hour)
	if err != nil {
		return "", nil, err
	}

	return rawToken, user, nil
}

// SetPassword sets a user's password hash after validating complexity.
func (s *Service) SetPassword(ctx context.Context, userID uuid.UUID, password string) error {
	if len(password) < 8 {
		return errors.New("password must be at least 8 characters")
	}
	hash, err := HashPassword(password)
	if err != nil {
		return err
	}
	return s.userRepo.SetPassword(ctx, userID, hash)
}
