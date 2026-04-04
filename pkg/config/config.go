package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server    ServerConfig    `yaml:"server"`
	Database  DatabaseConfig  `yaml:"database"`
	Search    SearchConfig    `yaml:"search"`
	GitStore  GitStoreConfig  `yaml:"gitstore"`
	RateLimit RateLimitConfig `yaml:"rate_limit"`
	Auth      AuthConfig      `yaml:"auth"`
}

type ServerConfig struct {
	Port    int    `yaml:"port"`
	Host    string `yaml:"host"`
	BaseURL string `yaml:"base_url"`
}

type DatabaseConfig struct {
	Driver       string `yaml:"driver"` // "sqlite" (default) or "postgres"
	URL          string `yaml:"url"`    // SQLite file path or PostgreSQL DSN
	MaxOpenConns int    `yaml:"max_open_conns"`
	MaxIdleConns int    `yaml:"max_idle_conns"`
}

type SearchConfig struct {
	IndexPath string `yaml:"index_path"` // Bleve index directory
}

type GitStoreConfig struct {
	BasePath string       `yaml:"base_path"`
	Mirror   MirrorConfig `yaml:"mirror"`
	Import   ImportConfig `yaml:"import"`
}

type MirrorConfig struct {
	Enabled       bool   `yaml:"enabled"`
	Provider      string `yaml:"provider"`
	BaseURL       string `yaml:"base_url"`
	Org           string `yaml:"org"`
	TokenEnv      string `yaml:"token_env"`
	AutoPush      bool   `yaml:"auto_push"`
	PushOnStartup bool   `yaml:"push_on_startup"`
}

type ImportConfig struct {
	Enabled          bool     `yaml:"enabled"`
	WebhookSecretEnv string   `yaml:"webhook_secret_env"`
	AllowedOrigins   []string `yaml:"allowed_origins"`
	AutoPublish      bool     `yaml:"auto_publish"`
	RequireSkillMD   bool     `yaml:"require_skill_md"`
}

type RateLimitConfig struct {
	ReadLimit      int `yaml:"read_limit"`
	ReadWindow     int `yaml:"read_window"`
	WriteLimit     int `yaml:"write_limit"`
	WriteWindow    int `yaml:"write_window"`
	DownloadLimit  int `yaml:"download_limit"`
	DownloadWindow int `yaml:"download_window"`
}

type AuthConfig struct {
	TokenPrefix string `yaml:"token_prefix"`
}

func DefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Port:    10070,
			Host:    "0.0.0.0",
			BaseURL: "http://localhost:10070",
		},
		Database: DatabaseConfig{
			Driver:       "sqlite",
			URL:          "./data/skillhub.db",
			MaxOpenConns: 25,
			MaxIdleConns: 5,
		},
		Search: SearchConfig{
			IndexPath: "./data/skills.bleve",
		},
		GitStore: GitStoreConfig{
			BasePath: "./data/repos",
			Mirror: MirrorConfig{
				TokenEnv: "GIT_MIRROR_TOKEN",
				AutoPush: true,
			},
			Import: ImportConfig{
				WebhookSecretEnv: "WEBHOOK_SECRET",
				AllowedOrigins:   []string{"https://github.com", "https://gitlab.com"},
				AutoPublish:      true,
				RequireSkillMD:   true,
			},
		},
		RateLimit: RateLimitConfig{
			ReadLimit:      120,
			ReadWindow:     600,
			WriteLimit:     30,
			WriteWindow:    120,
			DownloadLimit:  20,
			DownloadWindow: 120,
		},
		Auth: AuthConfig{
			TokenPrefix: "clh_",
		},
	}
}

func Load(configPath string) (*Config, error) {
	cfg := DefaultConfig()

	if configPath != "" {
		data, err := os.ReadFile(configPath)
		if err != nil {
			if !os.IsNotExist(err) {
				return nil, fmt.Errorf("read config: %w", err)
			}
		} else {
			if err := yaml.Unmarshal(data, cfg); err != nil {
				return nil, fmt.Errorf("parse config: %w", err)
			}
		}
	}

	applyEnvOverrides(cfg)
	return cfg, nil
}

func applyEnvOverrides(cfg *Config) {
	if v := os.Getenv("SKILLHUB_PORT"); v != "" {
		if port, err := strconv.Atoi(v); err == nil {
			cfg.Server.Port = port
		}
	}
	if v := os.Getenv("SKILLHUB_HOST"); v != "" {
		cfg.Server.Host = v
	}
	if v := os.Getenv("SKILLHUB_BASE_URL"); v != "" {
		cfg.Server.BaseURL = strings.TrimRight(v, "/")
	}
	if v := os.Getenv("SKILLHUB_DB_DRIVER"); v != "" {
		cfg.Database.Driver = v
	}
	if v := os.Getenv("SKILLHUB_DATABASE_URL"); v != "" {
		cfg.Database.URL = v
	}
	if v := os.Getenv("SKILLHUB_SEARCH_PATH"); v != "" {
		cfg.Search.IndexPath = v
	}
	if v := os.Getenv("SKILLHUB_GIT_PATH"); v != "" {
		cfg.GitStore.BasePath = v
	}
}
