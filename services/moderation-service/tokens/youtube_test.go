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
	src, _, cleanup := setupYouTubeSource(t)
	defer cleanup()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, r.ParseForm())
		assert.Equal(t, "refresh_token", r.Form.Get("grant_type"))
		w.Header().Set("Content-Type", "application/json")
		// Google does not reissue a refresh token on refresh.
		_, _ = w.Write([]byte(`{"access_token":"newYAcc","expires_in":3599}`))
	}))
	defer srv.Close()
	src.tokenURL = srv.URL

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

func setupYouTubeSource(t *testing.T) (*YouTubeSource, Cipher, func()) {
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

	src := NewYouTubeSource(pool, cipher, "test-client-id", "test-client-secret")
	return src, cipher, func() {
		pool.Close()
		_ = container.Terminate(ctx)
	}
}
