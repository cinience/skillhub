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
	Store     StoreConfig     `yaml:"store"`
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

type StoreConfig struct {
	Backend string          `yaml:"backend"` // "git" (default) | "s3" | "oss"
	S3      StoreS3Config   `yaml:"s3"`
	OSS     StoreOSSConfig  `yaml:"oss"`
}

type StoreS3Config struct {
	Bucket    string `yaml:"bucket"`
	Region    string `yaml:"region"`
	Prefix    string `yaml:"prefix"`     // object key prefix, default "skills"
	Endpoint  string `yaml:"endpoint"`   // custom endpoint (MinIO, etc.)
	AccessKey string `yaml:"access_key"` // optional, defaults to IAM
	SecretKey string `yaml:"secret_key"`
}

type StoreOSSConfig struct {
	Bucket    string `yaml:"bucket"`
	Region    string `yaml:"region"`   // e.g. "cn-hangzhou"
	Prefix    string `yaml:"prefix"`   // object key prefix, default "skills"
	Endpoint  string `yaml:"endpoint"` // e.g. "oss-cn-hangzhou.aliyuncs.com"
	AccessKey string `yaml:"access_key"`
	SecretKey string `yaml:"secret_key"`
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
	TokenPrefix string              `yaml:"token_prefix"`
	OAuth       map[string]OAuthProviderConfig `yaml:"oauth"`
}

type OAuthProviderConfig struct {
	ClientID     string `yaml:"client_id"`
	ClientSecret string `yaml:"client_secret"`
	BaseURL      string `yaml:"base_url"` // for self-hosted GitLab etc.
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

	// Store backend env overrides
	if v := os.Getenv("SKILLHUB_STORE_BACKEND"); v != "" {
		cfg.Store.Backend = v
	}
	if v := os.Getenv("SKILLHUB_S3_BUCKET"); v != "" {
		cfg.Store.S3.Bucket = v
	}
	if v := os.Getenv("SKILLHUB_S3_REGION"); v != "" {
		cfg.Store.S3.Region = v
	}
	if v := os.Getenv("SKILLHUB_S3_PREFIX"); v != "" {
		cfg.Store.S3.Prefix = v
	}
	if v := os.Getenv("SKILLHUB_S3_ENDPOINT"); v != "" {
		cfg.Store.S3.Endpoint = v
	}
	if v := os.Getenv("SKILLHUB_S3_ACCESS_KEY"); v != "" {
		cfg.Store.S3.AccessKey = v
	}
	if v := os.Getenv("SKILLHUB_S3_SECRET_KEY"); v != "" {
		cfg.Store.S3.SecretKey = v
	}
	if v := os.Getenv("SKILLHUB_OSS_BUCKET"); v != "" {
		cfg.Store.OSS.Bucket = v
	}
	if v := os.Getenv("SKILLHUB_OSS_REGION"); v != "" {
		cfg.Store.OSS.Region = v
	}
	if v := os.Getenv("SKILLHUB_OSS_PREFIX"); v != "" {
		cfg.Store.OSS.Prefix = v
	}
	if v := os.Getenv("SKILLHUB_OSS_ENDPOINT"); v != "" {
		cfg.Store.OSS.Endpoint = v
	}
	if v := os.Getenv("SKILLHUB_OSS_ACCESS_KEY"); v != "" {
		cfg.Store.OSS.AccessKey = v
	}
	if v := os.Getenv("SKILLHUB_OSS_SECRET_KEY"); v != "" {
		cfg.Store.OSS.SecretKey = v
	}

	// OAuth env overrides
	if cfg.Auth.OAuth == nil {
		cfg.Auth.OAuth = make(map[string]OAuthProviderConfig)
	}
	for _, provider := range []string{"github", "gitlab"} {
		prefix := "SKILLHUB_OAUTH_" + strings.ToUpper(provider) + "_"
		clientID := os.Getenv(prefix + "CLIENT_ID")
		clientSecret := os.Getenv(prefix + "CLIENT_SECRET")
		if clientID != "" && clientSecret != "" {
			p := cfg.Auth.OAuth[provider]
			p.ClientID = clientID
			p.ClientSecret = clientSecret
			if baseURL := os.Getenv(prefix + "BASE_URL"); baseURL != "" {
				p.BaseURL = baseURL
			}
			cfg.Auth.OAuth[provider] = p
		}
	}
}
