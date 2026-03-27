package listener

import (
	"os"
	"strconv"
	"time"
)

// ListenerConfig holds configurable parameters for ListenerBase.
// All fields have sensible defaults; use DefaultConfig() to start.
type ListenerConfig struct {
	// HeartbeatInterval controls how often PublishHeartbeat is called.
	// Default: 10s — must be strictly less than the source-manager HeartbeatTimeout (15s)
	// to prevent pods from being falsely detected as failed between heartbeats.
	// Previously 30s, which caused intermittent "no pods available" assignments.
	HeartbeatInterval time.Duration

	// AssignmentRefreshInterval controls how often QueryAssignments is called.
	// Default: 10s.
	AssignmentRefreshInterval time.Duration

	// StartupJitterMax is the upper bound for random startup delay.
	// Set to 0 to disable jitter (recommended in tests).
	// If 0, the LISTENER_STARTUP_JITTER_MAX env var is NOT read — jitter is fully disabled.
	// Default: 30s (env var LISTENER_STARTUP_JITTER_MAX can override at runtime when using DefaultConfig).
	StartupJitterMax time.Duration

	// DisableCoordinatorFiltering skips assignment-based filtering in UpdateAssignedSourceIDs.
	// Preserves the operational rollback mechanism from twitch-listener (SDK-06).
	DisableCoordinatorFiltering bool

	// OnFatalError is called when a background goroutine encounters a permanent failure.
	// sourceName is the goroutine name ("heartbeat", "assignment-refresh", "migration-subscriber").
	// If nil, goroutines retry indefinitely with exponential backoff (backward-compatible behavior).
	OnFatalError func(source string, err error)

	// Platform is the platform identifier for this listener (e.g. "kick", "twitch", "youtube").
	// When set, the migration subscriber will silently drop events for other platforms,
	// eliminating cross-platform log noise and unnecessary handler invocations.
	// The demand subscriber loop also uses this to filter incoming demand updates to only
	// sources matching this platform.
	Platform string

	// DisableDemandFiltering treats all assigned sources as having demand.
	// When true, the SDK demand subscriber loop exits immediately without subscribing
	// to source:demand Pub/Sub. Use for backward compatibility during rollout or for
	// listeners that should always connect to all assigned sources (e.g., twitch IRC).
	DisableDemandFiltering bool
}

// DefaultConfig returns a ListenerConfig with production-ready defaults.
// Reads LISTENER_STARTUP_JITTER_MAX from env (integer seconds); 0 or invalid disables jitter.
func DefaultConfig() ListenerConfig {
	jitterMax := 30 * time.Second
	if raw := os.Getenv("LISTENER_STARTUP_JITTER_MAX"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n <= 0 {
			jitterMax = 0
		} else {
			jitterMax = time.Duration(n) * time.Second
		}
	}
	return ListenerConfig{
		HeartbeatInterval:         10 * time.Second,
		AssignmentRefreshInterval: 10 * time.Second,
		StartupJitterMax:          jitterMax,
	}
}

// Env returns the value of the environment variable key,
// or defaultValue if the variable is unset or empty.
// Drop-in replacement for the getEnvOrDefault pattern used across all listeners.
func Env(key, defaultValue string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultValue
}
