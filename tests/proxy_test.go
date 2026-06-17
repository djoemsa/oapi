package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"oapi/internal/config"
	"oapi/internal/proxy"
	"oapi/internal/registry"
	"oapi/internal/rotation"
)

func TestRewriteRequest_Standard(t *testing.T) {
	reqBody := `{"model": "alias-model", "messages": [{"role": "user", "content": "hi"}]}`
	req := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewBufferString(reqBody))
	req.Header.Set("Authorization", "Bearer dummy-auth")
	req.Header.Set("Content-Type", "application/json")

	key := config.KeyConfig{
		ID:       "groq-key-1",
		Provider: "groq",
		Model:    "llama-3.1-8b-instant",
		APIKey:   "gsk_secret",
		Status:   "active",
	}

	rewritten, err := proxy.RewriteRequest(req, key)
	if err != nil {
		t.Fatalf("RewriteRequest failed: %v", err)
	}

	// Verify headers
	if rewritten.Header.Get("Authorization") != "Bearer gsk_secret" {
		t.Errorf("expected Authorization Bearer gsk_secret, got %s", rewritten.Header.Get("Authorization"))
	}
	if rewritten.Host != "api.groq.com" {
		t.Errorf("expected Host api.groq.com, got %s", rewritten.Host)
	}
	if rewritten.URL.String() != "https://api.groq.com/openai/v1/chat/completions" {
		t.Errorf("expected URL https://api.groq.com/openai/v1/chat/completions, got %s", rewritten.URL.String())
	}

	// Verify body
	bodyBytes, _ := io.ReadAll(rewritten.Body)
	var bodyMap map[string]interface{}
	json.Unmarshal(bodyBytes, &bodyMap)
	if bodyMap["model"] != "llama-3.1-8b-instant" {
		t.Errorf("expected model llama-3.1-8b-instant, got %v", bodyMap["model"])
	}
}

func TestRewriteRequest_Google(t *testing.T) {
	reqBody := `{"model": "alias-model", "messages": [{"role": "user", "content": "hi"}]}`
	req := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewBufferString(reqBody))
	req.Header.Set("Authorization", "Bearer dummy-auth")

	key := config.KeyConfig{
		ID:       "google-key-1",
		Provider: "google",
		Model:    "gemini-2.5-flash",
		APIKey:   "AIza_secret",
		Status:   "active",
	}

	rewritten, err := proxy.RewriteRequest(req, key)
	if err != nil {
		t.Fatalf("RewriteRequest failed: %v", err)
	}

	// Verify Authorization is stripped
	if val := rewritten.Header.Get("Authorization"); val != "" {
		t.Errorf("expected empty Authorization header, got %s", val)
	}

	// Verify URL query param key is injected
	expectedURL := "https://generativelanguage.googleapis.com/v1beta/openai/chat/completions?key=AIza_secret"
	if rewritten.URL.String() != expectedURL {
		t.Errorf("expected URL %s, got %s", expectedURL, rewritten.URL.String())
	}
}

func TestRewriteRequest_OpenRouter(t *testing.T) {
	reqBody := `{"model": "alias-model", "messages": []}`
	req := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewBufferString(reqBody))

	key := config.KeyConfig{
		ID:       "or-key-1",
		Provider: "openrouter",
		Model:    "gpt-oss",
		APIKey:   "or_secret",
		Status:   "active",
	}

	rewritten, err := proxy.RewriteRequest(req, key)
	if err != nil {
		t.Fatalf("RewriteRequest failed: %v", err)
	}

	if rewritten.Header.Get("HTTP-Referer") != "http://localhost" {
		t.Errorf("expected HTTP-Referer http://localhost, got %s", rewritten.Header.Get("HTTP-Referer"))
	}
	if rewritten.Header.Get("X-Title") != "oapi" {
		t.Errorf("expected X-Title oapi, got %s", rewritten.Header.Get("X-Title"))
	}
}

