package proxy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"

	"oapi/internal/config"
	"oapi/internal/registry"
)

// RewriteRequest performs payload and header rewriting for the selected key.
func RewriteRequest(req *http.Request, key config.KeyConfig) (*http.Request, error) {
	// 1. Read request body
	bodyBytes, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read request body: %w", err)
	}
	// Restore body for any other reads
	req.Body = io.NopCloser(bytes.NewReader(bodyBytes))

	// 2. Parse request body JSON
	var bodyMap map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &bodyMap); err != nil {
		return nil, fmt.Errorf("invalid JSON request body: %w", err)
	}

	// 3. Map alias to the provider model name
	bodyMap["model"] = key.Model

	// 4. Handle provider-specific payload overrides
	if key.Provider == "cerebras" {
		bodyMap["reasoning_effort"] = "none"
		bodyMap["disable_reasoning"] = true
		bodyMap["clear_thinking"] = true

		// Check and clamp max completion tokens
		limit := key.MaxCompletionTokens
		if limit <= 0 {
			limit = 60
		}

		if val, exists := bodyMap["max_completion_tokens"]; exists {
			if num, ok := val.(float64); ok {
				if int(num) > limit {
					bodyMap["max_completion_tokens"] = limit
				}
			} else {
				bodyMap["max_completion_tokens"] = limit
			}
		} else if val, exists := bodyMap["max_tokens"]; exists {
			if num, ok := val.(float64); ok {
				if int(num) > limit {
					bodyMap["max_tokens"] = limit
				}
			} else {
				bodyMap["max_tokens"] = limit
			}
		} else {
			bodyMap["max_completion_tokens"] = limit
		}
	}

	newBodyBytes, err := json.Marshal(bodyMap)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal rewritten request body: %w", err)
	}

	// 5. Build rewritten URL
	providerInfo, exists := registry.Providers[key.Provider]
	var baseURLStr string
	if exists {
		baseURLStr = providerInfo.BaseURL
	} else {
		return nil, fmt.Errorf("unsupported provider: %s", key.Provider)
	}

	u, err := url.Parse(baseURLStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse provider base URL: %w", err)
	}

	// Route path rewriting: join targetPath (stripping /v1 prefix) to base URL path
	targetPath := req.URL.Path
	if strings.HasPrefix(targetPath, "/v1") {
		targetPath = strings.TrimPrefix(targetPath, "/v1")
	}
	u.Path = path.Join(u.Path, targetPath)

	// Keep query params if any
	u.RawQuery = req.URL.RawQuery

	// 6. Create outReq with same context
	outReq, err := http.NewRequestWithContext(req.Context(), req.Method, u.String(), bytes.NewReader(newBodyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create forwarded request: %w", err)
	}

	// Copy headers
	for k, vv := range req.Header {
		for _, v := range vv {
			outReq.Header.Add(k, v)
		}
	}

	// Strip dummy Authorization and set the real key
	outReq.Header.Del("Authorization")
	outReq.Header.Set("Authorization", "Bearer "+key.APIKey)

	// Set Host header for HTTP forwarding
	outReq.Host = u.Host

	// OpenRouter specific headers
	if key.Provider == "openrouter" {
		if outReq.Header.Get("HTTP-Referer") == "" {
			outReq.Header.Set("HTTP-Referer", "http://localhost")
		}
		if outReq.Header.Get("X-Title") == "" {
			outReq.Header.Set("X-Title", "oapi")
		}
	}

	outReq.ContentLength = int64(len(newBodyBytes))
	outReq.Header.Set("Content-Length", strconv.Itoa(len(newBodyBytes)))

	return outReq, nil
}
