package rotation

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"oapi/internal/config"
)

var (
	ErrNoRouteMatched   = errors.New("no route matched for model")
	ErrAllKeysExhausted = errors.New("all keys exhausted")
)

// RateLimitError is returned when all keys matching a route are rate-limited.
type RateLimitError struct {
	EarliestAvailable time.Time
}

func (e *RateLimitError) Error() string {
	duration := time.Until(e.EarliestAvailable)
	if duration < 0 {
		duration = 0
	}
	return fmt.Sprintf("all providers rate limited, earliest available in %.0f seconds", duration.Seconds())
}

// RotationEngine manages matching and selecting keys based on routing rules.
type RotationEngine struct {
	kp      *KeyPool
	indices sync.Map // maps string to *uint64
}

// NewRotationEngine creates a new RotationEngine.
func NewRotationEngine(kp *KeyPool) *RotationEngine {
	return &RotationEngine{
		kp: kp,
	}
}

func (re *RotationEngine) getCounter(key string) uint64 {
	val, _ := re.indices.LoadOrStore(key, new(uint64))
	ptr := val.(*uint64)
	return atomic.AddUint64(ptr, 1) - 1
}

// RouteRequest selects the best key for the requested model alias, ignoring triedKeys.
func (re *RotationEngine) RouteRequest(model string, triedKeys map[string]bool) (config.KeyConfig, error) {
	re.kp.mu.RLock()
	routes := re.kp.cfg.Routes
	re.kp.mu.RUnlock()

	var matchedRoute *config.RouteConfig

	// 1. Exact match
	for i := range routes {
		if routes[i].ModelAlias == model {
			matchedRoute = &routes[i]
			break
		}
	}

	// 2. Wildcard match
	if matchedRoute == nil {
		for i := range routes {
			if routes[i].ModelAlias == "*" {
				matchedRoute = &routes[i]
				break
			}
		}
	}

	if matchedRoute == nil {
		return config.KeyConfig{}, ErrNoRouteMatched
	}

	// 3. Walk primary chain slots from left to right
	for idx, slot := range matchedRoute.Chain {
		slotKeys := re.kp.GetKeysForProviderAndModel(slot.Provider, slot.Model)
		if len(slotKeys) == 0 {
			continue
		}

		slotKey := fmt.Sprintf("primary:%s:%d", matchedRoute.ModelAlias, idx)
		val := re.getCounter(slotKey)

		// Try to find a usable key in the slot, starting at round-robin index
		n := len(slotKeys)
		for i := 0; i < n; i++ {
			candidate := slotKeys[(int(val)+i)%n]
			if triedKeys != nil && triedKeys[candidate.ID] {
				continue
			}

			usable, _ := re.kp.CanUseKey(candidate.ID)
			if usable {
				return candidate, nil
			}
		}
	}

	// 4. Activate Fallback Pool if primary chain is exhausted
	var fallbackKeys []config.KeyConfig
	for _, slot := range matchedRoute.Fallback {
		slotKeys := re.kp.GetKeysForProviderAndModel(slot.Provider, slot.Model)
		fallbackKeys = append(fallbackKeys, slotKeys...)
	}

	if len(fallbackKeys) > 0 {
		var usableFallbackKeys []config.KeyConfig
		for _, key := range fallbackKeys {
			if triedKeys != nil && triedKeys[key.ID] {
				continue
			}
			if usable, _ := re.kp.CanUseKey(key.ID); usable {
				usableFallbackKeys = append(usableFallbackKeys, key)
			}
		}

		if len(usableFallbackKeys) > 0 {
			fallbackKey := fmt.Sprintf("fallback:%s", matchedRoute.ModelAlias)
			val := re.getCounter(fallbackKey)
			selected := usableFallbackKeys[int(val)%len(usableFallbackKeys)]
			return selected, nil
		}
	}

	// 5. If everything is exhausted, find the earliest cooling expiration time
	var earliestCooldown time.Time
	hasCooldown := false

	// Helper to inspect keys
	inspectKey := func(key config.KeyConfig) {
		usable, nextAvailable := re.kp.CanUseKey(key.ID)
		if !usable && !nextAvailable.IsZero() {
			if !hasCooldown || nextAvailable.Before(earliestCooldown) {
				earliestCooldown = nextAvailable
				hasCooldown = true
			}
		}
	}

	// Inspect all keys in primary chain slots
	for _, slot := range matchedRoute.Chain {
		slotKeys := re.kp.GetKeysForProviderAndModel(slot.Provider, slot.Model)
		for _, key := range slotKeys {
			inspectKey(key)
		}
	}

	// Inspect all keys in fallback pool slots
	for _, slot := range matchedRoute.Fallback {
		slotKeys := re.kp.GetKeysForProviderAndModel(slot.Provider, slot.Model)
		for _, key := range slotKeys {
			inspectKey(key)
		}
	}

	if hasCooldown {
		return config.KeyConfig{}, &RateLimitError{EarliestAvailable: earliestCooldown}
	}

	return config.KeyConfig{}, ErrAllKeysExhausted
}
