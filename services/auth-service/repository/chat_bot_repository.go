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
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"golang.org/x/oauth2"
)

// chatBotQuerier is the slice of pgx the store needs, so it can run against a
// *pgxpool.Pool, a transaction, or a stub in tests.
type chatBotQuerier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// ChatBotAccount is one row of chat_bot_accounts (migration 115): the OAuth
// credential of a bot account that sends chat for a streamer. OwnerUserID is
// NULL for the global All-Chat Bot (admin-managed via /admin/chat-bots) and set
// for a streamer's custom bot connected from /commands. Tokens are stored with
// the same shared/encryption scheme every other OAuth token uses.
type ChatBotAccount struct {
	ID             string
	OwnerUserID    *string // NULL = the All-Chat Bot
	Platform       string
	PlatformUserID string
	Login          string
	DisplayName    string
	// AccessToken / RefreshToken are decrypted on read; empty when the row
	// carries none.
	AccessToken    string
	RefreshToken   string
	TokenExpiresAt *time.Time
	Scopes         []string
}

// ChatBotAccountStore persists and resolves chat bot credentials.
type ChatBotAccountStore struct {
	db     chatBotQuerier
	cipher StringCipher
}

// NewChatBotAccountStore builds the store over the shared pool. cipher nil ⇒
// tokens pass through unencrypted (matching encryptToken's nil behaviour used
// by the plain repository tests).
func NewChatBotAccountStore(db chatBotQuerier, cipher StringCipher) *ChatBotAccountStore {
	return &ChatBotAccountStore{db: db, cipher: cipher}
}

func (s *ChatBotAccountStore) encryptToken(token string) (string, error) {
	if s.cipher == nil || token == "" {
		return token, nil
	}
	return s.cipher.Encrypt(token)
}

func (s *ChatBotAccountStore) decryptToken(token string) (string, error) {
	if s.cipher == nil || token == "" {
		return token, nil
	}
	return s.cipher.Decrypt(token)
}

// StoreChatBotToken upserts a bot account's credential. ownerUserID may be ""
// (= the All-Chat Bot row). Re-consenting with a different account on the same
// platform replaces the row, exactly like StoreModCredential.
func (s *ChatBotAccountStore) StoreChatBotToken(
	ctx context.Context,
	ownerUserID, platform, platformUserID, login, displayName string,
	token *oauth2.Token,
	grantedScopes []string,
) error {
	if platform == "" {
		return fmt.Errorf("platform is required for storing a chat bot credential")
	}
	if platformUserID == "" {
		return fmt.Errorf("platform_user_id is required for storing a chat bot credential")
	}
	if login == "" {
		return fmt.Errorf("login is required for storing a chat bot credential")
	}
	if token == nil || token.AccessToken == "" {
		return fmt.Errorf("access token is required for storing a chat bot credential")
	}
	// Same 5-minute clock-skew buffer the other credential stores use: a token
	// already dead at write time is a refresh-loop birth certificate.
	if !token.Expiry.IsZero() && token.Expiry.Before(time.Now().Add(-5*time.Minute)) {
		return fmt.Errorf("refusing to store expired %s bot token (expiry: %s)",
			platform, token.Expiry.Format(time.RFC3339))
	}
	if grantedScopes == nil {
		grantedScopes = []string{}
	}

	var owner *string
	if ownerUserID != "" {
		owner = &ownerUserID
	}

	accessToken, err := s.encryptToken(token.AccessToken)
	if err != nil {
		return fmt.Errorf("failed to encrypt bot access token: %w", err)
	}
	refreshToken, err := s.encryptToken(token.RefreshToken)
	if err != nil {
		return fmt.Errorf("failed to encrypt bot refresh token: %w", err)
	}

	var expiry *time.Time
	if !token.Expiry.IsZero() {
		expiry = &token.Expiry
	}

	// The unique index is on (COALESCE(owner_user_id, sentinel), platform), so
	// the conflict target is the expression, not a plain column pair. Replacing
	// the whole row on conflict is deliberate: a re-consent that lands with a
	// different owner (or a different bot account) must not strand two rows on
	// one index.
	const query = `
		INSERT INTO chat_bot_accounts (
			owner_user_id, platform, platform_user_id, login, display_name,
			access_token_enc, refresh_token_enc, token_expires_at, scopes,
			created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, NOW(), NOW())
		ON CONFLICT (COALESCE(owner_user_id, '00000000-0000-0000-0000-000000000000'::uuid), platform) DO UPDATE SET
			owner_user_id      = EXCLUDED.owner_user_id,
			platform_user_id   = EXCLUDED.platform_user_id,
			login              = EXCLUDED.login,
			display_name       = EXCLUDED.display_name,
			access_token_enc   = EXCLUDED.access_token_enc,
			refresh_token_enc  = EXCLUDED.refresh_token_enc,
			token_expires_at   = EXCLUDED.token_expires_at,
			scopes             = EXCLUDED.scopes,
			updated_at         = NOW()`

	if _, err := s.db.Exec(ctx, query,
		owner, platform, platformUserID, login, displayName,
		accessToken, refreshToken, expiry, grantedScopes,
	); err != nil {
		return fmt.Errorf("failed to store %s chat bot credential: %w", platform, err)
	}
	return nil
}

