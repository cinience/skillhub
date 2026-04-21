package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

const (
	deviceCodeExpiry = 5 * time.Minute
	pollInterval     = 5 * time.Second
	maxActiveCodes   = 1000
	cleanupInterval  = 1 * time.Minute
)

type deviceEntry struct {
	UserCode   string
	DeviceCode string
	UserID     *uuid.UUID // set when authorized
	Token      string     // set when authorized
	ExpiresAt  time.Time
	Authorized bool
}

// DeviceAuthService manages the device authorization flow.
type DeviceAuthService struct {
	mu      sync.Mutex
	entries map[string]*deviceEntry // keyed by deviceCode
	byCodes map[string]string       // userCode -> deviceCode
	authSvc *Service
	baseURL string
	done    chan struct{}
}

func NewDeviceAuthService(authSvc *Service, baseURL string) *DeviceAuthService {
	s := &DeviceAuthService{
		entries: make(map[string]*deviceEntry),
		byCodes: make(map[string]string),
		authSvc: authSvc,
		baseURL: baseURL,
		done:    make(chan struct{}),
	}
	go s.cleanupLoop()
	return s
}

// Close stops the cleanup goroutine.
func (s *DeviceAuthService) Close() {
	close(s.done)
}

// cleanupLoop periodically removes expired entries instead of per-entry goroutines.
func (s *DeviceAuthService) cleanupLoop() {
	ticker := time.NewTicker(cleanupInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			s.mu.Lock()
			now := time.Now()
			for dc, entry := range s.entries {
				if now.After(entry.ExpiresAt.Add(time.Minute)) {
					delete(s.byCodes, entry.UserCode)
					delete(s.entries, dc)
				}
			}
			s.mu.Unlock()
		case <-s.done:
			return
		}
	}
}

// DeviceCodeResponse is returned to the CLI.
type DeviceCodeResponse struct {
	DeviceCode      string `json:"deviceCode"`
	UserCode        string `json:"userCode"`
	VerificationURL string `json:"verificationUrl"`
	ExpiresIn       int    `json:"expiresIn"`
	Interval        int    `json:"interval"`
}

// CreateCode generates a new device code and user code.
func (s *DeviceAuthService) CreateCode() (*DeviceCodeResponse, error) {
	deviceCode, err := randomDeviceCode()
	if err != nil {
		return nil, err
	}

	userCode, err := randomUserCode()
	if err != nil {
		return nil, err
	}

	entry := &deviceEntry{
		UserCode:   userCode,
		DeviceCode: deviceCode,
		ExpiresAt:  time.Now().Add(deviceCodeExpiry),
	}

	s.mu.Lock()
	if len(s.entries) >= maxActiveCodes {
		s.mu.Unlock()
		return nil, fmt.Errorf("too many active device codes, try again later")
	}
	s.entries[deviceCode] = entry
	s.byCodes[userCode] = deviceCode
	s.mu.Unlock()

	return &DeviceCodeResponse{
		DeviceCode:      deviceCode,
		UserCode:        userCode,
		VerificationURL: s.baseURL + "/auth/device/verify",
		ExpiresIn:       int(deviceCodeExpiry.Seconds()),
		Interval:        int(pollInterval.Seconds()),
	}, nil
}

// Authorize marks a user code as authorized by a logged-in user.
func (s *DeviceAuthService) Authorize(ctx context.Context, userCode string, userID uuid.UUID) error {
	s.mu.Lock()
	deviceCode, ok := s.byCodes[userCode]
	if !ok {
		s.mu.Unlock()
		return fmt.Errorf("invalid or expired user code")
	}

	entry, ok := s.entries[deviceCode]
	if !ok || time.Now().After(entry.ExpiresAt) {
		s.mu.Unlock()
		return fmt.Errorf("device code expired")
	}
	s.mu.Unlock()

	// Create token outside the lock to avoid holding it during DB calls
	rawToken, _, err := s.authSvc.CreateToken(ctx, userID, "device-auth", "full", 30*24*time.Hour)
	if err != nil {
		return fmt.Errorf("create token: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	// Re-check entry is still valid
	entry, ok = s.entries[deviceCode]
	if !ok || time.Now().After(entry.ExpiresAt) {
		return fmt.Errorf("device code expired")
	}

	entry.UserID = &userID
	entry.Token = rawToken
	entry.Authorized = true

	return nil
}

// PollToken checks if a device code has been authorized. Returns:
// - token if authorized
// - "pending" if not yet authorized
// - error if expired or invalid
func (s *DeviceAuthService) PollToken(deviceCode string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry, ok := s.entries[deviceCode]
	if !ok {
		return "", fmt.Errorf("invalid or expired device code")
	}

	if time.Now().After(entry.ExpiresAt) {
		return "", fmt.Errorf("device code expired")
	}

	if !entry.Authorized {
		return "pending", nil
	}

	return entry.Token, nil
}

func randomDeviceCode() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func randomUserCode() (string, error) {
	const charset = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789" // no I/O/0/1, len=32 (no modulo bias: 256%32==0)
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	code := make([]byte, 8)
	for i := range b {
		code[i] = charset[int(b[i])%len(charset)]
	}
	return string(code[:4]) + "-" + string(code[4:]), nil
}

// FormatUserCode normalizes user input (strips spaces/dashes, uppercases) and re-inserts the dash.
func FormatUserCode(input string) string {
	s := strings.ToUpper(strings.ReplaceAll(strings.ReplaceAll(input, "-", ""), " ", ""))
	if len(s) == 8 {
		return s[:4] + "-" + s[4:]
	}
	return s
}
