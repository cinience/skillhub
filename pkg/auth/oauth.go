package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/cinience/skillhub/pkg/config"
	"github.com/cinience/skillhub/pkg/model"
	"github.com/cinience/skillhub/pkg/repository"
)

// OAuthProvider holds resolved OAuth endpoints for a provider.
type OAuthProvider struct {
	Name         string
	ClientID     string
	ClientSecret string
	AuthURL      string
	TokenURL     string
	UserInfoURL  string
	Scopes       string
}

var oauthHTTPClient = &http.Client{Timeout: 15 * time.Second}

// OAuthService handles OAuth2 authentication flows.
type OAuthService struct {
	providers  map[string]*OAuthProvider
	oauthRepo  *repository.OAuthRepo
	userRepo   *repository.UserRepo
	authSvc    *Service
	baseURL    string
	stateStore sync.Map // state -> expiresAt (server-side CSRF store)
	done       chan struct{}
}

// NewOAuthService creates a new OAuthService from config.
func NewOAuthService(cfg map[string]config.OAuthProviderConfig, oauthRepo *repository.OAuthRepo, userRepo *repository.UserRepo, authSvc *Service, baseURL string) *OAuthService {
	providers := make(map[string]*OAuthProvider)

	for name, pcfg := range cfg {
		if pcfg.ClientID == "" || pcfg.ClientSecret == "" {
			continue
		}
		p := resolveProvider(name, pcfg)
		if p != nil {
			providers[name] = p
		}
	}

	svc := &OAuthService{
		providers: providers,
		oauthRepo: oauthRepo,
		userRepo:  userRepo,
		authSvc:   authSvc,
		baseURL:   baseURL,
		done:      make(chan struct{}),
	}
	go svc.cleanupStates()
	return svc
}

// Close stops the cleanup goroutine.
func (s *OAuthService) Close() {
	close(s.done)
}

// cleanupStates periodically removes expired OAuth state entries.
func (s *OAuthService) cleanupStates() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			now := time.Now()
			s.stateStore.Range(func(key, value any) bool {
				if now.After(value.(time.Time)) {
					s.stateStore.Delete(key)
				}
				return true
			})
		case <-s.done:
			return
		}
	}
}

func resolveProvider(name string, cfg config.OAuthProviderConfig) *OAuthProvider {
	switch name {
	case "github":
		return &OAuthProvider{
			Name:         "github",
			ClientID:     cfg.ClientID,
			ClientSecret: cfg.ClientSecret,
			AuthURL:      "https://github.com/login/oauth/authorize",
			TokenURL:     "https://github.com/login/oauth/access_token",
			UserInfoURL:  "https://api.github.com/user",
			Scopes:       "read:user user:email",
		}
	case "gitlab":
		base := "https://gitlab.com"
		if cfg.BaseURL != "" {
			base = strings.TrimRight(cfg.BaseURL, "/")
		}
		return &OAuthProvider{
			Name:         "gitlab",
			ClientID:     cfg.ClientID,
			ClientSecret: cfg.ClientSecret,
			AuthURL:      base + "/oauth/authorize",
			TokenURL:     base + "/oauth/token",
			UserInfoURL:  base + "/api/v4/user",
			Scopes:       "read_user",
		}
	default:
		return nil
	}
}

// HasProvider returns true if the named provider is configured.
func (s *OAuthService) HasProvider(name string) bool {
	_, ok := s.providers[name]
	return ok
}

// GetAuthURL returns the OAuth authorization URL for the provider.
func (s *OAuthService) GetAuthURL(providerName string) (string, string, error) {
	p, ok := s.providers[providerName]
	if !ok {
		return "", "", fmt.Errorf("unknown OAuth provider: %s", providerName)
	}

	state, err := randomState()
	if err != nil {
		return "", "", err
	}

	redirectURI := s.baseURL + "/auth/" + providerName + "/callback"

	params := url.Values{
		"client_id":     {p.ClientID},
		"redirect_uri":  {redirectURI},
		"response_type": {"code"},
		"scope":         {p.Scopes},
		"state":         {state},
	}

	// Store state server-side for CSRF validation
	s.stateStore.Store(state, time.Now().Add(5*time.Minute))

	return p.AuthURL + "?" + params.Encode(), state, nil
}

// ValidateState checks and consumes a server-side OAuth state token.
func (s *OAuthService) ValidateState(state string) bool {
	v, ok := s.stateStore.LoadAndDelete(state)
	if !ok {
		return false
	}
	expiresAt := v.(time.Time)
	return time.Now().Before(expiresAt)
}

// IsSecure returns true if the base URL uses HTTPS.
func (s *OAuthService) IsSecure() bool {
	return strings.HasPrefix(s.baseURL, "https://")
}

// OAuthUserInfo represents the user info from an OAuth provider.
type OAuthUserInfo struct {
	ID        string
	Login     string
	Email     string
	AvatarURL string
}

