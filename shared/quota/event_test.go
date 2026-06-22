// This file is part of All-Chat.
// Copyright (C) 2026 caesarakalaeii
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

package quota

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestQuotaEvent_JSONContract locks the wire format the discord-bot
// (services/discord-bot/src/index.js) destructures. If any json tag changes,
// the bot's embeds break silently — this test fails first.
func TestQuotaEvent_JSONContract(t *testing.T) {
	prev := QuotaStateCritical
	ev := QuotaEvent{
		Type:             EventStateChanged,
		Timestamp:        time.Unix(0, 0).UTC(),
		GlobalState:      QuotaStateExhausted,
		UsagePercentage:  96.5,
		UnitsUsed:        973685,
		UnitsLimit:       1009000,
		UnitsRemaining:   35315,
		PreviousState:    &prev,
		AffectedChannels: []string{"UC123"},
		Message:          "Quota crossed 95% threshold, now at 96.50%",
		Severity:         "error",
	}

	raw, err := json.Marshal(ev)
	require.NoError(t, err)

	var m map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(raw, &m))

	for _, key := range []string{
		"type", "timestamp", "global_state", "usage_percentage",
		"units_used", "units_limit", "units_remaining",
		"previous_state", "affected_channels", "message", "severity",
	} {
		assert.Containsf(t, m, key, "QuotaEvent JSON must contain %q for the discord-bot", key)
	}

	// usage_percentage must serialize as a JSON number (the bot calls .toFixed()).
	assert.JSONEq(t, `96.5`, string(m["usage_percentage"]))
	assert.JSONEq(t, `"EXHAUSTED"`, string(m["global_state"]))
	assert.JSONEq(t, `"state_changed"`, string(m["type"]))
}

// TestQuotaEvent_OmitsEmptyOptionals verifies previous_state / affected_channels
// are omitted when unset (they are marked omitempty).
func TestQuotaEvent_OmitsEmptyOptionals(t *testing.T) {
	ev := QuotaEvent{Type: EventThresholdCrossed, GlobalState: QuotaStateDegraded, Severity: "warning"}
	raw, err := json.Marshal(ev)
	require.NoError(t, err)

	var m map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(raw, &m))
	assert.NotContains(t, m, "previous_state")
	assert.NotContains(t, m, "affected_channels")
}
