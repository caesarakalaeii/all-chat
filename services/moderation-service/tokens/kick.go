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

package tokens

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// defaultKickTokenURL is Kick's OAuth 2.1 token endpoint (refresh grant).
const defaultKickTokenURL = "https://id.kick.com/oauth/token"

// KickCredential is a decrypted, ready-to-use Kick broadcaster credential. The Kick
// moderation API needs the broadcaster's NUMERIC user id (broadcaster == moderator for
// own-channel moderation), which is users.kick_id.
type KickCredential struct {
	AccessToken   string
	RefreshToken  string
	BroadcasterID string // numeric Kick user id (users.kick_id)
	GrantedScopes []string
	ExpiresAt     time.Time

	userRowID string // users.id, the write-back target on refresh
}

// KickSource resolves and refreshes a Kick broadcaster's own credential. Scope: a
// Kick-login account (auth_provider='kick') moderating its own channel — the primary
// credential lives on the users row (access_token/refresh_token/token_expires_at/
// granted_scopes/kick_id), mirroring the Twitch users-row path (ADR-0016). A linked
// Kick account (kick_oauth_tokens) is out of scope for v1: that table stores neither
// the numeric broadcaster id nor per-link granted scopes, so such a source is reported
// as missing a credential.
type KickSource struct {
	db           *pgxpool.Pool
	cipher       Cipher
	httpClient   *http.Client
	clientID     string
	clientSecret string
	tokenURL     string
}

// NewKickSource builds a KickSource. clientID/clientSecret are the All-Chat Kick
// application credentials used for the refresh grant.
func NewKickSource(db *pgxpool.Pool, cipher Cipher, clientID, clientSecret string) *KickSource {
	return &KickSource{
		db:           db,
		cipher:       cipher,
		httpClient:   &http.Client{Timeout: 10 * time.Second},
		clientID:     clientID,
		clientSecret: clientSecret,
		tokenURL:     defaultKickTokenURL,
	}
}

// kickResolveQuery selects the requesting user's own Kick credential for a channel.
// channelID from the overlay is the Kick slug, matched against users.username (the
// Kick login/slug) for a kick-login account the requester owns.
const kickResolveQuery = `
	SELECT access_token, refresh_token, token_expires_at, granted_scopes, kick_id, id::text
	FROM users
	WHERE id = $1
	  AND auth_provider = 'kick'
	  AND LOWER(username) = LOWER($2)
	  AND kick_id IS NOT NULL
	LIMIT 1`

// Resolve returns the requesting user's decrypted Kick credential for channelID (a
// Kick slug). Returns ErrNoCredential when the user holds none.
func (s *KickSource) Resolve(ctx context.Context, userID, channelID string) (*KickCredential, error) {
	var (
		encAccess, encRefresh string
		expiresAt             time.Time
		scopes                []string
		broadcasterID         string
		rowID                 string
	)
	err := s.db.QueryRow(ctx, kickResolveQuery, userID, channelID).Scan(
		&encAccess, &encRefresh, &expiresAt, &scopes, &broadcasterID, &rowID,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNoCredential
	}
	if err != nil {
		return nil, fmt.Errorf("resolve kick credential: %w", err)
	}

	access, err := s.cipher.DecryptString(encAccess)
	if err != nil {
		return nil, fmt.Errorf("decrypt access token: %w", err)
	}
	refresh, err := s.cipher.DecryptString(encRefresh)
	if err != nil {
		return nil, fmt.Errorf("decrypt refresh token: %w", err)
	}

	return &KickCredential{
		AccessToken:   access,
		RefreshToken:  refresh,
		BroadcasterID: broadcasterID,
		GrantedScopes: scopes,
		ExpiresAt:     expiresAt,
		userRowID:     rowID,
	}, nil
}

// Refresh exchanges the credential's refresh token for a new access token via Kick's
// OAuth endpoint, persists the re-encrypted tokens to the users row, and updates cred
// in place. granted_scopes is left untouched — it is owned by the consent flow, and a
// refresh-grant response never widens it.
func (s *KickSource) Refresh(ctx context.Context, cred *KickCredential) error {
	if cred.RefreshToken == "" {
		return errors.New("tokens: no kick refresh token available")
	}

	form := url.Values{
		"client_id":     {s.clientID},
		"client_secret": {s.clientSecret},
		"grant_type":    {"refresh_token"},
		"refresh_token": {cred.RefreshToken},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return fmt.Errorf("build kick refresh request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("kick token refresh request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("kick token refresh returned %s: %s", strconv.Itoa(resp.StatusCode), string(snippet))
	}

	var tr struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tr); err != nil {
		return fmt.Errorf("decode kick refresh response: %w", err)
	}
	if tr.AccessToken == "" {
		return errors.New("kick token refresh returned an empty access token")
	}

	newRefresh := tr.RefreshToken
	if newRefresh == "" {
		newRefresh = cred.RefreshToken // Kick may not rotate the refresh token
	}
	newExpiry := time.Now().Add(time.Duration(tr.ExpiresIn) * time.Second)

	encAccess, err := s.cipher.EncryptString(tr.AccessToken)
	if err != nil {
		return fmt.Errorf("encrypt refreshed access token: %w", err)
	}
	encRefresh, err := s.cipher.EncryptString(newRefresh)
	if err != nil {
		return fmt.Errorf("encrypt refreshed refresh token: %w", err)
	}

	const writeBack = `UPDATE users SET access_token=$1, refresh_token=$2, token_expires_at=$3, updated_at=NOW() WHERE id=$4`
	if _, err := s.db.Exec(ctx, writeBack, encAccess, encRefresh, newExpiry, cred.userRowID); err != nil {
		return fmt.Errorf("persist refreshed kick token: %w", err)
	}

	cred.AccessToken = tr.AccessToken
	cred.RefreshToken = newRefresh
	cred.ExpiresAt = newExpiry
	return nil
}
