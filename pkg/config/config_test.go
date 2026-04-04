package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Server.Port != 10070 {
		t.Errorf("Server.Port = %d, want 10070", cfg.Server.Port)
	}
	if cfg.Server.Host != "0.0.0.0" {
		t.Errorf("Server.Host = %q, want %q", cfg.Server.Host, "0.0.0.0")
	}
	if cfg.Database.Driver != "sqlite" {
		t.Errorf("Database.Driver = %q, want %q", cfg.Database.Driver, "sqlite")
	}
	if cfg.Database.MaxOpenConns != 25 {
		t.Errorf("Database.MaxOpenConns = %d, want 25", cfg.Database.MaxOpenConns)
	}
	if cfg.Search.IndexPath != "./data/skills.bleve" {
		t.Errorf("Search.IndexPath = %q, want %q", cfg.Search.IndexPath, "./data/skills.bleve")
	}
	if cfg.RateLimit.ReadLimit != 120 {
		t.Errorf("RateLimit.ReadLimit = %d, want 120", cfg.RateLimit.ReadLimit)
	}
	if cfg.Auth.TokenPrefix != "clh_" {
		t.Errorf("Auth.TokenPrefix = %q, want %q", cfg.Auth.TokenPrefix, "clh_")
	}
}

func TestLoadNonExistentFile(t *testing.T) {
	cfg, err := Load("/nonexistent/path.yaml")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	// Should return defaults
	if cfg.Server.Port != 10070 {
		t.Errorf("Server.Port = %d, want 10070 (default)", cfg.Server.Port)
	}
}

func TestLoadValidYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	data := []byte(`
server:
  port: 9090
  host: "127.0.0.1"
  base_url: "https://example.com"
`)
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Server.Port != 9090 {
		t.Errorf("Server.Port = %d, want 9090", cfg.Server.Port)
	}
	if cfg.Server.Host != "127.0.0.1" {
		t.Errorf("Server.Host = %q, want %q", cfg.Server.Host, "127.0.0.1")
	}
	// Defaults should still apply for unset fields
	if cfg.Database.MaxOpenConns != 25 {
		t.Errorf("Database.MaxOpenConns = %d, want 25 (default)", cfg.Database.MaxOpenConns)
	}
}

func TestLoadInvalidYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.yaml")
	if err := os.WriteFile(path, []byte("{{invalid yaml"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(path)
	if err == nil {
		t.Error("Load() expected error for invalid YAML, got nil")
	}
}

func TestEnvOverrides(t *testing.T) {
	t.Setenv("SKILLHUB_PORT", "3000")
	t.Setenv("SKILLHUB_HOST", "localhost")
	t.Setenv("SKILLHUB_BASE_URL", "https://test.com/")
	t.Setenv("SKILLHUB_DB_DRIVER", "postgres")
	t.Setenv("SKILLHUB_DATABASE_URL", "postgres://test:test@db:5432/test")
	t.Setenv("SKILLHUB_SEARCH_PATH", "/tmp/search.bleve")
	t.Setenv("SKILLHUB_GIT_PATH", "/tmp/repos")

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Server.Port != 3000 {
		t.Errorf("Server.Port = %d, want 3000", cfg.Server.Port)
	}
	if cfg.Server.Host != "localhost" {
		t.Errorf("Server.Host = %q, want %q", cfg.Server.Host, "localhost")
	}
	// BaseURL should have trailing slash trimmed
	if cfg.Server.BaseURL != "https://test.com" {
		t.Errorf("Server.BaseURL = %q, want %q", cfg.Server.BaseURL, "https://test.com")
	}
	if cfg.Database.Driver != "postgres" {
		t.Errorf("Database.Driver = %q, want %q", cfg.Database.Driver, "postgres")
	}
	if cfg.Database.URL != "postgres://test:test@db:5432/test" {
		t.Errorf("Database.URL = %q", cfg.Database.URL)
	}
	if cfg.Search.IndexPath != "/tmp/search.bleve" {
		t.Errorf("Search.IndexPath = %q", cfg.Search.IndexPath)
	}
	if cfg.GitStore.BasePath != "/tmp/repos" {
		t.Errorf("GitStore.BasePath = %q", cfg.GitStore.BasePath)
	}
}

func TestEnvOverrideInvalidPort(t *testing.T) {
	t.Setenv("SKILLHUB_PORT", "not-a-number")

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	// Invalid port should keep default
	if cfg.Server.Port != 10070 {
		t.Errorf("Server.Port = %d, want 10070 (default)", cfg.Server.Port)
	}
}
