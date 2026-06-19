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

// defaultGoogleTokenURL is Google's OAuth token endpoint (refresh grant).
const defaultGoogleTokenURL = "https://oauth2.googleapis.com/token"

// YouTubeCredential is a decrypted, ready-to-use YouTube broadcaster credential.
// Unlike Twitch/Kick it carries no numeric broadcaster id: a ban needs the live
// broadcast's liveChatId (resolved from the channel) and the banned user's channelId
// (from the message), not the broadcaster's id.
type YouTubeCredential struct {
	AccessToken   string
	RefreshToken  string
	GrantedScopes []string
	ExpiresAt     time.Time

	origin credOrigin // write-back target on refresh
	rowID  string     // users.id or youtube_oauth_tokens.id
}

// YouTubeSource resolves and refreshes a YouTube broadcaster's own credential. It
// supports both a YouTube-login account moderating its own channel (the primary
// credential on the users row, mirroring the Kick path) AND a linked YouTube channel —
// a streamer whose All-Chat login is a different platform but who connected a YouTube
// channel as a source. The linked (and per-channel) credential lives in
// youtube_oauth_tokens, keyed by the channel id (UC...), carrying the opt-in
// granted_scopes (migration 062). The exact per-channel token is preferred over the
// channel-agnostic users row when both exist. Channel ownership is also enforced by the
// handler's source-membership check and by YouTube itself (a ban in a channel you do not
// own/moderate returns 403).
type YouTubeSource struct {
	db           *pgxpool.Pool
	cipher       Cipher
	httpClient   *http.Client
	clientID     string
	clientSecret string
	tokenURL     string
}

// NewYouTubeSource builds a YouTubeSource. clientID/clientSecret are the All-Chat Google
// OAuth application credentials used for the refresh grant.
func NewYouTubeSource(db *pgxpool.Pool, cipher Cipher, clientID, clientSecret string) *YouTubeSource {
	return &YouTubeSource{
		db:           db,
		cipher:       cipher,
		httpClient:   &http.Client{Timeout: 10 * time.Second},
		clientID:     clientID,
		clientSecret: clientSecret,
		tokenURL:     defaultGoogleTokenURL,
	}
}

// youtubeResolveQuery selects the requesting user's own YouTube credential for a
// channel, scoped to identities the user owns. channelID from the overlay is the
// channel id (UC...): the exact per-channel token in youtube_oauth_tokens is preferred
// (pri ASC), falling back to the channel-agnostic users row of a YouTube-login account.
const youtubeResolveQuery = `
	SELECT access_token, refresh_token, token_expires_at, granted_scopes, origin, row_id
	FROM (
		SELECT y.access_token, y.refresh_token, y.expiry AS token_expires_at, y.granted_scopes,
		       2 AS origin, y.id::text AS row_id, 1 AS pri
		FROM youtube_oauth_tokens y
		WHERE y.user_id = $1
		  AND y.channel_id = $2
		UNION ALL
		SELECT u.access_token, u.refresh_token, u.token_expires_at, u.granted_scopes,
		       1 AS origin, u.id::text AS row_id, 2 AS pri
		FROM users u
		WHERE u.id = $1
		  AND u.auth_provider = 'youtube'
	) c
	ORDER BY c.pri ASC
	LIMIT 1`

// Resolve returns the requesting user's decrypted YouTube credential for channelID (a
// YouTube channel id, UC...). Returns ErrNoCredential when the user holds none.
func (s *YouTubeSource) Resolve(ctx context.Context, userID, channelID string) (*YouTubeCredential, error) {
	var (
		encAccess, encRefresh string
		expiresAt             time.Time
		scopes                []string
		origin                int
		rowID                 string
	)
	err := s.db.QueryRow(ctx, youtubeResolveQuery, userID, channelID).Scan(&encAccess, &encRefresh, &expiresAt, &scopes, &origin, &rowID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNoCredential
	}
	if err != nil {
		return nil, fmt.Errorf("resolve youtube credential: %w", err)
	}

	access, err := s.cipher.DecryptString(encAccess)
	if err != nil {
		return nil, fmt.Errorf("decrypt access token: %w", err)
	}
	refresh, err := s.cipher.DecryptString(encRefresh)
	if err != nil {
		return nil, fmt.Errorf("decrypt refresh token: %w", err)
	}

	return &YouTubeCredential{
		AccessToken:   access,
		RefreshToken:  refresh,
		GrantedScopes: scopes,
		ExpiresAt:     expiresAt,
		origin:        credOrigin(origin),
		rowID:         rowID,
	}, nil
}

// Refresh exchanges the credential's refresh token for a new access token via Google's
// OAuth endpoint, persists the re-encrypted tokens to the users row, and updates cred in
// place. Google does not reissue the refresh token on refresh, so the existing one is
// kept. granted_scopes is left untouched (owned by the consent flow).
func (s *YouTubeSource) Refresh(ctx context.Context, cred *YouTubeCredential) error {
	if cred.RefreshToken == "" {
		return errors.New("tokens: no youtube refresh token available")
	}

	form := url.Values{
		"client_id":     {s.clientID},
		"client_secret": {s.clientSecret},
		"grant_type":    {"refresh_token"},
		"refresh_token": {cred.RefreshToken},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return fmt.Errorf("build youtube refresh request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("youtube token refresh request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("youtube token refresh returned %s: %s", strconv.Itoa(resp.StatusCode), string(snippet))
	}

	var tr struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tr); err != nil {
		return fmt.Errorf("decode youtube refresh response: %w", err)
	}
	if tr.AccessToken == "" {
		return errors.New("youtube token refresh returned an empty access token")
	}

	newRefresh := tr.RefreshToken
	if newRefresh == "" {
		newRefresh = cred.RefreshToken // Google does not reissue the refresh token on refresh
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

	// Write the re-encrypted tokens back to the origin row. granted_scopes is left
	// untouched (owned by the consent flow). The linked row uses the `expiry` column and
	// keeps encryption_version=1 (we always write encrypted).
	var query string
	switch cred.origin {
	case originUsers:
		query = `UPDATE users SET access_token=$1, refresh_token=$2, token_expires_at=$3, updated_at=NOW() WHERE id=$4`
	case originLinked:
		query = `UPDATE youtube_oauth_tokens SET access_token=$1, refresh_token=$2, expiry=$3, encryption_version=1, updated_at=NOW() WHERE id=$4::uuid`
	default:
		return fmt.Errorf("tokens: unknown youtube credential origin %d", cred.origin)
	}
	if _, err := s.db.Exec(ctx, query, encAccess, encRefresh, newExpiry, cred.rowID); err != nil {
		return fmt.Errorf("persist refreshed youtube token: %w", err)
	}

	cred.AccessToken = tr.AccessToken
	cred.RefreshToken = newRefresh
	cred.ExpiresAt = newExpiry
	return nil
}
