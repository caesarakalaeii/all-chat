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
	kickUserK     = "55555555-eeee-5555-5555-555555555555" // kick-login streamer
	kickStrangerE = "66666666-ffff-6666-6666-666666666666"
	kickLinkedT   = "77777777-aaaa-7777-7777-777777777777" // twitch-login streamer who linked Kick
	kickModUser   = "88888888-bbbb-8888-8888-888888888888" // volunteer moderator, owns no channel
	kickLegacyL   = "99999999-cccc-9999-9999-999999999999" // listener-only Kick row (migration 062 legacy)
	kickNoScopeN  = "aaaaaaaa-dddd-aaaa-aaaa-aaaaaaaaaaaa" // controls a channel, granted no moderation
)

func TestKickResolve_UsersRowCredential(t *testing.T) {
	src, _, cleanup := setupKickSource(t)
	defer cleanup()

	cred, err := src.Resolve(context.Background(), kickUserK, "kickstreamer")
	require.NoError(t, err)
	assert.Equal(t, "kaccK", cred.AccessToken, "access token is decrypted")
	assert.Equal(t, "krefK", cred.RefreshToken)
	assert.Equal(t, "555", cred.BroadcasterID, "broadcaster id is users.kick_id")
	assert.Contains(t, cred.GrantedScopes, "moderation:ban")
}

func TestKickResolve_CaseInsensitiveSlug(t *testing.T) {
	src, _, cleanup := setupKickSource(t)
	defer cleanup()

	cred, err := src.Resolve(context.Background(), kickUserK, "KICKSTREAMER")
	require.NoError(t, err)
	assert.Equal(t, "555", cred.BroadcasterID)
}

func TestKickResolve_UnknownChannelIsNoCredential(t *testing.T) {
	src, _, cleanup := setupKickSource(t)
	defer cleanup()

	_, err := src.Resolve(context.Background(), kickUserK, "not-my-channel")
	assert.ErrorIs(t, err, ErrNoCredential)
}

func TestKickResolve_OtherUsersCredentialIsNotVisible(t *testing.T) {
	src, _, cleanup := setupKickSource(t)
	defer cleanup()

	_, err := src.Resolve(context.Background(), kickStrangerE, "kickstreamer")
	assert.ErrorIs(t, err, ErrNoCredential)
}

func TestKickRefresh_PersistsAndUpdatesCredential(t *testing.T) {
	src, _, cleanup := setupKickSource(t)
	defer cleanup()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, r.ParseForm())
		assert.Equal(t, "refresh_token", r.Form.Get("grant_type"))
		assert.Equal(t, "krefK", r.Form.Get("refresh_token"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"newKAcc","refresh_token":"newKRef","expires_in":7200}`))
	}))
	defer srv.Close()
	src.tokenURL = srv.URL

	cred, err := src.Resolve(context.Background(), kickUserK, "kickstreamer")
	require.NoError(t, err)
	require.NoError(t, src.Refresh(context.Background(), cred))
	assert.Equal(t, "newKAcc", cred.AccessToken)
	assert.Equal(t, "newKRef", cred.RefreshToken)

	// Re-resolve: the new tokens were persisted (and re-encrypted) to the users row,
	// and granted_scopes were left untouched.
	reread, err := src.Resolve(context.Background(), kickUserK, "kickstreamer")
	require.NoError(t, err)
	assert.Equal(t, "newKAcc", reread.AccessToken)
	assert.Contains(t, reread.GrantedScopes, "moderation:ban", "a refresh must not clobber granted_scopes")
}

func TestKickResolve_LinkedCredential(t *testing.T) {
	src, _, cleanup := setupKickSource(t)
	defer cleanup()

	// A non-Kick-login streamer resolves the moderation credential from kick_oauth_tokens.
	cred, err := src.Resolve(context.Background(), kickLinkedT, "linkedkick")
	require.NoError(t, err)
	assert.Equal(t, "laccT", cred.AccessToken, "linked access token is decrypted")
	assert.Equal(t, "lrefT", cred.RefreshToken)
	assert.Equal(t, "777", cred.BroadcasterID, "broadcaster id is kick_oauth_tokens.kick_user_id")
	assert.Contains(t, cred.GrantedScopes, "moderation:ban")
}

func TestKickResolve_LinkedScopedToRequestingUser(t *testing.T) {
	src, _, cleanup := setupKickSource(t)
	defer cleanup()

	// Another user cannot resolve the linked credential — it is scoped by user_id.
	_, err := src.Resolve(context.Background(), kickStrangerE, "linkedkick")
	assert.ErrorIs(t, err, ErrNoCredential)
}

func TestKickResolve_UsersRowPreferredOverLegacyLinkedRow(t *testing.T) {
	src, _, cleanup := setupKickSource(t)
	defer cleanup()

	// kickUserK has both a users row (preferred) and a legacy kick_oauth_tokens row
	// without kick_user_id. The users-row credential must win.
	cred, err := src.Resolve(context.Background(), kickUserK, "kickstreamer")
	require.NoError(t, err)
	assert.Equal(t, "kaccK", cred.AccessToken, "users-row credential wins over the legacy linked row")
	assert.Equal(t, "555", cred.BroadcasterID)
}

func TestKickRefresh_LinkedPersistsToLinkedRow(t *testing.T) {
	src, _, cleanup := setupKickSource(t)
	defer cleanup()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"linkedNewAcc","refresh_token":"linkedNewRef","expires_in":7200}`))
	}))
	defer srv.Close()
	src.tokenURL = srv.URL

	cred, err := src.Resolve(context.Background(), kickLinkedT, "linkedkick")
	require.NoError(t, err)
	require.NoError(t, src.Refresh(context.Background(), cred))

	// Re-resolve: the new tokens were persisted back to the kick_oauth_tokens row (not
	// the users row), and granted_scopes were left untouched.
	reread, err := src.Resolve(context.Background(), kickLinkedT, "linkedkick")
	require.NoError(t, err)
	assert.Equal(t, "linkedNewAcc", reread.AccessToken, "refresh wrote back to kick_oauth_tokens")
	assert.Equal(t, "777", reread.BroadcasterID)
	assert.Contains(t, reread.GrantedScopes, "moderation:ban", "a refresh must not clobber linked granted_scopes")
}

