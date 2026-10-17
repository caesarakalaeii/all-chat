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

package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/caesar/all-chat/services/kick-listener/websocket"
	"go.uber.org/zap"
)

func TestNewRealtimeClient_FollowsKick(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		body   string
		want   string
	}{
		{"centrifugo offered", http.StatusOK,
			`{"data":{"connections":[{"credentials":{"url":"wss://realtime.example/connection/websocket"},"provider":"centrifugo"}],"mode":"websocket"}}`,
			"centrifugo"},
		{"pusher offered", http.StatusOK,
			`{"data":{"connections":[{"credentials":{"app_key":"k","cluster":"us2"},"provider":"pusher"}],"mode":"websocket"}}`,
			"pusher"},
		// Kick's API down must not stop the listener: fall back to the known Pusher path.
		{"kick unreachable", http.StatusBadGateway, `{}`, "pusher"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer srv.Close()

			rt := newRealtimeClient(t.Context(), srv.URL, []string{"centrifugo", "pusher"},
				websocket.Config{AppKey: "k"}, nil, zap.NewNop())

			got := "pusher"
			if _, ok := rt.(*websocket.CentrifugoClient); ok {
				got = "centrifugo"
			}
			if got != tc.want {
				t.Fatalf("got %s client, want %s", got, tc.want)
			}
		})
	}
}