// ErrChatBotNotFound is returned when no bot account row matches the lookup.
var ErrChatBotNotFound = errors.New("chat bot account not found")

// GetChatBotAccount resolves one bot account by owner ("" = All-Chat Bot) and
// platform, with tokens decrypted. When ownerUserID is non-empty and that
// owner has no custom bot on the platform, ErrChatBotNotFound is returned.
func (s *ChatBotAccountStore) GetChatBotAccount(ctx context.Context, ownerUserID, platform string) (*ChatBotAccount, error) {
	var owner *string
	if ownerUserID != "" {
		owner = &ownerUserID
	}

	const query = `
		SELECT id, owner_user_id, platform, platform_user_id, login, display_name,
		       access_token_enc, refresh_token_enc, token_expires_at, scopes
		FROM chat_bot_accounts
		WHERE COALESCE(owner_user_id, '00000000-0000-0000-0000-000000000000'::uuid)
		      = COALESCE($1, '00000000-0000-0000-0000-000000000000'::uuid)
		  AND platform = $2`

	row := s.db.QueryRow(ctx, query, owner, platform)
	var acct ChatBotAccount
	var encAccess, encRefresh string
	var scopes []string
	if err := row.Scan(&acct.ID, &acct.OwnerUserID, &acct.Platform, &acct.PlatformUserID,
		&acct.Login, &acct.DisplayName, &encAccess, &encRefresh,
		&acct.TokenExpiresAt, &scopes); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrChatBotNotFound
		}
		return nil, fmt.Errorf("failed to read %s chat bot account: %w", platform, err)
	}

	access, err := s.decryptToken(encAccess)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt bot access token: %w", err)
	}
	refresh, err := s.decryptToken(encRefresh)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt bot refresh token: %w", err)
	}
	acct.AccessToken = access
	acct.RefreshToken = refresh
	if scopes == nil {
		scopes = []string{}
	}
	acct.Scopes = scopes
	return &acct, nil
}

// DeleteChatBotAccount removes the bot account row for one owner ("" = the
// All-Chat Bot) and platform. A missing row deletes nothing and succeeds.
func (s *ChatBotAccountStore) DeleteChatBotAccount(ctx context.Context, ownerUserID, platform string) error {
	var owner *string
	if ownerUserID != "" {
		owner = &ownerUserID
	}
	if _, err := s.db.Exec(ctx,
		`DELETE FROM chat_bot_accounts WHERE COALESCE(owner_user_id, '00000000-0000-0000-0000-000000000000'::uuid)
		 = COALESCE($1, '00000000-0000-0000-0000-000000000000'::uuid) AND platform = $2`,
		owner, platform); err != nil {
		return fmt.Errorf("failed to delete %s chat bot account: %w", platform, err)
	}
	return nil
}

