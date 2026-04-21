package store

import (
	"context"
	"io"
	"path"
	"strings"
)

// Store is the abstraction for skill file storage backends (git, S3, OSS, etc.).
type Store interface {
	// Publish writes files and creates a version. Returns a version identifier
	// (e.g. git commit hash, S3 ETag).
	Publish(ctx context.Context, opts PublishOpts) (string, error)

	// Archive returns a zip archive stream for the given version.
	Archive(owner, slug, version string) (io.ReadCloser, error)

	// GetFile reads a single file from a specific version.
	GetFile(owner, slug, version, path string) ([]byte, error)

	// ListVersions returns all version strings sorted descending.
	ListVersions(owner, slug string) ([]string, error)

	// Exists checks whether storage for the given skill exists.
	Exists(owner, slug string) bool

	// Rename renames skill storage from oldSlug to newSlug.
	Rename(owner, oldSlug, newSlug string) error
}

// sanitizeStorePath cleans a file path to prevent directory traversal in storage keys.
func sanitizeStorePath(p string) string {
	p = path.Clean(p)
	p = strings.TrimPrefix(p, "/")
	// Reject any remaining traversal
	if p == ".." || strings.HasPrefix(p, "../") || strings.Contains(p, "/../") {
		return "invalid"
	}
	return p
}

// PublishOpts contains backend-agnostic parameters for publishing a skill version.
type PublishOpts struct {
	Owner   string
	Slug    string
	Version string            // e.g. "1.0.0"
	Files   map[string][]byte // path → content
	Author  string
	Email   string
	Message string   // changelog / commit message
	Tags    []string // extra tags
}
