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

package oauth

import (
	"context"
	"testing"
	"time"

	"github.com/caesar/all-chat/shared/encryption"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.uber.org/zap"
	"golang.org/x/oauth2"
)

// youtube_oauth_tokens as the migrations leave it, reduced to the columns the store touches.
const storeTestSchema = `
CREATE TABLE youtube_oauth_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL,
    channel_id VARCHAR(255) NOT NULL,
    access_token TEXT NOT NULL,
    refresh_token TEXT NOT NULL,
    token_type VARCHAR(50) DEFAULT 'Bearer',
    expiry TIMESTAMP NOT NULL,
    encryption_version SMALLINT NOT NULL DEFAULT 0,
    updated_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(user_id, channel_id)
)`

func storeTestDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	if testing.Short() {
		t.Skip("needs PostgreSQL")
	}
	ctx := context.Background()
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "postgres:16-alpine",
			ExposedPorts: []string{"5432/tcp"},
			Env:          map[string]string{"POSTGRES_USER": "testuser", "POSTGRES_PASSWORD": "testpass", "POSTGRES_DB": "testdb"},
			WaitingFor:   wait.ForLog("database system is ready to accept connections").WithOccurrence(2).WithStartupTimeout(60 * time.Second),
		},
		Started: true,
	})
	if err != nil {
		t.Skipf("cannot start postgres testcontainer (docker unavailable?): %v", err)
	}
	t.Cleanup(func() { _ = container.Terminate(ctx) })
	host, err := container.Host(ctx)
	if err != nil {
		t.Fatal(err)
	}
	port, err := container.MappedPort(ctx, "5432")
	if err != nil {
		t.Fatal(err)
	}
	pool, err := pgxpool.New(ctx, "postgres://testuser:testpass@"+host+":"+port.Port()+"/testdb?sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if _, err := pool.Exec(ctx, storeTestSchema); err != nil {
		t.Fatal(err)
	}
	return pool
}

func testEncryptor(t *testing.T) *encryption.MultiKeyEncryptor {
	t.Helper()
	aes, err := encryption.NewAESEncryptor([]byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		t.Fatal(err)
	}
	enc, err := encryption.NewMultiKeyEncryptor([]encryption.KeyEntry{{Kid: 1, Cipher: aes}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	return enc
}

// A refresh write-back must never recreate a row: the row is deleted when the
// streamer disconnects YouTube in Settings, and a refresh that was already in flight
// must not bring their token back.
func TestUpdateToken_DoesNotRecreateADeletedRow(t *testing.T) {
	pool := storeTestDB(t)
	store := NewPostgresTokenStore(pool, testEncryptor(t), zap.NewNop())
	ctx := context.Background()
	userID := "11111111-1111-1111-1111-111111111111"

	refreshed := &oauth2.Token{AccessToken: "new-access", RefreshToken: "refresh", TokenType: "Bearer", Expiry: time.Now().Add(time.Hour)}
	if err := store.UpdateToken(ctx, userID, "UCgone", refreshed); err == nil {
		t.Error("updating a row that does not exist must report it")
	}
	var n int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM youtube_oauth_tokens`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("a refresh write-back recreated %d deleted token row(s)", n)
	}
}

func TestUpdateToken_ReplacesTheStoredToken(t *testing.T) {
	pool := storeTestDB(t)
	store := NewPostgresTokenStore(pool, testEncryptor(t), zap.NewNop())
	ctx := context.Background()
	userID := "11111111-1111-1111-1111-111111111111"
	if _, err := pool.Exec(ctx, `INSERT INTO youtube_oauth_tokens (user_id, channel_id, access_token, refresh_token, expiry)
		VALUES ($1, 'UCme', 'old-access', 'old-refresh', NOW())`, userID); err != nil {
		t.Fatal(err)
	}

	refreshed := &oauth2.Token{AccessToken: "new-access", RefreshToken: "new-refresh", TokenType: "Bearer", Expiry: time.Now().Add(time.Hour)}
	if err := store.UpdateToken(ctx, userID, "UCme", refreshed); err != nil {
		t.Fatalf("UpdateToken: %v", err)
	}
	got, err := store.GetToken(ctx, userID, "UCme")
	if err != nil {
		t.Fatalf("GetToken: %v", err)
	}
	if got.AccessToken != "new-access" || got.RefreshToken != "new-refresh" {
		t.Errorf("token = %+v, want the refreshed pair", got)
	}
}