// HandleCallback exchanges the code for a token, fetches user info, and returns or creates a user.
func (s *OAuthService) HandleCallback(ctx context.Context, providerName, code string) (string, *model.User, error) {
	p, ok := s.providers[providerName]
	if !ok {
		return "", nil, fmt.Errorf("unknown OAuth provider: %s", providerName)
	}

	redirectURI := s.baseURL + "/auth/" + providerName + "/callback"

	// Exchange code for token
	accessToken, err := exchangeCode(p, code, redirectURI)
	if err != nil {
		return "", nil, fmt.Errorf("exchange code: %w", err)
	}

	// Fetch user info
	info, err := fetchUserInfo(p, accessToken)
	if err != nil {
		return "", nil, fmt.Errorf("fetch user info: %w", err)
	}

	// Find or create user
	user, err := s.findOrCreateUser(ctx, providerName, info)
	if err != nil {
		return "", nil, err
	}

	// Create session token (30-day expiry)
	rawToken, _, err := s.authSvc.CreateToken(ctx, user.ID, "oauth-"+providerName, "full", 30*24*time.Hour)
	if err != nil {
		return "", nil, err
	}

	return rawToken, user, nil
}

func (s *OAuthService) findOrCreateUser(ctx context.Context, provider string, info *OAuthUserInfo) (*model.User, error) {
	// 1. Check existing OAuth identity
	identity, err := s.oauthRepo.GetByProviderAndExternalID(ctx, provider, info.ID)
	if err != nil {
		return nil, err
	}
	if identity != nil {
		user, err := s.userRepo.GetByID(ctx, identity.UserID)
		if err != nil {
			return nil, err
		}
		if user != nil && !user.IsBanned {
			return user, nil
		}
	}

	// 2. Match by email
	if info.Email != "" {
		user, err := s.userRepo.GetByEmail(ctx, info.Email)
		if err != nil {
			return nil, err
		}
		if user != nil {
			// Auto-link
			oauthID := &model.OAuthIdentity{
				ID:         uuid.New(),
				UserID:     user.ID,
				Provider:   provider,
				ExternalID: info.ID,
			}
			if info.Email != "" {
				oauthID.Email = &info.Email
			}
			if info.AvatarURL != "" {
				oauthID.AvatarURL = &info.AvatarURL
			}
			if err := s.oauthRepo.Create(ctx, oauthID); err != nil {
				return nil, fmt.Errorf("link oauth identity: %w", err)
			}
			return user, nil
		}
	}

	// 3. Create new user
	handle := info.Login
	if handle == "" {
		handle = provider + "-" + info.ID
	}
	// Ensure handle uniqueness
	existing, _ := s.userRepo.GetByHandle(ctx, handle)
	if existing != nil {
		handle = handle + "-" + info.ID[:8]
	}

	user := &model.User{
		ID:     uuid.New(),
		Handle: handle,
		Role:   "user",
	}
	if info.Email != "" {
		user.Email = &info.Email
	}
	if info.AvatarURL != "" {
		user.AvatarURL = &info.AvatarURL
	}

	if err := s.userRepo.Create(ctx, user); err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}

	// Link OAuth identity
	oauthID := &model.OAuthIdentity{
		ID:         uuid.New(),
		UserID:     user.ID,
		Provider:   provider,
		ExternalID: info.ID,
	}
	if info.Email != "" {
		oauthID.Email = &info.Email
	}
	if info.AvatarURL != "" {
		oauthID.AvatarURL = &info.AvatarURL
	}
	if err := s.oauthRepo.Create(ctx, oauthID); err != nil {
		return nil, fmt.Errorf("link oauth identity: %w", err)
	}

	return user, nil
}

func exchangeCode(p *OAuthProvider, code, redirectURI string) (string, error) {
	data := url.Values{
		"client_id":     {p.ClientID},
		"client_secret": {p.ClientSecret},
		"code":          {code},
		"redirect_uri":  {redirectURI},
		"grant_type":    {"authorization_code"},
	}

	req, err := http.NewRequest("POST", p.TokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := oauthHTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))

	var result struct {
		AccessToken string `json:"access_token"`
		Error       string `json:"error"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("parse token response: %w", err)
	}
	if result.Error != "" {
		return "", fmt.Errorf("oauth error: %s", result.Error)
	}
	if result.AccessToken == "" {
		return "", fmt.Errorf("no access token in response")
	}
	return result.AccessToken, nil
}

func fetchUserInfo(p *OAuthProvider, accessToken string) (*OAuthUserInfo, error) {
	req, err := http.NewRequest("GET", p.UserInfoURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")

	resp, err := oauthHTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))

	var raw map[string]interface{}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, err
	}

	info := &OAuthUserInfo{}

	switch p.Name {
	case "github":
		if v, ok := raw["id"].(float64); ok {
			info.ID = fmt.Sprintf("%.0f", v)
		}
		if v, ok := raw["login"].(string); ok {
			info.Login = v
		}
		if v, ok := raw["email"].(string); ok {
			info.Email = v
		}
		if v, ok := raw["avatar_url"].(string); ok {
			info.AvatarURL = v
		}
	case "gitlab":
		if v, ok := raw["id"].(float64); ok {
			info.ID = fmt.Sprintf("%.0f", v)
		}
		if v, ok := raw["username"].(string); ok {
			info.Login = v
		}
		if v, ok := raw["email"].(string); ok {
			info.Email = v
		}
		if v, ok := raw["avatar_url"].(string); ok {
			info.AvatarURL = v
		}
	}

	if info.ID == "" {
		return nil, fmt.Errorf("could not extract user ID from provider response")
	}

	return info, nil
}

func randomState() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
