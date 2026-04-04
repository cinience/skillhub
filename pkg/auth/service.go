package auth

import (
	"context"
	"crypto/subtle"
	"errors"
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
func (s *Service) CreateToken(ctx context.Context, userID uuid.UUID, label string) (string, *model.APIToken, error) {
	rawToken, prefix, tokenHash, err := GenerateToken("")
	if err != nil {
		return "", nil, err
	}

	token := &model.APIToken{
		ID:        uuid.New(),
		UserID:    userID,
		Label:     &label,
		Prefix:    prefix,
		TokenHash: tokenHash,
	}

	if err := s.tokenRepo.Create(ctx, token); err != nil {
		return "", nil, err
	}

	return rawToken, token, nil
}

// ValidateToken validates a raw token and returns the associated user.
func (s *Service) ValidateToken(ctx context.Context, rawToken string) (*model.User, error) {
	prefix := ExtractPrefix(rawToken)
	tokens, err := s.tokenRepo.GetByPrefix(ctx, prefix)
	if err != nil {
		return nil, err
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
				return nil, err
			}
			if user == nil || user.IsBanned {
				return nil, nil
			}
			return user, nil
		}
	}

	return nil, nil
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

	// Create a session token
	label := "web-session"
	rawToken, _, err := s.CreateToken(ctx, user.ID, label)
	if err != nil {
		return "", nil, err
	}

	return rawToken, user, nil
}

// SetPassword sets a user's password hash.
func (s *Service) SetPassword(ctx context.Context, userID uuid.UUID, password string) error {
	hash, err := HashPassword(password)
	if err != nil {
		return err
	}
	return s.userRepo.SetPassword(ctx, userID, hash)
}
