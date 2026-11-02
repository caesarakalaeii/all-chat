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
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// ErrLoginPlatform is returned when asked to disconnect the platform the account
// signs in with. Its tokens back the account itself and are deleted with it.
var ErrLoginPlatform = errors.New("the platform you sign in with cannot be disconnected")

// ErrUnsupportedPlatform is returned for a platform with no stored OAuth tokens.
var ErrUnsupportedPlatform = errors.New("unsupported platform")

// linkedTokenTable describes where a linked platform keeps its per-link OAuth
// tokens. Every identifier here is a fixed literal (never user input), so they are
// safe to interpolate into SQL.
type linkedTokenTable struct {
	table   string
	account string // the column naming the linked account, for display
	refresh string // the refresh-token column, or '' where the platform has none
	// modCredentials: the platform also has delegated-moderator credentials
	// (mod_oauth_credentials, ADR-0048), which are the same person's tokens.
	modCredentials bool
	// sourcesNeedToken: a source of this platform cannot run without the owner's
	// token (overlay-manager refuses to add one without it and the listener polls
	// with it), so disconnecting removes the user's sources of this platform.
	// Twitch, YouTube and Kick sources can run without it and stay.
	sourcesNeedToken bool
}

var linkedTokenTables = map[string]linkedTokenTable{
	"twitch":    {"twitch_oauth_tokens", "twitch_login", "refresh_token", true, false},
	"youtube":   {"youtube_oauth_tokens", "channel_id", "refresh_token", true, false},
	"kick":      {"kick_oauth_tokens", "channel_id", "refresh_token", true, false},
	"facebook":  {"facebook_oauth_tokens", "COALESCE(page_name, page_id)", "''", false, true},
	"instagram": {"instagram_oauth_tokens", "COALESCE(ig_username, ig_user_id)", "''", false, true},
}

// linkedPlatformOrder is the order connections are listed in.
var linkedPlatformOrder = []string{"twitch", "youtube", "kick", "facebook", "instagram"}

// PlatformConnection is one platform the account holds OAuth tokens for.
type PlatformConnection struct {
	Platform string   `json:"platform"`
	Accounts []string `json:"accounts"`
	// IsLogin marks the platform the account signs in with (users.auth_provider).
	IsLogin bool `json:"is_login"`
}

// RemovedToken is one deleted credential, decrypted, so the caller can revoke it at
// the provider. RefreshToken is empty where the platform issues none.
type RemovedToken struct {
	AccessToken  string
	RefreshToken string
}

// ListPlatformConnections returns every platform the user holds OAuth tokens for:
// the login provider (tokens on the users row) and each linked platform with a row
// in its token table or in mod_oauth_credentials.
func (r *UserRepository) ListPlatformConnections(ctx context.Context, userID string) ([]PlatformConnection, error) {
	var authProvider, username string
	if err := r.db.QueryRow(ctx, `SELECT auth_provider, username FROM users WHERE id = $1`, userID).
		Scan(&authProvider, &username); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to read user: %w", err)
	}

	out := []PlatformConnection{}
	for _, platform := range linkedPlatformOrder {
		t := linkedTokenTables[platform]
		query := fmt.Sprintf(`SELECT DISTINCT %s FROM %s WHERE user_id = $1`, t.account, t.table)
		args := []any{userID}
		if t.modCredentials {
			query += ` UNION SELECT COALESCE(platform_login, platform_user_id) FROM mod_oauth_credentials WHERE user_id = $1 AND platform = $2`
			args = append(args, platform)
		}
		rows, err := r.db.Query(ctx, query+` ORDER BY 1`, args...)
		if err != nil {
			return nil, fmt.Errorf("failed to list %s connections: %w", platform, err)
		}
		accounts, err := pgx.CollectRows(rows, pgx.RowTo[string])
		if err != nil {
			return nil, fmt.Errorf("failed to list %s connections: %w", platform, err)
		}
		isLogin := platform == authProvider
		if isLogin && len(accounts) == 0 {
			accounts = []string{username}
		}
		if len(accounts) > 0 {
			out = append(out, PlatformConnection{Platform: platform, Accounts: accounts, IsLogin: isLogin})
		}
	}
	return out, nil
}

// DisconnectPlatform deletes, in one transaction, every OAuth token the user holds
// for a linked platform: its per-link token table and, where the platform has them,
// the user's delegated-moderator credentials. It returns the deleted tokens
// decrypted so the caller can revoke them at the provider.
//
// The login provider is refused with ErrLoginPlatform: its tokens on the users row
// are how the account exists, and its per-link rows are rewritten on every sign-in.
// They are deleted with the account (DELETE /me).
func (r *UserRepository) DisconnectPlatform(ctx context.Context, userID, platform string) ([]RemovedToken, error) {
	t, ok := linkedTokenTables[platform]
	if !ok {
		return nil, ErrUnsupportedPlatform
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin disconnect: %w", err)
	}
	defer tx.Rollback(ctx)

	var authProvider string
	if err := tx.QueryRow(ctx, `SELECT auth_provider FROM users WHERE id = $1 FOR UPDATE`, userID).Scan(&authProvider); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to read user: %w", err)
	}
	if authProvider == platform {
		return nil, ErrLoginPlatform
	}

	removed, err := r.deleteReturningTokens(ctx, tx,
		fmt.Sprintf(`DELETE FROM %s WHERE user_id = $1 RETURNING access_token, %s, encryption_version`, t.table, t.refresh),
		userID)
	if err != nil {
		return nil, fmt.Errorf("failed to delete %s tokens: %w", platform, err)
	}
	if t.modCredentials {
		mod, err := r.deleteReturningTokens(ctx, tx,
			`DELETE FROM mod_oauth_credentials WHERE user_id = $1 AND platform = $2
			 RETURNING access_token, COALESCE(refresh_token, ''), encryption_version`,
			userID, platform)
		if err != nil {
			return nil, fmt.Errorf("failed to delete %s moderator credentials: %w", platform, err)
		}
		removed = append(removed, mod...)
	}
	if t.sourcesNeedToken {
		if _, err := tx.Exec(ctx, `
			DELETE FROM overlay_chat_sources
			WHERE platform = $2 AND overlay_id IN (SELECT id FROM overlays WHERE user_id = $1)`,
			userID, platform); err != nil {
			return nil, fmt.Errorf("failed to remove %s sources: %w", platform, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("failed to commit disconnect: %w", err)
	}
	return removed, nil
}

// deleteReturningTokens runs a DELETE ... RETURNING access, refresh, encryption_version
// and decrypts what it returns. A token that fails to decrypt is still deleted; it is
// only left out of the result, because it cannot be revoked anyway.
func (r *UserRepository) deleteReturningTokens(ctx context.Context, tx pgx.Tx, query string, args ...any) ([]RemovedToken, error) {
	rows, err := tx.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []RemovedToken
	for rows.Next() {
		var access, refresh string
		var version int
		if err := rows.Scan(&access, &refresh, &version); err != nil {
			return nil, err
		}
		if version >= 1 {
			if access, err = r.decryptToken(access); err != nil {
				continue
			}
			if refresh, err = r.decryptToken(refresh); err != nil {
				refresh = ""
			}
		}
		out = append(out, RemovedToken{AccessToken: access, RefreshToken: refresh})
	}
	return out, rows.Err()
}
