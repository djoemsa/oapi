package rotation

import (
	"context"
	"sync"
	"time"

	_ "time/tzdata"

	"oapi/internal/config"
	"oapi/internal/registry"
)

// KeyPool manages API key configurations, transient rate limit logs,
// persistent state synchronization, and background cooldown recovery.
type KeyPool struct {
	mu            sync.RWMutex
	cfg           *config.Config
	configPath    string
	stateMgr      *config.StateManager
	rpmTimestamps map[string][]time.Time
}

// NewKeyPool creates and starts a KeyPool instance with a background tick worker.
func NewKeyPool(ctx context.Context, cfg *config.Config, configPath string, stateMgr *config.StateManager) *KeyPool {
	pool := &KeyPool{
		cfg:           cfg,
		configPath:    configPath,
		stateMgr:      stateMgr,
		rpmTimestamps: make(map[string][]time.Time),
	}

	go pool.startBackgroundTick(ctx)

	return pool
}

// UpdateConfig replaces the underlying configuration thread-safely.
func (kp *KeyPool) UpdateConfig(newCfg *config.Config) {
	kp.mu.Lock()
	defer kp.mu.Unlock()
	kp.cfg = newCfg
}

// GetLimits returns the effective RPM and RPD limits for a given key,
// prioritizing user configuration overrides over model/provider defaults.
func (kp *KeyPool) GetLimits(key config.KeyConfig) (rpm int, rpd int) {
	// 1. Start with provider defaults
	if provider, exists := registry.Providers[key.Provider]; exists {
		rpm = provider.DefaultRPM
		rpd = provider.DefaultRPD
	}

	// 2. Apply Groq-specific model overrides
	if key.Provider == "groq" {
		if override, exists := registry.GroqModelOverrides[key.Model]; exists {
			if override.RPM > 0 {
				rpm = override.RPM
			}
			if override.RPD > 0 {
				rpd = override.RPD
			}
		}
	}

	// 3. Apply user-defined overrides in KeyConfig if present and non-zero
	if key.RPMLimit > 0 {
		rpm = key.RPMLimit
	}
	if key.RPDLimit > 0 {
		rpd = key.RPDLimit
	}

	return rpm, rpd
}

// GetKeysForProviderAndModel returns a copy of all key configs matching the criteria.
func (kp *KeyPool) GetKeysForProviderAndModel(provider, model string) []config.KeyConfig {
	kp.mu.RLock()
	defer kp.mu.RUnlock()

	var matched []config.KeyConfig
	for _, key := range kp.cfg.Keys {
		if key.Provider == provider && key.Model == model {
			matched = append(matched, key)
		}
	}
	return matched
}

// startBackgroundTick executes the recovery and cleanup routine every 5 seconds.
func (kp *KeyPool) startBackgroundTick(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			kp.Tick()
		}
	}
}

// Tick executes the maintenance routines: cleaning expired transient RPM logs,
// recovering cooling keys, and resetting daily request counters.
func (kp *KeyPool) Tick() {
	kp.mu.Lock()
	defer kp.mu.Unlock()

	now := time.Now()
	stateChanged := false
	configChanged := false

	// 1. Clean expired RPM timestamps (older than 60 seconds)
	for keyID, times := range kp.rpmTimestamps {
		cutoff := now.Add(-60 * time.Second)
		idx := -1
		for i, t := range times {
			if t.After(cutoff) {
				idx = i
				break
			}
		}
		if idx == -1 {
			delete(kp.rpmTimestamps, keyID)
		} else if idx > 0 {
			kp.rpmTimestamps[keyID] = times[idx:]
		}
	}

	// 2. Recovery of cooling keys and daily counter reset
	runtimeState := kp.stateMgr.GetState()
	for i, keyCfg := range kp.cfg.Keys {
		keyState, exists := runtimeState.Keys[keyCfg.ID]
		if !exists {
			continue
		}

		// A. Check if CoolingUntil has expired
		if keyState.CoolingUntil != nil {
			if now.After(*keyState.CoolingUntil) {
				kp.stateMgr.UpdateState(func(s *config.RuntimeState) {
					ks := s.Keys[keyCfg.ID]
					ks.CoolingUntil = nil
					s.Keys[keyCfg.ID] = ks
				})
				stateChanged = true

				// Reset status in config back to active if it was cooling
				if keyCfg.Status == "cooling_rpm" || keyCfg.Status == "cooling_rpd" {
					kp.cfg.Keys[i].Status = "active"
					configChanged = true
				}
			}
		}

		// B. Check for daily RPD reset
		if !keyState.FirstRequestToday.IsZero() {
			resetNeeded := false
			provider := registry.Providers[keyCfg.Provider]
			resetBehavior := provider.ResetBehavior
			if override, hasOverride := kp.cfg.Providers[keyCfg.Provider]; hasOverride && override.ResetBehavior != "" {
				resetBehavior = override.ResetBehavior
			}

			switch resetBehavior {
			case registry.ResetMidnightPT:
				loc, err := time.LoadLocation("America/Los_Angeles")
				if err != nil {
					loc = time.FixedZone("PST", -8*60*60)
				}
				nowPT := now.In(loc)
				firstPT := keyState.FirstRequestToday.In(loc)
				if nowPT.Year() != firstPT.Year() || nowPT.YearDay() != firstPT.YearDay() {
					resetNeeded = true
				}
			case registry.ResetRolling24h:
				if now.Sub(keyState.FirstRequestToday) >= 24*time.Hour {
					resetNeeded = true
				}
			default: // ResetMidnightLocal or others (default is midnight local time)
				nowLocal := now.Local()
				firstLocal := keyState.FirstRequestToday.Local()
				if nowLocal.Year() != firstLocal.Year() || nowLocal.YearDay() != firstLocal.YearDay() {
					resetNeeded = true
				}
			}

			if resetNeeded {
				kp.stateMgr.UpdateState(func(s *config.RuntimeState) {
					ks := s.Keys[keyCfg.ID]
					ks.RequestsToday = 0
					ks.FirstRequestToday = time.Time{}
					s.Keys[keyCfg.ID] = ks
				})
				stateChanged = true
			}
		}
	}

	if stateChanged {
		_ = kp.stateMgr.SaveState()
	}
	if configChanged {
		_ = config.SaveConfig(kp.configPath, kp.cfg)
	}
}

// KeyStats holds display metrics for a single key.
type KeyStats struct {
	RPMUsed   int
	RPDUsed   int
	LastUsed  time.Time
	Cooling   bool
	CoolUntil *time.Time
}

// GetKeyStats returns current runtime stats for a key by its ID.
// Thread-safe (uses RLock).
func (kp *KeyPool) GetKeyStats(keyID string) (KeyStats, bool) {
	kp.mu.RLock()
	defer kp.mu.RUnlock()

	found := false
	for _, k := range kp.cfg.Keys {
		if k.ID == keyID {
			found = true
			break
		}
	}
	if !found {
		return KeyStats{}, false
	}

	rpmUsed := len(kp.rpmTimestamps[keyID])
	state := kp.stateMgr.GetState()
	keyState, exists := state.Keys[keyID]
	if !exists {
		return KeyStats{
			RPMUsed: rpmUsed,
		}, true
	}

	cooling := false
	if keyState.CoolingUntil != nil {
		cooling = time.Now().Before(*keyState.CoolingUntil)
	}

	return KeyStats{
		RPMUsed:   rpmUsed,
		RPDUsed:   keyState.RequestsToday,
		LastUsed:  keyState.LastUsed,
		Cooling:   cooling,
		CoolUntil: keyState.CoolingUntil,
	}, true
}

