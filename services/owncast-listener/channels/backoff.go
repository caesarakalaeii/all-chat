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

package channels

import "time"

// Reconnect backoff for per-instance connections.
//
// ADR-0058: an unreachable or offline Owncast instance is a NORMAL state, not
// an error condition. A self-hosted instance goes down, upgrades, or moves
// without telling anyone, so the listener keeps retrying on a capped
// exponential backoff and never crash-loops. The cap (5 minutes) is well past
// the 24h source-manager staleness threshold and well past the liveness
// heartbeat provided by the api-gateway, so slow retries never deactivate a
// watched source.
const (
	backoffBase = 2 * time.Second
	backoffCap  = 5 * time.Minute
)

// nextBackoff returns the wait before the (attempt+1)-th reconnect try.
// attempt starts at 0 for the first retry. Pure function so the
// offline-instance pacing can be unit-tested without a live socket.
func nextBackoff(attempt int) time.Duration {
	if attempt < 0 {
		attempt = 0
	}
	d := backoffBase
	for i := 0; i < attempt && d < backoffCap; i++ {
		d *= 2
	}
	if d > backoffCap {
		d = backoffCap
	}
	return d
}
