package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type KeyState struct {
	RequestsThisMinute int        `json:"requests_this_minute"`
	RequestsToday      int        `json:"requests_today"`
	TokensThisMinute   int        `json:"tokens_this_minute"`
	CoolingUntil       *time.Time `json:"cooling_until"` // pointer to support null/nil in JSON
	LastUsed           time.Time  `json:"last_used"`
	FirstRequestToday  time.Time  `json:"first_request_today"`
}

type RuntimeState struct {
	Keys               map[string]KeyState `json:"keys"`
	TotalRequestsToday int                 `json:"total_requests_today"`
	UptimeStart        time.Time           `json:"uptime_start"`
}

type StateManager struct {
	mu        sync.RWMutex
	statePath string
	state     *RuntimeState
}

// NewStateManager creates a new StateManager for state.json.
func NewStateManager(configPath string) *StateManager {
	statePath := ResolveStatePath(configPath)
	return &StateManager{
		statePath: statePath,
		state: &RuntimeState{
			Keys:        make(map[string]KeyState),
			UptimeStart: time.Now(),
		},
	}
}

// ResolveStatePath finds the path for state.json which is placed next to oapi.yaml.
func ResolveStatePath(configPath string) string {
	dir := filepath.Dir(configPath)
	return filepath.Join(dir, "state.json")
}

// GetState returns a copy of the current state.
func (sm *StateManager) GetState() RuntimeState {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	// Deep copy keys map
	keysCopy := make(map[string]KeyState, len(sm.state.Keys))
	for k, v := range sm.state.Keys {
		keysCopy[k] = v
	}

	return RuntimeState{
		Keys:               keysCopy,
		TotalRequestsToday: sm.state.TotalRequestsToday,
		UptimeStart:        sm.state.UptimeStart,
	}
}

// UpdateState executes an update function under a write lock.
func (sm *StateManager) UpdateState(updateFn func(*RuntimeState)) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	updateFn(sm.state)
}

// LoadState reads the state from state.json if it exists.
func (sm *StateManager) LoadState() error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	data, err := os.ReadFile(sm.statePath)
	if err != nil {
		if os.IsNotExist(err) {
			// If file does not exist, start with a fresh state
			sm.state = &RuntimeState{
				Keys:        make(map[string]KeyState),
				UptimeStart: time.Now(),
			}
			return nil
		}
		return fmt.Errorf("failed to read state file: %w", err)
	}

	var loaded RuntimeState
	if err := json.Unmarshal(data, &loaded); err != nil {
		return fmt.Errorf("failed to parse state JSON: %w", err)
	}

	if loaded.Keys == nil {
		loaded.Keys = make(map[string]KeyState)
	}

	sm.state = &loaded
	return nil
}

// SaveState writes the current state to state.json atomically.
func (sm *StateManager) SaveState() error {
	sm.mu.RLock()
	data, err := json.MarshalIndent(sm.state, "", "  ")
	sm.mu.RUnlock()

	if err != nil {
		return fmt.Errorf("failed to marshal state to JSON: %w", err)
	}

	// Ensure parent directory exists
	dir := filepath.Dir(sm.statePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create state directory: %w", err)
	}

	// Write to temporary file first
	tmpFile := sm.statePath + ".tmp"
	if err := os.WriteFile(tmpFile, data, 0600); err != nil {
		return fmt.Errorf("failed to write temporary state file: %w", err)
	}

	// Rename temporary file to target path atomically
	if err := os.Rename(tmpFile, sm.statePath); err != nil {
		// Cleanup temporary file on error
		_ = os.Remove(tmpFile)
		return fmt.Errorf("failed to atomically rename state file: %w", err)
	}

	return nil
}
