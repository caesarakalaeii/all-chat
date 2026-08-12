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

// Package tokens resolves a broadcaster's own Twitch OAuth credential for a
// channel, so the moderation service can act AS the broadcaster (broadcaster ==
// moderator). The credential is the requesting user's own linked Twitch identity,
// selected per ADR-0016 (the users row for a Twitch-login account, or a
// twitch_oauth_tokens row for a YouTube/Kick-login account that linked Twitch).
// Tokens are encrypted at rest; this package decrypts on read and re-encrypts on
// refresh, mirroring services/auth-service/handlers/chat_send.go.
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

	"github.com/caesar/all-chat/shared/youtubetoken"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrNoCredential indicates the requesting user holds no credential for the channel —
// they are not the broadcaster (or never linked the platform). Shared with
// shared/youtubetoken so a missing YouTube credential returned by the (aliased)
// YouTubeSource.Resolve is recognised identically by the moderation dispatch path and
// the scope checkers. The handler maps this to 422: "you do not hold moderator
// credentials for this channel".
var ErrNoCredential = youtubetoken.ErrNoCredential

// defaultTwitchTokenURL is Twitch's OAuth token endpoint (refresh grant).
const defaultTwitchTokenURL = "https://id.twitch.tv/oauth2/token"

// credOrigin records which table a credential came from, so a refresh writes the
// new tokens back to the right row.
type credOrigin int

const (
	originUsers  credOrigin = 1 // users row (Twitch-login account)
	originLinked credOrigin = 2 // twitch_oauth_tokens row (linked Twitch credential)
)

// TwitchCredential is a decrypted, ready-to-use broadcaster credential.
type TwitchCredential struct {
	AccessToken   string    // decrypted
	RefreshToken  string    // decrypted
	BroadcasterID string    // Twitch user id == moderator id for own-channel moderation
	GrantedScopes []string  // OAuth scopes currently granted on this credential
	ExpiresAt     time.Time // access-token expiry

	origin credOrigin // write-back target on refresh
	rowID  string     // users.id or twitch_oauth_tokens.id
}

// Cipher decrypts tokens on read and encrypts them on refresh write-back.
// *encryption.MultiKeyEncryptor satisfies this.
type Cipher interface {
	EncryptString(plaintext string) (string, error)
	DecryptString(ciphertext string) (string, error)
}

// TwitchSource resolves and refreshes broadcaster Twitch credentials.
type TwitchSource struct {
	db           *pgxpool.Pool
	cipher       Cipher
	httpClient   *http.Client
	clientID     string
	clientSecret string
	tokenURL     string
}

// NewTwitchSource builds a TwitchSource. clientID/clientSecret are the All-Chat
// Twitch application credentials used for the refresh grant.
func NewTwitchSource(db *pgxpool.Pool, cipher Cipher, clientID, clientSecret string) *TwitchSource {
	return &TwitchSource{
		db:           db,
		cipher:       cipher,
		httpClient:   &http.Client{Timeout: 10 * time.Second},
		clientID:     clientID,
		clientSecret: clientSecret,
		tokenURL:     defaultTwitchTokenURL,
	}
}

// resolveQuery selects the best Twitch credential the requesting user holds for a
// channel. It scopes to the user's OWN identities (users.id / twitch_oauth_tokens.user_id):
// a streamer can moderate only channels for which they hold the broadcaster token.
// The users row is preferred over a linked row when both exist (pri ASC), matching
// ADR-0016's selection. channel_id from the overlay is the Twitch login.
const resolveQuery = `
	SELECT access_token, refresh_token, token_expires_at, granted_scopes, broadcaster_id, origin, row_id
	FROM (
		SELECT u.access_token, u.refresh_token, u.token_expires_at, u.granted_scopes,
		       u.twitch_id AS broadcaster_id, 1 AS origin, u.id::text AS row_id, 1 AS pri
		FROM users u
		WHERE u.id = $1
		  AND u.auth_provider = 'twitch'
		  AND LOWER(u.username) = LOWER($2)
		  AND u.twitch_id IS NOT NULL
		UNION ALL
		SELECT t.access_token, t.refresh_token, t.token_expires_at, t.granted_scopes,
		       t.twitch_user_id AS broadcaster_id, 2 AS origin, t.id::text AS row_id, 2 AS pri
		FROM twitch_oauth_tokens t
		WHERE t.user_id = $1
		  AND LOWER(t.twitch_login) = LOWER($2)
	) c
	ORDER BY c.pri ASC
	LIMIT 1`

