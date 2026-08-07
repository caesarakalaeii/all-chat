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
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// A delegated moderator's OWN platform credential (ADR-0048).
//
// Read from mod_oauth_credentials and nowhere else. That table is keyed on the MODERATOR, never
// on a channel, and the separation is not fastidiousness: the listeners select chat-reading
// credentials by channel with no user scoping (`twitch-eventsub-listener/channels/manager.go`
// matches `LOWER(twitch_login) = LOWER(channel_id)`), so a moderator-scoped row in those tables
// would become a candidate INGEST credential and could silently break chat on a real channel.

// ModCredential is a delegated moderator's decrypted credential for one platform.
//
// PlatformUserID is deliberately not called BroadcasterID. It is the id sent as the platform's
// *moderator* field, and the entire correctness of the delegated write path is that the two are
// different values: the broadcaster is the overlay owner's channel, resolved separately through
// the owner-reach anchor.
type ModCredential struct {
	AccessToken    string
	RefreshToken   string
	PlatformUserID string
	GrantedScopes  []string
	ExpiresAt      time.Time
}

// ModTwitchSource resolves and refreshes a delegated moderator's own Twitch credential.
type ModTwitchSource struct {
	db      *pgxpool.Pool
	cipher  Cipher
	refresh *twitchRefresher
}

// NewModTwitchSource builds a source over mod_oauth_credentials. clientID/clientSecret are the
// All-Chat Twitch application credentials used for the refresh grant.
func NewModTwitchSource(db *pgxpool.Pool, cipher Cipher, clientID, clientSecret string) *ModTwitchSource {
	return &ModTwitchSource{
		db:      db,
		cipher:  cipher,
		refresh: newTwitchRefresher(clientID, clientSecret),
	}
}

// Resolve returns the moderator's decrypted Twitch credential.
//
// It is keyed on the moderator alone — no channel — because Twitch's moderation scopes are
// role-based rather than channel-scoped: one consent serves every streamer who delegated Twitch
// to them. ErrNoCredential means they have not consented yet, which is the normal state of a
// fresh grant (consent is deferred to first use) rather than an error condition.
func (s *ModTwitchSource) Resolve(ctx context.Context, userID string) (*ModCredential, error) {
	const query = `
		SELECT access_token, refresh_token, COALESCE(token_expires_at, TIMESTAMP 'epoch'),
		       granted_scopes, platform_user_id
		FROM mod_oauth_credentials
		WHERE user_id = $1 AND platform = 'twitch'`

	var encAccess, encRefresh, platformUserID string
	var expiresAt time.Time
	var scopes []string
	err := s.db.QueryRow(ctx, query, userID).Scan(&encAccess, &encRefresh, &expiresAt, &scopes, &platformUserID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNoCredential
	}
	if err != nil {
		return nil, fmt.Errorf("resolve moderator twitch credential: %w", err)
	}

	access, err := s.cipher.DecryptString(encAccess)
	if err != nil {
		return nil, fmt.Errorf("decrypt moderator access token: %w", err)
	}
	refresh, err := s.cipher.DecryptString(encRefresh)
	if err != nil {
		return nil, fmt.Errorf("decrypt moderator refresh token: %w", err)
	}

	return &ModCredential{
		AccessToken:    access,
		RefreshToken:   refresh,
		PlatformUserID: platformUserID,
		GrantedScopes:  scopes,
		ExpiresAt:      expiresAt,
	}, nil
}

// Refresh exchanges the moderator's refresh token for a new access token and writes the
// re-encrypted pair back to their own row.
//
// token-refresh-service keeps these fresh on a schedule (#656); this is the proactive/reactive
// path for the moment of use, so a token that lapsed between cycles does not cost the moderator a
// failed action. granted_scopes is left untouched — it is owned by the consent flow, and a
// refresh grant never widens it.
func (s *ModTwitchSource) Refresh(ctx context.Context, userID string, cred *ModCredential) error {
	if cred.RefreshToken == "" {
		return errors.New("tokens: no moderator refresh token available")
	}

	refreshed, err := s.refresh.exchange(ctx, cred.RefreshToken)
	if err != nil {
		return err
	}

	encAccess, err := s.cipher.EncryptString(refreshed.accessToken)
	if err != nil {
		return fmt.Errorf("encrypt refreshed moderator access token: %w", err)
	}
	encRefresh, err := s.cipher.EncryptString(refreshed.refreshToken)
	if err != nil {
		return fmt.Errorf("encrypt refreshed moderator refresh token: %w", err)
	}

	// Scoped to (user, platform) rather than a row id: the table's UNIQUE(user_id, platform)
	// makes that exact, and it cannot address another moderator's row by accident.
	const update = `
		UPDATE mod_oauth_credentials
		SET access_token = $1, refresh_token = $2, token_expires_at = $3, updated_at = NOW()
		WHERE user_id = $4 AND platform = 'twitch'`
	if _, err := s.db.Exec(ctx, update, encAccess, encRefresh, refreshed.expiresAt, userID); err != nil {
		return fmt.Errorf("persist refreshed moderator token: %w", err)
	}

	cred.AccessToken = refreshed.accessToken
	cred.RefreshToken = refreshed.refreshToken
	cred.ExpiresAt = refreshed.expiresAt
	return nil
}