func TestRewriteRequest_Cerebras(t *testing.T) {
	key := config.KeyConfig{
		ID:                  "cerebras-key-1",
		Provider:            "cerebras",
		Model:               "gpt-oss-120b",
		APIKey:              "csk_secret",
		MaxCompletionTokens: 100,
		Status:              "active",
	}

	// Case 1: no tokens specified
	reqBody := `{"model": "alias-model", "messages": []}`
	req := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewBufferString(reqBody))
	rewritten, err := proxy.RewriteRequest(req, key)
	if err != nil {
		t.Fatalf("RewriteRequest failed: %v", err)
	}

	bodyBytes, _ := io.ReadAll(rewritten.Body)
	var bodyMap map[string]interface{}
	json.Unmarshal(bodyBytes, &bodyMap)

	if bodyMap["reasoning_effort"] != "none" || bodyMap["disable_reasoning"] != true || bodyMap["clear_thinking"] != true {
		t.Errorf("expected Cerebras reasoning suppression, got %+v", bodyMap)
	}
	if bodyMap["max_completion_tokens"] != float64(100) {
		t.Errorf("expected max_completion_tokens 100, got %v", bodyMap["max_completion_tokens"])
	}

	// Case 2: max_completion_tokens specified but over limit
	reqBody2 := `{"model": "alias-model", "max_completion_tokens": 150, "messages": []}`
	req2 := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewBufferString(reqBody2))
	rewritten2, _ := proxy.RewriteRequest(req2, key)
	bodyBytes2, _ := io.ReadAll(rewritten2.Body)
	var bodyMap2 map[string]interface{}
	json.Unmarshal(bodyBytes2, &bodyMap2)

	if bodyMap2["max_completion_tokens"] != float64(100) {
		t.Errorf("expected max_completion_tokens clamped to 100, got %v", bodyMap2["max_completion_tokens"])
	}

	// Case 3: max_tokens specified but under limit
	reqBody3 := `{"model": "alias-model", "max_tokens": 50, "messages": []}`
	req3 := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewBufferString(reqBody3))
	rewritten3, _ := proxy.RewriteRequest(req3, key)
	bodyBytes3, _ := io.ReadAll(rewritten3.Body)
	var bodyMap3 map[string]interface{}
	json.Unmarshal(bodyBytes3, &bodyMap3)

	if bodyMap3["max_tokens"] != float64(50) {
		t.Errorf("expected max_tokens to remain 50, got %v", bodyMap3["max_tokens"])
	}
}

