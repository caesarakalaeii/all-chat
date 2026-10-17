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

// KickRealtimeConnectionURL is where kick.com's own web client gets its Pusher
// credentials. Kick rotates the app key without notice (on 2026-10-17 the old
// key started failing every connect with Pusher 4001 "App key not in this
// cluster"), so the client resolves the key before each connect instead of
// trusting the configured one.
const KickRealtimeConnectionURL = "https://web.kick.com/api/v1/realtime/connection"

var credentialsHTTPClient = &http.Client{Timeout: 10 * time.Second}

type realtimeConnectionResponse struct {
	Data struct {
		Connections []struct {
			Provider    string `json:"provider"`
			Credentials struct {
				AppKey  string `json:"app_key"`
				Cluster string `json:"cluster"`
			} `json:"credentials"`
		} `json:"connections"`
	} `json:"data"`
}

// fetchPusherCredentials asks Kick for the Pusher app key and cluster its web
// client would use. Only pusher is offered: offered centrifugo too, Kick
// answers with a centrifugo URL, which this client does not speak.
func fetchPusherCredentials(ctx context.Context, client *http.Client, url string) (appKey, cluster string, err error) {
	reqBody, err := json.Marshal(map[string]any{
		"client":       map[string]string{"id": uuid.NewString(), "type": "web"},
		"capabilities": map[string]any{"accepted_providers": []map[string]string{{"provider": "pusher"}}},
	})
	if err != nil {
		return "", "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(reqBody))
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "AllChat/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return "", "", fmt.Errorf("realtime connection endpoint returned %d: %s", resp.StatusCode, body)
	}

	var parsed realtimeConnectionResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return "", "", fmt.Errorf("decode realtime connection response: %w", err)
	}
	for _, conn := range parsed.Data.Connections {
		if conn.Provider == "pusher" && conn.Credentials.AppKey != "" && conn.Credentials.Cluster != "" {
			return conn.Credentials.AppKey, conn.Credentials.Cluster, nil
		}
	}
	return "", "", fmt.Errorf("realtime connection response has no pusher credentials")
}

// refreshCredentials swaps in Kick's current app key and puts its cluster
// first. On any failure it keeps the configured key, so a Kick API outage
// degrades to the old behavior rather than blocking the connect.
func (c *Client) refreshCredentials() {
	if c.config.CredentialsURL == "" {
		return
	}

	ctx, cancel := context.WithTimeout(c.ctx, 10*time.Second)
	defer cancel()

	appKey, cluster, err := fetchPusherCredentials(ctx, credentialsHTTPClient, c.config.CredentialsURL)
	if err != nil {
		c.logger.Warn("Could not resolve Kick Pusher credentials, using configured app key",
			zap.String("app_key", c.config.AppKey),
			zap.Error(err),
		)
		return
	}

	if appKey != c.config.AppKey {
		c.logger.Info("Kick Pusher app key changed",
			zap.String("old_app_key", c.config.AppKey),
			zap.String("new_app_key", appKey),
			zap.String("cluster", cluster),
		)
		c.config.AppKey = appKey
	}

	// clusterAttempts tries activeCluster first, then the configured ones.
	c.activeCluster = cluster
}
