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
	"fmt"
	"sync"
	"testing"
	"time"

	"go.uber.org/zap"
)

// Regression for the "test overlay flapping" loop: a reconnect replays the
// whole chat buffer (up to 500 entries) into the send channel. With the
// fire-and-drop Send() that closes on a full channel, a burst larger than the
// channel buffer self-closed the connection mid-replay — the client received
// nothing, never advanced its ?since= watermark, reconnected, and flapped
// forever. SendBlocking must apply backpressure (block until the writer drains)
// instead of dropping, so the full burst is delivered intact.
func TestSendBlocking_DeliversFullBurstWithoutDropping(t *testing.T) {
	c := &Connection{
		send:   make(chan []byte, 4), // deliberately smaller than the burst
		done:   make(chan struct{}),
		logger: zap.NewNop(),
	}

	const burst = 64 // >> channel buffer, mirrors replay (500) > send buffer (256)
	received := make([]string, 0, burst)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() { // stand-in for writePump draining at the client's pace
		defer wg.Done()
		for i := 0; i < burst; i++ {
			select {
			case msg := <-c.send:
				received = append(received, string(msg))
				time.Sleep(time.Millisecond) // simulate a slowish client write
			case <-time.After(2 * time.Second):
				return
			}
		}
	}()

	for i := 0; i < burst; i++ {
		if ok := c.SendBlocking([]byte(fmt.Sprintf("m%d", i))); !ok {
			t.Fatalf("SendBlocking dropped message %d; replay must block, not drop", i)
		}
	}
	wg.Wait()

	if len(received) != burst {
		t.Fatalf("expected %d messages delivered, got %d", burst, len(received))
	}
	if c.IsClosed() {
		t.Fatal("connection must stay open after a full replay burst")
	}
}

// A blocked SendBlocking must unblock and report failure when the connection
// closes underneath it (e.g. a dead client whose writePump exited) — never
// deadlock and never panic on a closed channel.
func TestSendBlocking_ReturnsFalseWhenClosedWhileBlocked(t *testing.T) {
	c := &Connection{
		send:   make(chan []byte, 1),
		done:   make(chan struct{}),
		logger: zap.NewNop(),
	}
	c.send <- []byte("fill the only slot") // next send has nowhere to go

	go func() {
		time.Sleep(20 * time.Millisecond)
		c.Close() // no drainer running; close must release the blocked sender
	}()

	done := make(chan bool, 1)
	go func() { done <- c.SendBlocking([]byte("blocked")) }()

	select {
	case ok := <-done:
		if ok {
			t.Fatal("SendBlocking should report failure when the connection closes mid-send")
		}
	case <-time.After(time.Second):
		t.Fatal("SendBlocking deadlocked instead of unblocking on Close")
	}
}
