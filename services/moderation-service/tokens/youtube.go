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

	userRowID string // users.id, the write-back target on refresh
}

// YouTubeSource resolves and refreshes a YouTube broadcaster's own credential. Scope: a
// YouTube-login account (auth_provider='youtube') moderating its own channel — the
// primary credential lives on the users row (mirrors the Kick path). Channel ownership
// is enforced by the handler's source-membership check and by YouTube itself (a ban in
// a channel you don't own/moderate returns 403), so the channel id is not matched here.
// A linked YouTube account (youtube_oauth_tokens) is out of scope for v1: that table has
// no granted_scopes column, so the force-ssl opt-in cannot be tracked there.
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

const youtubeResolveQuery = `
	SELECT access_token, refresh_token, token_expires_at, granted_scopes, id::text
	FROM users
	WHERE id = $1
	  AND auth_provider = 'youtube'
	LIMIT 1`

// Resolve returns the requesting user's decrypted YouTube credential. channelID is
// accepted for interface symmetry but not used in the query (see the type doc).
// Returns ErrNoCredential when the user holds none.
func (s *YouTubeSource) Resolve(ctx context.Context, userID, _ string) (*YouTubeCredential, error) {
	var (
		encAccess, encRefresh string
		expiresAt             time.Time
		scopes                []string
		rowID                 string
	)
	err := s.db.QueryRow(ctx, youtubeResolveQuery, userID).Scan(&encAccess, &encRefresh, &expiresAt, &scopes, &rowID)
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
		userRowID:     rowID,
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

	const writeBack = `UPDATE users SET access_token=$1, refresh_token=$2, token_expires_at=$3, updated_at=NOW() WHERE id=$4`
	if _, err := s.db.Exec(ctx, writeBack, encAccess, encRefresh, newExpiry, cred.userRowID); err != nil {
		return fmt.Errorf("persist refreshed youtube token: %w", err)
	}

	cred.AccessToken = tr.AccessToken
	cred.RefreshToken = newRefresh
	cred.ExpiresAt = newExpiry
	return nil
}