// Resolve returns the requesting user's decrypted Twitch credential for channelID
// (a Twitch login). Returns ErrNoCredential when the user holds none.
func (s *TwitchSource) Resolve(ctx context.Context, userID, channelID string) (*TwitchCredential, error) {
	var (
		encAccess, encRefresh string
		expiresAt             time.Time
		scopes                []string
		broadcasterID         string
		origin                int
		rowID                 string
	)
	err := s.db.QueryRow(ctx, resolveQuery, userID, channelID).Scan(
		&encAccess, &encRefresh, &expiresAt, &scopes, &broadcasterID, &origin, &rowID,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNoCredential
	}
	if err != nil {
		return nil, fmt.Errorf("resolve twitch credential: %w", err)
	}

	access, err := s.cipher.DecryptString(encAccess)
	if err != nil {
		return nil, fmt.Errorf("decrypt access token: %w", err)
	}
	refresh, err := s.cipher.DecryptString(encRefresh)
	if err != nil {
		return nil, fmt.Errorf("decrypt refresh token: %w", err)
	}

	return &TwitchCredential{
		AccessToken:   access,
		RefreshToken:  refresh,
		BroadcasterID: broadcasterID,
		GrantedScopes: scopes,
		ExpiresAt:     expiresAt,
		origin:        credOrigin(origin),
		rowID:         rowID,
	}, nil
}

// Refresh exchanges the credential's refresh token for a new access token via
// Twitch's OAuth endpoint, persists the re-encrypted tokens to the origin row, and
// updates cred in place. granted_scopes is left untouched — it is owned by the
// consent / token-refresh flows, and a refresh-grant response never widens it.
func (s *TwitchSource) Refresh(ctx context.Context, cred *TwitchCredential) error {
	if cred.RefreshToken == "" {
		return errors.New("tokens: no refresh token available")
	}

	refresher := &twitchRefresher{
		httpClient: s.httpClient, clientID: s.clientID, clientSecret: s.clientSecret, tokenURL: s.tokenURL,
	}
	refreshed, err := refresher.exchange(ctx, cred.RefreshToken)
	if err != nil {
		return err
	}
	newRefresh, newExpiry := refreshed.refreshToken, refreshed.expiresAt

	encAccess, err := s.cipher.EncryptString(refreshed.accessToken)
	if err != nil {
		return fmt.Errorf("encrypt refreshed access token: %w", err)
	}
	encRefresh, err := s.cipher.EncryptString(newRefresh)
	if err != nil {
		return fmt.Errorf("encrypt refreshed refresh token: %w", err)
	}

	var query string
	switch cred.origin {
	case originUsers:
		query = `UPDATE users SET access_token=$1, refresh_token=$2, token_expires_at=$3, updated_at=NOW() WHERE id=$4`
	case originLinked:
		query = `UPDATE twitch_oauth_tokens SET access_token=$1, refresh_token=$2, token_expires_at=$3, updated_at=NOW() WHERE id=$4`
	default:
		return fmt.Errorf("tokens: unknown credential origin %d", cred.origin)
	}
	if _, err := s.db.Exec(ctx, query, encAccess, encRefresh, newExpiry, cred.rowID); err != nil {
		return fmt.Errorf("persist refreshed token: %w", err)
	}

	cred.AccessToken = refreshed.accessToken
	cred.RefreshToken = newRefresh
	cred.ExpiresAt = newExpiry
	return nil
}

// ErrOwnerChannelUnverified reports that the overlay owner cannot prove they control a channel.
//
// The owner-reach anchor (ADR-0048): delegation never exceeds what the owner could do themselves,
// so a moderator may only act on a channel the owner demonstrably controls. It proves **control
// only** — never that the owner holds a moderation scope, a live token or premium. Requiring any
// of those would deny delegation to exactly the streamer who delegates *because* they do not
// moderate themselves.
//
// Aliased from shared/youtubetoken (like ErrNoCredential) rather than declared here, because the
// YouTube anchor lives in that package: two separate sentinels would compare unequal under
// errors.Is, and the dispatcher would report an unanchored owner as a 502 platform error instead of
// the actionable owner_channel_unverified. The Twitch and Kick anchors below return this same value.
var ErrOwnerChannelUnverified = youtubetoken.ErrOwnerChannelUnverified

