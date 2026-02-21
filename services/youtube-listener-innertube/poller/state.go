package poller

import (
	"sync"
	"time"
)

// StreamState represents the current state of the polling loop
type StreamState string

const (
	StateActive  StreamState = "active"  // Polling normally
	StateFailed  StreamState = "failed"  // Fatal error, stopped
	StateOffline StreamState = "offline" // Stream ended
)

// State tracks the current polling state with thread-safe access
type State struct {
	Current      StreamState
	LastError    error
	LastPollTime time.Time
	mu           sync.RWMutex
}

// NewState creates a new state tracker initialized to Active
func NewState() *State {
	return &State{
		Current:      StateActive,
		LastPollTime: time.Now(),
	}
}

// SetState updates the current state (thread-safe)
func (s *State) SetState(state StreamState) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Current = state
}

// GetState returns the current state (thread-safe)
func (s *State) GetState() StreamState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.Current
}

// SetError records the last error encountered
func (s *State) SetError(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.LastError = err
}

// GetError returns the last error (thread-safe)
func (s *State) GetError() error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.LastError
}

// UpdatePollTime records the last successful poll time
func (s *State) UpdatePollTime() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.LastPollTime = time.Now()
}

// GetLastPollTime returns the last poll time (thread-safe)
func (s *State) GetLastPollTime() time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.LastPollTime
}
