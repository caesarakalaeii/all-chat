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
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func testLogger() zapShim { return loggerAdapter{zap.NewNop()} }

func testZapLogger() *zap.Logger { return zap.NewNop() }

// Fixtures below pin the wire shapes against live captures of
// web7.rumble.com/chat/api/chat/445201096/stream taken 2026-09-08, and the
// pop-up chat client bundle (rumble.com/chat/popup) for rant/deletion fields.

func TestValidateChatID(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"445201096", true},
		{"1", true},
		{"0", false},
		{"0123", false},
		{"", false},
		{"v7f8kam", false},
		{"44520a96", false},
		{"44 520196", false},
		{"-123", false},
	}
	for _, tc := range cases {
		assert.Equal(t, tc.want, validateChatID(tc.in), "input %q", tc.in)
	}
}

func TestChatStreamURL(t *testing.T) {
	url, err := chatStreamURL("", "445201096")
	require.NoError(t, err)
	assert.Equal(t, "https://web7.rumble.com/chat/api/chat/445201096/stream", url)

	_, err = chatStreamURL("", "not-a-number")
	assert.Error(t, err)

	_, err = chatStreamURL("", "")
	assert.Error(t, err)

	_, err = chatStreamURL("", "0")
	assert.Error(t, err)
}

func TestParseSSEFrame(t *testing.T) {
	// Real keepalive frame from a live capture (comment line only).
	_, ok := parseSSEFrame(": -1")
	assert.False(t, ok)

	// Real event frame captured live from web7.rumble.com (2026-09-08).
	frame := "data: {\"request_id\":\"qKOb\",\"type\":\"messages\",\"data\":{\"messages\":[],\"users\":[],\"channels\":[]}}"
	data, ok := parseSSEFrame(frame)
	require.True(t, ok)
	assert.Contains(t, data, `"type":"messages"`)

	// CRLF-terminated multi-line data (spec-compliant).
	frame = "data: {\"type\":\"init\"}\r\ndata: second\r\n"
	data, ok = parseSSEFrame(frame)
	require.True(t, ok)
	assert.Contains(t, data, "second")
}

func TestDecodeStreamEvent(t *testing.T) {
	ev := decodeStreamEvent(`{"type":"init","data":{"chat":{"id":"445201096"}}}`)
	require.NotNil(t, ev)
	assert.Equal(t, "init", ev.Type)

	// Non-JSON payload (keepalive residue, HTML error page): dropped, not an error.
	assert.Nil(t, decodeStreamEvent(": -1"))
	assert.Nil(t, decodeStreamEvent("<html>cloudflare</html>"))
	assert.Nil(t, decodeStreamEvent(""))

	// Invalid JSON object: dropped.
	assert.Nil(t, decodeStreamEvent(`{"type": `))
}

func TestDecodeInitData_ShapeFromLiveCapture(t *testing.T) {
	raw := []byte(`{
		"chat": {"id": "445201096"},
		"messages": [{
			"id": "2737847970726173781",
			"time": "2026-09-08T14:51:03+00:00",
			"user_id": "7211755",
			"text": "@DEEREMANN hello",
			"blocks": [{"type": "text.1", "data": {"text": "@DEEREMANN hello"}}],
			"type": "regular"
		}],
		"users": [{
			"id": "7211755",
			"username": "jaffowhit",
			"link": "/user/jaffowhit",
			"is_follower": true,
			"color": "#2c93bc",
			"badges": ["verified"]
		}],
		"channels": [],
		"ad_read": null,
		"can_moderate": false,
		"config": {"rants": {"enable": true}},
		"subscribers_only_chat": {"on": false, "can_enable": false, "can_write": false}
	}`)

	initData, err := decodeInitData(raw)
	require.NoError(t, err)
	assert.Equal(t, "445201096", initData.Chat.ID)
	require.Len(t, initData.Messages, 1)
	msg := initData.Messages[0]
	assert.Equal(t, "2737847970726173781", msg.ID)
	assert.Equal(t, "2026-09-08T14:51:03+00:00", msg.Time)
	assert.Equal(t, "7211755", msg.UserID)
	assert.Equal(t, "@DEEREMANN hello", msg.Text)
	assert.Equal(t, "regular", msg.Type)
	require.Len(t, initData.Users, 1)
	user := initData.Users[0]
	assert.Equal(t, "jaffowhit", user.Username)
	assert.Equal(t, "#2c93bc", user.Color)
	assert.Equal(t, []string{"verified"}, user.Badges)
	assert.True(t, user.IsFollower)
}