// ownerAnchorQuery mirrors resolveQuery's UNION and ADR-0016 preference, minus the two things the
// anchor must not care about: it selects no token material and applies no scope predicate.
const ownerAnchorQuery = `
	SELECT broadcaster_id
	FROM (
		SELECT u.twitch_id AS broadcaster_id, 1 AS pri
		FROM users u
		WHERE u.id = $1
		  AND u.auth_provider = 'twitch'
		  AND LOWER(u.username) = LOWER($2)
		  AND u.twitch_id IS NOT NULL
		UNION ALL
		SELECT t.twitch_user_id AS broadcaster_id, 2 AS pri
		FROM twitch_oauth_tokens t
		WHERE t.user_id = $1
		  AND LOWER(t.twitch_login) = LOWER($2)
	) c
	ORDER BY c.pri ASC
	LIMIT 1`

// OwnerTwitchAnchor returns the numeric Twitch broadcaster id for a channel the overlay owner
// controls, or ErrOwnerChannelUnverified.
//
// A Twitch source's channel_id IS the login, which is what makes this answerable: the owner holds
// a credential row whose login equals it. The numeric id it yields is the `broadcaster_id` the
// delegated write needs — the moderator's own credential supplies only the `moderator_id`.
func (s *TwitchSource) OwnerTwitchAnchor(ctx context.Context, ownerUserID, channelID string) (string, error) {
	var broadcasterID string
	err := s.db.QueryRow(ctx, ownerAnchorQuery, ownerUserID, channelID).Scan(&broadcasterID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrOwnerChannelUnverified
	}
	if err != nil {
		return "", fmt.Errorf("resolve owner twitch anchor: %w", err)
	}
	if broadcasterID == "" {
		return "", ErrOwnerChannelUnverified
	}
	return broadcasterID, nil
}

// twitchRefresher performs the Twitch OAuth refresh grant. Shared by the broadcaster and the
// delegated-moderator credential sources: the exchange is identical, only the row it is written
// back to differs, and duplicating it would let the two drift.
type twitchRefresher struct {
	httpClient   *http.Client
	clientID     string
	clientSecret string
	tokenURL     string
}

func newTwitchRefresher(clientID, clientSecret string) *twitchRefresher {
	return &twitchRefresher{
		httpClient:   &http.Client{Timeout: 10 * time.Second},
		clientID:     clientID,
		clientSecret: clientSecret,
		tokenURL:     defaultTwitchTokenURL,
	}
}

// refreshedToken is one successful refresh-grant response, normalized.
type refreshedToken struct {
	accessToken  string
	refreshToken string
	expiresAt    time.Time
}

// exchange trades a refresh token for a fresh access token.
func (r *twitchRefresher) exchange(ctx context.Context, refreshToken string) (refreshedToken, error) {
	form := url.Values{
		"client_id":     {r.clientID},
		"client_secret": {r.clientSecret},
		"grant_type":    {"refresh_token"},
		"refresh_token": {refreshToken},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return refreshedToken{}, fmt.Errorf("build refresh request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return refreshedToken{}, fmt.Errorf("twitch token refresh request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return refreshedToken{}, fmt.Errorf("twitch token refresh returned %s: %s",
			strconv.Itoa(resp.StatusCode), string(snippet))
	}

	var tr struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tr); err != nil {
		return refreshedToken{}, fmt.Errorf("decode refresh response: %w", err)
	}
	if tr.AccessToken == "" {
		return refreshedToken{}, errors.New("twitch token refresh returned an empty access token")
	}

	// Twitch may rotate the refresh token; keep the old one if it didn't.
	rotated := tr.RefreshToken
	if rotated == "" {
		rotated = refreshToken
	}
	return refreshedToken{
		accessToken:  tr.AccessToken,
		refreshToken: rotated,
		expiresAt:    time.Now().Add(time.Duration(tr.ExpiresIn) * time.Second),
	}, nil
}
