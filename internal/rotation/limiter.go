package rotation

import (
	"time"

	"oapi/internal/config"
	"oapi/internal/registry"
)

// CanUseKey checks if a key is available for use by analyzing its status,
// cooldown duration, sliding window RPM, and daily RPD limit.
// If limited, returns false and the time when the key is expected to become available.
func (kp *KeyPool) CanUseKey(keyID string) (bool, time.Time) {
	kp.mu.Lock()
	defer kp.mu.Unlock()

	now := time.Now()

	// 1. Locate key configuration
	var keyCfg *config.KeyConfig
	for i := range kp.cfg.Keys {
		if kp.cfg.Keys[i].ID == keyID {
			keyCfg = &kp.cfg.Keys[i]
			break
		}
	}
	if keyCfg == nil {
		return false, time.Time{}
	}

	// 2. Disabled or manually marked error keys are unusable
	if keyCfg.Status == "disabled" || keyCfg.Status == "error" {
		return false, time.Time{}
	}

	// 3. Get key runtime metrics
	runtimeState := kp.stateMgr.GetState()
	keyState, exists := runtimeState.Keys[keyID]
	if !exists {
		// Fresh key is active and ready
		return true, time.Time{}
	}

	// 4. Check if cooling period is still in progress
	if keyState.CoolingUntil != nil && now.Before(*keyState.CoolingUntil) {
		return false, *keyState.CoolingUntil
	}

	rpmLimit, rpdLimit := kp.GetLimits(*keyCfg)

	// 5. Clean expired timestamps from local cache to verify current sliding RPM
	cutoff := now.Add(-60 * time.Second)
	var activeTimes []time.Time
	for _, t := range kp.rpmTimestamps[keyID] {
		if t.After(cutoff) {
			activeTimes = append(activeTimes, t)
		}
	}
	kp.rpmTimestamps[keyID] = activeTimes

	// 6. Check sliding window RPM limits
	if rpmLimit > 0 && len(activeTimes) >= rpmLimit {
		// Earliest next slot is exactly 60 seconds after the oldest request in the window
		nextAvailable := activeTimes[0].Add(60 * time.Second)
		return false, nextAvailable
	}

	// 7. Check daily RPD limits
	if rpdLimit > 0 && keyState.RequestsToday >= rpdLimit {
		nextAvailable := kp.getNextResetTime(*keyCfg, keyState.FirstRequestToday)
		return false, nextAvailable
	}

	return true, time.Time{}
}

// RecordRequest registers a successful API request, updates sliding window
// timestamps, updates daily counters, and flushes runtime state.
func (kp *KeyPool) RecordRequest(keyID string) {
	kp.mu.Lock()
	defer kp.mu.Unlock()

	now := time.Now()

	var keyCfg *config.KeyConfig
	for i := range kp.cfg.Keys {
		if kp.cfg.Keys[i].ID == keyID {
			keyCfg = &kp.cfg.Keys[i]
			break
		}
	}
	if keyCfg == nil {
		return
	}

	// 1. Log request timestamp in sliding window
	kp.rpmTimestamps[keyID] = append(kp.rpmTimestamps[keyID], now)

	// 2. Update state manager counters
	kp.stateMgr.UpdateState(func(s *config.RuntimeState) {
		ks, exists := s.Keys[keyID]
		if !exists {
			ks = config.KeyState{}
		}

		ks.RequestsToday++
		ks.LastUsed = now
		if ks.FirstRequestToday.IsZero() {
			ks.FirstRequestToday = now
		}

		// Update requests_this_minute for UI visualization
		cutoff := now.Add(-60 * time.Second)
		count := 0
		for _, t := range kp.rpmTimestamps[keyID] {
			if t.After(cutoff) {
				count++
			}
		}
		ks.RequestsThisMinute = count

		s.Keys[keyID] = ks
		s.TotalRequestsToday++
	})

	_ = kp.stateMgr.SaveState()
}

// MarkCooling puts a key into a cooling state because it encountered a 429 rate limit.
func (kp *KeyPool) MarkCooling(keyID string, retryAfter time.Duration, isRPD bool) {
	kp.mu.Lock()
	defer kp.mu.Unlock()

	now := time.Now()

	var keyCfg *config.KeyConfig
	var keyCfgIdx = -1
	for i := range kp.cfg.Keys {
		if kp.cfg.Keys[i].ID == keyID {
			keyCfg = &kp.cfg.Keys[i]
			keyCfgIdx = i
			break
		}
	}
	if keyCfg == nil {
		return
	}

	var coolingUntil time.Time
	if isRPD {
		coolingUntil = kp.getNextResetTime(*keyCfg, time.Time{})
		if keyCfgIdx != -1 {
			kp.cfg.Keys[keyCfgIdx].Status = "cooling_rpd"
		}
	} else {
		if retryAfter > 0 {
			coolingUntil = now.Add(retryAfter)
		} else {
			// Cooling until start of next 60s window (when the oldest request expires)
			timestamps := kp.rpmTimestamps[keyID]
			if len(timestamps) > 0 {
				coolingUntil = timestamps[0].Add(60 * time.Second)
			} else {
				coolingUntil = now.Add(10 * time.Second) // Default fallback
			}
		}
		if keyCfgIdx != -1 {
			kp.cfg.Keys[keyCfgIdx].Status = "cooling_rpm"
		}
	}

	kp.stateMgr.UpdateState(func(s *config.RuntimeState) {
		ks, exists := s.Keys[keyID]
		if !exists {
			ks = config.KeyState{}
		}
		ks.CoolingUntil = &coolingUntil
		s.Keys[keyID] = ks
	})

	_ = kp.stateMgr.SaveState()
	_ = config.SaveConfig(kp.configPath, kp.cfg)
}

// getNextResetTime determines when a key's daily limit resets based on its provider behavior.
func (kp *KeyPool) getNextResetTime(key config.KeyConfig, firstRequestToday time.Time) time.Time {
	now := time.Now()
	provider := registry.Providers[key.Provider]
	resetBehavior := provider.ResetBehavior

	if override, hasOverride := kp.cfg.Providers[key.Provider]; hasOverride && override.ResetBehavior != "" {
		resetBehavior = override.ResetBehavior
	}

	switch resetBehavior {
	case registry.ResetMidnightPT:
		loc, err := time.LoadLocation("America/Los_Angeles")
		if err != nil {
			loc = time.FixedZone("PST", -8*60*60)
		}
		nowPT := now.In(loc)
		midnightPT := time.Date(nowPT.Year(), nowPT.Month(), nowPT.Day()+1, 0, 0, 0, 0, loc)
		return midnightPT.In(time.Local)

	case registry.ResetRolling24h:
		if firstRequestToday.IsZero() {
			return now.Add(24 * time.Hour)
		}
		return firstRequestToday.Add(24 * time.Hour)

	default: // ResetMidnightLocal or others (midnight local time)
		nowLocal := now.Local()
		midnightLocal := time.Date(nowLocal.Year(), nowLocal.Month(), nowLocal.Day()+1, 0, 0, 0, 0, time.Local)
		return midnightLocal
	}
}
