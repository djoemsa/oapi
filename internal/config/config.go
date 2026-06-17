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

// GetGlobalConfigPath returns the path to the user's global config file.
func GetGlobalConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
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

	// If using the local repository config file, always load keys, routes, and providers
	// from the user's global config.yaml
	if path == "oapi.yaml" {
		globalPath, err := GetGlobalConfigPath()
		if err == nil {
			if _, statErr := os.Stat(globalPath); statErr == nil {
				if globalData, readErr := os.ReadFile(globalPath); readErr == nil {
					globalCfg := &Config{}
					if unmarshalErr := yaml.Unmarshal(globalData, globalCfg); unmarshalErr == nil {
						cfg.Keys = globalCfg.Keys
						cfg.Routes = globalCfg.Routes
						cfg.Providers = globalCfg.Providers
					}
				}
			}
		}
	}

	return cfg, nil
}

// SaveConfig writes the configuration to the target path atomically.
func SaveConfig(path string, cfg *Config) error {
	// If it is the local repository configuration, split the saves:
	// Server config goes to local oapi.yaml; keys/routes/providers go to user's global config.yaml
	if path == "oapi.yaml" {
		// 1. Save local server configuration to ./oapi.yaml
		localCfg := &Config{
			Server: cfg.Server,
		}
		localData, err := yaml.Marshal(localCfg)
		if err != nil {
			return fmt.Errorf("failed to marshal local config: %w", err)
		}
		if err := os.WriteFile("oapi.yaml", localData, 0644); err != nil {
			return fmt.Errorf("failed to write local config: %w", err)
		}

		// 2. Save keys, routes, and providers to global config.yaml
		globalPath, err := GetGlobalConfigPath()
		if err != nil {
			return fmt.Errorf("failed to resolve global config path: %w", err)
		}
		globalCfg := &Config{
			Keys:      cfg.Keys,
			Routes:    cfg.Routes,
			Providers: cfg.Providers,
		}
		// Keep server block valid on global config
		globalCfg.Server.Port = 8080
		globalCfg.Server.Host = "127.0.0.1"

		dir := filepath.Dir(globalPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create global config directory: %w", err)
		}

		globalData, err := yaml.Marshal(globalCfg)
		if err != nil {
			return fmt.Errorf("failed to marshal global config: %w", err)
		}

		tmpFile := globalPath + ".tmp"
		if err := os.WriteFile(tmpFile, globalData, 0600); err != nil {
			return fmt.Errorf("failed to write temporary global config: %w", err)
		}
		if err := os.Rename(tmpFile, globalPath); err != nil {
			_ = os.Remove(tmpFile)
			return fmt.Errorf("failed to atomically save global config: %w", err)
		}

		return nil
	}

	// Normal save for other paths
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
		_ = os.Remove(tmpFile)
		return fmt.Errorf("failed to atomically rename config file: %w", err)
	}

	return nil
}

// MaskAPIKey masks an API key as 'sk-...xxxx'.
func MaskAPIKey(key string) string {
	if len(key) < 8 {
		return "sk-...xxxx"
	}
	return "sk-..." + key[len(key)-4:]
}
