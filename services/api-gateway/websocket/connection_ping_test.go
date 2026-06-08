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
	"encoding/json"
	"testing"
	"time"

	"github.com/caesar/all-chat/services/api-gateway/models"
	"go.uber.org/zap"
)

// newTestConnection builds a Connection without a real socket. handleMessage
// only touches c.send (via c.Send) and the logger, so a buffered channel and a
// nop logger are enough to exercise the message-handling branches in isolation.
func newTestConnection() *Connection {
	return &Connection{
		send:   make(chan []byte, 8),
		logger: zap.NewNop(),
	}
}

// readSend returns the next queued outbound frame, or fails if none arrives.
func readSend(t *testing.T, c *Connection) models.WSMessage {
	t.Helper()
	select {
	case raw := <-c.send:
		var msg models.WSMessage
		if err := json.Unmarshal(raw, &msg); err != nil {
			t.Fatalf("queued frame is not a WSMessage: %v", err)
		}
		return msg
	case <-time.After(time.Second):
		t.Fatal("expected an outbound frame, got none")
		return models.WSMessage{}
	}
}

// A client liveness probe (app-level ping) must be answered with a pong so the
// client's heartbeat round-trips even on an idle channel with no chat flowing.
func TestHandleMessage_ClientPingRepliesPong(t *testing.T) {
	c := newTestConnection()

	ping, _ := json.Marshal(models.NewPing())
	c.handleMessage(ping)

	reply := readSend(t, c)
	if reply.Type != models.WSMessageTypePong {
		t.Fatalf("expected pong reply to client ping, got %q", reply.Type)
	}
}

// A client pong (reply to the server's heartbeat, if any) is informational only
// and must not generate a reply — otherwise two heartbeating peers ping-pong
// forever.
func TestHandleMessage_ClientPongIsSilent(t *testing.T) {
	c := newTestConnection()

	pong, _ := json.Marshal(models.NewPong())
	c.handleMessage(pong)

	select {
	case raw := <-c.send:
		t.Fatalf("client pong should not be answered, but got a frame: %s", raw)
	case <-time.After(100 * time.Millisecond):
		// expected: nothing queued
	}
}
