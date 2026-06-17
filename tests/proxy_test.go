package tests

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http/httptest"
	"testing"

	"oapi/internal/config"
	"oapi/internal/proxy"
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
