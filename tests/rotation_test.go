package tests

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"oapi/internal/config"
	"oapi/internal/rotation"
)

func setupTestEngine(t *testing.T, cfg *config.Config) (*rotation.KeyPool, *rotation.RotationEngine, string, func()) {
	tmpDir, err := os.MkdirTemp("", "oapi-test-rotation-*")
	if err != nil {
		t.Fatalf("failed to create temp directory: %v", err)
	}

	configPath := filepath.Join(tmpDir, "config.yaml")
	err = config.SaveConfig(configPath, cfg)
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("failed to save config: %v", err)
	}

	stateMgr := config.NewStateManager(configPath)
	err = stateMgr.LoadState()
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("failed to load state: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	pool := rotation.NewKeyPool(ctx, cfg, configPath, stateMgr)
	engine := rotation.NewRotationEngine(pool)

	cleanup := func() {
		cancel()
		os.RemoveAll(tmpDir)
	}

	return pool, engine, configPath, cleanup
}

func TestRouteMatching(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Keys = []config.KeyConfig{
		{
			ID:       "key-a",
			Provider: "groq",
			Model:    "model-a",
			Status:   "active",
		},
		{
			ID:       "key-wildcard",
			Provider: "google",
			Model:    "model-wildcard",
			Status:   "active",
		},
	}
	cfg.Routes = []config.RouteConfig{
		{
			Name:       "exact-route",
			ModelAlias: "alias-a",
			Chain: []config.SlotConfig{
				{Provider: "groq", Model: "model-a"},
			},
		},
		{
			Name:       "wildcard-route",
			ModelAlias: "*",
			Chain: []config.SlotConfig{
				{Provider: "google", Model: "model-wildcard"},
			},
		},
	}

	_, engine, _, cleanup := setupTestEngine(t, cfg)
	defer cleanup()

	// 1. Test exact match
	key, err := engine.RouteRequest("alias-a", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if key.ID != "key-a" {
		t.Errorf("expected key-a, got %s", key.ID)
	}

	// 2. Test wildcard match
	key, err = engine.RouteRequest("alias-other", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if key.ID != "key-wildcard" {
		t.Errorf("expected key-wildcard, got %s", key.ID)
	}
}

func TestPrimaryChainTraversal(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Keys = []config.KeyConfig{
		{
			ID:       "key-1",
			Provider: "groq",
			Model:    "model-1",
			Status:   "active",
		},
		{
			ID:       "key-2",
			Provider: "google",
			Model:    "model-2",
			Status:   "active",
		},
	}
	cfg.Routes = []config.RouteConfig{
		{
			Name:       "route-1",
			ModelAlias: "alias-1",
			Chain: []config.SlotConfig{
				{Provider: "groq", Model: "model-1"},
				{Provider: "google", Model: "model-2"},
			},
		},
	}

	pool, engine, _, cleanup := setupTestEngine(t, cfg)
	defer cleanup()

	// First try: both active, should return key-1 (first in chain)
	key, err := engine.RouteRequest("alias-1", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if key.ID != "key-1" {
		t.Errorf("expected key-1, got %s", key.ID)
	}

	// Now rate limit key-1
	pool.MarkCooling("key-1", 10*time.Second, false)

	// Second try: key-1 cooling, should fall back to key-2
	key, err = engine.RouteRequest("alias-1", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if key.ID != "key-2" {
		t.Errorf("expected key-2, got %s", key.ID)
	}
}

func TestRoundRobinInsideSlot(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Keys = []config.KeyConfig{
		{
			ID:       "key-1",
			Provider: "groq",
			Model:    "model-1",
			Status:   "active",
		},
		{
			ID:       "key-2",
			Provider: "groq",
			Model:    "model-1",
			Status:   "active",
		},
		{
			ID:       "key-3",
			Provider: "groq",
			Model:    "model-1",
			Status:   "active",
		},
	}
	cfg.Routes = []config.RouteConfig{
		{
			Name:       "route-1",
			ModelAlias: "alias-1",
			Chain: []config.SlotConfig{
				{Provider: "groq", Model: "model-1"},
			},
		},
	}

	pool, engine, _, cleanup := setupTestEngine(t, cfg)
	defer cleanup()

	// Check round robin progression: key-1 -> key-2 -> key-3 -> key-1
	expected := []string{"key-1", "key-2", "key-3", "key-1"}
	for i, exp := range expected {
		key, err := engine.RouteRequest("alias-1", nil)
		if err != nil {
			t.Fatalf("unexpected error at request %d: %v", i, err)
		}
		if key.ID != exp {
			t.Errorf("request %d: expected %s, got %s", i, exp, key.ID)
		}
	}

	// Mark key-2 cooling and verify we skip it: should be key-2 -> key-3 -> key-1 -> key-3
	pool.MarkCooling("key-2", 10*time.Second, false)
	key, err := engine.RouteRequest("alias-1", nil) // index is currently 0, but counter incremented to 4. 4 % 3 = 1 (key-2 is cooling). So it advances to key-3.
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if key.ID != "key-3" {
		t.Errorf("expected key-3, got %s", key.ID)
	}
}

func TestFallbackPool(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Keys = []config.KeyConfig{
		{
			ID:       "primary-key",
			Provider: "groq",
			Model:    "model-primary",
			Status:   "active",
		},
		{
			ID:       "fallback-key-1",
			Provider: "google",
			Model:    "model-fallback",
			Status:   "active",
		},
		{
			ID:       "fallback-key-2",
			Provider: "google",
			Model:    "model-fallback",
			Status:   "active",
		},
	}
	cfg.Routes = []config.RouteConfig{
		{
			Name:       "route-1",
			ModelAlias: "alias-1",
			Chain: []config.SlotConfig{
				{Provider: "groq", Model: "model-primary"},
			},
			Fallback: []config.SlotConfig{
				{Provider: "google", Model: "model-fallback"},
			},
		},
	}

	pool, engine, _, cleanup := setupTestEngine(t, cfg)
	defer cleanup()

	// Rate limit primary-key
	pool.MarkCooling("primary-key", 10*time.Second, false)

	// Traversal should activate fallback pool and round-robin across its keys
	key1, err := engine.RouteRequest("alias-1", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	key2, err := engine.RouteRequest("alias-1", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have selected fallback-key-1 and fallback-key-2
	ids := map[string]bool{key1.ID: true, key2.ID: true}
	if !ids["fallback-key-1"] || !ids["fallback-key-2"] {
		t.Errorf("expected fallback keys to be rotated, got %s and %s", key1.ID, key2.ID)
	}
}

func TestExhaustionAndRateLimitError(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Keys = []config.KeyConfig{
		{
			ID:       "key-1",
			Provider: "groq",
			Model:    "model-1",
			Status:   "active",
		},
		{
			ID:       "key-2",
			Provider: "google",
			Model:    "model-2",
			Status:   "active",
		},
	}
	cfg.Routes = []config.RouteConfig{
		{
			Name:       "route-1",
			ModelAlias: "alias-1",
			Chain: []config.SlotConfig{
				{Provider: "groq", Model: "model-1"},
				{Provider: "google", Model: "model-2"},
			},
		},
	}

	pool, engine, _, cleanup := setupTestEngine(t, cfg)
	defer cleanup()

	now := time.Now()
	t1 := now.Add(5 * time.Second)

	// Rate limit both keys with different durations
	pool.MarkCooling("key-1", 5*time.Second, false)
	pool.MarkCooling("key-2", 10*time.Second, false)

	// Both rate limited, should return RateLimitError with earliest cooldown (key-1)
	_, err := engine.RouteRequest("alias-1", nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var rateErr *rotation.RateLimitError
	if !errors.As(err, &rateErr) {
		t.Fatalf("expected RateLimitError, got %T: %v", err, err)
	}

	// The earliest cooldown should be around t1 (within clock skew)
	diff := rateErr.EarliestAvailable.Sub(t1)
	if diff < -time.Second || diff > time.Second {
		t.Errorf("expected earliest available time close to %v, got %v (diff: %v)", t1, rateErr.EarliestAvailable, diff)
	}
}

func TestGetKeyStats(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Keys = []config.KeyConfig{
		{
			ID:       "key-1",
			Provider: "groq",
			Model:    "model-1",
			Status:   "active",
		},
	}
	pool, _, _, cleanup := setupTestEngine(t, cfg)
	defer cleanup()

	// 1. Check initially active key
	stats, found := pool.GetKeyStats("key-1")
	if !found {
		t.Fatalf("expected key-1 to be found")
	}
	if stats.Cooling {
		t.Errorf("expected key-1 not to be cooling")
	}
	if stats.RPMUsed != 0 {
		t.Errorf("expected RPMUsed to be 0, got %d", stats.RPMUsed)
	}

	// 2. Mark cooling and verify stats
	pool.MarkCooling("key-1", 10*time.Second, false)
	stats, found = pool.GetKeyStats("key-1")
	if !found {
		t.Fatalf("expected key-1 to be found")
	}
	if !stats.Cooling {
		t.Errorf("expected key-1 to be cooling")
	}

	// 3. Request stats for non-existent key
	_, found = pool.GetKeyStats("key-nonexistent")
	if found {
		t.Errorf("expected key-nonexistent not to be found")
	}
}