func TestDecodeMessagesData_RantShape(t *testing.T) {
	raw := []byte(`{
		"messages": [{
			"id": "2737850000000000000",
			"time": "2026-09-08T15:00:00+00:00",
			"user_id": "100",
			"text": "great stream",
			"type": "regular",
			"rant": {"price_cents": 500, "duration": 300}
		}],
		"users": [],
		"channels": []
	}`)
	d, err := decodeMessagesData(raw)
	require.NoError(t, err)
	require.Len(t, d.Messages, 1)
	require.NotNil(t, d.Messages[0].Rant)
	assert.Equal(t, 500, d.Messages[0].Rant.PriceCents)
	assert.Equal(t, 300, d.Messages[0].Rant.Duration)
}

func TestDecodeMessagesData_DeletionShape(t *testing.T) {
	raw := []byte(`{
		"messages": [{
			"id": "2737850000000000001",
			"time": "2026-09-08T15:01:00+00:00",
			"user_id": "100",
			"type": "regular",
			"is_deleted": true
		}],
		"users": [],
		"channels": []
	}`)
	d, err := decodeMessagesData(raw)
	require.NoError(t, err)
	require.Len(t, d.Messages, 1)
	assert.True(t, d.Messages[0].IsDeleted)
}

func TestMessageText_FallsBackToBlocks(t *testing.T) {
	msg := &SSEMessage{
		Blocks: []byte(`[{"type":"text.1","data":{"text":"part one"}},{"type":"emote","data":{}},{"type":"text.1","data":{"text":"part two"}}]`),
	}
	assert.Equal(t, "part one part two", messageText(msg))

	assert.Equal(t, "", messageText(&SSEMessage{}))
	assert.Equal(t, "plain", messageText(&SSEMessage{Text: "plain"}))
	// Malformed blocks degrade to empty text, never a panic.
	assert.Equal(t, "", messageText(&SSEMessage{Blocks: []byte(`not json`)}))
}

// TestStreamClient_ParsesLiveShape feeds the real init+messages SSE framing
// captured from web7.rumble.com (2026-09-08) through the parser.
func TestStreamClient_ParsesLiveShape(t *testing.T) {
	var mu sync.Mutex
	var msgs []SSEMessage
	var initSeen bool

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "text/event-stream", r.Header.Get("Accept"))
		w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
		fmt.Fprint(w, ": -1\n\n")
		fmt.Fprint(w, "data: {\"type\":\"init\",\"data\":{\"chat\":{\"id\":\"445201096\"},\"messages\":[{\"id\":\"2737847970726173781\",\"time\":\"2026-09-08T14:51:03+00:00\",\"user_id\":\"7211755\",\"text\":\"hello from history\",\"blocks\":[{\"type\":\"text.1\",\"data\":{\"text\":\"hello from history\"}}],\"type\":\"regular\"}],\"users\":[{\"id\":\"7211755\",\"username\":\"jaffowhit\",\"color\":\"#2c93bc\",\"badges\":[\"verified\"]}],\"channels\":[],\"config\":{\"rants\":{\"enable\":true}}}}\n\n")
		fmt.Fprint(w, "data: {\"request_id\":\"qKOb\",\"type\":\"messages\",\"data\":{\"messages\":[{\"id\":\"2737847970726173782\",\"time\":\"2026-09-08T14:52:00+00:00\",\"user_id\":\"7211755\",\"text\":\"live one\",\"type\":\"regular\"}],\"users\":[],\"channels\":[]}}\n\n")
	}))

	client := NewStreamClient(testLogger(), srv.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	done := make(chan struct{})
	go func() {
		defer close(done)
		client.Run(ctx, srv.Client(), "445201096", "", StreamCallbacks{
			OnMessage: func(_ string, msg SSEMessage) {
				mu.Lock()
				msgs = append(msgs, msg)
				mu.Unlock()
			},
			OnInit: func(_ string, _ streamChatConfig) {
				mu.Lock()
				initSeen = true
				mu.Unlock()
			},
		})
	}()
	<-done

	mu.Lock()
	defer mu.Unlock()
	assert.True(t, initSeen, "init callback should fire")
	require.Len(t, msgs, 2, "history + live messages should both be delivered")
	assert.Equal(t, "hello from history", msgs[0].Text)
	assert.Equal(t, "live one", msgs[1].Text)
}

