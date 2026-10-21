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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// KickRealtimeConnectionURL is where kick.com's own web client asks which
// realtime provider to use (ADR-0077). Kick rotates the Pusher app key without
// notice (on 2026-10-17 the old key started failing every connect with Pusher
// 4001 "App key not in this cluster") and answers Centrifugo to any client
// that accepts it, so nothing about the transport is hardcoded.
const KickRealtimeConnectionURL = "https://web.kick.com/api/v1/realtime/connection"

// KickRealtimeTokenURL hands out the guest connection tokens Kick's
// Centrifugo requires (a 45-minute JWT for guest:<client_id>, no login).
const KickRealtimeTokenURL = "https://web.kick.com/api/v1/realtime/auth/connection"

// Realtime providers as Kick names them.
const (
	ProviderPusher     = "pusher"
	ProviderCentrifugo = "centrifugo"
)

var kickAPIClient = &http.Client{Timeout: 10 * time.Second}

// Realtime is what the listener needs from a Kick realtime transport. The
// Pusher Client and the CentrifugoClient both implement it.
type Realtime interface {
	Connect() error
	Disconnect() error
	Subscribe(chatroomID int) error
	SubscribeWithAuth(chatroomID int, authToken string) error
	Unsubscribe(chatroomID int) error
	IsConnected() bool
	GetSocketID() string
	LastActivityAt() time.Time
	IsStale() bool
	ReconnectChan() <-chan struct{}
	SetDeletionHandler(handler DeletionHandler)
}

// RealtimeConnection is the provider Kick chose and what it needs:
// AppKey and Cluster for Pusher, URL for Centrifugo.
type RealtimeConnection struct {
	Provider string
	AppKey   string
	Cluster  string
	URL      string
}

type realtimeConnectionResponse struct {
	Data struct {
		Connections []struct {
			Provider    string `json:"provider"`
			Credentials struct {
				AppKey  string `json:"app_key"`
				Cluster string `json:"cluster"`
				URL     string `json:"url"`
			} `json:"credentials"`
		} `json:"connections"`
	} `json:"data"`
}

// ResolveRealtime asks Kick which of the given providers to use, offering them
// the way kick.com's web client does. Kick picks one; a connection for a
// provider that was not offered, or with empty credentials, is not usable.
func ResolveRealtime(ctx context.Context, url string, providers []string) (RealtimeConnection, error) {
	accepted := make([]map[string]string, 0, len(providers))
	offered := make(map[string]bool, len(providers))
	for _, p := range providers {
		accepted = append(accepted, map[string]string{"provider": p})
		offered[p] = true
	}

	var parsed realtimeConnectionResponse
	err := postKickJSON(ctx, url, map[string]any{
		"client":       map[string]string{"id": uuid.NewString(), "type": "web"},
		"capabilities": map[string]any{"accepted_providers": accepted},
	}, &parsed)
	if err != nil {
		return RealtimeConnection{}, err
	}

	for _, conn := range parsed.Data.Connections {
		c := conn.Credentials
		switch {
		case !offered[conn.Provider]:
		case conn.Provider == ProviderPusher && c.AppKey != "" && c.Cluster != "":
			return RealtimeConnection{Provider: ProviderPusher, AppKey: c.AppKey, Cluster: c.Cluster}, nil
		case conn.Provider == ProviderCentrifugo && c.URL != "":
			return RealtimeConnection{Provider: ProviderCentrifugo, URL: c.URL}, nil
		}
	}
	return RealtimeConnection{}, fmt.Errorf("realtime connection response has no usable connection for %v", providers)
}

// fetchCentrifugoToken gets a guest connection token for one connection.
func fetchCentrifugoToken(ctx context.Context, url, clientID string) (string, error) {
	var parsed struct {
		Data struct {
			Token string `json:"token"`
		} `json:"data"`
	}
	if err := postKickJSON(ctx, url, map[string]string{"client_id": clientID}, &parsed); err != nil {
		return "", err
	}
	if parsed.Data.Token == "" {
		return "", fmt.Errorf("realtime auth response has no token")
	}
	return parsed.Data.Token, nil
}

func postKickJSON(ctx context.Context, url string, body, out any) error {
	reqBody, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(reqBody))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "AllChat/1.0")

	resp, err := kickAPIClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("%s returned %d: %s", url, resp.StatusCode, b)
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decode %s response: %w", url, err)
	}
	return nil
}

// refreshCredentials swaps in Kick's current Pusher app key and puts its
// cluster first. On any failure it keeps the configured key, so a Kick API
// outage degrades to the old behavior rather than blocking the connect.
func (c *Client) refreshCredentials() {
	if c.config.CredentialsURL == "" {
		return
	}

	conn, err := ResolveRealtime(c.ctx, c.config.CredentialsURL, []string{ProviderPusher})
	if err != nil {
		c.logger.Warn("Could not resolve Kick Pusher credentials, using configured app key",
			zap.String("app_key", c.config.AppKey),
			zap.Error(err),
		)
		return
	}

	if conn.AppKey != c.config.AppKey {
		c.logger.Info("Kick Pusher app key changed",
			zap.String("old_app_key", c.config.AppKey),
			zap.String("new_app_key", conn.AppKey),
			zap.String("cluster", conn.Cluster),
		)
		c.config.AppKey = conn.AppKey
	}

	// clusterAttempts tries activeCluster first, then the configured ones.
	c.activeCluster = conn.Cluster
}