func TestServer_Endpoints(t *testing.T) {
	// Setup config
	tmpDir, err := os.MkdirTemp("", "oapi-test-server-*")
	if err != nil {
		t.Fatalf("failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "config.yaml")
	cfg := config.DefaultConfig()
	cfg.Server.Port = 9091 // Use separate port to avoid conflicts
	cfg.Server.Host = "127.0.0.1"
	cfg.Server.DummyAPIKey = "test-token"
	cfg.Routes = []config.RouteConfig{
		{Name: "route1", ModelAlias: "model-1"},
	}

	if err := config.SaveConfig(configPath, cfg); err != nil {
		t.Fatalf("failed to save config: %v", err)
	}

	stateMgr := config.NewStateManager(configPath)
	if err := stateMgr.LoadState(); err != nil {
		t.Fatalf("failed to load state: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pool := rotation.NewKeyPool(ctx, cfg, configPath, stateMgr)
	engine := rotation.NewRotationEngine(pool)

	srv := proxy.NewServer(cfg, configPath, stateMgr, pool, engine)
	if err := srv.Start(ctx); err != nil {
		t.Fatalf("failed to start server: %v", err)
	}
	defer srv.Stop()

	// Give the server a small moment to start up
	time.Sleep(50 * time.Millisecond)

	// 1. Test health check (no auth required)
	resp, err := http.Get("http://127.0.0.1:9091/health")
	if err != nil {
		t.Fatalf("failed to GET health: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}
	var health map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&health)
	if health["status"] != "healthy" {
		t.Errorf("expected status healthy, got %v", health["status"])
	}

	// 2. Test models list (auth required)
	req, _ := http.NewRequest("GET", "http://127.0.0.1:9091/v1/models", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	resp2, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to GET models: %v", err)
	}
	defer resp2.Body.Close()

	if resp2.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp2.StatusCode)
	}
	var models map[string]interface{}
	json.NewDecoder(resp2.Body).Decode(&models)
	dataList, _ := models["data"].([]interface{})
	if len(dataList) != 1 {
		t.Errorf("expected 1 model, got %d", len(dataList))
	}
}

func TestServer_Completions_Rotation(t *testing.T) {
	// 1. Setup a mock LLM provider server that counts requests
	requestCount := 0
	mockBackend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		if requestCount == 1 {
			// First key: return 429 Too Many Requests
			w.Header().Set("Retry-After", "5")
			w.Header().Set("x-ratelimit-remaining-requests", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte(`{"error": {"message": "Rate limit exceeded"}}`))
			return
		}
		// Second key: return 200 OK
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"choices": [{"message": {"content": "success"}}]}`))
	}))
	defer mockBackend.Close()

	// 2. Setup config
	tmpDir, err := os.MkdirTemp("", "oapi-test-rotation-*")
	if err != nil {
		t.Fatalf("failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "config.yaml")
	cfg := config.DefaultConfig()
	cfg.Server.Port = 9092
	cfg.Server.Host = "127.0.0.1"
	cfg.Server.DummyAPIKey = "" // no auth required for simplicity
	cfg.Keys = []config.KeyConfig{
		{
			ID:       "key-1",
			Provider: "groq",
			Model:    "llama-3.1-8b-instant",
			APIKey:   "key1",
			Status:   "active",
		},
		{
			ID:       "key-2",
			Provider: "groq",
			Model:    "llama-3.1-8b-instant",
			APIKey:   "key2",
			Status:   "active",
		},
	}
	cfg.Routes = []config.RouteConfig{
		{
			Name:       "default",
			ModelAlias: "*",
			Chain: []config.SlotConfig{
				{Provider: "groq", Model: "llama-3.1-8b-instant"},
			},
		},
	}

	// Override registry base URL for groq to point to our mock backend!
	registry.Providers["groq"] = registry.ProviderInfo{
		ID:            "groq",
		BaseURL:       mockBackend.URL,
		DefaultRPM:    30,
		DefaultRPD:    14400,
		ResetBehavior: registry.ResetRolling24h,
	}

	if err := config.SaveConfig(configPath, cfg); err != nil {
		t.Fatalf("failed to save config: %v", err)
	}

	stateMgr := config.NewStateManager(configPath)
	if err := stateMgr.LoadState(); err != nil {
		t.Fatalf("failed to load state: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pool := rotation.NewKeyPool(ctx, cfg, configPath, stateMgr)
	engine := rotation.NewRotationEngine(pool)

	srv := proxy.NewServer(cfg, configPath, stateMgr, pool, engine)
	if err := srv.Start(ctx); err != nil {
		t.Fatalf("failed to start server: %v", err)
	}
	defer srv.Stop()

	time.Sleep(50 * time.Millisecond)

	// Fire completions request
	reqBody := `{"model": "alias-model", "messages": [{"role": "user", "content": "hi"}]}`
	resp, err := http.Post("http://127.0.0.1:9092/v1/chat/completions", "application/json", bytes.NewBufferString(reqBody))
	if err != nil {
		t.Fatalf("failed to send completions request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected status 200, got %d. Body: %s", resp.StatusCode, body)
	}

	var resMap map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&resMap)
	choices := resMap["choices"].([]interface{})
	choice := choices[0].(map[string]interface{})
	msg := choice["message"].(map[string]interface{})
	if msg["content"] != "success" {
		t.Errorf("expected content 'success', got %v", msg["content"])
	}

	// Verify key-1 got marked as cooling_rpd or cooling_rpm
	if pool.GetKeysForProviderAndModel("groq", "llama-3.1-8b-instant")[0].Status != "cooling_rpd" {
		t.Errorf("expected key-1 status to be cooling_rpd, got %s", pool.GetKeysForProviderAndModel("groq", "llama-3.1-8b-instant")[0].Status)
	}
}

func TestServer_Completions_Streaming(t *testing.T) {
	// Setup mock streaming backend
	mockBackend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("data: {\"choices\": [{\"delta\": {\"content\": \"hello\"}}]}\n\ndata: [DONE]\n\n"))
	}))
	defer mockBackend.Close()

	// Setup config
	tmpDir, err := os.MkdirTemp("", "oapi-test-stream-*")
	if err != nil {
		t.Fatalf("failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "config.yaml")
	cfg := config.DefaultConfig()
	cfg.Server.Port = 9093
	cfg.Server.Host = "127.0.0.1"
	cfg.Keys = []config.KeyConfig{
		{
			ID:       "key-stream",
			Provider: "mistral",
			Model:    "mistral-small-2506",
			APIKey:   "key1",
			Status:   "active",
		},
	}
	cfg.Routes = []config.RouteConfig{
		{
			Name:       "default",
			ModelAlias: "*",
			Chain: []config.SlotConfig{
				{Provider: "mistral", Model: "mistral-small-2506"},
			},
		},
	}

	// Override registry base URL for mistral to point to our mock backend!
	registry.Providers["mistral"] = registry.ProviderInfo{
		ID:            "mistral",
		BaseURL:       mockBackend.URL,
		DefaultRPM:    300,
		DefaultRPD:    0,
		ResetBehavior: registry.ResetNone,
	}

	if err := config.SaveConfig(configPath, cfg); err != nil {
		t.Fatalf("failed to save config: %v", err)
	}

	stateMgr := config.NewStateManager(configPath)
	stateMgr.LoadState()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pool := rotation.NewKeyPool(ctx, cfg, configPath, stateMgr)
	engine := rotation.NewRotationEngine(pool)

	srv := proxy.NewServer(cfg, configPath, stateMgr, pool, engine)
	if err := srv.Start(ctx); err != nil {
		t.Fatalf("failed to start server: %v", err)
	}
	defer srv.Stop()

	time.Sleep(50 * time.Millisecond)

	reqBody := `{"model": "alias-model", "stream": true, "messages": []}`
	resp, err := http.Post("http://127.0.0.1:9093/v1/chat/completions", "application/json", bytes.NewBufferString(reqBody))
	if err != nil {
		t.Fatalf("failed to send completions request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	bodyBytes, _ := io.ReadAll(resp.Body)
	if !bytes.Contains(bodyBytes, []byte("hello")) {
		t.Errorf("expected stream data to contain 'hello', got %s", bodyBytes)
	}
}

func TestServer_GracefulShutdown(t *testing.T) {
	// 1. Setup mock backend that introduces artificial delay
	requestReceived := make(chan struct{})
	mockBackend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(requestReceived)
		time.Sleep(200 * time.Millisecond) // artificial delay
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"choices": [{"message": {"content": "shutdown-success"}}]}`))
	}))
	defer mockBackend.Close()

	// Override registry base URL for groq-shutdown provider
	registry.Providers["groq-shutdown"] = registry.ProviderInfo{
		ID:            "groq-shutdown",
		BaseURL:       mockBackend.URL,
		DefaultRPM:    30,
		DefaultRPD:    14400,
		ResetBehavior: registry.ResetRolling24h,
	}

	// 2. Setup config
	tmpDir, err := os.MkdirTemp("", "oapi-test-shutdown-*")
	if err != nil {
		t.Fatalf("failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "config.yaml")
	cfg := config.DefaultConfig()
	cfg.Server.Port = 9094
	cfg.Server.Host = "127.0.0.1"
	cfg.Server.DummyAPIKey = ""
	cfg.Keys = []config.KeyConfig{
		{
			ID:       "key-shutdown-1",
			Provider: "groq-shutdown",
			Model:    "llama-3-shutdown",
			APIKey:   "key-shutdown",
			Status:   "active",
		},
	}
	cfg.Routes = []config.RouteConfig{
		{
			Name:       "default",
			ModelAlias: "*",
			Chain: []config.SlotConfig{
				{Provider: "groq-shutdown", Model: "llama-3-shutdown"},
			},
		},
	}

	if err := config.SaveConfig(configPath, cfg); err != nil {
		t.Fatalf("failed to save config: %v", err)
	}

	stateMgr := config.NewStateManager(configPath)
	if err := stateMgr.LoadState(); err != nil {
		t.Fatalf("failed to load state: %v", err)
	}

	// Create cancelable context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pool := rotation.NewKeyPool(ctx, cfg, configPath, stateMgr)
	engine := rotation.NewRotationEngine(pool)

	srv := proxy.NewServer(cfg, configPath, stateMgr, pool, engine)
	if err := srv.Start(ctx); err != nil {
		t.Fatalf("failed to start server: %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	// Fire completions request in a separate goroutine
	errChan := make(chan error, 1)
	respChan := make(chan *http.Response, 1)

	go func() {
		reqBody := `{"model": "alias-model", "messages": [{"role": "user", "content": "hi"}]}`
		resp, err := http.Post("http://127.0.0.1:9094/v1/chat/completions", "application/json", bytes.NewBufferString(reqBody))
		if err != nil {
			errChan <- err
			return
		}
		respChan <- resp
	}()

	// Wait until the request has reached the backend
	<-requestReceived

	// Cancel the context while request is in-flight to trigger shutdown
	cancel()

	// Wait for the request result
	select {
	case err := <-errChan:
		t.Fatalf("request failed during shutdown: %v", err)
	case resp := <-respChan:
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("expected status 200 during graceful shutdown, got %d. Body: %s", resp.StatusCode, body)
		}
		var resMap map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&resMap)
		choices := resMap["choices"].([]interface{})
		choice := choices[0].(map[string]interface{})
		msg := choice["message"].(map[string]interface{})
		if msg["content"] != "shutdown-success" {
			t.Errorf("expected content 'shutdown-success', got %v", msg["content"])
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timeout waiting for request response during shutdown")
	}

	// Ensure the server has stopped (srv.Stop should return immediately or be done)
	srv.Stop()

	// Verify that state.json has been written / saved (check if state.json is created)
	statePath := filepath.Join(tmpDir, "state.json")
	if _, err := os.Stat(statePath); os.IsNotExist(err) {
		t.Error("expected state file to exist after graceful shutdown Stop()")
	}
}

// TestServer_Completions_500Error verifies that when the upstream LLM provider returns
// a 500 Internal Server Error, the proxy skips the failing key and, if no remaining
// keys are available, returns a non-200 error status (503) to the client.
func TestServer_Completions_500Error(t *testing.T) {
	// Mock backend that always returns 500
	mockBackend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error": {"message": "Internal server error"}}`))
	}))
	defer mockBackend.Close()

	// Override registry base URL for a test-500 provider
	registry.Providers["groq-500"] = registry.ProviderInfo{
		ID:            "groq-500",
		BaseURL:       mockBackend.URL,
		DefaultRPM:    30,
		DefaultRPD:    14400,
		ResetBehavior: registry.ResetRolling24h,
	}
	defer delete(registry.Providers, "groq-500")

	tmpDir, err := os.MkdirTemp("", "oapi-test-500-*")
	if err != nil {
		t.Fatalf("failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "config.yaml")
	cfg := config.DefaultConfig()
	cfg.Server.Port = 9095
	cfg.Server.Host = "127.0.0.1"
	cfg.Server.DummyAPIKey = ""
	cfg.Keys = []config.KeyConfig{
		{
			ID:       "key-500",
			Provider: "groq-500",
			Model:    "llama-500",
			APIKey:   "key-500-secret",
			Status:   "active",
		},
	}
	cfg.Routes = []config.RouteConfig{
		{
			Name:       "default",
			ModelAlias: "*",
			Chain: []config.SlotConfig{
				{Provider: "groq-500", Model: "llama-500"},
			},
		},
	}

	if err := config.SaveConfig(configPath, cfg); err != nil {
		t.Fatalf("failed to save config: %v", err)
	}

	stateMgr := config.NewStateManager(configPath)
	if err := stateMgr.LoadState(); err != nil {
		t.Fatalf("failed to load state: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pool := rotation.NewKeyPool(ctx, cfg, configPath, stateMgr)
	engine := rotation.NewRotationEngine(pool)

	srv := proxy.NewServer(cfg, configPath, stateMgr, pool, engine)
	if err := srv.Start(ctx); err != nil {
		t.Fatalf("failed to start server: %v", err)
	}
	defer srv.Stop()

	time.Sleep(50 * time.Millisecond)

	reqBody := `{"model": "alias-model", "messages": [{"role": "user", "content": "hi"}]}`
	resp, err := http.Post("http://127.0.0.1:9095/v1/chat/completions", "application/json", bytes.NewBufferString(reqBody))
	if err != nil {
		t.Fatalf("failed to send completions request: %v", err)
	}
	defer resp.Body.Close()

	// With all keys exhausted after 500 errors, proxy must NOT return 200
	if resp.StatusCode == http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected non-200 status when upstream returns 500, got 200. Body: %s", body)
	}

	// Should be 503 (no available keys) or 429 (all providers exhausted)
	if resp.StatusCode != http.StatusServiceUnavailable && resp.StatusCode != http.StatusTooManyRequests {
		body, _ := io.ReadAll(resp.Body)
		t.Errorf("expected 503 or 429 when all keys fail with 500, got %d. Body: %s", resp.StatusCode, body)
	}
}
