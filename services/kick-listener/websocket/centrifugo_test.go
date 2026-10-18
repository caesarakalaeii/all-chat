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
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

// fakeCentrifugo speaks the slice of Centrifugo's JSON protocol that Kick's
// server was measured to use (2026-10-18): connect with a token, subscribe,
// refresh, `{}` pings, newline-separated replies in one frame.
type fakeCentrifugo struct {
	t      *testing.T
	srv    *httptest.Server
	ttl    int // connection token ttl handed out in connect/refresh replies
	tokens atomic.Int32

	mu    sync.Mutex
	conns []*fakeConn
	newC  chan *fakeConn
}

type fakeConn struct {
	ws      *websocket.Conn
	writeMu sync.Mutex
	cmds    chan map[string]json.RawMessage // every command except connect
	pongs   chan struct{}
	closed  chan struct{}
	token   string // token sent with connect
}

func newFakeCentrifugo(t *testing.T, ttl int) *fakeCentrifugo {
	f := &fakeCentrifugo{t: t, ttl: ttl, newC: make(chan *fakeConn, 16)}
	upgrader := websocket.Upgrader{}
	mux := http.NewServeMux()
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			ClientID string `json:"client_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.ClientID == "" {
			t.Errorf("token request without client_id: %v", err)
		}
		fmt.Fprintf(w, `{"data":{"token":"tok-%d"},"message":"success"}`, f.tokens.Add(1))
	})
	mux.HandleFunc("/connection/websocket", func(w http.ResponseWriter, r *http.Request) {
		ws, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("upgrade: %v", err)
			return
		}
		fc := &fakeConn{ws: ws, cmds: make(chan map[string]json.RawMessage, 1024), pongs: make(chan struct{}, 16), closed: make(chan struct{})}
		go f.serve(fc)
	})
	f.srv = httptest.NewServer(mux)
	t.Cleanup(f.srv.Close)
	return f
}

func (f *fakeCentrifugo) wsURL() string {
	return "ws" + strings.TrimPrefix(f.srv.URL, "http") + "/connection/websocket"
}

func (f *fakeCentrifugo) serve(fc *fakeConn) {
	defer close(fc.closed)
	for {
		_, frame, err := fc.ws.ReadMessage()
		if err != nil {
			return
		}
		for _, line := range strings.Split(string(frame), "\n") {
			if strings.TrimSpace(line) == "{}" {
				fc.pongs <- struct{}{}
				continue
			}
			var cmd map[string]json.RawMessage
			if err := json.Unmarshal([]byte(line), &cmd); err != nil {
				f.t.Errorf("bad command %q: %v", line, err)
				return
			}
			id := string(cmd["id"])
			switch {
			case cmd["connect"] != nil:
				var c struct {
					Token string `json:"token"`
				}
				_ = json.Unmarshal(cmd["connect"], &c)
				fc.token = c.Token
				fc.send(fmt.Sprintf(`{"id":%s,"connect":{"client":"c","version":"6.9.7 PRO","expires":true,"ttl":%d,"ping":25,"pong":true}}`, id, f.ttl))
				f.mu.Lock()
				f.conns = append(f.conns, fc)
				f.mu.Unlock()
				f.newC <- fc
			case cmd["subscribe"] != nil:
				fc.send(fmt.Sprintf(`{"id":%s,"subscribe":{}}`, id))
				fc.cmds <- cmd
			case cmd["refresh"] != nil:
				fc.send(fmt.Sprintf(`{"id":%s,"refresh":{"client":"c","expires":true,"ttl":%d}}`, id, f.ttl))
				fc.cmds <- cmd
			default:
				fc.cmds <- cmd
			}
		}
	}
}

func (fc *fakeConn) send(frame string) {
	fc.writeMu.Lock()
	defer fc.writeMu.Unlock()
	_ = fc.ws.WriteMessage(websocket.TextMessage, []byte(frame))
}

// nextConn waits for the client to complete a connect handshake.
func (f *fakeCentrifugo) nextConn(t *testing.T) *fakeConn {
	t.Helper()
	select {
	case fc := <-f.newC:
		return fc
	case <-time.After(3 * time.Second):
		t.Fatal("client never connected")
		return nil
	}
}

// nextCmd waits for the next command of the given kind on a connection.
func (fc *fakeConn) nextCmd(t *testing.T, kind string) json.RawMessage {
	t.Helper()
	deadline := time.After(3 * time.Second)
	for {
		select {
		case cmd := <-fc.cmds:
			if cmd[kind] != nil {
				return cmd[kind]
			}
		case <-deadline:
			t.Fatalf("no %s command", kind)
			return nil
		}
	}
}

func subscribedChannel(t *testing.T, raw json.RawMessage) string {
	t.Helper()
	var s struct {
		Channel string `json:"channel"`
	}
	if err := json.Unmarshal(raw, &s); err != nil {
		t.Fatalf("decode subscribe: %v", err)
	}
	return s.Channel
}

func newTestCentrifugo(f *fakeCentrifugo, onMessage MessageHandler) *CentrifugoClient {
	c := NewCentrifugoClient(f.wsURL(), f.srv.URL+"/token", onMessage, zap.NewNop())
	c.minBackoff = 10 * time.Millisecond
	return c
}

// A chat publication exactly as Kick's server sent it on 2026-10-18, two to a frame.
const kickChatPush = `{"push":{"channel":"chatrooms.42.v2","pub":{"data":{"event":"App\\Events\\ChatMessageEvent","data":"{\"id\":\"m1\",\"chatroom_id\":42,\"content\":\"hello\",\"type\":\"message\",\"sender\":{\"id\":7,\"username\":\"viewer\",\"slug\":\"viewer\",\"identity\":{\"color\":\"#fff\",\"badges\":[]}}}"}}}}`
const kickDeletePush = `{"push":{"channel":"chatrooms.42.v2","pub":{"data":{"event":"App\\Events\\MessageDeletedEvent","data":"{\"id\":\"d1\",\"message\":{\"id\":\"m1\"}}"}}}}`

func TestCentrifugo_SubscribesAndDeliversChat(t *testing.T) {
	f := newFakeCentrifugo(t, 2700)
	got := make(chan string, 4)
	c := newTestCentrifugo(f, func(channel string, m *KickChatMessage) {
		got <- channel + " " + m.Sender.Username + ": " + m.Content
	})
	deleted := make(chan string, 1)
	c.SetDeletionHandler(func(channel string, e *KickMessageDeletedEvent) { deleted <- channel })
	if err := c.Connect(); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	t.Cleanup(func() { _ = c.Disconnect() })

	fc := f.nextConn(t)
	if fc.token != "tok-1" {
		t.Errorf("connect token = %q, want tok-1", fc.token)
	}
	if !c.IsConnected() {
		t.Error("IsConnected() = false after connect")
	}

	if err := c.SubscribeWithAuth(42, "pusher-auth-is-ignored"); err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	if ch := subscribedChannel(t, fc.nextCmd(t, "subscribe")); ch != "chatrooms.42.v2" {
		t.Fatalf("subscribed %q, want chatrooms.42.v2", ch)
	}

	fc.send(kickChatPush + "\n" + kickDeletePush)
	select {
	case m := <-got:
		if m != "chatrooms.42.v2 viewer: hello" {
			t.Errorf("message = %q", m)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("chat message never delivered")
	}
	select {
	case ch := <-deleted:
		if ch != "chatrooms.42.v2" {
			t.Errorf("deletion channel = %q", ch)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("deletion never delivered")
	}
}

func TestCentrifugo_AnswersPing(t *testing.T) {
	f := newFakeCentrifugo(t, 2700)
	c := newTestCentrifugo(f, nil)
	if err := c.Connect(); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	t.Cleanup(func() { _ = c.Disconnect() })

	fc := f.nextConn(t)
	fc.send(`{}`)
	select {
	case <-fc.pongs:
	case <-time.After(3 * time.Second):
		t.Fatal("ping never answered")
	}
}

// Kick refuses the 129th subscription on one connection, so the client must
// spread chatrooms over several connections and drop the ones it emptied.
func TestCentrifugo_PoolsConnectionsAtCapacity(t *testing.T) {
	f := newFakeCentrifugo(t, 2700)
	c := newTestCentrifugo(f, nil)
	c.maxPerConn = 2
	if err := c.Connect(); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	t.Cleanup(func() { _ = c.Disconnect() })
	first := f.nextConn(t)

	for _, id := range []int{1, 2, 3} {
		if err := c.Subscribe(id); err != nil {
			t.Fatalf("Subscribe(%d): %v", id, err)
		}
	}
	second := f.nextConn(t)

	got := map[string]bool{
		subscribedChannel(t, first.nextCmd(t, "subscribe")):  true,
		subscribedChannel(t, first.nextCmd(t, "subscribe")):  true,
		subscribedChannel(t, second.nextCmd(t, "subscribe")): true,
	}
	for _, want := range []string{"chatrooms.1.v2", "chatrooms.2.v2", "chatrooms.3.v2"} {
		if !got[want] {
			t.Errorf("%s never subscribed; got %v", want, got)
		}
	}

	if err := c.Unsubscribe(3); err != nil {
		t.Fatalf("Unsubscribe: %v", err)
	}
	select {
	case <-second.closed:
	case <-time.After(3 * time.Second):
		t.Fatal("emptied connection was not closed")
	}
	select {
	case <-first.closed:
		t.Fatal("first connection must stay open")
	default:
	}
}

func TestCentrifugo_ReconnectResubscribesWithFreshToken(t *testing.T) {
	f := newFakeCentrifugo(t, 2700)
	c := newTestCentrifugo(f, nil)
	if err := c.Connect(); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	t.Cleanup(func() { _ = c.Disconnect() })
	fc := f.nextConn(t)
	if err := c.Subscribe(7); err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	fc.nextCmd(t, "subscribe")

	_ = fc.ws.Close() // server goes away

	again := f.nextConn(t)
	if again.token != "tok-2" {
		t.Errorf("reconnect token = %q, want a fresh tok-2", again.token)
	}
	if ch := subscribedChannel(t, again.nextCmd(t, "subscribe")); ch != "chatrooms.7.v2" {
		t.Fatalf("resubscribed %q, want chatrooms.7.v2", ch)
	}
}

func TestCentrifugo_RefreshesTokenBeforeExpiry(t *testing.T) {
	f := newFakeCentrifugo(t, 1) // 1s ttl, so a refresh is due well inside the test
	c := newTestCentrifugo(f, nil)
	if err := c.Connect(); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	t.Cleanup(func() { _ = c.Disconnect() })
	fc := f.nextConn(t)

	var r struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(fc.nextCmd(t, "refresh"), &r); err != nil {
		t.Fatalf("decode refresh: %v", err)
	}
	if r.Token != "tok-2" {
		t.Errorf("refresh token = %q, want tok-2", r.Token)
	}
}

func TestCentrifugo_DisconnectReportsDown(t *testing.T) {
	f := newFakeCentrifugo(t, 2700)
	c := newTestCentrifugo(f, nil)
	if err := c.Connect(); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	fc := f.nextConn(t)
	if err := c.Disconnect(); err != nil {
		t.Fatalf("Disconnect: %v", err)
	}
	if c.IsConnected() {
		t.Error("IsConnected() = true after Disconnect")
	}
	select {
	case <-fc.closed:
	case <-time.After(3 * time.Second):
		t.Fatal("socket not closed on Disconnect")
	}
}

// Both transports must be interchangeable behind the interface main uses.
var (
	_ Realtime = (*Client)(nil)
	_ Realtime = (*CentrifugoClient)(nil)
)
