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
// own-channel moderation), which is users.kick_id (primary login) or
// kick_oauth_tokens.kick_user_id (linked account).
type KickCredential struct {
	AccessToken   string
	RefreshToken  string
	BroadcasterID string // numeric Kick user id
	GrantedScopes []string
	ExpiresAt     time.Time

	origin credOrigin // write-back target on refresh
	rowID  string     // users.id or kick_oauth_tokens.id
}

// KickSource resolves and refreshes a Kick broadcaster's own credential. It supports
// both a Kick-login account moderating its own channel (the primary credential on the
// users row: access_token/refresh_token/token_expires_at/granted_scopes/kick_id,
// mirroring the Twitch users-row path, ADR-0016) AND a linked Kick account — a streamer
// whose All-Chat login is a different platform but who connected Kick as a source. The
// linked credential lives in kick_oauth_tokens, keyed by the channel slug, carrying the
// numeric broadcaster id (kick_user_id) and the opt-in granted_scopes (migration 062).
// The users row is preferred when both exist. Legacy listener-only rows without a
// kick_user_id are ignored (they cannot satisfy the moderation API).
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

// kickResolveQuery selects the requesting user's own Kick credential for a channel,
// scoped to identities the user owns. channelID from the overlay is the Kick slug:
// matched against users.username for a Kick-login account, and against
// kick_oauth_tokens.channel_id for a linked Kick account. The users row is preferred
// (pri ASC) when both exist. The kick_oauth_tokens branch requires a non-null
// kick_user_id — legacy listener-only rows lack the numeric broadcaster id moderation
// needs and must not be resolved.
const kickResolveQuery = `
	SELECT access_token, refresh_token, token_expires_at, granted_scopes, broadcaster_id, origin, row_id
	FROM (
		SELECT u.access_token, u.refresh_token, u.token_expires_at, u.granted_scopes,
		       u.kick_id AS broadcaster_id, 1 AS origin, u.id::text AS row_id, 1 AS pri
		FROM users u
		WHERE u.id = $1
		  AND u.auth_provider = 'kick'
		  AND LOWER(u.username) = LOWER($2)
		  AND u.kick_id IS NOT NULL
		UNION ALL
		SELECT k.access_token, k.refresh_token, k.expiry AS token_expires_at, k.granted_scopes,
		       k.kick_user_id AS broadcaster_id, 2 AS origin, k.id::text AS row_id, 2 AS pri
		FROM kick_oauth_tokens k
		WHERE k.user_id = $1
		  AND LOWER(k.channel_id) = LOWER($2)
		  AND k.kick_user_id IS NOT NULL
	) c
	ORDER BY c.pri ASC
	LIMIT 1`

