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
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/caesar/all-chat/shared/encryption"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

const (
	userA     = "11111111-aaaa-1111-1111-111111111111" // twitch-login streamer
	userB     = "22222222-bbbb-2222-2222-222222222222" // youtube-login, linked twitch
	userC     = "33333333-cccc-3333-3333-333333333333" // has BOTH a users row and a linked row
	strangerD = "44444444-dddd-4444-4444-444444444444"
)

func TestResolve_UsersRowCredential(t *testing.T) {
	src, cipher, cleanup := setupTokenSource(t)
	defer cleanup()

	cred, err := src.Resolve(context.Background(), userA, "streamerA")
	require.NoError(t, err)
	assert.Equal(t, "accA", cred.AccessToken, "access token is decrypted")
	assert.Equal(t, "refA", cred.RefreshToken)
	assert.Equal(t, "1001", cred.BroadcasterID, "broadcaster id is users.twitch_id")
	assert.Contains(t, cred.GrantedScopes, "moderator:manage:chat_messages")
	_ = cipher
}

func TestResolve_CaseInsensitiveLogin(t *testing.T) {
	src, _, cleanup := setupTokenSource(t)
	defer cleanup()

	cred, err := src.Resolve(context.Background(), userA, "STREAMERA")
	require.NoError(t, err)
	assert.Equal(t, "1001", cred.BroadcasterID)
}

func TestResolve_LinkedTokenCredential(t *testing.T) {
	src, _, cleanup := setupTokenSource(t)
	defer cleanup()

	cred, err := src.Resolve(context.Background(), userB, "streamerB")
	require.NoError(t, err)
	assert.Equal(t, "accB", cred.AccessToken)
	assert.Equal(t, "2002", cred.BroadcasterID, "broadcaster id is twitch_oauth_tokens.twitch_user_id")
	assert.Contains(t, cred.GrantedScopes, "moderator:manage:banned_users")
}

func TestResolve_PrefersUsersRowOverLinked(t *testing.T) {
	src, _, cleanup := setupTokenSource(t)
	defer cleanup()

	// userC has a users row (twitch_id 3003) AND a linked row (twitch_user_id 9999)
	// for the same login; the users row must win.
	cred, err := src.Resolve(context.Background(), userC, "dualstreamer")
	require.NoError(t, err)
	assert.Equal(t, "3003", cred.BroadcasterID)
	assert.Equal(t, "accC-users", cred.AccessToken)
}

func TestResolve_UnknownChannelIsNoCredential(t *testing.T) {
	src, _, cleanup := setupTokenSource(t)
	defer cleanup()

	_, err := src.Resolve(context.Background(), userA, "not-my-channel")
	assert.ErrorIs(t, err, ErrNoCredential)
}

func TestResolve_OtherUsersCredentialIsNotVisible(t *testing.T) {
	src, _, cleanup := setupTokenSource(t)
	defer cleanup()

	// A stranger cannot resolve userA's channel: the credential is scoped to its owner.
	_, err := src.Resolve(context.Background(), strangerD, "streamerA")
	assert.ErrorIs(t, err, ErrNoCredential)
}

func TestRefresh_PersistsAndUpdatesCredential(t *testing.T) {
	src, cipher, cleanup := setupTokenSource(t)
	defer cleanup()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, r.ParseForm())
		assert.Equal(t, "refresh_token", r.Form.Get("grant_type"))
		assert.Equal(t, "refA", r.Form.Get("refresh_token"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"newAccA","refresh_token":"newRefA","expires_in":3600}`))
	}))
	defer srv.Close()
	src.tokenURL = srv.URL

	cred, err := src.Resolve(context.Background(), userA, "streamerA")
	require.NoError(t, err)

	require.NoError(t, src.Refresh(context.Background(), cred))
	assert.Equal(t, "newAccA", cred.AccessToken, "in-memory credential updated")
	assert.Equal(t, "newRefA", cred.RefreshToken)
	assert.WithinDuration(t, time.Now().Add(time.Hour), cred.ExpiresAt, time.Minute)

	// Re-resolve: the new tokens were persisted (and re-encrypted) to the users row.
	reread, err := src.Resolve(context.Background(), userA, "streamerA")
	require.NoError(t, err)
	assert.Equal(t, "newAccA", reread.AccessToken)
	assert.Equal(t, "newRefA", reread.RefreshToken)
	_ = cipher
}

func TestRefresh_KeepsOldRefreshTokenWhenResponseOmitsIt(t *testing.T) {
	src, _, cleanup := setupTokenSource(t)
	defer cleanup()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"rotatedAcc","expires_in":3600}`)) // no refresh_token
	}))
	defer srv.Close()
	src.tokenURL = srv.URL

	cred, err := src.Resolve(context.Background(), userB, "streamerB")
	require.NoError(t, err)
	oldRefresh := cred.RefreshToken

	require.NoError(t, src.Refresh(context.Background(), cred))
	assert.Equal(t, "rotatedAcc", cred.AccessToken)
	assert.Equal(t, oldRefresh, cred.RefreshToken, "an omitted refresh_token keeps the existing one")
}

