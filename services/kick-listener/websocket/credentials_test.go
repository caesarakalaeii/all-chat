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
// test unless the request offers only the pusher provider, because offering
// centrifugo makes Kick answer with a provider this client cannot speak.
func realtimeServer(t *testing.T, status int, body string) *httptest.Server {
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
		p := req.Capabilities.AcceptedProviders
		if len(p) != 1 || p[0].Provider != "pusher" {
			t.Errorf("accepted_providers = %+v, want only pusher", p)
		}
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestFetchPusherCredentials(t *testing.T) {
	srv := realtimeServer(t, http.StatusOK,
		`{"data":{"connections":[{"credentials":{"app_key":"34bf7a0ff419a2a775b9","cluster":"us2"},"provider":"pusher"}],"mode":"websocket"},"message":"success"}`)

	key, cluster, err := fetchPusherCredentials(t.Context(), srv.Client(), srv.URL)
	if err != nil {
		t.Fatalf("fetchPusherCredentials: %v", err)
	}
	if key != "34bf7a0ff419a2a775b9" || cluster != "us2" {
		t.Fatalf("got (%q, %q), want (34bf7a0ff419a2a775b9, us2)", key, cluster)
	}
}

func TestFetchPusherCredentials_NoPusherConnection(t *testing.T) {
	srv := realtimeServer(t, http.StatusOK,
		`{"data":{"connections":[{"credentials":{"url":"wss://realtime.example/connection/websocket"},"provider":"centrifugo"}],"mode":"websocket"}}`)

	if _, _, err := fetchPusherCredentials(t.Context(), srv.Client(), srv.URL); err == nil {
		t.Fatal("expected an error when Kick offers no pusher connection")
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
