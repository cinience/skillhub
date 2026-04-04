package gitstore

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-git/go-git/v5"
	gitconfig "github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

type GitStore struct {
	basePath string
	locks    sync.Map // per-repo mutexes
}

func New(basePath string) (*GitStore, error) {
	if err := os.MkdirAll(basePath, 0755); err != nil {
		return nil, fmt.Errorf("create git store base path: %w", err)
	}
	return &GitStore{basePath: basePath}, nil
}

func (g *GitStore) repoLock(owner, slug string) *sync.Mutex {
	key := owner + "/" + slug
	actual, _ := g.locks.LoadOrStore(key, &sync.Mutex{})
	return actual.(*sync.Mutex)
}

// RepoPath returns the bare repo path: {basePath}/{owner}/{slug}.git
func (g *GitStore) RepoPath(owner, slug string) string {
	return filepath.Join(g.basePath, owner, slug+".git")
}

// InitRepo creates a bare repo (idempotent).
func (g *GitStore) InitRepo(owner, slug string) error {
	path := g.RepoPath(owner, slug)
	if _, err := os.Stat(path); err == nil {
		return nil // already exists
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("create owner dir: %w", err)
	}
	_, err := git.PlainInit(path, true)
	if err != nil {
		return fmt.Errorf("git init bare: %w", err)
	}
	return nil
}

type PublishOpts struct {
	Owner   string
	Slug    string
	Version string            // "1.0.0" → tag "v1.0.0"
	Files   map[string][]byte // path → content
	Author  string
	Email   string
	Message string   // changelog as commit message
	Tags    []string // extra tags
}

// Publish writes files into the repo and creates a version tag.
func (g *GitStore) Publish(ctx context.Context, opts PublishOpts) (string, error) {
	mu := g.repoLock(opts.Owner, opts.Slug)
	mu.Lock()
	defer mu.Unlock()

	if err := g.InitRepo(opts.Owner, opts.Slug); err != nil {
		return "", err
	}

	repoPath := g.RepoPath(opts.Owner, opts.Slug)
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return "", fmt.Errorf("open repo: %w", err)
	}

	// Use a temporary worktree to stage and commit files
	tmpDir, err := os.MkdirTemp("", "skillhub-publish-*")
	if err != nil {
		return "", fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	// Clone bare repo to temp working copy
	workRepo, err := git.PlainClone(tmpDir, false, &git.CloneOptions{
		URL: repoPath,
	})
	if err != nil {
		// If repo is empty (no commits yet), init a new repo
		if err == git.ErrRepositoryAlreadyExists || strings.Contains(err.Error(), "remote repository is empty") {
			workRepo, err = git.PlainInit(tmpDir, false)
			if err != nil {
				return "", fmt.Errorf("init temp repo: %w", err)
			}
		} else {
			return "", fmt.Errorf("clone to temp: %w", err)
		}
	}

	wt, err := workRepo.Worktree()
	if err != nil {
		return "", fmt.Errorf("get worktree: %w", err)
	}

	// Remove all existing files in worktree
	entries, _ := os.ReadDir(tmpDir)
	for _, e := range entries {
		if e.Name() == ".git" {
			continue
		}
		os.RemoveAll(filepath.Join(tmpDir, e.Name()))
	}

	// Write files
	for path, content := range opts.Files {
		fullPath := filepath.Join(tmpDir, path)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			return "", fmt.Errorf("create dir for %s: %w", path, err)
		}
		if err := os.WriteFile(fullPath, content, 0644); err != nil {
			return "", fmt.Errorf("write file %s: %w", path, err)
		}
		if _, err := wt.Add(path); err != nil {
			return "", fmt.Errorf("git add %s: %w", path, err)
		}
	}

	// Commit
	commitMsg := opts.Message
	if commitMsg == "" {
		commitMsg = fmt.Sprintf("Release v%s", opts.Version)
	}
	commitHash, err := wt.Commit(commitMsg, &git.CommitOptions{
		All: true,
		Author: &object.Signature{
			Name:  opts.Author,
			Email: opts.Email,
			When:  time.Now(),
		},
	})
	if err != nil {
		return "", fmt.Errorf("commit: %w", err)
	}

	// Push back to bare repo
	_, err = workRepo.CreateRemote(&gitconfig.RemoteConfig{
		Name: "bare",
		URLs: []string{repoPath},
	})
	if err != nil && err != git.ErrRemoteExists {
		return "", fmt.Errorf("create remote: %w", err)
	}
	err = workRepo.Push(&git.PushOptions{
		RemoteName: "bare",
		RefSpecs:   []gitconfig.RefSpec{"refs/heads/master:refs/heads/master"},
	})
	if err != nil && err != git.NoErrAlreadyUpToDate {
		// Try "main" branch
		err = workRepo.Push(&git.PushOptions{
			RemoteName: "bare",
			RefSpecs:   []gitconfig.RefSpec{"refs/heads/main:refs/heads/master"},
		})
		if err != nil && err != git.NoErrAlreadyUpToDate {
			return "", fmt.Errorf("push to bare: %w", err)
		}
	}

	// Create version tag on bare repo
	tagName := "v" + opts.Version
	_, err = repo.CreateTag(tagName, commitHash, &git.CreateTagOptions{
		Message: commitMsg,
		Tagger: &object.Signature{
			Name:  opts.Author,
			Email: opts.Email,
			When:  time.Now(),
		},
	})
	if err != nil {
		return "", fmt.Errorf("create tag %s: %w", tagName, err)
	}

	// Update "latest" tag (force by deleting and recreating)
	_ = repo.DeleteTag("latest")
	_, err = repo.CreateTag("latest", commitHash, &git.CreateTagOptions{
		Message: fmt.Sprintf("Latest: v%s", opts.Version),
		Tagger: &object.Signature{
			Name:  opts.Author,
			Email: opts.Email,
			When:  time.Now(),
		},
	})
	if err != nil {
		return "", fmt.Errorf("create latest tag: %w", err)
	}

	// Create extra tags
	for _, tag := range opts.Tags {
		_ = repo.DeleteTag(tag)
		repo.CreateTag(tag, commitHash, &git.CreateTagOptions{
			Message: tag,
			Tagger: &object.Signature{
				Name:  opts.Author,
				Email: opts.Email,
				When:  time.Now(),
			},
		})
	}

	return commitHash.String(), nil
}