// setupTokenSource spins up a throwaway Postgres seeded with three credential
// shapes (users row, linked row, both) and returns a TwitchSource wired to a real
// AES cipher used to encrypt the seeded tokens.
func setupTokenSource(t *testing.T) (*TwitchSource, Cipher, func()) {
	t.Helper()
	ctx := context.Background()

	aes, err := encryption.NewAESEncryptor([]byte("0123456789abcdef0123456789abcdef"))
	require.NoError(t, err)
	cipher, err := encryption.NewMultiKeyEncryptor([]encryption.KeyEntry{{Kid: 0x01, Cipher: aes}}, nil)
	require.NoError(t, err)

	req := testcontainers.ContainerRequest{
		Image:        "postgres:16-alpine",
		ExposedPorts: []string{"5432/tcp"},
		Env: map[string]string{
			"POSTGRES_USER":     "testuser",
			"POSTGRES_PASSWORD": "testpass",
			"POSTGRES_DB":       "testdb",
		},
		WaitingFor: wait.ForLog("database system is ready to accept connections").
			WithOccurrence(2).WithStartupTimeout(60 * time.Second),
	}
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req, Started: true,
	})
	require.NoError(t, err, "start postgres container")

	host, err := container.Host(ctx)
	require.NoError(t, err)
	port, err := container.MappedPort(ctx, "5432")
	require.NoError(t, err)

	pool, err := pgxpool.New(ctx, "postgres://testuser:testpass@"+host+":"+port.Port()+"/testdb?sslmode=disable")
	require.NoError(t, err)

	const schema = `
		CREATE TABLE users (
			id UUID PRIMARY KEY,
			username VARCHAR(100) NOT NULL,
			auth_provider VARCHAR(20),
			twitch_id VARCHAR(50),
			access_token TEXT NOT NULL,
			refresh_token TEXT NOT NULL,
			token_expires_at TIMESTAMP NOT NULL,
			granted_scopes TEXT[] NOT NULL DEFAULT '{}',
			updated_at TIMESTAMP DEFAULT NOW()
		);
		CREATE TABLE twitch_oauth_tokens (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			user_id UUID NOT NULL,
			twitch_user_id VARCHAR(50) NOT NULL,
			twitch_login VARCHAR(100) NOT NULL,
			access_token TEXT NOT NULL,
			refresh_token TEXT NOT NULL,
			token_expires_at TIMESTAMP NOT NULL,
			granted_scopes TEXT[] NOT NULL DEFAULT '{}',
			updated_at TIMESTAMP DEFAULT NOW()
		);`
	_, err = pool.Exec(ctx, schema)
	require.NoError(t, err)

	enc := func(s string) string {
		v, encErr := cipher.EncryptString(s)
		require.NoError(t, encErr)
		return v
	}
	exp := time.Now().Add(time.Hour)

	// userA: twitch-login streamer.
	_, err = pool.Exec(ctx, `INSERT INTO users (id, username, auth_provider, twitch_id, access_token, refresh_token, token_expires_at, granted_scopes)
		VALUES ($1,'streamerA','twitch','1001',$2,$3,$4,$5)`,
		userA, enc("accA"), enc("refA"), exp, []string{"user:read:chat", "moderator:manage:chat_messages"})
	require.NoError(t, err)

	// userB: youtube-login account that linked Twitch 'streamerB'.
	_, err = pool.Exec(ctx, `INSERT INTO users (id, username, auth_provider, access_token, refresh_token, token_expires_at)
		VALUES ($1,'ytuserB','youtube',$2,$3,$4)`, userB, enc("ignored"), enc("ignored"), exp)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO twitch_oauth_tokens (user_id, twitch_user_id, twitch_login, access_token, refresh_token, token_expires_at, granted_scopes)
		VALUES ($1,'2002','streamerB',$2,$3,$4,$5)`,
		userB, enc("accB"), enc("refB"), exp, []string{"moderator:manage:banned_users"})
	require.NoError(t, err)

	// userC: twitch-login 'dualstreamer' AND a stale linked row for the same login.
	_, err = pool.Exec(ctx, `INSERT INTO users (id, username, auth_provider, twitch_id, access_token, refresh_token, token_expires_at, granted_scopes)
		VALUES ($1,'dualstreamer','twitch','3003',$2,$3,$4,$5)`,
		userC, enc("accC-users"), enc("refC"), exp, []string{"moderator:manage:chat_messages"})
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO twitch_oauth_tokens (user_id, twitch_user_id, twitch_login, access_token, refresh_token, token_expires_at, granted_scopes)
		VALUES ($1,'9999','dualstreamer',$2,$3,$4,$5)`,
		userC, enc("accC-linked"), enc("refC2"), exp, []string{"moderator:manage:banned_users"})
	require.NoError(t, err)

	src := NewTwitchSource(pool, cipher, "test-client-id", "test-client-secret")
	return src, cipher, func() {
		pool.Close()
		_ = container.Terminate(ctx)
	}
}
