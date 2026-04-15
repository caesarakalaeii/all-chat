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

	"github.com/caesar/all-chat/services/discord-listener/gateway"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// memStateStore is an in-memory SessionStore for resume tests.
// It returns ("", nil) for missing keys — matching the "no session" case.
type memStateStore struct {
	data map[string]string
}

func newMemStateStore(initial map[string]string) *memStateStore {
	s := &memStateStore{data: make(map[string]string)}
	for k, v := range initial {
		s.data[k] = v
	}
	return s
}

func (m *memStateStore) Set(_ context.Context, key, value string) error {
	m.data[key] = value
	return nil
}

func (m *memStateStore) Get(_ context.Context, key string) (string, error) {
	return m.data[key], nil // returns ("", nil) for missing keys
}

// fakeGatewayServer creates an httptest server that upgrades to WebSocket, sends a
// HELLO payload, waits for ONE message from the client, records it, then closes.
// The caller receives the recorded client message via the returned channel.
func fakeGatewayServer(t *testing.T) (*httptest.Server, chan gateway.GatewayPayload) {
	t.Helper()
	clientMsgCh := make(chan gateway.GatewayPayload, 1)

	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		// Send HELLO
		hello := gateway.GatewayPayload{Op: gateway.OpHello}
		helloData, _ := json.Marshal(gateway.HelloData{HeartbeatInterval: 45000})
		hello.D = json.RawMessage(helloData)
		if err := conn.WriteJSON(hello); err != nil {
			return
		}

		// Read the client's first response (IDENTIFY or RESUME)
		_, msg, err := conn.ReadMessage()
		if err != nil {
			return
		}
		var payload gateway.GatewayPayload
		if jsonErr := json.Unmarshal(msg, &payload); jsonErr == nil {
			select {
			case clientMsgCh <- payload:
			default:
			}
		}
		// Close cleanly — Connect() will return on read error
	}))

	return srv, clientMsgCh
}

// wsURLFromHTTP converts an httptest server URL (http://...) to ws://.
func wsURLFromHTTP(url string) string {
	return "ws" + strings.TrimPrefix(url, "http")
}

// TestResumeWhenSessionExists verifies that Connect() sends op=6 RESUME (not op=2 IDENTIFY)
// when the store already contains session_id and resume_gateway_url.
func TestResumeWhenSessionExists(t *testing.T) {
	srv, clientMsgCh := fakeGatewayServer(t)
	defer srv.Close()

	store := newMemStateStore(map[string]string{
		gateway.RedisKeySessionID: "sid1",
		gateway.RedisKeyResumeURL: wsURLFromHTTP(srv.URL), // must point to test server, not real Discord
		gateway.RedisKeySeq:       "42",
	})

	log, _ := zap.NewDevelopment()
	client := gateway.NewGatewayClient(
		"bot-token",
		wsURLFromHTTP(srv.URL),
		store,
		log,
		nil, nil, nil,
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Connect blocks; run in goroutine
	go func() {
		_ = client.Connect(ctx)
	}()

	sent := <-clientMsgCh
	assert.Equal(t, gateway.OpResume, sent.Op, "expected op=6 RESUME when session exists, got op=%d", sent.Op)

	var resumeData gateway.ResumeData
	require.NoError(t, json.Unmarshal(sent.D, &resumeData))
	assert.Equal(t, "bot-token", resumeData.Token)
	assert.Equal(t, "sid1", resumeData.SessionID)
	assert.Equal(t, 42, resumeData.Seq)
}

// TestIdentifyWhenNoSession verifies that Connect() falls back to op=2 IDENTIFY
// when the store is empty (no prior session).
func TestIdentifyWhenNoSession(t *testing.T) {
	srv, clientMsgCh := fakeGatewayServer(t)
	defer srv.Close()

	store := newMemStateStore(nil) // empty store

	log, _ := zap.NewDevelopment()
	client := gateway.NewGatewayClient(
		"bot-token",
		wsURLFromHTTP(srv.URL),
		store,
		log,
		nil, nil, nil,
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = client.Connect(ctx)
	}()

	sent := <-clientMsgCh
	assert.Equal(t, gateway.OpIdentify, sent.Op, "expected op=2 IDENTIFY when no session, got op=%d", sent.Op)
}

// TestInvalidSessionFalseClears verifies that receiving op=9 with d=false clears
// all three Redis session keys (session_id, resume_url, seq).
func TestInvalidSessionFalseClears(t *testing.T) {
	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}

	// Store is populated after server creation so resume_url points to test server.
	store := newMemStateStore(nil)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		// Send HELLO
		hello := gateway.GatewayPayload{Op: gateway.OpHello}
		helloData, _ := json.Marshal(gateway.HelloData{HeartbeatInterval: 45000})
		hello.D = json.RawMessage(helloData)
		_ = conn.WriteJSON(hello)

		// Wait for client's RESUME/IDENTIFY
		conn.ReadMessage() //nolint:errcheck

		// Send op=9 with d=false (not resumable)
		invalidSession := gateway.GatewayPayload{Op: gateway.OpInvalidSession}
		dBytes, _ := json.Marshal(false)
		invalidSession.D = json.RawMessage(dBytes)
		_ = conn.WriteJSON(invalidSession)
		// Close after sending
	}))
	defer srv.Close()

	// Populate store now that the test server URL is available.
	store.data[gateway.RedisKeySessionID] = "sid1"
	store.data[gateway.RedisKeyResumeURL] = wsURLFromHTTP(srv.URL)
	store.data[gateway.RedisKeySeq] = "42"

	log, _ := zap.NewDevelopment()
	client := gateway.NewGatewayClient(
		"bot-token",
		wsURLFromHTTP(srv.URL),
		store,
		log,
		nil, nil, nil,
	)

	// Connect blocks until InvalidSession is received and returns error
	_ = client.Connect(context.Background())

	// All three keys must be cleared
	sessionID, _ := store.Get(context.Background(), gateway.RedisKeySessionID)
	resumeURL, _ := store.Get(context.Background(), gateway.RedisKeyResumeURL)
	seq, _ := store.Get(context.Background(), gateway.RedisKeySeq)

	assert.Equal(t, "", sessionID, "session_id must be cleared on InvalidSession d=false")
	assert.Equal(t, "", resumeURL, "resume_url must be cleared on InvalidSession d=false")
	assert.Equal(t, "", seq, "seq must be cleared on InvalidSession d=false")
}