// setupKickSource spins up a throwaway Postgres seeded with a kick-login streamer and
// returns a KickSource wired to a real AES cipher used to encrypt the seeded tokens.
func setupKickSource(t *testing.T) (*KickSource, Cipher, func()) {
	t.Helper()
	src, cipher, _, cleanup := setupKickSourceWithPool(t)
	return src, cipher, cleanup
}

// setupKickSourceWithPool is setupKickSource plus the pool, for the delegated-moderator
// credential store — which lives in its own table and needs its own seeding.
func setupKickSourceWithPool(t *testing.T) (*KickSource, Cipher, *pgxpool.Pool, func()) {
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
			kick_id VARCHAR(50),
			access_token TEXT NOT NULL,
			refresh_token TEXT NOT NULL,
			token_expires_at TIMESTAMP NOT NULL,
			granted_scopes TEXT[] NOT NULL DEFAULT '{}',
			updated_at TIMESTAMP DEFAULT NOW()
		);
		CREATE TABLE kick_oauth_tokens (
			id SERIAL PRIMARY KEY,
			user_id UUID NOT NULL,
			channel_id VARCHAR(255) NOT NULL,
			kick_user_id VARCHAR(255),
			access_token TEXT NOT NULL,
			refresh_token TEXT NOT NULL,
			token_type VARCHAR(50) DEFAULT 'Bearer',
			expiry TIMESTAMP NOT NULL,
			granted_scopes TEXT[] NOT NULL DEFAULT '{}',
			encryption_version INT NOT NULL DEFAULT 1,
			created_at TIMESTAMP DEFAULT NOW(),
			updated_at TIMESTAMP DEFAULT NOW(),
			UNIQUE(user_id, channel_id)
		);
		CREATE TABLE mod_oauth_credentials (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			user_id UUID NOT NULL,
			platform VARCHAR(20) NOT NULL,
			platform_user_id VARCHAR(100) NOT NULL,
			platform_login VARCHAR(200),
			access_token TEXT NOT NULL,
			refresh_token TEXT,
			token_expires_at TIMESTAMP,
			granted_scopes TEXT[] NOT NULL DEFAULT '{}',
			updated_at TIMESTAMP DEFAULT NOW(),
			UNIQUE (user_id, platform)
		);`
	_, err = pool.Exec(ctx, schema)
	require.NoError(t, err)

	enc := func(s string) string {
		v, encErr := cipher.EncryptString(s)
		require.NoError(t, encErr)
		return v
	}
	exp := time.Now().Add(time.Hour)

	// kickUserK: kick-login streamer that opted into moderation.
	_, err = pool.Exec(ctx, `INSERT INTO users (id, username, auth_provider, kick_id, access_token, refresh_token, token_expires_at, granted_scopes)
		VALUES ($1,'kickstreamer','kick','555',$2,$3,$4,$5)`,
		kickUserK, enc("kaccK"), enc("krefK"), exp, []string{"user:read", "moderation:ban"})
	require.NoError(t, err)

	// kickLinkedT: a twitch-login streamer who LINKED Kick (auth_provider='twitch', no
	// users.kick_id usable for moderation). Their Kick moderation credential lives only
	// in kick_oauth_tokens, keyed by the channel slug, carrying the numeric broadcaster
	// id (kick_user_id) and the opt-in granted_scopes.
	_, err = pool.Exec(ctx, `INSERT INTO users (id, username, auth_provider, access_token, refresh_token, token_expires_at, granted_scopes)
		VALUES ($1,'twitchnative','twitch',$2,$3,$4,$5)`,
		kickLinkedT, enc("tacc"), enc("tref"), exp, []string{"user:read:chat"})
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO kick_oauth_tokens (user_id, channel_id, kick_user_id, access_token, refresh_token, expiry, granted_scopes, encryption_version)
		VALUES ($1,'linkedkick','777',$2,$3,$4,$5,1)`,
		kickLinkedT, enc("laccT"), enc("lrefT"), exp, []string{"user:read", "moderation:ban"})
	require.NoError(t, err)

	// kickUserK also has a legacy listener row in kick_oauth_tokens for the same slug
	// (no kick_user_id). The users-row credential must win, and this row must never be
	// resolved for moderation (it lacks the numeric broadcaster id).
	_, err = pool.Exec(ctx, `INSERT INTO kick_oauth_tokens (user_id, channel_id, access_token, refresh_token, expiry, encryption_version)
		VALUES ($1,'kickstreamer',$2,$3,$4,1)`,
		kickUserK, enc("listenerAcc"), enc("listenerRef"), exp)
	require.NoError(t, err)

	// kickLegacyL: a streamer whose ONLY Kick row is a legacy listener row (migration 062
	// predates kick_user_id). It cannot satisfy the moderation API and must not anchor either.
	_, err = pool.Exec(ctx, `INSERT INTO users (id, username, auth_provider, access_token, refresh_token, token_expires_at)
		VALUES ($1,'legacyowner','twitch',$2,$3,$4)`, kickLegacyL, enc("x"), enc("y"), exp)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO kick_oauth_tokens (user_id, channel_id, access_token, refresh_token, expiry, encryption_version)
		VALUES ($1,'legacychannel',$2,$3,$4,1)`, kickLegacyL, enc("lacc"), enc("lref"), exp)
	require.NoError(t, err)

	// kickNoScopeN: a kick-login streamer who granted nothing beyond login. They control their
	// channel, which is all the anchor is allowed to care about.
	_, err = pool.Exec(ctx, `INSERT INTO users (id, username, auth_provider, kick_id, access_token, refresh_token, token_expires_at, granted_scopes)
		VALUES ($1,'noscopestreamer','kick','888',$2,$3,$4,$5)`,
		kickNoScopeN, enc("nacc"), enc("nref"), exp, []string{"user:read"})
	require.NoError(t, err)

	// kickModUser: a volunteer who consented to moderate on Kick, owns no channel, and also
	// holds a Twitch moderator credential — so the Kick source must not pick the wrong row.
	_, err = pool.Exec(ctx, `INSERT INTO users (id, username, auth_provider, access_token, refresh_token, token_expires_at)
		VALUES ($1,'kickvolunteer','kick',$2,$3,$4)`, kickModUser, enc("ignored"), enc("ignored"), exp)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO mod_oauth_credentials
		(user_id, platform, platform_user_id, platform_login, access_token, refresh_token, token_expires_at, granted_scopes)
		VALUES ($1,'kick','9001','kickvolunteer',$2,$3,$4,$5)`,
		kickModUser, enc("kmodAcc"), enc("kmodRef"), exp, []string{"user:read", "moderation:ban"})
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO mod_oauth_credentials
		(user_id, platform, platform_user_id, platform_login, access_token, refresh_token, token_expires_at, granted_scopes)
		VALUES ($1,'twitch','7007','kickvolunteer',$2,$3,$4,$5)`,
		kickModUser, enc("twitchModAcc"), enc("twitchModRef"), exp, []string{"moderator:manage:banned_users"})
	require.NoError(t, err)

	src := NewKickSource(pool, cipher, "test-client-id", "test-client-secret")
	return src, cipher, pool, func() {
		pool.Close()
		_ = container.Terminate(ctx)
	}
}
