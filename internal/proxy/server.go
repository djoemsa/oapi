package proxy

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"oapi/internal/config"
	"oapi/internal/rotation"
)

type Server struct {
	cfg        *config.Config
	configPath string
	stateMgr   *config.StateManager
	pool       *rotation.KeyPool
	engine     *rotation.RotationEngine
	httpServer *http.Server
	startTime  time.Time
	wg         sync.WaitGroup
	stopped    chan struct{}
	stopOnce   sync.Once
}

func NewServer(cfg *config.Config, configPath string, stateMgr *config.StateManager, pool *rotation.KeyPool, engine *rotation.RotationEngine) *Server {
	return &Server{
		cfg:        cfg,
		configPath: configPath,
		stateMgr:   stateMgr,
		pool:       pool,
		engine:     engine,
		startTime:  time.Now(),
		stopped:    make(chan struct{}),
	}
}

func (s *Server) Start(ctx context.Context) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/v1/models", s.handleModels)
	mux.HandleFunc("/v1/chat/completions", s.handleCompletions)

	addr := fmt.Sprintf("%s:%d", s.cfg.Server.Host, s.cfg.Server.Port)
	s.httpServer = &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	log.Printf("Proxy server starting on http://%s", addr)
	// Bind to listener so we can catch startup port errors early
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}

	// Spawn context watcher for shutdown
	go func() {
		<-ctx.Done()
		log.Println("Shutdown signal received, starting graceful shutdown...")
		if err := s.Stop(); err != nil {
			log.Printf("Server stop error: %v", err)
		}
	}()

	go func() {
		if err := s.httpServer.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("HTTP server Serve error: %v", err)
		}
	}()

	return nil
}

func (s *Server) Stopped() <-chan struct{} {
	return s.stopped
}

