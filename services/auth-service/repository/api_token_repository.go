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

// Persistence for personal access tokens (api_tokens, migration 086).
//
// Two rules govern every query in this file:
//
//  1. The plaintext token never appears here. Create takes a digest the caller
//     computed with middleware.HashAPIToken; nothing in this file can return, log or
//     store a token.
//  2. No projection selects token_hash. A digest that never leaves this file cannot be
//     serialised into a response by accident — the same reasoning as
//     moderation-service's grantColumns comment for invite_token_hash.

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// MaxAPITokensPerUser caps how many live tokens one account may hold. A cap exists
// because tokens are long-lived by default: without one, a scripted client that mints
// a token per launch would accumulate credentials nobody ever revokes.
const MaxAPITokensPerUser = 20

// ErrAPITokenLimitReached is returned by CreateAPIToken when the user already holds
// MaxAPITokensPerUser live tokens.
var ErrAPITokenLimitReached = errors.New("personal access token limit reached")

// APIToken is the METADATA of a token — deliberately without the digest, and
// obviously without the plaintext. This is exactly what the list endpoint may expose.
type APIToken struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	Scopes     []string   `json:"scopes"`
	CreatedAt  time.Time  `json:"created_at"`
	LastUsedAt *time.Time `json:"last_used_at"`
	ExpiresAt  *time.Time `json:"expires_at"`
	RevokedAt  *time.Time `json:"revoked_at"`
}

// APITokenRepository owns the api_tokens table.
type APITokenRepository struct {
	db *pgxpool.Pool
}

// NewAPITokenRepository creates an APITokenRepository.
func NewAPITokenRepository(db *pgxpool.Pool) *APITokenRepository {
	return &APITokenRepository{db: db}
}

// apiTokenColumns is the shared projection for every read. token_hash is absent on
// purpose (see the file comment).
const apiTokenColumns = `
	id::text, name, scopes, created_at, last_used_at, expires_at, revoked_at`

// CreateAPIToken persists a new token for userID and returns its metadata.
//
// tokenHash must be the SHA-256 digest of the plaintext (middleware.HashAPIToken).
// The plaintext is the caller's to show once and then forget; this function has no
// way to see it, which is the point.
//
// The live-token cap is enforced inside a transaction that locks the owning user row,
// so two concurrent creates cannot both observe "19 tokens" and both insert.
func (r *APITokenRepository) CreateAPIToken(
	ctx context.Context,
	userID, name string,
	tokenHash []byte,
	scopes []string,
	expiresAt *time.Time,
) (*APIToken, error) {
	if len(tokenHash) == 0 {
		return nil, errors.New("CreateAPIToken: empty token hash")
	}
	if scopes == nil {
		scopes = []string{}
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("CreateAPIToken begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var lockedUser string
	err = tx.QueryRow(ctx, `SELECT id::text FROM users WHERE id = $1 FOR UPDATE`, userID).Scan(&lockedUser)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("CreateAPIToken lock user: %w", err)
	}

	var live int
	if err := tx.QueryRow(ctx, `
		SELECT COUNT(*) FROM api_tokens
		WHERE user_id = $1 AND revoked_at IS NULL`, userID).Scan(&live); err != nil {
		return nil, fmt.Errorf("CreateAPIToken count: %w", err)
	}
	if live >= MaxAPITokensPerUser {
		return nil, ErrAPITokenLimitReached
	}

	var token APIToken
	err = tx.QueryRow(ctx, `
		INSERT INTO api_tokens (user_id, name, token_hash, scopes, expires_at)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING `+apiTokenColumns,
		userID, name, tokenHash, scopes, expiresAt,
	).Scan(&token.ID, &token.Name, &token.Scopes, &token.CreatedAt,
		&token.LastUsedAt, &token.ExpiresAt, &token.RevokedAt)
	if err != nil {
		return nil, fmt.Errorf("CreateAPIToken insert: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("CreateAPIToken commit: %w", err)
	}
	return &token, nil
}

// ListAPITokensByUser returns the user's tokens, newest first.
//
// Revoked rows are included so the management UI can show what was revoked and when;
// they are self-evidently unusable because revoked_at is set. Nothing in the returned
// struct can identify the secret.
func (r *APITokenRepository) ListAPITokensByUser(ctx context.Context, userID string) ([]APIToken, error) {
	rows, err := r.db.Query(ctx, `
		SELECT `+apiTokenColumns+`
		FROM api_tokens
		WHERE user_id = $1
		ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, fmt.Errorf("ListAPITokensByUser query: %w", err)
	}
	defer rows.Close()

	tokens := []APIToken{}
	for rows.Next() {
		var t APIToken
		if err := rows.Scan(&t.ID, &t.Name, &t.Scopes, &t.CreatedAt,
			&t.LastUsedAt, &t.ExpiresAt, &t.RevokedAt); err != nil {
			return nil, fmt.Errorf("ListAPITokensByUser scan: %w", err)
		}
		tokens = append(tokens, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ListAPITokensByUser rows: %w", err)
	}
	return tokens, nil
}

// RevokeAPIToken marks one of the user's tokens revoked and returns its metadata.
//
// The user_id predicate is the authorization check: a token id belonging to someone
// else is indistinguishable from one that does not exist (ErrNotFound), so this
// endpoint cannot be used to probe for other users' token ids.
//
// revoked_at is set only when it is still NULL, making a second revoke a no-op rather
// than a rewrite of history — but the row is still returned, so the endpoint is
// idempotent from the client's perspective.
func (r *APITokenRepository) RevokeAPIToken(ctx context.Context, userID, tokenID string) (*APIToken, error) {
	var token APIToken
	err := r.db.QueryRow(ctx, `
		UPDATE api_tokens
		   SET revoked_at = COALESCE(revoked_at, NOW())
		 WHERE id = $1 AND user_id = $2
		RETURNING `+apiTokenColumns,
		tokenID, userID,
	).Scan(&token.ID, &token.Name, &token.Scopes, &token.CreatedAt,
		&token.LastUsedAt, &token.ExpiresAt, &token.RevokedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("RevokeAPIToken: %w", err)
	}
	return &token, nil
}
