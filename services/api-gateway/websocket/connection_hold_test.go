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
	"slices"
	"testing"

	"go.uber.org/zap"
)

// Regression for the status-page "Overlay connection" blips: the socket joins
// the pool before the replay fetch, so a live broadcast during that window was
// written ahead of the connected frame. Held live frames must go out after the
// whole connect burst, in arrival order, and later frames after them.
func TestHoldLive_LiveFramesFollowConnectBurst(t *testing.T) {
	c := &Connection{
		send:   make(chan []byte, 16),
		done:   make(chan struct{}),
		logger: zap.NewNop(),
	}

	c.HoldLive()
	c.Send([]byte("live1"))
	c.SendBlocking([]byte("connected"))
	c.Send([]byte("live2"))
	c.SendBlocking([]byte("replay1"))
	c.ReleaseLive()
	c.Send([]byte("live3"))

	var got []string
	for len(c.send) > 0 {
		got = append(got, string(<-c.send))
	}
	want := []string{"connected", "replay1", "live1", "live2", "live3"}
	if !slices.Equal(got, want) {
		t.Fatalf("frame order = %v, want %v", got, want)
	}
}

// A client that cannot take the burst is dropped once the held frames reach
// the send-buffer bound, as Send does for a full channel, instead of growing
// the held queue without limit.
func TestHoldLive_OverflowClosesConnection(t *testing.T) {
	c := &Connection{
		send:   make(chan []byte, 2),
		done:   make(chan struct{}),
		logger: zap.NewNop(),
	}

	c.HoldLive()
	c.Send([]byte("a"))
	c.Send([]byte("b"))
	if c.Send([]byte("c")) {
		t.Fatal("Send past the hold bound must fail")
	}
	if !c.IsClosed() {
		t.Fatal("connection must close when held frames overflow")
	}
}
