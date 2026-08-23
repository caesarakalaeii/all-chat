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

package websocket

import (
	"testing"

	"go.uber.org/zap"
)

// A mod_action frame as the message-processor publishes it on overlay:{id}.
// held_text is the full text of a message AutoMod withheld from chat — nobody
// but the owner has ever seen it.
const modActionFrame = `{"id":"m1","platform":"twitch","event":{"type":"mod_action",` +
	`"metadata":{"action":"automod_hold","moderator_login":"mod","target_login":"viewer",` +
	`"held_text":"secret held text","automod_category":"harassment"}}}`

const chatFrame = `{"id":"m2","platform":"twitch","data":{"content":"hello"}}`

// newPoolTestConnection builds an owner/anonymous Connection without a real
// socket. BroadcastFiltered only touches the classification accessors and
// Send(), so a buffered send channel and a nop logger suffice.
func newPoolTestConnection(userID string) *Connection {
	return NewConnection(nil, "ov", userID, nil, zap.NewNop())
}

// receivedFrame returns the frame queued on conn, or "" if none was queued.
// The read is non-blocking rather than timed: Send enqueues into the buffered
// channel synchronously, so BroadcastFiltered has already delivered everything
// it is going to by the time it returns.
func receivedFrame(t *testing.T, conn *Connection) string {
	t.Helper()
	select {
	case raw := <-conn.send:
		return string(raw)
	default:
		return ""
	}
}

// The public overlay socket path accepts OBS browser sources with no token at
// all, so a mod_action frame — which carries the full held text of an
// AutoMod-blocked message — must reach only sockets that proved ownership.
// Ordinary chat still goes to everyone.
func TestBroadcastFiltered_ModFrameReachesOnlyOwnerConnections(t *testing.T) {
	pool := NewPool("ov", zap.NewNop())

	owner := newPoolTestConnection("owner-user")
	owner.SetOwner(true)
	anonymous := newPoolTestConnection("obs")
	viewer := NewViewerConnection(nil, "ov", "viewer-user", nil, zap.NewNop())

	pool.Add(owner)
	pool.Add(anonymous)
	pool.Add(viewer)

	if sent := pool.BroadcastFiltered([]byte(chatFrame), BroadcastFilter{}); sent != 3 {
		t.Fatalf("ordinary chat frame should reach all 3 connections, reached %d", sent)
	}
	for name, conn := range map[string]*Connection{"owner": owner, "anonymous": anonymous, "viewer": viewer} {
		if got := receivedFrame(t, conn); got == "" {
			t.Errorf("%s connection received no ordinary chat frame", name)
		}
	}

	if sent := pool.BroadcastFiltered([]byte(modActionFrame), BroadcastFilter{OwnerOnly: true}); sent != 1 {
		t.Fatalf("mod_action frame should reach only the owner connection, reached %d", sent)
	}
	if got := receivedFrame(t, owner); got == "" {
		t.Error("owner connection received no mod_action frame; the owner's mod log would be empty")
	}
	if got := receivedFrame(t, anonymous); got != "" {
		t.Errorf("anonymous connection received a mod_action frame: an AutoMod held message would reach an OBS browser source: %s", got)
	}
	if got := receivedFrame(t, viewer); got != "" {
		t.Errorf("viewer connection received a mod_action frame: an AutoMod held message would reach a public viewer: %s", got)
	}
}

// The owner and engagement-only filters compose: an owner's participate widget
// asked for poll/prediction frames only, so it must not receive mod frames
// either.
func TestBroadcastFiltered_EngagementOnlyOwnerGetsNoModFrame(t *testing.T) {
	pool := NewPool("ov", zap.NewNop())

	owner := newPoolTestConnection("owner-user")
	owner.SetOwner(true)
	owner.SetEngagementOnly(true)
	pool.Add(owner)

	if sent := pool.BroadcastFiltered([]byte(modActionFrame), BroadcastFilter{OwnerOnly: true}); sent != 0 {
		t.Fatalf("mod_action frame should not reach an engagement-only socket, reached %d", sent)
	}
	if got := receivedFrame(t, owner); got != "" {
		t.Errorf("engagement-only owner socket received a mod_action frame: %s", got)
	}
}
