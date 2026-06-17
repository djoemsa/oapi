package tests

import (
	"bytes"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"oapi/internal/config"
	"oapi/internal/registry"
	"oapi/internal/testutil"
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

func TestStateLoadSave(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "oapi-test-state-*")
	if err != nil {
		t.Fatalf("failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "config.yaml")
	sm := config.NewStateManager(configPath)

	// Load non-existent state (should succeed and initialize defaults)
	err = sm.LoadState()
	if err != nil {
		t.Fatalf("expected load of non-existent state to succeed, got error: %v", err)
	}

	stateCopy := sm.GetState()
	if len(stateCopy.Keys) != 0 {
		t.Errorf("expected empty key state, got %d keys", len(stateCopy.Keys))
	}

	// Update state
	sm.UpdateState(func(s *config.RuntimeState) {
		s.TotalRequestsToday = 42
		s.Keys["groq_1"] = config.KeyState{
			RequestsThisMinute: 5,
			RequestsToday:      10,
		}
	})

	// Save state
	err = sm.SaveState()
	if err != nil {
		t.Fatalf("failed to save state: %v", err)
	}

	// Create a new manager and load state back
	sm2 := config.NewStateManager(configPath)
	err = sm2.LoadState()
	if err != nil {
		t.Fatalf("failed to load state back: %v", err)
	}

	loadedState := sm2.GetState()
	if loadedState.TotalRequestsToday != 42 {
		t.Errorf("expected 42 requests, got %d", loadedState.TotalRequestsToday)
	}

	keyState, exists := loadedState.Keys["groq_1"]
	if !exists || keyState.RequestsThisMinute != 5 || keyState.RequestsToday != 10 {
		t.Errorf("unexpected loaded key state: exists=%t, %+v", exists, keyState)
	}
}

type mockHTTPClient struct {
	doFn func(req *http.Request) (*http.Response, error)
}

func (m *mockHTTPClient) Do(req *http.Request) (*http.Response, error) {
	return m.doFn(req)
}

func TestProbeKey(t *testing.T) {
	key := config.KeyConfig{
		ID:       "test-key",
		Provider: "groq",
		Model:    "llama-3.1-8b-instant",
		APIKey:   "gsk_test",
	}

	// Test 200 OK
	client200 := &mockHTTPClient{
		doFn: func(req *http.Request) (*http.Response, error) {
			if req.URL.String() != "https://api.groq.com/openai/v1/chat/completions" {
				t.Errorf("unexpected URL: %s", req.URL.String())
			}
			if req.Header.Get("Authorization") != "Bearer gsk_test" {
				t.Errorf("unexpected Authorization header: %s", req.Header.Get("Authorization"))
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewBufferString(`{}`)),
			}, nil
		},
	}
	status, err := testutil.ProbeKey(client200, key)
	if err != nil || status != "active" {
		t.Errorf("expected active with no error, got status %s, error %v", status, err)
	}

	// Test 401 Unauthorized
	client401 := &mockHTTPClient{
		doFn: func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusUnauthorized,
				Body:       io.NopCloser(bytes.NewBufferString(`{}`)),
			}, nil
		},
	}
	status, err = testutil.ProbeKey(client401, key)
	if err == nil || status != "error" {
		t.Errorf("expected error status and non-nil error, got status %s, error %v", status, err)
	}

	// Test 429 Too Many Requests
	client429 := &mockHTTPClient{
		doFn: func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusTooManyRequests,
				Body:       io.NopCloser(bytes.NewBufferString(`{}`)),
			}, nil
		},
	}
	status, err = testutil.ProbeKey(client429, key)
	if err == nil || status != "cooling_rpm" {
		t.Errorf("expected cooling_rpm status and non-nil error, got status %s, error %v", status, err)
	}

	// Test Google param rewrite
	googleKey := config.KeyConfig{
		ID:       "google-key",
		Provider: "google",
		Model:    "gemini-2.5-flash",
		APIKey:   "g_test",
	}
	clientGoogle := &mockHTTPClient{
		doFn: func(req *http.Request) (*http.Response, error) {
			expectedURL := "https://generativelanguage.googleapis.com/v1beta/openai/chat/completions"
			if req.URL.String() != expectedURL {
				t.Errorf("expected URL %s, got %s", expectedURL, req.URL.String())
			}
			expectedAuth := "Bearer g_test"
			if req.Header.Get("Authorization") != expectedAuth {
				t.Errorf("expected Authorization header '%s' for Google, got '%s'", expectedAuth, req.Header.Get("Authorization"))
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewBufferString(`{}`)),
			}, nil
		},
	}
	status, err = testutil.ProbeKey(clientGoogle, googleKey)
	if err != nil || status != "active" {
		t.Errorf("expected active with no error for Google, got status %s, error %v", status, err)
	}
}
