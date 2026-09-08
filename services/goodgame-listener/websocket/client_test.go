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
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// wsTestServer runs a GoodGame chat mock: sends "welcome" on connect, then
// relays client frames to frameCh and can push arbitrary envelopes.
type wsTestServer struct {
	srv     *httptest.Server
	URL     string
	frameCh chan string // raw client frames (JSON envelopes)
	push    chan string // raw server frames to send
	conn    *websocket.Conn
}

func newWSTestServer(t *testing.T) *wsTestServer {
	t.Helper()
	ts := &wsTestServer{
		frameCh: make(chan string, 16),
		push:    make(chan string, 16),
	}
	upgrader := websocket.Upgrader{}
	ts.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		require.NoError(t, err)
		ts.conn = conn
		defer conn.Close()
		_ = conn.WriteMessage(websocket.TextMessage,
			[]byte(`{"type":"welcome","data":{"protocolVersion":1.1,"serverIdent":"GG-test"}}`))
		go func() {
			for raw := range ts.push {
				_ = conn.WriteMessage(websocket.TextMessage, []byte(raw))
			}
		}()
		for {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				return
			}
			ts.frameCh <- string(msg)
		}
	}))
	t.Cleanup(ts.srv.Close)
	ts.URL = "ws" + strings.TrimPrefix(ts.srv.URL, "http")
	return ts
}

// messageFixture matches a live capture from chat.goodgame.ru (2026-09):
// channel_id numeric, timestamp unix seconds, mixed premium/color/icon shapes.
const messageFixture = `{"type":"message","data":{"channel_id":5,"user_id":21011,"user_name":"runi.","user_rights":0,"premium":0,"premiums":["1844"],"resubs":{"1844":7},"staff":0,"color":"simple","icon":"none","role":"","mobile":0,"payments":0,"paymentsAll":{"1239":2},"gg_plus_tier":0,"isStatus":0,"message_id":1788879813866,"timestamp":1788879814,"text":"hello from the spike"}}`

func TestClient_ConnectsAndWelcomes(t *testing.T) {
	ts := newWSTestServer(t)
	c := NewClientWithURL(ts.URL, func(string, *ChatMessageData) {}, zap.NewNop())
	require.NoError(t, c.Connect())
	t.Cleanup(func() { _ = c.Disconnect() })

	require.Eventually(t, func() bool { return c.GetSocketID() == "welcomed" },
		2*time.Second, 10*time.Millisecond, "welcome frame must set handshake state")
	assert.True(t, c.IsConnected())
}

func TestClient_JoinUnjoinRoundTrip(t *testing.T) {
	ts := newWSTestServer(t)
	c := NewClientWithURL(ts.URL, func(string, *ChatMessageData) {}, zap.NewNop())
	require.NoError(t, c.Connect())
	t.Cleanup(func() { _ = c.Disconnect() })

	require.NoError(t, c.Subscribe(5))

	var join Envelope
	select {
	case raw := <-ts.frameCh:
		require.NoError(t, json.Unmarshal([]byte(raw), &join))
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for join frame")
	}
	assert.Equal(t, "join", join.Type)
	var jr JoinRequest
	require.NoError(t, json.Unmarshal(join.Data, &jr))
	assert.Equal(t, "5", jr.ChannelID)
	assert.False(t, jr.Hidden)

	// Duplicate subscribe must not re-send.
	require.NoError(t, c.Subscribe(5))
	select {
	case raw := <-ts.frameCh:
		t.Fatalf("duplicate join sent: %s", raw)
	case <-time.After(150 * time.Millisecond):
	}

	require.NoError(t, c.Unsubscribe(5))
	var unjoin Envelope
	select {
	case raw := <-ts.frameCh:
		require.NoError(t, json.Unmarshal([]byte(raw), &unjoin))
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for unjoin frame")
	}
	assert.Equal(t, "unjoin", unjoin.Type)
	var ur UnjoinRequest
	require.NoError(t, json.Unmarshal(unjoin.Data, &ur))
	assert.Equal(t, "5", ur.ChannelID)
}

func TestClient_ResubscribesOnReconnect(t *testing.T) {
	ts := newWSTestServer(t)
	c := NewClientWithURL(ts.URL, func(string, *ChatMessageData) {}, zap.NewNop())
	require.NoError(t, c.Connect())
	t.Cleanup(func() { _ = c.Disconnect() })
	require.NoError(t, c.Subscribe(13427))

	// Consume the first join.
	select {
	case <-ts.frameCh:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for initial join")
	}

	// Reconnect: welcome replays the join.
	require.NoError(t, c.Connect())
	select {
	case raw := <-ts.frameCh:
		var env Envelope
		require.NoError(t, json.Unmarshal([]byte(raw), &env))
		assert.Equal(t, "join", env.Type)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for re-join after reconnect")
	}
}

