package tests

import (
	"os"
	"path/filepath"
	"testing"

	"oapi/internal/config"
	"oapi/internal/registry"
)

func TestProviderRegistry(t *testing.T) {
	// Verify groq provider registry defaults
	groq, exists := registry.Providers["groq"]
	if !exists {
		t.Fatal("expected groq provider to exist in registry")
	}
	if groq.DefaultRPM != 30 || groq.DefaultRPD != 14400 {
		t.Errorf("unexpected groq defaults: RPM %d, RPD %d", groq.DefaultRPM, groq.DefaultRPD)
	}

	// Verify Groq model overrides
	override, exists := registry.GroqModelOverrides["llama-3.3-70b-versatile"]
	if !exists {
		t.Fatal("expected llama-3.3-70b-versatile override to exist")
	}
	if override.RPD != 1000 || override.TPM != 12000 {
		t.Errorf("unexpected model override values: RPD %d, TPM %d", override.RPD, override.TPM)
	}
}

func TestConfigLoadSave(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "oapi-test-*")
	if err != nil {
		t.Fatalf("failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "config.yaml")

	// 1. Load config for non-existent file (should fail)
	_, err = config.LoadConfig(configPath)
	if err == nil {
		t.Fatal("expected error loading non-existent config path, got nil")
	}

	// 2. Save a default config
	cfg := config.DefaultConfig()
	cfg.Server.Port = 9090
	cfg.Server.Host = "0.0.0.0"
	cfg.Keys = []config.KeyConfig{
		{
			ID:       "test-key",
			Provider: "google",
			Model:    "gemini-2.5-flash",
			APIKey:   "secret-key",
			Status:   "active",
		},
	}

	err = config.SaveConfig(configPath, cfg)
	if err != nil {
		t.Fatalf("failed to save config: %v", err)
	}

	// 3. Load it back and verify values
	loaded, err := config.LoadConfig(configPath)
	if err != nil {
		t.Fatalf("failed to load saved config: %v", err)
	}

	if loaded.Server.Port != 9090 || loaded.Server.Host != "0.0.0.0" {
		t.Errorf("unexpected server settings: port %d, host %s", loaded.Server.Port, loaded.Server.Host)
	}

	if len(loaded.Keys) != 1 || loaded.Keys[0].ID != "test-key" || loaded.Keys[0].APIKey != "secret-key" {
		t.Errorf("unexpected keys: %v", loaded.Keys)
	}
}