func (s *Server) Stop() error {
	s.stopOnce.Do(func() {
		if s.httpServer != nil {
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			// Shutdown HTTP listener
			if err := s.httpServer.Shutdown(shutdownCtx); err != nil {
				log.Printf("HTTP server shutdown error: %v", err)
			}

			// Wait for in-flight requests to complete
			done := make(chan struct{})
			go func() {
				s.wg.Wait()
				close(done)
			}()

			select {
			case <-done:
				log.Println("All in-flight requests completed.")
			case <-shutdownCtx.Done():
				log.Println("Graceful shutdown timeout exceeded; forcing state flush and exit.")
			}

			// Save state atomically
			_ = s.stateMgr.SaveState()
		}
		close(s.stopped)
	})
	return nil
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	s.wg.Add(1)
	defer s.wg.Done()

	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	s.pool.Tick() // force tick to get fresh counters
	runtimeState := s.stateMgr.GetState()
	activeCount := 0
	for _, k := range s.cfg.Keys {
		if k.Status == "active" {
			activeCount++
		}
	}

	health := map[string]interface{}{
		"status":         "healthy",
		"uptime":         time.Since(s.startTime).String(),
		"keys_active":    activeCount,
		"requests_today": runtimeState.TotalRequestsToday,
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(health)
}

func (s *Server) handleModels(w http.ResponseWriter, r *http.Request) {
	s.wg.Add(1)
	defer s.wg.Done()

	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	// Authenticate if dummy API key is configured
	if !s.authorize(r) {
		s.writeAPIError(w, http.StatusUnauthorized, "Unauthorized: invalid dummy API key", "unauthorized")
		return
	}

	var data []map[string]interface{}
	for _, route := range s.cfg.Routes {
		data = append(data, map[string]interface{}{
			"id":       route.ModelAlias,
			"object":   "model",
			"created":  s.startTime.Unix(),
			"owned_by": "oapi",
		})
	}

	models := map[string]interface{}{
		"object": "list",
		"data":   data,
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(models)
}

func (s *Server) handleCompletions(w http.ResponseWriter, r *http.Request) {
	s.wg.Add(1)
	defer s.wg.Done()

	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	// 1. Authenticate client
	if !s.authorize(r) {
		s.writeAPIError(w, http.StatusUnauthorized, "Unauthorized: invalid dummy API key", "unauthorized")
		return
	}

	// Read body to extract model
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		s.writeAPIError(w, http.StatusBadRequest, "Failed to read request body", "bad_request")
		return
	}
	r.Body = io.NopCloser(bytes.NewReader(bodyBytes))

	var bodyMap map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &bodyMap); err != nil {
		s.writeAPIError(w, http.StatusBadRequest, "Invalid JSON body", "invalid_json")
		return
	}

	modelStr, _ := bodyMap["model"].(string)
	if modelStr == "" {
		s.writeAPIError(w, http.StatusBadRequest, "Missing model parameter", "missing_model")
		return
	}

	stream, _ := bodyMap["stream"].(bool)

	// 2. Retry loop with key rotation
	triedKeys := make(map[string]bool)
	client := &http.Client{Timeout: 120 * time.Second}

	for {
		// Get next key
		key, err := s.engine.RouteRequest(modelStr, triedKeys)
		if err != nil {
			// Check if all keys rate-limited (exhausted)
			var rateErr *rotation.RateLimitError
			if errors.As(err, &rateErr) {
				seconds := time.Until(rateErr.EarliestAvailable).Seconds()
				if seconds < 1 {
					seconds = 1
				}
				w.Header().Set("Retry-After", fmt.Sprintf("%.0f", seconds))
				s.writeAPIError(w, http.StatusTooManyRequests, fmt.Sprintf("All providers exhausted. Retry after %.0f seconds.", seconds), "all_providers_exhausted")
				return
			}
			s.writeAPIError(w, http.StatusServiceUnavailable, err.Error(), "no_available_keys")
			return
		}

		// Rewrite request
		rewrittenReq, err := RewriteRequest(r, key)
		if err != nil {
			log.Printf("Failed to rewrite request for key %s: %v", key.ID, err)
			triedKeys[key.ID] = true
			continue
		}

		// Forward request
		resp, err := client.Do(rewrittenReq)
		if err != nil {
			log.Printf("Failed to forward request to provider %s (key %s): %v", key.Provider, key.ID, err)
			triedKeys[key.ID] = true
			continue
		}

		// Handle response status
		if resp.StatusCode == http.StatusOK {
			// Record success
			s.pool.RecordRequest(key.ID)

			// Copy response headers (except standard connection headers)
			for k, vv := range resp.Header {
				if k == "Content-Length" || k == "Connection" || k == "Transfer-Encoding" {
					continue
				}
				for _, v := range vv {
					w.Header().Add(k, v)
				}
			}
			w.WriteHeader(resp.StatusCode)

			// Stream response or copy body
			if stream {
				s.pipeSSEStream(w, resp.Body)
			} else {
				_, _ = io.Copy(w, resp.Body)
			}
			resp.Body.Close()
			return
		}

		// Handle 429 Rate Limits
		if resp.StatusCode == http.StatusTooManyRequests {
			resp.Body.Close()
			isRPD := false
			if resp.Header.Get("x-ratelimit-remaining-requests-day") == "0" ||
				resp.Header.Get("x-ratelimit-remaining-requests") == "0" {
				isRPD = true
			}

			var retryAfter time.Duration
			if raStr := resp.Header.Get("Retry-After"); raStr != "" {
				if sec, err := strconv.Atoi(raStr); err == nil {
					retryAfter = time.Duration(sec) * time.Second
				}
			}
			if retryAfter == 0 {
				if rstStr := resp.Header.Get("x-ratelimit-reset-requests"); rstStr != "" {
					if sec, err := strconv.ParseFloat(rstStr, 64); err == nil {
						retryAfter = time.Duration(sec * float64(time.Second))
					}
				}
			}

			s.pool.MarkCooling(key.ID, retryAfter, isRPD)
			log.Printf("Key %s rate limited (429), cooling registered (isRPD: %t, retryAfter: %v)", key.ID, isRPD, retryAfter)
			triedKeys[key.ID] = true
			continue
		}

		// Handle other HTTP errors (4xx / 5xx)
		log.Printf("Provider %s returned status %d for key %s", key.Provider, resp.StatusCode, key.ID)
		resp.Body.Close()

		// If invalid key (401/403), mark key configuration status as error
		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			s.pool.MarkCooling(key.ID, 0, false) // temporary cooldown representation or manual status set if possible
			// wait, our limiter doesn't let us mark status "error" easily unless we mutate it in config,
			// but we can just register cooling / skip it.
		}

		triedKeys[key.ID] = true
	}
}

func (s *Server) authorize(r *http.Request) bool {
	if s.cfg.Server.DummyAPIKey == "" {
		return true
	}
	authHeader := r.Header.Get("Authorization")
	if !strings.HasPrefix(authHeader, "Bearer ") {
		return false
	}
	token := strings.TrimPrefix(authHeader, "Bearer ")
	return token == s.cfg.Server.DummyAPIKey
}

func (s *Server) pipeSSEStream(w http.ResponseWriter, body io.Reader) {
	flusher, ok := w.(http.Flusher)
	reader := bufio.NewReader(body)

	for {
		line, err := reader.ReadBytes('\n')
		if err != nil {
			break
		}
		_, _ = w.Write(line)
		if ok {
			flusher.Flush()
		}
	}
}

func (s *Server) writeAPIError(w http.ResponseWriter, status int, message string, errType string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	errObj := map[string]interface{}{
		"error": map[string]string{
			"message": message,
			"type":    errType,
			"code":    errType,
		},
	}
	_ = json.NewEncoder(w).Encode(errObj)
}
