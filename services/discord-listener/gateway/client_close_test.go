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
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/caesar/all-chat/services/discord-listener/gateway"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

// TestClose_UnblocksConnectWaitingOnRead is the root cause of the intermittent
// ten-minute `test (discord-listener)` hang.
//
// Close() signals the `done` channel, but Connect() only observes `done` in the select
// at the top of its read loop — by the time Close() is called it is normally parked in
// a blocking conn.ReadMessage(), which no channel can interrupt. Nothing in Close()
// touches c.conn, so the read is only released as a side effect of cancelling the
// context that was passed to DialContext. A caller that follows the documented
// contract — Close() to stop the session, as the leadership-loss path does — is
// therefore not guaranteed to get Connect() to return at all.
//
// This test holds the server silent so Connect() is certain to be blocked in
// ReadMessage, then calls ONLY Close(), with the context left live. Before the fix
// Connect() never returns and this test reports the hang in seconds instead of letting
// the package burn Go's ten-minute timeout.
//
// Found from a SIGQUIT stack dump of a real hang: the parked test goroutine was
// TestGatewayClient_ReconnectAfterClose at client_test.go:138, the unguarded `<-done1`
// after `client.Close(); cancel1()`, with Connect still in ReadMessage at
// client.go:209 and the server handler in its own ReadMessage. Reproduces about 1 run
// in 20 of the gateway package on a four-core machine.
func TestClose_UnblocksConnectWaitingOnRead(t *testing.T) {
	upgrader := websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
	connected := make(chan struct{}, 1)
	releaseHandler := make(chan struct{})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		hello := gateway.GatewayPayload{Op: gateway.OpHello}
		helloData, _ := json.Marshal(gateway.HelloData{HeartbeatInterval: 45000})
		hello.D = json.RawMessage(helloData)
		if err := conn.WriteJSON(hello); err != nil {
			return
		}

		// Consume the client's IDENTIFY/RESUME, then go silent and hold the socket
		// open. Connect() is now blocked in ReadMessage with nothing to read.
		if _, _, err := conn.ReadMessage(); err != nil {
			return
		}
		select {
		case connected <- struct{}{}:
		default:
		}
		<-releaseHandler
	}))
	defer srv.Close()
	defer close(releaseHandler)

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")
	store := &MockRedis{store: make(map[string]string)}
	log, _ := zap.NewDevelopment()
	client := gateway.NewGatewayClient("tok", wsURL, store, log, nil, nil, nil)

	// The context stays live for the whole test: Close() alone must be enough.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	connectReturned := make(chan error, 1)
	go func() {
		connectReturned <- client.Connect(ctx)
	}()

	select {
	case <-connected:
	case <-time.After(5 * time.Second):
		t.Fatal("handshake did not complete, so the read-blocked state under test was never reached")
	}

	client.Close()

	select {
	case <-connectReturned:
	case <-time.After(5 * time.Second):
		t.Fatal("Close() did not make a read-blocked Connect() return: it closes the done channel " +
			"but never the socket, so the blocking ReadMessage is left parked. This is the " +
			"intermittent ten-minute hang in test (discord-listener).")
	}
}

// TestConnect_ReturnsWhenContextCancelledDuringRead is the other half of the same
// defect, on the path where the caller cancels the context instead of calling Close().
//
// Connect() checks ctx.Done() only between reads, so a cancellation that arrives while
// it is parked in conn.ReadMessage() is not observed. Today the read happens to break
// because gorilla ties the socket to the context passed to DialContext, but that is a
// side effect of how the connection was dialled, not something Connect() guarantees.
// The remaining rare hang after fixing Close() was exactly this path: a SIGQUIT dump
// caught the second Connect() of TestGatewayClient_ReconnectAfterClose parked at
// client_test.go:156, the `<-done2` after `cancel2()`, once in 200 package runs.
//
// Cancellation must stop the read loop on its own so the guarantee does not depend on
// the dialer.
func TestConnect_ReturnsWhenContextCancelledDuringRead(t *testing.T) {
	upgrader := websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
	connected := make(chan struct{}, 1)
	releaseHandler := make(chan struct{})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		hello := gateway.GatewayPayload{Op: gateway.OpHello}
		helloData, _ := json.Marshal(gateway.HelloData{HeartbeatInterval: 45000})
		hello.D = json.RawMessage(helloData)
		if err := conn.WriteJSON(hello); err != nil {
			return
		}
		if _, _, err := conn.ReadMessage(); err != nil {
			return
		}
		select {
		case connected <- struct{}{}:
		default:
		}
		<-releaseHandler
	}))
	defer srv.Close()
	defer close(releaseHandler)

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")
	store := &MockRedis{store: make(map[string]string)}
	log, _ := zap.NewDevelopment()
	client := gateway.NewGatewayClient("tok", wsURL, store, log, nil, nil, nil)

	ctx, cancel := context.WithCancel(context.Background())
	connectReturned := make(chan error, 1)
	go func() {
		connectReturned <- client.Connect(ctx)
	}()

	select {
	case <-connected:
	case <-time.After(5 * time.Second):
		t.Fatal("handshake did not complete, so the read-blocked state under test was never reached")
	}

	cancel()

	select {
	case <-connectReturned:
	case <-time.After(5 * time.Second):
		t.Fatal("cancelling the context did not make a read-blocked Connect() return: the read loop " +
			"only checks ctx.Done() between reads, so the blocking ReadMessage stayed parked")
	}
}
