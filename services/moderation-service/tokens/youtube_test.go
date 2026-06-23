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
	ytUserY     = "77777777-1111-7777-7777-777777777777" // youtube-login streamer
	ytStrangerF = "88888888-2222-8888-8888-888888888888"
	ytLinkedT   = "99999999-3333-9999-9999-999999999999" // twitch-login streamer who linked YouTube
)

const ytForceSSL = "https://www.googleapis.com/auth/youtube.force-ssl"

func TestYouTubeResolve_UsersRowCredential(t *testing.T) {
	src, _, cleanup := setupYouTubeSource(t)
	defer cleanup()

	cred, err := src.Resolve(context.Background(), ytUserY, "UCanything")
	require.NoError(t, err)
	assert.Equal(t, "yaccY", cred.AccessToken, "access token is decrypted")
	assert.Equal(t, "yrefY", cred.RefreshToken)
	assert.Contains(t, cred.GrantedScopes, ytForceSSL)
}

func TestYouTubeResolve_OtherUserIsNoCredential(t *testing.T) {
	src, _, cleanup := setupYouTubeSource(t)
	defer cleanup()

	_, err := src.Resolve(context.Background(), ytStrangerF, "UCanything")
	assert.ErrorIs(t, err, ErrNoCredential)
}

func TestYouTubeRefresh_PersistsAndKeepsScopes(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, r.ParseForm())
		assert.Equal(t, "refresh_token", r.Form.Get("grant_type"))
		w.Header().Set("Content-Type", "application/json")
		// Google does not reissue a refresh token on refresh.
		_, _ = w.Write([]byte(`{"access_token":"newYAcc","expires_in":3599}`))
	}))
	defer srv.Close()
	src, _, cleanup := setupYouTubeSource(t, WithTokenURL(srv.URL))
	defer cleanup()

	cred, err := src.Resolve(context.Background(), ytUserY, "UCanything")
	require.NoError(t, err)
	oldRefresh := cred.RefreshToken

	require.NoError(t, src.Refresh(context.Background(), cred))
	assert.Equal(t, "newYAcc", cred.AccessToken)
	assert.Equal(t, oldRefresh, cred.RefreshToken, "an omitted refresh_token keeps the existing one")

	reread, err := src.Resolve(context.Background(), ytUserY, "UCanything")
	require.NoError(t, err)
	assert.Equal(t, "newYAcc", reread.AccessToken)
	assert.Contains(t, reread.GrantedScopes, ytForceSSL, "a refresh must not clobber granted_scopes")
}

func TestYouTubeResolve_PerChannelPreferredOverUsersRow(t *testing.T) {
	src, _, cleanup := setupYouTubeSource(t)
	defer cleanup()

	// The youtube-login user resolving their exact channel gets the per-channel token,
	// not the channel-agnostic users-row credential.
	cred, err := src.Resolve(context.Background(), ytUserY, "UCself")
	require.NoError(t, err)
	assert.Equal(t, "yaccSelf", cred.AccessToken, "exact per-channel token wins over the users row")
	assert.Contains(t, cred.GrantedScopes, ytForceSSL)
}

func TestYouTubeResolve_LinkedChannelCredential(t *testing.T) {
	src, _, cleanup := setupYouTubeSource(t)
	defer cleanup()

	// A non-YouTube-login streamer resolves the moderation credential from
	// youtube_oauth_tokens by exact channel id.
	cred, err := src.Resolve(context.Background(), ytLinkedT, "UClinked")
	require.NoError(t, err)
	assert.Equal(t, "laccLinked", cred.AccessToken, "linked access token is decrypted")
	assert.Equal(t, "lrefLinked", cred.RefreshToken)
	assert.Contains(t, cred.GrantedScopes, ytForceSSL)
}

func TestYouTubeResolve_LinkedWrongChannelIsNoCredential(t *testing.T) {
	src, _, cleanup := setupYouTubeSource(t)
	defer cleanup()

	// The linked credential is keyed by channel id; a different channel finds nothing
	// (the user has no users-row youtube credential to fall back to).
	_, err := src.Resolve(context.Background(), ytLinkedT, "UCnotmine")
	assert.ErrorIs(t, err, ErrNoCredential)
}