// Archive generates a zip archive for a version, streaming output.
func (g *GitStore) Archive(owner, slug, version string) (io.ReadCloser, error) {
	repoPath := g.RepoPath(owner, slug)
	tagName := "v" + version

	cmd := exec.Command("git", "archive", "--format=zip",
		fmt.Sprintf("--prefix=%s-%s/", slug, version), tagName)
	cmd.Dir = repoPath

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("git archive stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("git archive start: %w", err)
	}

	// Return a reader that waits for the command to finish on Close
	return &cmdReadCloser{ReadCloser: stdout, cmd: cmd}, nil
}

type cmdReadCloser struct {
	io.ReadCloser
	cmd *exec.Cmd
}

func (c *cmdReadCloser) Close() error {
	c.ReadCloser.Close()
	return c.cmd.Wait()
}

// GetFile reads a single file from a specific version.
func (g *GitStore) GetFile(owner, slug, version, path string) ([]byte, error) {
	repoPath := g.RepoPath(owner, slug)
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return nil, fmt.Errorf("open repo: %w", err)
	}

	tagName := "v" + version
	ref, err := repo.Tag(tagName)
	if err != nil {
		return nil, fmt.Errorf("get tag %s: %w", tagName, err)
	}

	tagObj, err := repo.TagObject(ref.Hash())
	if err != nil {
		return nil, fmt.Errorf("get tag object: %w", err)
	}

	commit, err := tagObj.Commit()
	if err != nil {
		return nil, fmt.Errorf("get commit from tag: %w", err)
	}

	tree, err := commit.Tree()
	if err != nil {
		return nil, fmt.Errorf("get tree: %w", err)
	}

	file, err := tree.File(path)
	if err != nil {
		return nil, fmt.Errorf("file not found: %s: %w", path, err)
	}

	content, err := file.Contents()
	if err != nil {
		return nil, fmt.Errorf("read file content: %w", err)
	}

	return []byte(content), nil
}

// ListTags returns all version tags sorted by semver descending.
func (g *GitStore) ListTags(owner, slug string) ([]string, error) {
	repoPath := g.RepoPath(owner, slug)
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return nil, fmt.Errorf("open repo: %w", err)
	}

	tags, err := repo.Tags()
	if err != nil {
		return nil, fmt.Errorf("list tags: %w", err)
	}

	var versions []string
	err = tags.ForEach(func(ref *plumbing.Reference) error {
		name := ref.Name().Short()
		if strings.HasPrefix(name, "v") && name != "latest" {
			versions = append(versions, strings.TrimPrefix(name, "v"))
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	// Sort descending by semver
	sort.Slice(versions, func(i, j int) bool {
		return compareSemver(versions[i], versions[j]) > 0
	})
	return versions, nil
}

// Rename renames a repo and creates a symlink from old to new.
func (g *GitStore) Rename(owner, oldSlug, newSlug string) error {
	oldPath := g.RepoPath(owner, oldSlug)
	newPath := g.RepoPath(owner, newSlug)

	if err := os.Rename(oldPath, newPath); err != nil {
		return fmt.Errorf("rename repo: %w", err)
	}

	// Create symlink for backward compatibility
	if err := os.Symlink(newPath, oldPath); err != nil {
		return fmt.Errorf("create symlink: %w", err)
	}
	return nil
}

// Exists checks if a repo exists.
func (g *GitStore) Exists(owner, slug string) bool {
	_, err := os.Stat(g.RepoPath(owner, slug))
	return err == nil
}

// Mirror clones a bare repo for backup.
func (g *GitStore) Mirror(owner, slug, destPath string) error {
	repoPath := g.RepoPath(owner, slug)
	cmd := exec.Command("git", "clone", "--mirror", repoPath, destPath)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git clone --mirror: %s: %w", stderr.String(), err)
	}
	return nil
}

// BasePath returns the base path of the git store.
func (g *GitStore) BasePath() string {
	return g.basePath
}

// compareSemver compares two semver strings. Returns -1, 0, or 1.
func compareSemver(a, b string) int {
	ap := parseSemverParts(a)
	bp := parseSemverParts(b)
	for i := 0; i < 3; i++ {
		if ap[i] < bp[i] {
			return -1
		}
		if ap[i] > bp[i] {
			return 1
		}
	}
	return 0
}

func parseSemverParts(v string) [3]int {
	if idx := strings.IndexAny(v, "-+"); idx != -1 {
		v = v[:idx]
	}
	parts := strings.SplitN(v, ".", 3)
	var result [3]int
	for i := 0; i < 3 && i < len(parts); i++ {
		result[i], _ = strconv.Atoi(parts[i])
	}
	return result
}