// TestStreamClient_UnknownEventTypesLoggedAndDropped proves a rumble.com
// format change (new event type, new fields) cannot panic or crash the pump.
func TestStreamClient_UnknownEventTypesLoggedAndDropped(t *testing.T) {
	var mu sync.Mutex
	var msgs []SSEMessage

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"type\":\"brand_new_event\",\"data\":{\"anything\":[1,2,3]}}\n\n")
		fmt.Fprint(w, "data: not-json-at-all\n\n")
		fmt.Fprint(w, "data: {\"type\":\"messages\",\"data\":{\"messages\":[{\"id\":\"1\",\"text\":\"still works\"}],\"users\":[]}}\n\n")
	}))

	client := NewStreamClient(testLogger(), srv.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	done := make(chan struct{})
	go func() {
		defer close(done)
		client.Run(ctx, srv.Client(), "1234", "", StreamCallbacks{
			OnMessage: func(_ string, msg SSEMessage) {
				mu.Lock()
				msgs = append(msgs, msg)
				mu.Unlock()
			},
		})
	}()
	<-done

	mu.Lock()
	defer mu.Unlock()
	require.Len(t, msgs, 1, "only the known messages event should deliver")
	assert.Equal(t, "still works", msgs[0].Text)
}

// TestStreamClient_SessionCookieForwarded asserts the optional cookie reaches
// the wire (RUMBLE_SESSION_COOKIE support, ADR-0061).
func TestStreamClient_SessionCookieForwarded(t *testing.T) {
	gotCookie := make(chan string, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case gotCookie <- r.Header.Get("Cookie"):
		default:
		}
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"type\":\"init\",\"data\":{\"chat\":{\"id\":\"7\"}}}\n\n")
	}))

	client := NewStreamClient(testLogger(), srv.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	done := make(chan struct{})
	go func() {
		defer close(done)
		client.Run(ctx, srv.Client(), "7", "session=abc123", StreamCallbacks{})
	}()
	<-done

	select {
	case cookie := <-gotCookie:
		assert.Equal(t, "session=abc123", cookie)
	default:
		t.Fatal("expected session cookie to be forwarded")
	}
}

// TestStreamClient_Non200ReturnsConnLost asserts a 4xx/5xx stream is reported
// as a lost connection so the caller applies backoff.
func TestStreamClient_Non200ReturnsConnLost(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Rumble returns a structured 400 for malformed chat ids.
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, "{\"errors\":[{\"code\":\"400\",\"message\":\"Missing or malformed chat id\"}]}")
	}))

	client := NewStreamClient(testLogger(), srv.URL)
	// Invalid chat id is caught client-side before the request; a numeric id
	// reaches the 400 server and is reported as a lost connection.
	result := client.Run(context.Background(), srv.Client(), "999999", "", StreamCallbacks{})
	assert.Equal(t, streamConnLost, result)
}

// TestClientSubscribeReceivesMessage walks the full Client path: Subscribe ->
// goroutine -> parser -> message handler.
func TestClientSubscribeReceivesMessage(t *testing.T) {
	msgCh := make(chan SSEMessage, 4)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"type\":\"messages\",\"data\":{\"messages\":[{\"id\":\"9\",\"time\":\"2026-09-08T15:00:00+00:00\",\"user_id\":\"3\",\"text\":\"via client\",\"type\":\"regular\"}],\"users\":[],\"channels\":[]}}\n\n")
		time.Sleep(300 * time.Millisecond)
	}))

	ws := NewClient(Config{BaseURL: srv.URL}, func(_ string, msg *SSEMessage) { msgCh <- *msg }, testZapLogger())
	require.NoError(t, ws.Connect())
	require.NoError(t, ws.Subscribe("445201096"))

	select {
	case msg := <-msgCh:
		assert.Equal(t, "via client", msg.Text)
	case <-time.After(3 * time.Second):
		t.Fatal("message never delivered through client")
	}

	require.NoError(t, ws.Unsubscribe("445201096"))
}

// TestClientDeletionDispatched asserts is_deleted messages route to the
// deletion handler instead of the message handler.
func TestClientDeletionDispatched(t *testing.T) {
	deleted := make(chan string, 4)
	live := make(chan SSEMessage, 4)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"type\":\"messages\",\"data\":{\"messages\":[{\"id\":\"11\",\"is_deleted\":true,\"type\":\"regular\"},{\"id\":\"12\",\"text\":\"kept\",\"type\":\"regular\"}],\"users\":[],\"channels\":[]}}\n\n")
		time.Sleep(300 * time.Millisecond)
	}))

	ws := NewClient(Config{BaseURL: srv.URL}, func(_ string, msg *SSEMessage) { live <- *msg }, testZapLogger())
	ws.SetDeletionHandler(func(_ string, messageID string) { deleted <- messageID })
	require.NoError(t, ws.Connect())
	require.NoError(t, ws.Subscribe("555"))

	select {
	case id := <-deleted:
		assert.Equal(t, "11", id)
	case <-time.After(3 * time.Second):
		t.Fatal("deletion never dispatched")
	}
	select {
	case msg := <-live:
		assert.Equal(t, "12", msg.ID)
	case <-time.After(3 * time.Second):
		t.Fatal("live message never dispatched")
	}
	require.NoError(t, ws.Unsubscribe("555"))
}

