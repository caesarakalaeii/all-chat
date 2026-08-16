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

// Integration tests for api_tokens (migration 086) against a real PostgreSQL with the
// full migration set applied, so the constraints under test are the production ones —
// same approach as mod_credential_repository_test.go.

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/caesar/all-chat/shared/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// setupAPITokenTestDB starts a PostgreSQL container with no pre-created schema, so the
// real migration set owns the database. Unlike setupMigrationTestDB it SKIPS rather than
// fails when no container runtime is available (the precedent is
// setupViewerRepoTestDB), because these tests assert on migration 086's constraints and
// a machine without Docker cannot say anything about them either way.
func setupAPITokenTestDB(t *testing.T) (*pgxpool.Pool, func()) {
	t.Helper()
	ctx := context.Background()

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "postgres:16-alpine",
			ExposedPorts: []string{"5432/tcp"},
			Env: map[string]string{
				"POSTGRES_USER":     "testuser",
				"POSTGRES_PASSWORD": "testpass",
				"POSTGRES_DB":       "testdb",
			},
			WaitingFor: wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60 * time.Second),
		},
		Started: true,
	})
	if err != nil {
		t.Skipf("cannot start postgres testcontainer (docker unavailable?): %v", err)
	}

	host, err := container.Host(ctx)
	if err != nil {
		t.Fatalf("container host: %v", err)
	}
	port, err := container.MappedPort(ctx, "5432")
	if err != nil {
		t.Fatalf("container port: %v", err)
	}
	pool, err := pgxpool.New(ctx, "postgres://testuser:testpass@"+host+":"+port.Port()+"/testdb?sslmode=disable")
	if err != nil {
		t.Fatalf("connection pool: %v", err)
	}
	return pool, func() {
		pool.Close()
		_ = container.Terminate(ctx)
	}
}

// apiTokenTestRepo builds an APITokenRepository over a migrated database plus one user.
func apiTokenTestRepo(t *testing.T) (*APITokenRepository, *pgxpool.Pool, string, func()) {
	t.Helper()
	pool, cleanup := setupAPITokenTestDB(t)
	runMigrations(t, pool, loadUpMigrations(t))

	var userID string
	err := pool.QueryRow(context.Background(), `
		INSERT INTO users (twitch_id, auth_provider, username, display_name,
		                   access_token, refresh_token, token_expires_at)
		VALUES ('828282', 'twitch', 'deck_user', 'Deck User',
		        'access', 'refresh', NOW() + INTERVAL '4 hours')
		RETURNING id`).Scan(&userID)
	if err != nil {
		cleanup()
		t.Fatalf("failed to insert user: %v", err)
	}
	return NewAPITokenRepository(pool), pool, userID, cleanup
}

