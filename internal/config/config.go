package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type ServerConfig struct {
	Port        int    `yaml:"port"`
	Host        string `yaml:"host"`
	DummyAPIKey string `yaml:"dummy_api_key"`
}

type KeyConfig struct {
	ID                  string `yaml:"id"`
	Provider            string `yaml:"provider"`
	Model               string `yaml:"model"`
	APIKey              string `yaml:"api_key"`
	RPMLimit            int    `yaml:"rpm_limit,omitempty"`
	RPDLimit            int    `yaml:"rpd_limit,omitempty"`
	TPMLimit            int    `yaml:"tpm_limit,omitempty"`
	MaxCompletionTokens int    `yaml:"max_completion_tokens,omitempty"`
	Status              string `yaml:"status"`
}

type SlotConfig struct {
	Provider string `yaml:"provider"`
	Model    string `yaml:"model"`
}

type RouteConfig struct {
	Name       string       `yaml:"name"`
	ModelAlias string       `yaml:"model_alias"`
	Chain      []SlotConfig `yaml:"chain"`
	Fallback   []SlotConfig `yaml:"fallback,omitempty"`
}

type ProviderOverride struct {
	ResetBehavior string `yaml:"reset_behavior"`
}

type Config struct {
	Server    ServerConfig                `yaml:"server"`
	Keys      []KeyConfig                 `yaml:"keys"`
	Routes    []RouteConfig               `yaml:"routes"`
	Providers map[string]ProviderOverride `yaml:"providers,omitempty"`
}

// DefaultConfig returns the default configuration values.
func DefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Port:        8080,
			Host:        "127.0.0.1",
			DummyAPIKey: "",
		},
		Keys:   []KeyConfig{},
		Routes: []RouteConfig{},
		Providers: map[string]ProviderOverride{
			"groq":     {ResetBehavior: "rolling_24h"},
			"google":   {ResetBehavior: "midnight_pt"},
			"cerebras": {ResetBehavior: "continuous"},
			"mistral":  {ResetBehavior: "none"},
		},
	}
}

// ResolveConfigPath checks configuration paths in order of precedence:
// 1. flagPath (if provided)
// 2. ./oapi.yaml
// 3. ~/.config/oapi/config.yaml
func ResolveConfigPath(flagPath string) (string, error) {
	if flagPath != "" {
		return flagPath, nil
	}

	// Check current directory
	if _, err := os.Stat("oapi.yaml"); err == nil {
		return "oapi.yaml", nil
	}

	// Fallback to ~/.config/oapi/config.yaml
	home, err := os.UserHomeDir()
	if err != nil {
		// If we can't resolve home, fallback to current directory
		return "oapi.yaml", nil
	}

	return filepath.Join(home, ".config", "oapi", "config.yaml"), nil
}

// LoadConfig reads and parses the configuration file at the given path.
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	cfg := DefaultConfig()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config YAML: %w", err)
	}

	return cfg, nil
}

// SaveConfig writes the configuration to the target path atomically.
func SaveConfig(path string, cfg *Config) error {
	// Ensure parent directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal config to YAML: %w", err)
	}

	// Write to temporary file first
	tmpFile := path + ".tmp"
	if err := os.WriteFile(tmpFile, data, 0600); err != nil {
		return fmt.Errorf("failed to write temporary config file: %w", err)
	}

	// Rename temporary file to target path atomically
	if err := os.Rename(tmpFile, path); err != nil {
		// Cleanup the temporary file on error
		_ = os.Remove(tmpFile)
		return fmt.Errorf("failed to atomically rename config file: %w", err)
	}

	return nil
}
