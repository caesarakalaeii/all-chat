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

package gateway_test

import (
	"testing"
	"time"

	"github.com/caesar/all-chat/services/discord-listener/gateway"
)

// TestAwaitClientMessage_ReportsErrorWhenNoHandshakeArrives pins the bound that keeps a
// broken handshake from becoming a ten-minute CI hang.
//
// WHY THIS EXISTS: fakeGatewayServer's handler has three silent `return` paths — a
// failed WebSocket upgrade, a failed HELLO write and a failed read of the client's
// first frame. On any of them nothing is ever sent to clientMsgCh. The resume tests
// used to consume that channel with a bare `sent := <-clientMsgCh`, which has no
// timeout, so one unlucky handshake blocked the test until Go's default 10-minute
// package timeout fired and the whole `test (discord-listener)` job failed with only
// "Process completed with exit code 1" to show for it. Observed on CI as a 652s job on
// run 32640274074 and a 667s job on run 32003547058 — the latter on a dependabot
// branch that touched only two files under services/twitch-eventsub-listener, which is
// what established the hang as pre-existing rather than caused by any one change.
//
// This test drives the empty-channel path directly, because the race that empties it
// in production is timing-dependent and does not reproduce on demand.
func TestAwaitClientMessage_ReportsErrorWhenNoHandshakeArrives(t *testing.T) {
	neverFed := make(chan gateway.GatewayPayload, 1)

	start := time.Now()
	_, err := awaitClientMessage(neverFed, 200*time.Millisecond)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("an unfed handshake channel must be reported as an error, not treated as a received frame")
	}
	if elapsed > 5*time.Second {
		t.Fatalf("the wait took %s: it must fail fast rather than burn Go's package timeout on CI", elapsed)
	}
}

// TestAwaitClientMessage_ReturnsTheFrameWhenOneArrives is the positive half: the bound
// must not swallow a frame that did arrive.
func TestAwaitClientMessage_ReturnsTheFrameWhenOneArrives(t *testing.T) {
	fed := make(chan gateway.GatewayPayload, 1)
	fed <- gateway.GatewayPayload{Op: gateway.OpResume}

	sent, err := awaitClientMessage(fed, 200*time.Millisecond)
	if err != nil {
		t.Fatalf("a frame was queued, so the wait must return it: %v", err)
	}
	if sent.Op != gateway.OpResume {
		t.Fatalf("returned op = %d, want %d", sent.Op, gateway.OpResume)
	}
}