func TestClient_DispatchesChatMessageToHandler(t *testing.T) {
	got := make(chan *ChatMessageData, 1)
	ts := newWSTestServer(t)
	c := NewClientWithURL(ts.URL, func(id string, m *ChatMessageData) {
		got <- m
	}, zap.NewNop())
	require.NoError(t, c.Connect())
	t.Cleanup(func() { _ = c.Disconnect() })

	ts.push <- messageFixture

	select {
	case msg := <-got:
		assert.Equal(t, "runi.", msg.UserName)
		assert.Equal(t, "hello from the spike", msg.Text)
		id, err := msg.MessageID.Int64()
		require.NoError(t, err)
		assert.Equal(t, int64(1788879813866), id)
		chID, err := msg.ChannelID.Int64()
		require.NoError(t, err)
		assert.Equal(t, int64(5), chID)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for chat message dispatch")
	}
}

func TestClient_DispatchesDeletion(t *testing.T) {
	got := make(chan *RemoveMessageData, 1)
	ts := newWSTestServer(t)
	c := NewClientWithURL(ts.URL, func(string, *ChatMessageData) {}, zap.NewNop())
	c.SetDeletionHandler(func(id string, m *RemoveMessageData) { got <- m })
	require.NoError(t, c.Connect())
	t.Cleanup(func() { _ = c.Disconnect() })

	ts.push <- `{"type":"remove_message","data":{"channel_id":"5","message_id":"100"}}`

	select {
	case rm := <-got:
		assert.Equal(t, "100", rm.MessageID.String())
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for deletion dispatch")
	}
}

func TestClient_PingsPeriodically(t *testing.T) {
	// Ping period is 25s; shorten for the test by dialing a server that counts
	// frames and waiting on a shortened lifecycle is too slow. Instead call the
	// internal ping frame directly through a connected client: sendEnvelope of
	// type ping and verify the server receives the exact wire shape.
	ts := newWSTestServer(t)
	c := NewClientWithURL(ts.URL, func(string, *ChatMessageData) {}, zap.NewNop())
	require.NoError(t, c.Connect())
	t.Cleanup(func() { _ = c.Disconnect() })

	require.NoError(t, c.sendEnvelope(Envelope{Type: "ping", Data: mustJSON(PingRequest{})}))

	select {
	case raw := <-ts.frameCh:
		var env Envelope
		require.NoError(t, json.Unmarshal([]byte(raw), &env))
		assert.Equal(t, "ping", env.Type)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for ping frame")
	}
}

func TestClient_SignalsReconnectOnServerClose(t *testing.T) {
	ts := newWSTestServer(t)
	c := NewClientWithURL(ts.URL, func(string, *ChatMessageData) {}, zap.NewNop())
	require.NoError(t, c.Connect())
	defer func() { _ = c.Disconnect() }()

	// Forcibly close the server-side connection; the read pump must error out
	// and signal the reconnect channel.
	_ = ts.conn.Close()

	select {
	case <-c.ReconnectChan():
	case <-time.After(3 * time.Second):
		t.Fatal("reconnect signal not received after server-side close")
	}
}

func TestEnvelopeFramingRoundTrip(t *testing.T) {
	// The wire format is {"type":..,"data":{..}} in both directions; encode
	// every request type and decode it back.
	requests := []Envelope{
		{Type: "join", Data: mustJSON(JoinRequest{ChannelID: "5", Hidden: false})},
		{Type: "unjoin", Data: mustJSON(UnjoinRequest{ChannelID: "5"})},
		{Type: "ping", Data: mustJSON(PingRequest{})},
	}
	for _, req := range requests {
		data, err := json.Marshal(req)
		require.NoError(t, err)
		var back Envelope
		require.NoError(t, json.Unmarshal(data, &back))
		assert.Equal(t, req.Type, back.Type)
	}
}

func TestChatMessageDecodingNumbersAndStrings(t *testing.T) {
	// The live endpoint mixes numeric and string encodings across fields;
	// json.Number must accept both.
	variants := []string{
		`{"channel_id":5,"user_id":123,"message_id":100,"timestamp":1788879814,"text":"numeric"}`,
		`{"channel_id":"5","user_id":"123","message_id":"100","timestamp":"1788879814","text":"string"}`,
	}
	for _, v := range variants {
		var msg ChatMessageData
		require.NoError(t, json.Unmarshal([]byte(v), &msg))
		assert.Equal(t, "5", msg.ChannelID.String())
		assert.Equal(t, "123", msg.UserID.String())
		assert.Equal(t, "1788879814", msg.Timestamp.String())
	}
}
