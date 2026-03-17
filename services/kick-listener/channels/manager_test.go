package channels_test

import (
	"strings"
	"testing"
)

// TestManager_SourceIDNormalization verifies that the strip-at-intake pattern
// correctly reduces composite coordinator IDs (e.g. "abc123:kick") to bare UUIDs.
// This is the behavior that must hold in cmd/main.go before passing assignedSourceIDs
// to channels.NewManager.
func TestManager_SourceIDNormalization(t *testing.T) {
	coordinatorSourceIDs := []string{
		"d5e6f7a8-0000-0000-0000-000000000001:kick",
		"d5e6f7a8-0000-0000-0000-000000000002:kick",
		"d5e6f7a8-0000-0000-0000-000000000003", // already bare — no colon
	}

	assignedIDs := make(map[string]bool)
	for _, raw := range coordinatorSourceIDs {
		sourceID := raw
		if colonIdx := strings.LastIndexByte(sourceID, ':'); colonIdx != -1 {
			sourceID = sourceID[:colonIdx]
		}
		assignedIDs[sourceID] = true
	}

	// Bare UUIDs must be present
	if !assignedIDs["d5e6f7a8-0000-0000-0000-000000000001"] {
		t.Error("bare UUID 1 should be in map")
	}
	if !assignedIDs["d5e6f7a8-0000-0000-0000-000000000002"] {
		t.Error("bare UUID 2 should be in map")
	}
	if !assignedIDs["d5e6f7a8-0000-0000-0000-000000000003"] {
		t.Error("already-bare UUID should be in map")
	}

	// Suffixed keys must NOT be present
	if assignedIDs["d5e6f7a8-0000-0000-0000-000000000001:kick"] {
		t.Error("suffixed key must not be in map")
	}
	if assignedIDs["d5e6f7a8-0000-0000-0000-000000000002:kick"] {
		t.Error("suffixed key must not be in map")
	}
}
