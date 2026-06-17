package testutil

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"oapi/internal/config"
	"oapi/internal/registry"
)

// HTTPClient is a mockable interface matching http.Client's Do method.
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// DefaultHTTPClient returns a default net/http client configured with 10s timeout.
func DefaultHTTPClient() HTTPClient {
	return &http.Client{
		Timeout: 10 * time.Second,
	}
}

// RequestPayload represents the minimal chat completion request payload used for probing.
type RequestPayload struct {
	Model     string    `json:"model"`
	Messages  []Message `json:"messages"`
	MaxTokens int       `json:"max_tokens"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ProbeKey tests connection for a key configuration and maps results to statuses:
// "active" | "error" | "cooling_rpm". Returns status and optional error.
func ProbeKey(client HTTPClient, key config.KeyConfig) (string, error) {
	// Find base URL from registry (fallback to empty if custom/unknown)
	baseURL := ""
	if prov, exists := registry.Providers[key.Provider]; exists {
		baseURL = prov.BaseURL
	}

	if baseURL == "" {
		return "error", fmt.Errorf("unknown provider: %s", key.Provider)
	}

	// Prepare standard request payload
	payload := RequestPayload{
		Model:     key.Model,
		Messages:  []Message{{Role: "user", Content: "Hi"}},
		MaxTokens: 5,
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return "error", fmt.Errorf("failed to marshal payload: %w", err)
	}

	// Prepare URL
	targetURL := fmt.Sprintf("%s/chat/completions", baseURL)

	req, err := http.NewRequest("POST", targetURL, bytes.NewBuffer(bodyBytes))
	if err != nil {
		return "error", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	// Set auth headers
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", key.APIKey))

	// OpenRouter special headers
	if key.Provider == "openrouter" {
		req.Header.Set("HTTP-Referer", "http://localhost")
		req.Header.Set("X-Title", "oapi")
	}

	resp, err := client.Do(req)
	if err != nil {
		return "error", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Analyze status code
	switch resp.StatusCode {
	case http.StatusOK:
		return "active", nil
	case http.StatusUnauthorized, http.StatusForbidden:
		return "error", fmt.Errorf("authentication failed (status %d)", resp.StatusCode)
	case http.StatusTooManyRequests:
		return "cooling_rpm", fmt.Errorf("rate limited (status 429)")
	default:
		// Attempt to read error message from body
		bodyErrBytes, _ := io.ReadAll(resp.Body)
		bodyErrMsg := ""
		if len(bodyErrBytes) > 0 {
			bodyErrMsg = fmt.Sprintf(": %s", string(bodyErrBytes))
		}
		return "error", fmt.Errorf("provider returned status %d%s", resp.StatusCode, bodyErrMsg)
	}
}
