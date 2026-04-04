package gitstore

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/cinience/skillhub/internal/config"
)

type ImportService struct {
	gitStore *GitStore
	cfg      config.ImportConfig
	secret   string
	// onNewVersion is called when a new version is imported.
	// It should be set by the caller (e.g., SkillService) to register the version.
	OnNewVersion func(ctx context.Context, owner, slug, version string, repoPath string) error
}

func NewImportService(gs *GitStore, cfg config.ImportConfig) *ImportService {
	secret := os.Getenv(cfg.WebhookSecretEnv)
	return &ImportService{gitStore: gs, cfg: cfg, secret: secret}
}

func (i *ImportService) Enabled() bool {
	return i.cfg.Enabled
}

// VerifySignature verifies HMAC-SHA256 webhook signature.
func (i *ImportService) VerifySignature(payload []byte, signature string) bool {
	if i.secret == "" {
		return false
	}
	mac := hmac.New(sha256.New, []byte(i.secret))
	mac.Write(payload)
	expected := hex.EncodeToString(mac.Sum(nil))
	// GitHub sends "sha256=<hex>", GitLab sends just "<hex>"
	signature = strings.TrimPrefix(signature, "sha256=")
	return hmac.Equal([]byte(expected), []byte(signature))
}

type WebhookEvent struct {
	Provider string // "github", "gitlab", "gitea"
	RepoURL  string
	Ref      string // e.g., "refs/tags/v1.0.0"
	Tag      string // e.g., "v1.0.0"
	Commit   string
}

// ParseGitHubWebhook parses a GitHub push webhook payload.
func ParseGitHubWebhook(payload []byte) (*WebhookEvent, error) {
	var data struct {
		Ref        string `json:"ref"`
		After      string `json:"after"`
		Repository struct {
			CloneURL string `json:"clone_url"`
		} `json:"repository"`
	}
	if err := json.Unmarshal(payload, &data); err != nil {
		return nil, err
	}
	if !strings.HasPrefix(data.Ref, "refs/tags/v") {
		return nil, fmt.Errorf("not a version tag: %s", data.Ref)
	}
	tag := strings.TrimPrefix(data.Ref, "refs/tags/")
	return &WebhookEvent{
		Provider: "github",
		RepoURL:  data.Repository.CloneURL,
		Ref:      data.Ref,
		Tag:      tag,
		Commit:   data.After,
	}, nil
}

// ParseGitLabWebhook parses a GitLab tag push webhook payload.
func ParseGitLabWebhook(payload []byte) (*WebhookEvent, error) {
	var data struct {
		Ref        string `json:"ref"`
		CheckoutSHA string `json:"checkout_sha"`
		Project    struct {
			GitHTTPURL string `json:"git_http_url"`
		} `json:"project"`
	}
	if err := json.Unmarshal(payload, &data); err != nil {
		return nil, err
	}
	if !strings.HasPrefix(data.Ref, "refs/tags/v") {
		return nil, fmt.Errorf("not a version tag: %s", data.Ref)
	}
	tag := strings.TrimPrefix(data.Ref, "refs/tags/")
	return &WebhookEvent{
		Provider: "gitlab",
		RepoURL:  data.Project.GitHTTPURL,
		Ref:      data.Ref,
		Tag:      tag,
		Commit:   data.CheckoutSHA,
	}, nil
}

// ParseGiteaWebhook parses a Gitea push webhook payload (same format as GitHub).
func ParseGiteaWebhook(payload []byte) (*WebhookEvent, error) {
	return ParseGitHubWebhook(payload) // Gitea uses GitHub-compatible format
}

// HandleWebhook processes a webhook event: fetches the repo and registers the version.
func (i *ImportService) HandleWebhook(ctx context.Context, event *WebhookEvent) error {
	if !i.Enabled() {
		return fmt.Errorf("import is not enabled")
	}

	// Validate allowed origin
	allowed := false
	for _, origin := range i.cfg.AllowedOrigins {
		if strings.HasPrefix(event.RepoURL, origin) {
			allowed = true
			break
		}
	}
	if !allowed {
		return fmt.Errorf("repo URL %s not in allowed origins", event.RepoURL)
	}

	// Derive owner and slug from repo URL
	slug := repoNameFromURL(event.RepoURL)
	owner := "imported" // default owner for webhook imports

	version := strings.TrimPrefix(event.Tag, "v")

	repoPath := i.gitStore.RepoPath(owner, slug)

	// Clone or fetch
	if i.gitStore.Exists(owner, slug) {
		// Fetch new tags
		cmd := exec.CommandContext(ctx, "git", "fetch", "--tags", event.RepoURL)
		cmd.Dir = repoPath
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("git fetch: %w", err)
		}
	} else {
		// Clone bare
		if err := os.MkdirAll(filepath.Dir(repoPath), 0755); err != nil {
			return err
		}
		cmd := exec.CommandContext(ctx, "git", "clone", "--bare", event.RepoURL, repoPath)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("git clone --bare: %w", err)
		}
	}

	if i.OnNewVersion != nil {
		return i.OnNewVersion(ctx, owner, slug, version, repoPath)
	}
	return nil
}

type ImportOpts struct {
	Slug       string
	OwnerID    string
	TagPattern string
}

// ImportFromURL clones a repo and registers all version tags.
func (i *ImportService) ImportFromURL(ctx context.Context, repoURL string, opts ImportOpts) error {
	slug := opts.Slug
	if slug == "" {
		slug = repoNameFromURL(repoURL)
	}
	owner := "imported"
	if opts.OwnerID != "" {
		owner = opts.OwnerID
	}

	repoPath := i.gitStore.RepoPath(owner, slug)
	if err := os.MkdirAll(filepath.Dir(repoPath), 0755); err != nil {
		return err
	}

	cmd := exec.CommandContext(ctx, "git", "clone", "--bare", repoURL, repoPath)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git clone --bare: %w", err)
	}

	// List tags and register versions
	tags, err := i.gitStore.ListTags(owner, slug)
	if err != nil {
		return err
	}

	for _, version := range tags {
		if i.OnNewVersion != nil {
			if err := i.OnNewVersion(ctx, owner, slug, version, repoPath); err != nil {
				return fmt.Errorf("register version %s: %w", version, err)
			}
		}
	}

	return nil
}

func repoNameFromURL(url string) string {
	// Extract repo name from URL like https://github.com/org/repo-name.git
	parts := strings.Split(strings.TrimSuffix(url, ".git"), "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return "unknown"
}