// Resolve returns the requesting user's decrypted Kick credential for channelID (a
// Kick slug). Returns ErrNoCredential when the user holds none.
func (s *KickSource) Resolve(ctx context.Context, userID, channelID string) (*KickCredential, error) {
	var (
		encAccess, encRefresh string
		expiresAt             time.Time
		scopes                []string
		broadcasterID         string
		origin                int
		rowID                 string
	)
	err := s.db.QueryRow(ctx, kickResolveQuery, userID, channelID).Scan(
		&encAccess, &encRefresh, &expiresAt, &scopes, &broadcasterID, &origin, &rowID,
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
		origin:        credOrigin(origin),
		rowID:         rowID,
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

	refresher := &kickRefresher{
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

	// Write the re-encrypted tokens back to the origin row. granted_scopes /
	// kick_user_id are left untouched (owned by the consent flow). The linked row uses
	// the `expiry` column and keeps encryption_version=1 (we always write encrypted).
	var query string
	switch cred.origin {
	case originUsers:
		query = `UPDATE users SET access_token=$1, refresh_token=$2, token_expires_at=$3, updated_at=NOW() WHERE id=$4`
	case originLinked:
		query = `UPDATE kick_oauth_tokens SET access_token=$1, refresh_token=$2, expiry=$3, encryption_version=1, updated_at=NOW() WHERE id=$4::int`
	default:
		return fmt.Errorf("tokens: unknown kick credential origin %d", cred.origin)
	}
	if _, err := s.db.Exec(ctx, query, encAccess, encRefresh, newExpiry, cred.rowID); err != nil {
		return fmt.Errorf("persist refreshed kick token: %w", err)
	}

	cred.AccessToken = refreshed.accessToken
	cred.RefreshToken = newRefresh
	cred.ExpiresAt = newExpiry
	return nil
}

// kickOwnerAnchorQuery mirrors kickResolveQuery's UNION and its preference order, minus the two
// things the anchor must not care about: it selects no token material and applies no scope
// predicate. The kick_user_id NOT NULL requirement is not one of those — without the numeric id
// there is no broadcaster_user_id to return, so a legacy listener row anchors nothing.
const kickOwnerAnchorQuery = `
	SELECT broadcaster_id
	FROM (
		SELECT u.kick_id AS broadcaster_id, 1 AS pri
		FROM users u
		WHERE u.id = $1
		  AND u.auth_provider = 'kick'
		  AND LOWER(u.username) = LOWER($2)
		  AND u.kick_id IS NOT NULL
		UNION ALL
		SELECT k.kick_user_id AS broadcaster_id, 2 AS pri
		FROM kick_oauth_tokens k
		WHERE k.user_id = $1
		  AND LOWER(k.channel_id) = LOWER($2)
		  AND k.kick_user_id IS NOT NULL
	) c
	ORDER BY c.pri ASC
	LIMIT 1`

// OwnerKickAnchor returns the numeric Kick broadcaster id for a channel the overlay owner
// controls, or ErrOwnerChannelUnverified (ADR-0048's owner-reach anchor).
//
// This is the ONLY legitimate source of `broadcaster_user_id` on a delegated Kick write. Kick's
// moderation endpoints carry no moderator field — the acting identity is implied by the token —
// so the broadcaster id is the single id in the request, and taking it from the moderator's own
// credential (which is what resolving by the caller does) would point the call at the
// moderator's channel instead of the streamer's.
//
// Like the Twitch anchor it proves control only: no scope predicate, no token read.
func (s *KickSource) OwnerKickAnchor(ctx context.Context, ownerUserID, channelID string) (string, error) {
	var broadcasterID string
	err := s.db.QueryRow(ctx, kickOwnerAnchorQuery, ownerUserID, channelID).Scan(&broadcasterID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrOwnerChannelUnverified
	}
	if err != nil {
		return "", fmt.Errorf("resolve owner kick anchor: %w", err)
	}
	if broadcasterID == "" {
		return "", ErrOwnerChannelUnverified
	}
	return broadcasterID, nil
}

// kickRefresher performs the Kick OAuth 2.1 refresh grant. Shared by the broadcaster and the
// delegated-moderator credential sources: the exchange is identical, only the row it is written
// back to differs, and duplicating it would let the two drift.
type kickRefresher struct {
	httpClient   *http.Client
	clientID     string
	clientSecret string
	tokenURL     string
}

func newKickRefresher(clientID, clientSecret string) *kickRefresher {
	return &kickRefresher{
		httpClient:   &http.Client{Timeout: 10 * time.Second},
		clientID:     clientID,
		clientSecret: clientSecret,
		tokenURL:     defaultKickTokenURL,
	}
}

// exchange trades a refresh token for a fresh access token. A response that omits the refresh
// token keeps the caller's existing one — Kick does not always rotate it, and dropping it would
// end the credential's life at the next expiry.
func (r *kickRefresher) exchange(ctx context.Context, refreshToken string) (refreshedToken, error) {
	form := url.Values{
		"client_id":     {r.clientID},
		"client_secret": {r.clientSecret},
		"grant_type":    {"refresh_token"},
		"refresh_token": {refreshToken},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return refreshedToken{}, fmt.Errorf("build kick refresh request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return refreshedToken{}, fmt.Errorf("kick token refresh request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return refreshedToken{}, fmt.Errorf("kick token refresh returned %s: %s",
			strconv.Itoa(resp.StatusCode), string(snippet))
	}

	var tr struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tr); err != nil {
		return refreshedToken{}, fmt.Errorf("decode kick refresh response: %w", err)
	}
	if tr.AccessToken == "" {
		return refreshedToken{}, errors.New("kick token refresh returned an empty access token")
	}

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
