package gitstore

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"time"

	"github.com/cinience/skillhub/pkg/config"
)

var mirrorHTTPClient = &http.Client{Timeout: 15 * time.Second}

type MirrorService struct {
	gitStore *GitStore
	cfg      config.MirrorConfig
	token    string
}

func NewMirrorService(gs *GitStore, cfg config.MirrorConfig) *MirrorService {
	token := os.Getenv(cfg.TokenEnv)
	return &MirrorService{gitStore: gs, cfg: cfg, token: token}
}

func (m *MirrorService) Enabled() bool {
	return m.cfg.Enabled && m.token != ""
}

// PushMirror pushes a local bare repo to the configured remote.
func (m *MirrorService) PushMirror(ctx context.Context, owner, slug string) error {
	if !m.Enabled() {
		return nil
	}

	remoteURL, err := m.ensureRemoteRepo(ctx, slug)
	if err != nil {
		return fmt.Errorf("ensure remote repo: %w", err)
	}

	repoPath := m.gitStore.RepoPath(owner, slug)

	// Use credential helper to avoid leaking token in /proc/*/cmdline
	credHelper := fmt.Sprintf("!f(){ echo username=token; echo password=%s; }; f", m.token)
	cmd := exec.CommandContext(ctx, "git",
		"-c", "credential.helper="+credHelper,
		"push", "--mirror", remoteURL,
	)
	cmd.Dir = repoPath
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git push --mirror failed: %w", err)
	}

	return nil
}

// PushAll pushes all repos to the remote.
func (m *MirrorService) PushAll(ctx context.Context) error {
	if !m.Enabled() {
		return nil
	}

	entries, err := os.ReadDir(m.gitStore.BasePath())
	if err != nil {
		return err
	}

	for _, ownerDir := range entries {
		if !ownerDir.IsDir() {
			continue
		}
		repoEntries, err := os.ReadDir(filepath.Join(m.gitStore.BasePath(), ownerDir.Name()))
		if err != nil {
			continue
		}
		for _, repoDir := range repoEntries {
			if !strings.HasSuffix(repoDir.Name(), ".git") {
				continue
			}
			slug := strings.TrimSuffix(repoDir.Name(), ".git")
			if err := m.PushMirror(ctx, ownerDir.Name(), slug); err != nil {
				log.Printf("mirror push failed for %s/%s: %v", ownerDir.Name(), slug, err)
			}
		}
	}
	return nil
}

func (m *MirrorService) remoteBaseURL() string {
	switch m.cfg.Provider {
	case "github":
		return "https://github.com"
	case "gitlab":
		if m.cfg.BaseURL != "" {
			return strings.TrimRight(m.cfg.BaseURL, "/")
		}
		return "https://gitlab.com"
	case "gitea":
		return strings.TrimRight(m.cfg.BaseURL, "/")
	default:
		return "https://github.com"
	}
}

func (m *MirrorService) ensureRemoteRepo(ctx context.Context, slug string) (string, error) {
	repoName := "skill-" + slug
	baseURL := m.remoteBaseURL()

	switch m.cfg.Provider {
	case "github":
		return m.ensureGitHubRepo(ctx, repoName, baseURL)
	case "gitlab":
		return m.ensureGitLabRepo(ctx, repoName, baseURL)
	case "gitea":
		return m.ensureGiteaRepo(ctx, repoName, baseURL)
	default:
		return fmt.Sprintf("%s/%s/%s.git", baseURL, m.cfg.Org, repoName), nil
	}
}

func (m *MirrorService) ensureGitHubRepo(ctx context.Context, repoName, baseURL string) (string, error) {
	apiURL := "https://api.github.com"
	url := fmt.Sprintf("%s/orgs/%s/repos", apiURL, m.cfg.Org)

	body, _ := json.Marshal(map[string]interface{}{
		"name":    repoName,
		"private": true,
	})
	req, _ := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+m.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := mirrorHTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	// 201 created or 422 already exists are both OK
	if resp.StatusCode != 201 && resp.StatusCode != 422 {
		return "", fmt.Errorf("github create repo: status %d", resp.StatusCode)
	}

	return fmt.Sprintf("https://github.com/%s/%s.git", m.cfg.Org, repoName), nil
}

func (m *MirrorService) ensureGitLabRepo(ctx context.Context, repoName, baseURL string) (string, error) {
	apiURL := baseURL + "/api/v4/projects"

	body, _ := json.Marshal(map[string]interface{}{
		"name":         repoName,
		"namespace_id": m.cfg.Org,
		"visibility":   "private",
	})
	req, _ := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewReader(body))
	req.Header.Set("PRIVATE-TOKEN", m.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := mirrorHTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	host := strings.TrimPrefix(baseURL, "https://")
	host = strings.TrimPrefix(host, "http://")
	return fmt.Sprintf("https://%s/%s/%s.git", host, m.cfg.Org, repoName), nil
}

func (m *MirrorService) ensureGiteaRepo(ctx context.Context, repoName, baseURL string) (string, error) {
	apiURL := baseURL + fmt.Sprintf("/api/v1/orgs/%s/repos", m.cfg.Org)

	body, _ := json.Marshal(map[string]interface{}{
		"name":    repoName,
		"private": true,
	})
	req, _ := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewReader(body))
	req.Header.Set("Authorization", "token "+m.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := mirrorHTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	host := strings.TrimPrefix(baseURL, "https://")
	host = strings.TrimPrefix(host, "http://")
	return fmt.Sprintf("https://%s/%s/%s.git", host, m.cfg.Org, repoName), nil
}
