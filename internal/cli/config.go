package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type CLIConfig struct {
	Registry  string `yaml:"registry"`
	Token     string `yaml:"token"`
	SkillsDir string `yaml:"skills_dir,omitempty"`
}

func configDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: cannot determine home directory: %v\n", err)
		os.Exit(1)
	}
	return filepath.Join(home, ".skillhub")
}

func configPath() string {
	return filepath.Join(configDir(), "config.yaml")
}

// SkillsDir returns the directory where skills are installed.
// Reads from the loaded config; falls back to ~/.skillhub/skills.
func SkillsDir(cfg *CLIConfig) string {
	if cfg != nil && cfg.SkillsDir != "" {
		// Expand ~ if present
		dir := cfg.SkillsDir
		if len(dir) > 1 && dir[:2] == "~/" {
			home, _ := os.UserHomeDir()
			dir = filepath.Join(home, dir[2:])
		}
		return dir
	}
	return filepath.Join(configDir(), "skills")
}

func LoadConfig() (*CLIConfig, error) {
	data, err := os.ReadFile(configPath())
	if err != nil {
		if os.IsNotExist(err) {
			return &CLIConfig{Registry: "http://localhost:10070"}, nil
		}
		return nil, fmt.Errorf("reading config: %w", err)
	}

	var cfg CLIConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}
	if cfg.Registry == "" {
		cfg.Registry = "http://localhost:10070"
	}
	return &cfg, nil
}

func SaveConfig(cfg *CLIConfig) error {
	dir := configDir()
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}

	if err := os.WriteFile(configPath(), data, 0600); err != nil {
		return fmt.Errorf("writing config: %w", err)
	}
	return nil
}
