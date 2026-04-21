package store

import (
	"context"
	"io"

	"github.com/cinience/skillhub/pkg/gitstore"
)

// GitBackend wraps *gitstore.GitStore to implement the Store interface.
type GitBackend struct {
	gs *gitstore.GitStore
}

func NewGitBackend(gs *gitstore.GitStore) *GitBackend {
	return &GitBackend{gs: gs}
}

func (g *GitBackend) Publish(ctx context.Context, opts PublishOpts) (string, error) {
	return g.gs.Publish(ctx, gitstore.PublishOpts{
		Owner:   opts.Owner,
		Slug:    opts.Slug,
		Version: opts.Version,
		Files:   opts.Files,
		Author:  opts.Author,
		Email:   opts.Email,
		Message: opts.Message,
		Tags:    opts.Tags,
	})
}

func (g *GitBackend) Archive(owner, slug, version string) (io.ReadCloser, error) {
	return g.gs.Archive(owner, slug, version)
}

func (g *GitBackend) GetFile(owner, slug, version, path string) ([]byte, error) {
	return g.gs.GetFile(owner, slug, version, path)
}

func (g *GitBackend) ListVersions(owner, slug string) ([]string, error) {
	return g.gs.ListTags(owner, slug)
}

func (g *GitBackend) Exists(owner, slug string) bool {
	return g.gs.Exists(owner, slug)
}

func (g *GitBackend) Rename(owner, oldSlug, newSlug string) error {
	return g.gs.Rename(owner, oldSlug, newSlug)
}
