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

	"github.com/caesar/all-chat/shared/youtubetoken"
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
	return resolveModCredential(ctx, s.db, s.cipher, userID, "twitch")
}

// resolveModCredential reads one moderator credential row and decrypts it.
//
// Shared by every platform's mod source: the row shape is identical and the platform is a value,
// so the only thing a per-platform copy could add is a place for the two to drift apart. The
// platform is always a literal supplied by the source, never caller input.
func resolveModCredential(
	ctx context.Context, db *pgxpool.Pool, cipher Cipher, userID, platform string,
) (*ModCredential, error) {
	const query = `
		SELECT access_token, refresh_token, COALESCE(token_expires_at, TIMESTAMP 'epoch'),
		       granted_scopes, platform_user_id
		FROM mod_oauth_credentials
		WHERE user_id = $1 AND platform = $2`

	var encAccess, encRefresh, platformUserID string
	var expiresAt time.Time
	var scopes []string
	err := db.QueryRow(ctx, query, userID, platform).Scan(&encAccess, &encRefresh, &expiresAt, &scopes, &platformUserID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNoCredential
	}
	if err != nil {
		return nil, fmt.Errorf("resolve moderator %s credential: %w", platform, err)
	}

	access, err := cipher.DecryptString(encAccess)
	if err != nil {
		return nil, fmt.Errorf("decrypt moderator access token: %w", err)
	}
	refresh, err := cipher.DecryptString(encRefresh)
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

// persistRefreshedModCredential writes a refreshed token pair back to the moderator's own row.
//
// Scoped to (user, platform) rather than a row id: the table's UNIQUE(user_id, platform) makes
// that exact, and it cannot address another moderator's row by accident. granted_scopes is left
// untouched — it is owned by the consent flow, and a refresh grant never widens it.
func persistRefreshedModCredential(
	ctx context.Context, db *pgxpool.Pool, cipher Cipher, userID, platform string, refreshed refreshedToken,
) error {
	encAccess, err := cipher.EncryptString(refreshed.accessToken)
	if err != nil {
		return fmt.Errorf("encrypt refreshed moderator access token: %w", err)
	}
	encRefresh, err := cipher.EncryptString(refreshed.refreshToken)
	if err != nil {
		return fmt.Errorf("encrypt refreshed moderator refresh token: %w", err)
	}

	const update = `
		UPDATE mod_oauth_credentials
		SET access_token = $1, refresh_token = $2, token_expires_at = $3, updated_at = NOW()
		WHERE user_id = $4 AND platform = $5`
	if _, err := db.Exec(ctx, update, encAccess, encRefresh, refreshed.expiresAt, userID, platform); err != nil {
		return fmt.Errorf("persist refreshed moderator token: %w", err)
	}
	return nil
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
	if err := persistRefreshedModCredential(ctx, s.db, s.cipher, userID, "twitch", refreshed); err != nil {
		return err
	}

	cred.AccessToken = refreshed.accessToken
	cred.RefreshToken = refreshed.refreshToken
	cred.ExpiresAt = refreshed.expiresAt
	return nil
}

// ModYouTubeSource resolves and refreshes a delegated moderator's own YouTube credential.
//
// The credential is one force-ssl grant, which is all YouTube needs from a moderator: its
// liveChatBans endpoint identifies the actor by the token and the target by channel id, with no
// moderator field to fill in. YouTube then re-checks on every call that the token's account owns or
// moderates that live chat — the platform-enforced authority the whole design defers to.
//
// PlatformUserID here is the moderator's GOOGLE account id (what the consent callback resolves from
// Google's userinfo), not a YouTube channel id. It is recorded for attribution only; nothing in the
// request carries it.
type ModYouTubeSource struct {
	db      *pgxpool.Pool
	cipher  Cipher
	refresh *youtubetoken.Refresher
}

// NewModYouTubeSource builds a source over mod_oauth_credentials. clientID/clientSecret are the
// All-Chat Google OAuth application credentials used for the refresh grant; tokenURL may be empty
// to use Google's (it exists as a test seam).
func NewModYouTubeSource(db *pgxpool.Pool, cipher Cipher, clientID, clientSecret, tokenURL string) *ModYouTubeSource {
	return &ModYouTubeSource{
		db:      db,
		cipher:  cipher,
		refresh: youtubetoken.NewRefresher(clientID, clientSecret, tokenURL),
	}
}

// Resolve returns the moderator's decrypted YouTube credential, or ErrNoCredential when they have
// not consented for YouTube yet (the normal state of a fresh grant).
func (s *ModYouTubeSource) Resolve(ctx context.Context, userID string) (*ModCredential, error) {
	return resolveModCredential(ctx, s.db, s.cipher, userID, "youtube")
}

// Refresh exchanges the moderator's refresh token for a new access token and writes the
// re-encrypted pair back to their own row.
func (s *ModYouTubeSource) Refresh(ctx context.Context, userID string, cred *ModCredential) error {
	if cred.RefreshToken == "" {
		return errors.New("tokens: no moderator refresh token available")
	}

	refreshed, err := s.refresh.Exchange(ctx, cred.RefreshToken)
	if err != nil {
		return err
	}
	// Google does not reissue the refresh token, and Exchange already preserves the old one — so
	// this write never blanks a still-valid refresh token.
	converted := refreshedToken{
		accessToken:  refreshed.AccessToken,
		refreshToken: refreshed.RefreshToken,
		expiresAt:    refreshed.ExpiresAt,
	}
	if err := persistRefreshedModCredential(ctx, s.db, s.cipher, userID, "youtube", converted); err != nil {
		return err
	}

	cred.AccessToken = converted.accessToken
	cred.RefreshToken = converted.refreshToken
	cred.ExpiresAt = converted.expiresAt
	return nil
}

// ModKickSource resolves and refreshes a delegated moderator's own Kick credential.
//
// Same shape as the Twitch source, and the same table, because Kick's moderation scopes are
// role-based too: one consent serves every streamer who delegated Kick to them.
//
// What differs is what the credential is FOR. Kick's moderation endpoints take no moderator
// field, so this token is the entire proof of who is acting — there is no id in the request to
// cross-check it against, which is why the broadcaster id must come from the owner-reach anchor
// (KickSource.OwnerKickAnchor) and never from here.
type ModKickSource struct {
	db      *pgxpool.Pool
	cipher  Cipher
	refresh *kickRefresher
}

// NewModKickSource builds a source over mod_oauth_credentials. clientID/clientSecret are the
// All-Chat Kick application credentials used for the refresh grant.
func NewModKickSource(db *pgxpool.Pool, cipher Cipher, clientID, clientSecret string) *ModKickSource {
	return &ModKickSource{
		db:      db,
		cipher:  cipher,
		refresh: newKickRefresher(clientID, clientSecret),
	}
}

// Resolve returns the moderator's decrypted Kick credential, or ErrNoCredential when they have
// not consented for Kick yet (the normal state of a fresh grant).
func (s *ModKickSource) Resolve(ctx context.Context, userID string) (*ModCredential, error) {
	return resolveModCredential(ctx, s.db, s.cipher, userID, "kick")
}

// Refresh exchanges the moderator's refresh token for a new access token and writes the
// re-encrypted pair back to their own row.
//
// token-refresh-service keeps these fresh on a schedule (it reads the platform off the row, so
// Kick needed nothing there); this is the proactive/reactive path for the moment of use, so a
// token that lapsed between cycles does not cost the moderator a failed action.
func (s *ModKickSource) Refresh(ctx context.Context, userID string, cred *ModCredential) error {
	if cred.RefreshToken == "" {
		return errors.New("tokens: no moderator refresh token available")
	}

	refreshed, err := s.refresh.exchange(ctx, cred.RefreshToken)
	if err != nil {
		return err
	}
	if err := persistRefreshedModCredential(ctx, s.db, s.cipher, userID, "kick", refreshed); err != nil {
		return err
	}

	cred.AccessToken = refreshed.accessToken
	cred.RefreshToken = refreshed.refreshToken
	cred.ExpiresAt = refreshed.expiresAt
	return nil
}