// ListChatBotAccounts returns every bot account for one owner ("" = All-Chat
// Bot), tokens decrypted, for the status endpoint. Owner-less rows only match
// the NULL owner — a custom bot is never leaked into the admin status.
func (s *ChatBotAccountStore) ListChatBotAccounts(ctx context.Context, ownerUserID string) ([]*ChatBotAccount, error) {
	var owner *string
	if ownerUserID != "" {
		owner = &ownerUserID
	}

	const query = `
		SELECT id, owner_user_id, platform, platform_user_id, login, display_name,
		       access_token_enc, refresh_token_enc, token_expires_at, scopes
		FROM chat_bot_accounts
		WHERE COALESCE(owner_user_id, '00000000-0000-0000-0000-000000000000'::uuid)
		      = COALESCE($1, '00000000-0000-0000-0000-000000000000'::uuid)`

	rows, err := s.db.Query(ctx, query, owner)
	if err != nil {
		return nil, fmt.Errorf("failed to list chat bot accounts: %w", err)
	}
	defer rows.Close()

	out := []*ChatBotAccount{}
	for rows.Next() {
		var acct ChatBotAccount
		var encAccess, encRefresh string
		var scopes []string
		if err := rows.Scan(&acct.ID, &acct.OwnerUserID, &acct.Platform, &acct.PlatformUserID,
			&acct.Login, &acct.DisplayName, &encAccess, &encRefresh,
			&acct.TokenExpiresAt, &scopes); err != nil {
			return nil, fmt.Errorf("failed to scan chat bot account: %w", err)
		}
		access, err := s.decryptToken(encAccess)
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt bot access token: %w", err)
		}
		refresh, err := s.decryptToken(encRefresh)
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt bot refresh token: %w", err)
		}
		acct.AccessToken = access
		acct.RefreshToken = refresh
		if scopes == nil {
			scopes = []string{}
		}
		acct.Scopes = scopes
		out = append(out, &acct)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate chat bot accounts: %w", err)
	}
	return out, nil
}

// GetChatBotSenderSetting reads the per-user sender choice. No row is the
// default "bot" and never an error — engagement-service writes the row only
// when the streamer explicitly picks.
func (s *ChatBotAccountStore) GetChatBotSenderSetting(ctx context.Context, userID string) (string, error) {
	var sender string
	err := s.db.QueryRow(ctx,
		`SELECT sender FROM chat_bot_settings WHERE user_id = $1`, userID).Scan(&sender)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "bot", nil
		}
		return "", fmt.Errorf("failed to read chat bot sender setting: %w", err)
	}
	return sender, nil
}

// UpsertChatBotSenderSetting writes the per-user sender choice. Used by tests
// and available for future reuse; engagement-service owns the live write path.
func (s *ChatBotAccountStore) UpsertChatBotSenderSetting(ctx context.Context, userID, sender string) error {
	if _, err := s.db.Exec(ctx, `
		INSERT INTO chat_bot_settings (user_id, sender, updated_at)
		VALUES ($1, $2, NOW())
		ON CONFLICT (user_id) DO UPDATE SET sender = EXCLUDED.sender, updated_at = NOW()`,
		userID, sender); err != nil {
		return fmt.Errorf("failed to store chat bot sender setting: %w", err)
	}
	return nil
}

// StoreRefreshedBotToken persists a rotated bot token after a proactive refresh
// (the YouTube bot send refreshes near expiry and must not race another replica
// into an invalidated token). Keyed on the (owner-coalesced, platform) pair.
// login is accepted for signature symmetry with the full store call and is not
// part of the key — the login only changes with a re-consent.
func (s *ChatBotAccountStore) StoreRefreshedBotToken(ctx context.Context, ownerUserID, platform, login, accessToken, refreshToken string, expiresAt time.Time) error {
	var owner *string
	if ownerUserID != "" {
		owner = &ownerUserID
	}
	encAccess, err := s.encryptToken(accessToken)
	if err != nil {
		return fmt.Errorf("failed to encrypt bot access token: %w", err)
	}
	encRefresh, err := s.encryptToken(refreshToken)
	if err != nil {
		return fmt.Errorf("failed to encrypt bot refresh token: %w", err)
	}
	if _, err := s.db.Exec(ctx, `
		UPDATE chat_bot_accounts
		SET access_token_enc = $1, refresh_token_enc = $2, token_expires_at = $3, updated_at = NOW()
		WHERE COALESCE(owner_user_id, '00000000-0000-0000-0000-000000000000'::uuid)
		      = COALESCE($4, '00000000-0000-0000-0000-000000000000'::uuid)
		  AND platform = $5`,
		encAccess, encRefresh, expiresAt, owner, platform); err != nil {
		return fmt.Errorf("failed to store refreshed %s bot token: %w", platform, err)
	}
	return nil
}
