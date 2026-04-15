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
