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

// Package token fetches anonymous Picarto chat JWTs.
//
// The pop-out chat page obtains its chat WebSocket token from Picarto's
// internal GraphQL endpoint with the generateJwtToken(channel_name:) query —
// no authentication needed for a read-only viewer identity (userId 0).
// Verified live during the ADR-0059 spike; undocumented, treat as volatile.
package token

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	// graphqlURL is Picarto's internal GraphQL API used by the web client.
	graphqlURL = "https://ptvintern.picarto.tv/ptvapi"

	// generateTokenQuery requests a chat JWT bound to a channel name. The
	// anonymous viewer identity comes back with userId 0 inside the JWT.
	generateTokenQuery = `query generateToken($channelId: Int, $channelName: String, $userId: Int) {
  generateJwtToken(channel_id: $channelId, channel_name: $channelName, user_id: $userId) {
    key
  }
}`
)

// Fetcher retrieves chat tokens over HTTP.
type Fetcher struct {
	client *http.Client
}

// NewFetcher creates a token fetcher with a 15s HTTP timeout.
func NewFetcher() *Fetcher {
	return &Fetcher{client: &http.Client{Timeout: 15 * time.Second}}
}

type gqlRequest struct {
	Query     string                 `json:"query"`
	Variables map[string]interface{} `json:"variables"`
}

type gqlResponse struct {
	Data struct {
		GenerateJwtToken struct {
			Key string `json:"key"`
		} `json:"generateJwtToken"`
	} `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors,omitempty"`
}

// Fetch returns a fresh anonymous chat JWT for channelName.
func (f *Fetcher) Fetch(ctx context.Context, channelName string) (string, error) {
	body, err := json.Marshal(gqlRequest{
		Query: generateTokenQuery,
		Variables: map[string]interface{}{
			"channelName": channelName,
		},
	})
	if err != nil {
		return "", fmt.Errorf("failed to marshal token query: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, graphqlURL, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("failed to create token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Origin", "https://picarto.tv")
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; AllChat/1.0)")

	resp, err := f.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("token request failed: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if err != nil {
		return "", fmt.Errorf("failed to read token response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("token endpoint returned %d: %.200s", resp.StatusCode, string(raw))
	}

	var parsed gqlResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return "", fmt.Errorf("failed to decode token response: %w", err)
	}
	if len(parsed.Errors) > 0 {
		return "", fmt.Errorf("token query error: %s", parsed.Errors[0].Message)
	}
	if parsed.Data.GenerateJwtToken.Key == "" {
		return "", fmt.Errorf("token response contained no key")
	}
	return parsed.Data.GenerateJwtToken.Key, nil
}