// TestClientReconnectSignalledOnStreamLoss asserts a dropped stream signals
// ReconnectChan so the caller's backoff loop can drive the reconnect.
func TestClientReconnectSignalledOnStreamLoss(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"type\":\"init\",\"data\":{\"chat\":{\"id\":\"8\"}}}\n\n")
		// Handler return closes the stream abruptly.
	}))

	ws := NewClient(Config{BaseURL: srv.URL}, nil, testZapLogger())
	require.NoError(t, ws.Connect())
	require.NoError(t, ws.Subscribe("8"))

	select {
	case <-ws.ReconnectChan():
		// expected
	case <-time.After(3 * time.Second):
		t.Fatal("stream loss did not signal reconnect channel")
	}
	require.NoError(t, ws.Unsubscribe("8"))
}

// TestClientUnsubscribeDoesNotSignalReconnect asserts a deliberate unsubscribe
// (channel removed from DB) is not misread as a lost connection needing
// reconnection.
func TestClientUnsubscribeDoesNotSignalReconnect(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Never ends; unsubscribe cancels the request context.
		<-r.Context().Done()
	}))

	ws := NewClient(Config{BaseURL: srv.URL}, nil, testZapLogger())
	require.NoError(t, ws.Connect())
	require.NoError(t, ws.Subscribe("66"))
	time.Sleep(100 * time.Millisecond)
	require.NoError(t, ws.Unsubscribe("66"))

	select {
	case <-ws.ReconnectChan():
		t.Fatal("unsubscribe must not signal reconnect")
	case <-time.After(300 * time.Millisecond):
		// expected
	}
}

// TestClientDoubleSubscribeIsIdempotent asserts a second Subscribe for a
// running stream does not spawn a duplicate goroutine.
func TestClientDoubleSubscribeIsIdempotent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		for {
			select {
			case <-r.Context().Done():
				return
			case <-time.After(200 * time.Millisecond):
				fmt.Fprint(w, ": -1\n\n")
				if f, ok := w.(http.Flusher); ok {
					f.Flush()
				}
			}
		}
	}))

	ws := NewClient(Config{BaseURL: srv.URL}, nil, testZapLogger())
	require.NoError(t, ws.Connect())
	require.NoError(t, ws.Subscribe("77"))
	require.NoError(t, ws.Subscribe("77"))

	ws.mu.RLock()
	count := len(ws.streams)
	ws.mu.RUnlock()
	assert.Equal(t, 1, count, "duplicate subscribe must not create a second stream")
	require.NoError(t, ws.Unsubscribe("77"))
}

// TestSubscribeBeforeConnectFails mirrors kick-listener's startup guard.
func TestSubscribeBeforeConnectFails(t *testing.T) {
	ws := NewClient(Config{}, nil, testZapLogger())
	assert.Error(t, ws.Subscribe("445201096"))
}

// TestIsStaleAndLastActivity ports kick-listener's zombie-connection probe
// contract used by the liveness handler.
func TestIsStaleAndLastActivity(t *testing.T) {
	ws := NewClient(Config{}, nil, testZapLogger())

	// No streams: never stale (nothing connected yet).
	assert.False(t, ws.IsStale())
	assert.True(t, ws.LastActivityAt().IsZero())

	// A stream silent beyond the threshold is stale.
	ws.mu.Lock()
	stream := &chatStream{chatID: "1", lastActive: time.Now().Add(-2 * staleLivenessThreshold)}
	ws.streams["1"] = stream
	ws.mu.Unlock()
	assert.True(t, ws.IsStale())
	assert.False(t, ws.LastActivityAt().IsZero())

	// Fresh activity clears it.
	stream.activeMu.Lock()
	stream.lastActive = time.Now()
	stream.activeMu.Unlock()
	assert.False(t, ws.IsStale())

	// One live stream among zombies is enough to stay not-stale
	// (IsStale requires ALL streams silent).
	ws.mu.Lock()
	ws.streams["2"] = &chatStream{chatID: "2", lastActive: time.Now()}
	ws.mu.Unlock()
	assert.False(t, ws.IsStale())

	ws.mu.Lock()
	ws.streams = make(map[string]*chatStream)
	ws.mu.Unlock()
}
