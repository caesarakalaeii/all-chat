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
	"os"
	"testing"
	"time"

	"go.uber.org/zap"
)

// TestCentrifugoLive runs the real client against Kick: it asks Kick for the
// provider, subscribes the busiest live chatroom plus enough others to need a
// second pooled connection, and waits for real chat. Kick's servers are not
// something CI can depend on, so it only runs with KICK_LIVE_TEST=1.
func TestCentrifugoLive(t *testing.T) {
	if os.Getenv("KICK_LIVE_TEST") == "" {
		t.Skip("set KICK_LIVE_TEST=1 to run against Kick's live servers")
	}

	conn, err := ResolveRealtime(t.Context(), KickRealtimeConnectionURL, []string{ProviderCentrifugo, ProviderPusher})
	if err != nil {
		t.Fatalf("ResolveRealtime: %v", err)
	}
	if conn.Provider != ProviderCentrifugo {
		t.Fatalf("Kick answered %q; this test needs centrifugo", conn.Provider)
	}

	live := busiestLiveChatroom(t)
	got := make(chan *KickChatMessage, 1)
	c := NewCentrifugoClient(conn.URL, KickRealtimeTokenURL, func(_ string, m *KickChatMessage) {
		if m.ChatroomID == live {
			select {
			case got <- m:
			default:
			}
		}
	}, zap.NewNop())
	if err := c.Connect(); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	t.Cleanup(func() { _ = c.Disconnect() })

	if err := c.Subscribe(live); err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	// Fill past one connection's capacity; ids need not exist to subscribe.
	for id := 1; id <= centrifugoMaxPerConn+20; id++ {
		_ = c.Subscribe(id)
	}

	select {
	case m := <-got:
		t.Logf("chatroom %d: %s: %s", live, m.Sender.Username, m.Content)
	case <-time.After(60 * time.Second):
		t.Fatalf("no chat from live chatroom %d within 60s", live)
	}

	deadline := time.Now().Add(15 * time.Second)
	for !c.IsConnected() && time.Now().Before(deadline) {
		time.Sleep(200 * time.Millisecond)
	}
	c.mu.Lock()
	pool := len(c.conns)
	c.mu.Unlock()
	if pool < 2 || !c.IsConnected() {
		t.Fatalf("pool = %d connections, connected = %v; want >= 2 and connected", pool, c.IsConnected())
	}
}

func busiestLiveChatroom(t *testing.T) int {
	t.Helper()
	var streams struct {
		Data struct {
			Livestreams []struct {
				Channel struct {
					Slug string `json:"slug"`
				} `json:"channel"`
			} `json:"livestreams"`
		} `json:"data"`
	}
	getJSON(t, "https://web.kick.com/api/v1/livestreams?limit=1&sort=viewer_count_desc", &streams)
	if len(streams.Data.Livestreams) == 0 {
		t.Fatal("no live streams on Kick")
	}
	var channel struct {
		Chatroom struct {
			ID int `json:"id"`
		} `json:"chatroom"`
	}
	getJSON(t, "https://kick.com/api/v2/channels/"+streams.Data.Livestreams[0].Channel.Slug, &channel)
	return channel.Chatroom.ID
}

func getJSON(t *testing.T, url string, out any) {
	t.Helper()
	req, _ := http.NewRequestWithContext(t.Context(), http.MethodGet, url, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0 Safari/537.36")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		t.Fatalf("decode %s: %v", url, err)
	}
}