func TestCreateAPIToken_StoresOnlyTheDigest(t *testing.T) {
	repo, pool, userID, cleanup := apiTokenTestRepo(t)
	defer cleanup()
	ctx := context.Background()

	plaintext, hash, err := middleware.GenerateAPIToken()
	if err != nil {
		t.Fatalf("GenerateAPIToken: %v", err)
	}

	token, err := repo.CreateAPIToken(ctx, userID, "Stream Deck", hash,
		[]string{middleware.ScopeEngagementWrite}, nil)
	if err != nil {
		t.Fatalf("CreateAPIToken: %v", err)
	}
	if token.ID == "" || token.Name != "Stream Deck" {
		t.Fatalf("unexpected token metadata: %+v", token)
	}
	if token.ExpiresAt != nil || token.RevokedAt != nil || token.LastUsedAt != nil {
		t.Fatalf("a fresh token should have no expiry/revocation/use: %+v", token)
	}

	// The row holds the digest, and nothing resembling the plaintext.
	var stored []byte
	if err := pool.QueryRow(ctx,
		`SELECT token_hash FROM api_tokens WHERE id = $1`, token.ID).Scan(&stored); err != nil {
		t.Fatalf("reading back token_hash: %v", err)
	}
	if string(stored) != string(hash) {
		t.Fatalf("token_hash is not the sha256 digest we passed")
	}
	var plaintextRows int
	if err := pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM api_tokens WHERE name = $1`, plaintext).Scan(&plaintextRows); err != nil {
		t.Fatalf("counting: %v", err)
	}
	if plaintextRows != 0 {
		t.Fatalf("the plaintext token appears in the table")
	}
}

// The resolver used by the middleware must agree with what the repository wrote: this
// is the seam where a mismatch in hashing or column naming would show up.
func TestCreateAPIToken_ResolvesThroughTheMiddlewareResolver(t *testing.T) {
	repo, pool, userID, cleanup := apiTokenTestRepo(t)
	defer cleanup()
	ctx := context.Background()

	plaintext, hash, err := middleware.GenerateAPIToken()
	if err != nil {
		t.Fatalf("GenerateAPIToken: %v", err)
	}
	created, err := repo.CreateAPIToken(ctx, userID, "Stream Deck", hash,
		[]string{middleware.ScopeChatWrite}, nil)
	if err != nil {
		t.Fatalf("CreateAPIToken: %v", err)
	}

	resolver := middleware.NewPgxAPITokenResolver(pool)

	identity, err := resolver.ResolveAPIToken(ctx, middleware.HashAPIToken(plaintext))
	if err != nil {
		t.Fatalf("ResolveAPIToken on a live token: %v", err)
	}
	if identity.UserID != userID || identity.Username != "deck_user" {
		t.Fatalf("resolved identity does not match the owning user: %+v", identity)
	}
	if identity.TokenID != created.ID {
		t.Fatalf("resolved token id = %q, want %q", identity.TokenID, created.ID)
	}
	if len(identity.Scopes) != 1 || identity.Scopes[0] != middleware.ScopeChatWrite {
		t.Fatalf("resolved scopes = %v", identity.Scopes)
	}
	if len(identity.Roles) != 1 || identity.Roles[0] != "user" {
		t.Fatalf("resolved roles = %v, want [user]", identity.Roles)
	}

	// last_used_at is written by the resolver as best-effort telemetry.
	var lastUsed *time.Time
	if err := pool.QueryRow(ctx,
		`SELECT last_used_at FROM api_tokens WHERE id = $1`, created.ID).Scan(&lastUsed); err != nil {
		t.Fatalf("reading last_used_at: %v", err)
	}
	if lastUsed == nil {
		t.Fatalf("last_used_at was not recorded on a successful resolve")
	}

	// A revoked token stops resolving within one request — no cache to invalidate.
	if _, err := repo.RevokeAPIToken(ctx, userID, created.ID); err != nil {
		t.Fatalf("RevokeAPIToken: %v", err)
	}
	if _, err := resolver.ResolveAPIToken(ctx, middleware.HashAPIToken(plaintext)); !errors.Is(err, middleware.ErrAPITokenNotFound) {
		t.Fatalf("expected ErrAPITokenNotFound for a revoked token, got %v", err)
	}
}

func TestResolveAPIToken_ExpiredTokenIsRejected(t *testing.T) {
	repo, pool, userID, cleanup := apiTokenTestRepo(t)
	defer cleanup()
	ctx := context.Background()

	plaintext, hash, err := middleware.GenerateAPIToken()
	if err != nil {
		t.Fatalf("GenerateAPIToken: %v", err)
	}
	past := time.Now().Add(-time.Hour)
	if _, err := repo.CreateAPIToken(ctx, userID, "expired", hash, []string{middleware.ScopeChatWrite}, &past); err != nil {
		t.Fatalf("CreateAPIToken: %v", err)
	}

	resolver := middleware.NewPgxAPITokenResolver(pool)
	if _, err := resolver.ResolveAPIToken(ctx, middleware.HashAPIToken(plaintext)); !errors.Is(err, middleware.ErrAPITokenNotFound) {
		t.Fatalf("expected ErrAPITokenNotFound for an expired token, got %v", err)
	}
}

func TestListAPITokensByUser_NeverExposesTheDigest(t *testing.T) {
	repo, _, userID, cleanup := apiTokenTestRepo(t)
	defer cleanup()
	ctx := context.Background()

	for _, name := range []string{"first", "second"} {
		_, hash, err := middleware.GenerateAPIToken()
		if err != nil {
			t.Fatalf("GenerateAPIToken: %v", err)
		}
		if _, err := repo.CreateAPIToken(ctx, userID, name, hash, []string{middleware.ScopeChatWrite}, nil); err != nil {
			t.Fatalf("CreateAPIToken(%s): %v", name, err)
		}
	}

	tokens, err := repo.ListAPITokensByUser(ctx, userID)
	if err != nil {
		t.Fatalf("ListAPITokensByUser: %v", err)
	}
	if len(tokens) != 2 {
		t.Fatalf("expected 2 tokens, got %d", len(tokens))
	}
	// Newest first.
	if tokens[0].Name != "second" {
		t.Fatalf("expected newest-first ordering, got %q first", tokens[0].Name)
	}
	// Another user sees none of them.
	other, err := repo.ListAPITokensByUser(ctx, "00000000-0000-0000-0000-000000000000")
	if err != nil {
		t.Fatalf("ListAPITokensByUser(other): %v", err)
	}
	if len(other) != 0 {
		t.Fatalf("tokens leaked across users: %+v", other)
	}
}

func TestRevokeAPIToken(t *testing.T) {
	repo, _, userID, cleanup := apiTokenTestRepo(t)
	defer cleanup()
	ctx := context.Background()

	_, hash, err := middleware.GenerateAPIToken()
	if err != nil {
		t.Fatalf("GenerateAPIToken: %v", err)
	}
	created, err := repo.CreateAPIToken(ctx, userID, "deck", hash, []string{middleware.ScopeChatWrite}, nil)
	if err != nil {
		t.Fatalf("CreateAPIToken: %v", err)
	}

	revoked, err := repo.RevokeAPIToken(ctx, userID, created.ID)
	if err != nil {
		t.Fatalf("RevokeAPIToken: %v", err)
	}
	if revoked.RevokedAt == nil {
		t.Fatalf("revoked_at was not set")
	}

	// Revoking twice is a no-op that keeps the original timestamp: revocation history
	// must not be rewritten by a retry.
	again, err := repo.RevokeAPIToken(ctx, userID, created.ID)
	if err != nil {
		t.Fatalf("second RevokeAPIToken: %v", err)
	}
	if !again.RevokedAt.Equal(*revoked.RevokedAt) {
		t.Fatalf("revoked_at was rewritten: %v -> %v", revoked.RevokedAt, again.RevokedAt)
	}

	// Someone else's token is indistinguishable from a nonexistent one.
	if _, err := repo.RevokeAPIToken(ctx, "00000000-0000-0000-0000-000000000000", created.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound revoking another user's token, got %v", err)
	}
}

func TestCreateAPIToken_EnforcesTheLiveCap(t *testing.T) {
	repo, _, userID, cleanup := apiTokenTestRepo(t)
	defer cleanup()
	ctx := context.Background()

	var lastID string
	for i := 0; i < MaxAPITokensPerUser; i++ {
		_, hash, err := middleware.GenerateAPIToken()
		if err != nil {
			t.Fatalf("GenerateAPIToken: %v", err)
		}
		token, err := repo.CreateAPIToken(ctx, userID, "deck", hash, []string{middleware.ScopeChatWrite}, nil)
		if err != nil {
			t.Fatalf("CreateAPIToken #%d: %v", i, err)
		}
		lastID = token.ID
	}

	_, hash, err := middleware.GenerateAPIToken()
	if err != nil {
		t.Fatalf("GenerateAPIToken: %v", err)
	}
	if _, err := repo.CreateAPIToken(ctx, userID, "one too many", hash, []string{middleware.ScopeChatWrite}, nil); !errors.Is(err, ErrAPITokenLimitReached) {
		t.Fatalf("expected ErrAPITokenLimitReached, got %v", err)
	}

	// The cap counts LIVE tokens only: revoking one frees a slot.
	if _, err := repo.RevokeAPIToken(ctx, userID, lastID); err != nil {
		t.Fatalf("RevokeAPIToken: %v", err)
	}
	if _, err := repo.CreateAPIToken(ctx, userID, "after revoke", hash, []string{middleware.ScopeChatWrite}, nil); err != nil {
		t.Fatalf("CreateAPIToken after freeing a slot: %v", err)
	}
}

// Deleting the user must take their tokens with them (ON DELETE CASCADE, migration 086).
func TestAPITokens_CascadeOnUserDelete(t *testing.T) {
	repo, pool, userID, cleanup := apiTokenTestRepo(t)
	defer cleanup()
	ctx := context.Background()

	_, hash, err := middleware.GenerateAPIToken()
	if err != nil {
		t.Fatalf("GenerateAPIToken: %v", err)
	}
	if _, err := repo.CreateAPIToken(ctx, userID, "deck", hash, []string{middleware.ScopeChatWrite}, nil); err != nil {
		t.Fatalf("CreateAPIToken: %v", err)
	}
	if _, err := pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, userID); err != nil {
		t.Fatalf("deleting the user: %v", err)
	}

	var remaining int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM api_tokens WHERE user_id = $1`, userID).Scan(&remaining); err != nil {
		t.Fatalf("counting: %v", err)
	}
	if remaining != 0 {
		t.Fatalf("%d token rows survived the user deletion", remaining)
	}
}