func TestYouTubeRefresh_LinkedPersistsToLinkedRow(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"linkedYNewAcc","expires_in":3599}`))
	}))
	defer srv.Close()
	src, _, cleanup := setupYouTubeSource(t, WithTokenURL(srv.URL))
	defer cleanup()

	cred, err := src.Resolve(context.Background(), ytLinkedT, "UClinked")
	require.NoError(t, err)
	require.NoError(t, src.Refresh(context.Background(), cred))

	reread, err := src.Resolve(context.Background(), ytLinkedT, "UClinked")
	require.NoError(t, err)
	assert.Equal(t, "linkedYNewAcc", reread.AccessToken, "refresh wrote back to youtube_oauth_tokens")
	assert.Contains(t, reread.GrantedScopes, ytForceSSL, "a refresh must not clobber linked granted_scopes")
}

func setupYouTubeSource(t *testing.T, opts ...Option) (*YouTubeSource, Cipher, func()) {
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
			access_token TEXT NOT NULL,
			refresh_token TEXT NOT NULL,
			token_expires_at TIMESTAMP NOT NULL,
			granted_scopes TEXT[] NOT NULL DEFAULT '{}',
			updated_at TIMESTAMP DEFAULT NOW()
		);
		CREATE TABLE youtube_oauth_tokens (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			user_id UUID NOT NULL,
			channel_id VARCHAR(255) NOT NULL,
			access_token TEXT NOT NULL,
			refresh_token TEXT NOT NULL,
			token_type VARCHAR(50) DEFAULT 'Bearer',
			expiry TIMESTAMP NOT NULL,
			granted_scopes TEXT[] NOT NULL DEFAULT '{}',
			encryption_version INT NOT NULL DEFAULT 1,
			created_at TIMESTAMP DEFAULT NOW(),
			updated_at TIMESTAMP DEFAULT NOW(),
			UNIQUE(user_id, channel_id)
		);`
	_, err = pool.Exec(ctx, schema)
	require.NoError(t, err)

	enc := func(s string) string {
		v, encErr := cipher.EncryptString(s)
		require.NoError(t, encErr)
		return v
	}
	exp := time.Now().Add(time.Hour)

	_, err = pool.Exec(ctx, `INSERT INTO users (id, username, auth_provider, access_token, refresh_token, token_expires_at, granted_scopes)
		VALUES ($1,'ytstreamer','youtube',$2,$3,$4,$5)`,
		ytUserY, enc("yaccY"), enc("yrefY"), exp, []string{"https://www.googleapis.com/auth/youtube.readonly", ytForceSSL})
	require.NoError(t, err)

	// ytUserY also has an exact per-channel token for "UCself" — the moderation service
	// must prefer it over the channel-agnostic users-row credential.
	_, err = pool.Exec(ctx, `INSERT INTO youtube_oauth_tokens (user_id, channel_id, access_token, refresh_token, expiry, granted_scopes, encryption_version)
		VALUES ($1,'UCself',$2,$3,$4,$5,1)`,
		ytUserY, enc("yaccSelf"), enc("yrefSelf"), exp, []string{"https://www.googleapis.com/auth/youtube.readonly", ytForceSSL})
	require.NoError(t, err)

	// ytLinkedT: a twitch-login streamer who LINKED a YouTube channel. There is no
	// users row with auth_provider='youtube' for them; their moderation credential lives
	// only in youtube_oauth_tokens, keyed by the channel id (UC...).
	_, err = pool.Exec(ctx, `INSERT INTO users (id, username, auth_provider, access_token, refresh_token, token_expires_at, granted_scopes)
		VALUES ($1,'twitchnativeyt','twitch',$2,$3,$4,$5)`,
		ytLinkedT, enc("tacc"), enc("tref"), exp, []string{"user:read:chat"})
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO youtube_oauth_tokens (user_id, channel_id, access_token, refresh_token, expiry, granted_scopes, encryption_version)
		VALUES ($1,'UClinked',$2,$3,$4,$5,1)`,
		ytLinkedT, enc("laccLinked"), enc("lrefLinked"), exp, []string{"https://www.googleapis.com/auth/youtube.readonly", ytForceSSL})
	require.NoError(t, err)

	src := NewYouTubeSource(pool, cipher, "test-client-id", "test-client-secret", opts...)
	return src, cipher, func() {
		pool.Close()
		_ = container.Terminate(ctx)
	}
}
