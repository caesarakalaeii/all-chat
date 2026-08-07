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

package repository

import (
	"context"
	"fmt"
	"time"

	"golang.org/x/oauth2"
)

// StoreModCredential persists a delegated moderator's OWN platform credential (ADR-0048).
//
// This is deliberately a separate table from every other credential store, keyed on the
// moderator rather than on a channel. Two listeners select a channel's credential by channel
// with NO user scoping — twitch-eventsub-listener by `LOWER(twitch_login) = LOWER(channel_id)`
// and kick-listener by `kick_oauth_tokens WHERE channel_id = $1` — so a moderator-scoped row in
// either table could become a candidate INGEST credential and silently break chat on a real
// channel. Writing here also means a moderator's consent can never touch their own login
// credential or its granted_scopes, which removes any interaction with the scope-downgrade
// guard.
//
// One row per (moderator, platform), so there is exactly one refresh owner and nothing races
// token-refresh-service. Re-consenting with a different account on the same platform replaces
// the row; the capabilities payload echoes which account is acting so it is never a mystery.
func (r *UserRepository) StoreModCredential(
	ctx context.Context,
	userID, platform, platformUserID, platformLogin string,
	token *oauth2.Token,
	grantedScopes []string,
) error {
	if userID == "" {
		return fmt.Errorf("user_id is required for storing a moderator credential")
	}
	if platform == "" {
		return fmt.Errorf("platform is required for storing a moderator credential")
	}
	// The platform id is what we send as moderator_id and what the platform re-checks the role
	// against. A credential we cannot attribute to a platform identity is unusable.
	if platformUserID == "" {
		return fmt.Errorf("platform_user_id is required for storing a moderator credential")
	}
	if token == nil || token.AccessToken == "" {
		return fmt.Errorf("access token is required for storing a moderator credential")
	}
	if !token.Expiry.IsZero() && token.Expiry.Before(time.Now().Add(-5*time.Minute)) {
		return fmt.Errorf("refusing to store expired %s moderator token (expiry: %s)",
			platform, token.Expiry.Format(time.RFC3339))
	}

	accessToken, err := r.encryptToken(token.AccessToken)
	if err != nil {
		return fmt.Errorf("failed to encrypt moderator access token: %w", err)
	}
	refreshToken, err := r.encryptToken(token.RefreshToken)
	if err != nil {
		return fmt.Errorf("failed to encrypt moderator refresh token: %w", err)
	}

	if grantedScopes == nil {
		grantedScopes = []string{}
	}

	var expiry *time.Time
	if !token.Expiry.IsZero() {
		expiry = &token.Expiry
	}

	const query = `
		INSERT INTO mod_oauth_credentials (
			user_id, platform, platform_user_id, platform_login,
			access_token, refresh_token, token_type, token_expires_at,
			granted_scopes, encryption_version, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, COALESCE(NULLIF($7, ''), 'bearer'), $8, $9, 1, NOW(), NOW())
		ON CONFLICT (user_id, platform)
		DO UPDATE SET
			platform_user_id   = EXCLUDED.platform_user_id,
			platform_login     = EXCLUDED.platform_login,
			access_token       = EXCLUDED.access_token,
			refresh_token      = EXCLUDED.refresh_token,
			token_type         = EXCLUDED.token_type,
			token_expires_at   = EXCLUDED.token_expires_at,
			granted_scopes     = EXCLUDED.granted_scopes,
			encryption_version = EXCLUDED.encryption_version,
			updated_at         = NOW()`

	if _, err := r.db.Exec(ctx, query,
		userID, platform, platformUserID, platformLogin,
		accessToken, refreshToken, token.TokenType, expiry, grantedScopes,
	); err != nil {
		return fmt.Errorf("failed to store %s moderator credential: %w", platform, err)
	}

	return nil
}

// DeleteModCredential removes a moderator's credential for one platform, so revoking their last
// grant (or a user disconnecting) does not leave a usable token behind.
func (r *UserRepository) DeleteModCredential(ctx context.Context, userID, platform string) error {
	const query = `DELETE FROM mod_oauth_credentials WHERE user_id = $1 AND platform = $2`
	if _, err := r.db.Exec(ctx, query, userID, platform); err != nil {
		return fmt.Errorf("failed to delete %s moderator credential: %w", platform, err)
	}
	return nil
}