// TestInvalidSessionTruePreserves verifies that receiving op=9 with d=true preserves
// all three Redis session keys (so the next Connect() attempt can RESUME again).
func TestInvalidSessionTruePreserves(t *testing.T) {
	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}

	store := newMemStateStore(nil)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		hello := gateway.GatewayPayload{Op: gateway.OpHello}
		helloData, _ := json.Marshal(gateway.HelloData{HeartbeatInterval: 45000})
		hello.D = json.RawMessage(helloData)
		_ = conn.WriteJSON(hello)

		conn.ReadMessage() //nolint:errcheck

		// Send op=9 with d=true (resumable — keys should be preserved)
		invalidSession := gateway.GatewayPayload{Op: gateway.OpInvalidSession}
		dBytes, _ := json.Marshal(true)
		invalidSession.D = json.RawMessage(dBytes)
		_ = conn.WriteJSON(invalidSession)
	}))
	defer srv.Close()

	store.data[gateway.RedisKeySessionID] = "sid1"
	store.data[gateway.RedisKeyResumeURL] = wsURLFromHTTP(srv.URL)
	store.data[gateway.RedisKeySeq] = "42"

	log, _ := zap.NewDevelopment()
	client := gateway.NewGatewayClient(
		"bot-token",
		wsURLFromHTTP(srv.URL),
		store,
		log,
		nil, nil, nil,
	)

	_ = client.Connect(context.Background())

	sessionID, _ := store.Get(context.Background(), gateway.RedisKeySessionID)
	resumeURL, _ := store.Get(context.Background(), gateway.RedisKeyResumeURL)
	seq, _ := store.Get(context.Background(), gateway.RedisKeySeq)

	assert.Equal(t, "sid1", sessionID, "session_id must be preserved on InvalidSession d=true")
	assert.Equal(t, wsURLFromHTTP(srv.URL), resumeURL, "resume_url must be preserved on InvalidSession d=true")
	assert.Equal(t, "42", seq, "seq must be preserved on InvalidSession d=true")
}

// TestReconnectPreservesSession verifies that op=7 Reconnect does NOT modify Redis keys.
func TestReconnectPreservesSession(t *testing.T) {
	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}

	store := newMemStateStore(nil)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		hello := gateway.GatewayPayload{Op: gateway.OpHello}
		helloData, _ := json.Marshal(gateway.HelloData{HeartbeatInterval: 45000})
		hello.D = json.RawMessage(helloData)
		_ = conn.WriteJSON(hello)

		conn.ReadMessage() //nolint:errcheck

		// Send op=7 Reconnect
		reconnect := gateway.GatewayPayload{Op: gateway.OpReconnect}
		_ = conn.WriteJSON(reconnect)
	}))
	defer srv.Close()

	store.data[gateway.RedisKeySessionID] = "sid-reconnect"
	store.data[gateway.RedisKeyResumeURL] = wsURLFromHTTP(srv.URL)
	store.data[gateway.RedisKeySeq] = "99"

	log, _ := zap.NewDevelopment()
	client := gateway.NewGatewayClient(
		"bot-token",
		wsURLFromHTTP(srv.URL),
		store,
		log,
		nil, nil, nil,
	)

	_ = client.Connect(context.Background())

	sessionID, _ := store.Get(context.Background(), gateway.RedisKeySessionID)
	resumeURL, _ := store.Get(context.Background(), gateway.RedisKeyResumeURL)
	seq, _ := store.Get(context.Background(), gateway.RedisKeySeq)

	assert.Equal(t, "sid-reconnect", sessionID, "session_id must be preserved on Reconnect")
	assert.Equal(t, wsURLFromHTTP(srv.URL), resumeURL, "resume_url must be preserved on Reconnect")
	assert.Equal(t, "99", seq, "seq must be preserved on Reconnect")
}
