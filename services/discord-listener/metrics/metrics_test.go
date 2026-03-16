package metrics_test

import (
	"testing"

	"github.com/caesar/all-chat/services/discord-listener/metrics"
)

// TestMetricRegistration verifies that all four discord-listener metrics are
// registered successfully (promauto panics on duplicate registration) and that
// their exported setter/increment functions can be called without panic.
func TestMetricRegistration(t *testing.T) {
	// These calls exercise promauto registration and exported API surface.
	// Any panic here (e.g. duplicate registration) will fail the test.
	metrics.IncGatewayEvent("MESSAGE_CREATE")
	metrics.SetActiveGuilds(3)
	metrics.SetShardOwnership(1)
	metrics.IncResumeAttempt("success")
}

// TestShardOwnershipToggle verifies that SetShardOwnership can be toggled
// between 1 (held) and 0 (not held) without panic.
func TestShardOwnershipToggle(t *testing.T) {
	metrics.SetShardOwnership(1)
	metrics.SetShardOwnership(0)
}
