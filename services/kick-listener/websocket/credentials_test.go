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
	"reflect"
	"testing"

	"go.uber.org/zap"
)

// realtimeServer mimics web.kick.com/api/v1/realtime/connection. It fails the
// test unless the request offers exactly the wanted providers, in order.
func realtimeServer(t *testing.T, status int, body string, wantProviders ...string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		var req struct {
			Capabilities struct {
				AcceptedProviders []struct {
					Provider string `json:"provider"`
				} `json:"accepted_providers"`
			} `json:"capabilities"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode request: %v", err)
		}
		if len(wantProviders) == 0 {
			wantProviders = []string{"pusher"}
		}
		var got []string
		for _, p := range req.Capabilities.AcceptedProviders {
			got = append(got, p.Provider)
		}
		if !reflect.DeepEqual(got, wantProviders) {
			t.Errorf("accepted_providers = %v, want %v", got, wantProviders)
		}
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestResolveRealtime_Pusher(t *testing.T) {
	srv := realtimeServer(t, http.StatusOK,
		`{"data":{"connections":[{"credentials":{"app_key":"34bf7a0ff419a2a775b9","cluster":"us2"},"provider":"pusher"}],"mode":"websocket"},"message":"success"}`)

	got, err := ResolveRealtime(t.Context(), srv.URL, []string{"pusher"})
	if err != nil {
		t.Fatalf("ResolveRealtime: %v", err)
	}
	want := RealtimeConnection{Provider: ProviderPusher, AppKey: "34bf7a0ff419a2a775b9", Cluster: "us2"}
	if got != want {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestResolveRealtime_Centrifugo(t *testing.T) {
	srv := realtimeServer(t, http.StatusOK,
		`{"data":{"connections":[{"credentials":{"url":"wss://realtime.us-east-1.platform.kick.com/connection/websocket"},"provider":"centrifugo"}],"mode":"websocket"},"message":"success"}`,
		"centrifugo", "pusher")

	got, err := ResolveRealtime(t.Context(), srv.URL, []string{"centrifugo", "pusher"})
	if err != nil {
		t.Fatalf("ResolveRealtime: %v", err)
	}
	want := RealtimeConnection{Provider: ProviderCentrifugo, URL: "wss://realtime.us-east-1.platform.kick.com/connection/websocket"}
	if got != want {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

// Kick answering with a provider we did not offer, or with empty credentials,
// must not be mistaken for a usable connection.
func TestResolveRealtime_NoUsableConnection(t *testing.T) {
	srv := realtimeServer(t, http.StatusOK,
		`{"data":{"connections":[{"credentials":{"url":"wss://realtime.example/connection/websocket"},"provider":"centrifugo"},{"credentials":{},"provider":"pusher"}],"mode":"websocket"}}`)

	if _, err := ResolveRealtime(t.Context(), srv.URL, []string{"pusher"}); err == nil {
		t.Fatal("expected an error when Kick offers no usable connection")
	}
}

func TestRefreshCredentials_AppliesRotatedKey(t *testing.T) {
	srv := realtimeServer(t, http.StatusOK,
		`{"data":{"connections":[{"credentials":{"app_key":"newkey","cluster":"eu"},"provider":"pusher"}],"mode":"websocket"}}`)

	c := NewClient(Config{AppKey: "oldkey", Clusters: []string{"us2", "eu"}, CredentialsURL: srv.URL}, nil, zap.NewNop())
	c.activeCluster = "us2"
	c.refreshCredentials()

	if c.config.AppKey != "newkey" {
		t.Errorf("AppKey = %q, want newkey", c.config.AppKey)
	}
	if got, want := c.clusterAttempts(), []string{"eu", "us2"}; !reflect.DeepEqual(got, want) {
		t.Errorf("clusterAttempts = %v, want %v", got, want)
	}
}

func TestRefreshCredentials_KeepsConfiguredKeyOnFailure(t *testing.T) {
	srv := realtimeServer(t, http.StatusInternalServerError, `{"message":"boom"}`)

	c := NewClient(Config{AppKey: "oldkey", Clusters: []string{"us2"}, CredentialsURL: srv.URL}, nil, zap.NewNop())
	c.refreshCredentials()

	if c.config.AppKey != "oldkey" {
		t.Errorf("AppKey = %q, want oldkey kept", c.config.AppKey)
	}
	if got, want := c.clusterAttempts(), []string{"us2"}; !reflect.DeepEqual(got, want) {
		t.Errorf("clusterAttempts = %v, want %v", got, want)
	}
}
