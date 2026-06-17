package tests

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"oapi/internal/config"
	"oapi/internal/rotation"
)

func TestKeyPoolLimiter(t *testing.T) {
	// Create temporary directory for config and state files
	tmpDir, err := os.MkdirTemp("", "oapi-test-limiter-*")
	if err != nil {
		t.Fatalf("failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "config.yaml")

	// 1. Setup config with keys that have low rate limits for testing
	cfg := config.DefaultConfig()
	cfg.Keys = []config.KeyConfig{
		{
			ID:       "groq-test",
			Provider: "groq",
			Model:    "llama-3.1-8b-instant",
			APIKey:   "groq_key_1",
			RPMLimit: 3,  // RPM limit of 3
			RPDLimit: 5,  // RPD limit of 5
			Status:   "active",
		},
		{
			ID:       "google-test",
			Provider: "google",
			Model:    "gemini-2.5-flash",
			APIKey:   "google_key_1",
			RPMLimit: 2,
			RPDLimit: 10,
			Status:   "active",
		},
	}

	err = config.SaveConfig(configPath, cfg)
	if err != nil {
		t.Fatalf("failed to save config: %v", err)
	}

	stateMgr := config.NewStateManager(configPath)
	err = stateMgr.LoadState()
	if err != nil {
		t.Fatalf("failed to load state: %v", err)
	}

	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize KeyPool
	pool := rotation.NewKeyPool(ctx, cfg, configPath, stateMgr)

	// --- 1. Test RPM limits (sliding window) ---
	// Record 3 requests for "groq-test"
	for i := 0; i < 3; i++ {
		ok, nextTime := pool.CanUseKey("groq-test")
		if !ok {
			t.Errorf("expected groq-test key to be usable at request %d, got limited until %v", i+1, nextTime)
		}
		pool.RecordRequest("groq-test")
	}

	// 4th request must be rate limited
	ok, nextTime := pool.CanUseKey("groq-test")
	if ok {
		t.Error("expected groq-test key to be RPM rate limited (4th request in same minute), but it was usable")
	}
	if nextTime.Before(time.Now()) {
		t.Errorf("expected rate limited next availability time to be in the future, got %v", nextTime)
	}

	// --- 2. Test MarkCooling & Recovery ---
	// Mark google-test as cooling manually (RPM cooling) for 100 milliseconds
	pool.MarkCooling("google-test", 100*time.Millisecond, false)

	// Should be cooling now
	ok, _ = pool.CanUseKey("google-test")
	if ok {
		t.Error("expected google-test key to be cooling immediately after MarkCooling")
	}

	// Verify status in config got updated to "cooling_rpm"
	if pool.GetKeysForProviderAndModel("google", "gemini-2.5-flash")[0].Status != "cooling_rpm" {
		t.Errorf("expected config status to be cooling_rpm, got %s", pool.GetKeysForProviderAndModel("google", "gemini-2.5-flash")[0].Status)
	}

	// Wait 150ms for cooling to expire
	time.Sleep(150 * time.Millisecond)

	// Trigger tick manually to speed up tests instead of waiting 5s for background worker
	pool.Tick()

	// Should be recovered and active now
	ok, _ = pool.CanUseKey("google-test")
	if !ok {
		t.Error("expected google-test key to be recovered and active after cooling period expired and Tick executed")
	}

	// Verify status in config got updated to "active"
	if pool.GetKeysForProviderAndModel("google", "gemini-2.5-flash")[0].Status != "active" {
		t.Errorf("expected config status to be active after recovery, got %s", pool.GetKeysForProviderAndModel("google", "gemini-2.5-flash")[0].Status)
	}

	// --- 3. Test RPD limits ---
	// Record 2 more requests for "groq-test" to reach daily limit (currently has 3 requests)
	for i := 0; i < 2; i++ {
		// Note: since RPM is still limited (as we did them within 60s), we will temporarily clear the local RPM logs to isolate RPD limit checks
		pool.Tick() // this clean timestamps if time passes, but we can just clear it:
	}

	// Let's clear the local rpm timestamps manually to test RPD in isolation
	pool.UpdateConfig(cfg) // reset config statuses
	// Clear the timestamps slice for "groq-test"
	// We'll update the KeyPool to reset timestamps in test
	// We can do that by forcing requests directly in state
	stateMgr.UpdateState(func(s *config.RuntimeState) {
		ks := s.Keys["groq-test"]
		ks.RequestsToday = 5 // max limit is 5
		s.Keys["groq-test"] = ks
	})

	ok, nextTime = pool.CanUseKey("groq-test")
	if ok {
		t.Error("expected groq-test key to be daily RPD limited when RequestsToday >= RPDLimit")
	}

	// --- 4. Test RPD reset logic (Rolling 24h) ---
	// Groq provider reset is rolling_24h. Mock FirstRequestToday to be 25 hours ago.
	stateMgr.UpdateState(func(s *config.RuntimeState) {
		ks := s.Keys["groq-test"]
		ks.RequestsToday = 5
		ks.FirstRequestToday = time.Now().Add(-25 * time.Hour)
		s.Keys["groq-test"] = ks
	})

	pool.Tick() // run tick to trigger resets

	// RequestsToday should have been reset to 0
	loadedState := stateMgr.GetState()
	if loadedState.Keys["groq-test"].RequestsToday != 0 {
		t.Errorf("expected groq-test requests_today to be reset to 0 after 25 hours, got %d", loadedState.Keys["groq-test"].RequestsToday)
	}

	// --- 5. Test RPD reset logic (Midnight PT) ---
	// Google provider reset is midnight_pt.
	// Set FirstRequestToday to 15 hours ago, but force different day in PT if needed.
	// To make it deterministic: find the first request time that belongs to a different PT day.
	loc, err := time.LoadLocation("America/Los_Angeles")
	if err != nil {
		loc = time.FixedZone("PST", -8*60*60)
	}
	nowPT := time.Now().In(loc)
	// Create a time that is definitely on the previous calendar day in PT
	yesterdayPT := nowPT.AddDate(0, 0, -1)
	yesterdayLocal := yesterdayPT.In(time.Local)

	stateMgr.UpdateState(func(s *config.RuntimeState) {
		ks := s.Keys["google-test"]
		ks.RequestsToday = 8
		ks.FirstRequestToday = yesterdayLocal
		s.Keys["google-test"] = ks
	})

	pool.Tick()

	loadedState = stateMgr.GetState()
	if loadedState.Keys["google-test"].RequestsToday != 0 {
		t.Errorf("expected google-test requests_today to be reset to 0 after day boundary cross in PT, got %d", loadedState.Keys["google-test"].RequestsToday)
	}
}
