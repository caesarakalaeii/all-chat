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

package channels

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// TestQueryActiveTwitchChannelCredentials verifies the credential lookup that feeds
// SyncChannels, including the ADR-0016 path: a channel whose owner signed up via
// YouTube/Kick has no matching users row, but linked credentials in
// twitch_oauth_tokens must surface a token and has_chat_scope=true.
func TestQueryActiveTwitchChannelCredentials(t *testing.T) {
	pool, cleanup := setupQueryTestDB(t)
	defer cleanup()
	ctx := context.Background()

	mustExec := func(sql string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("exec failed: %v\nsql: %s", err, sql)
		}
	}

	mustExec(`INSERT INTO overlays (id, is_active) VALUES
		('11111111-1111-1111-1111-111111111111', true)`)

	// Channel A: classic case — Twitch-login user with chat scopes and a valid token.
	mustExec(`INSERT INTO overlay_chat_sources (id, overlay_id, platform, channel_id, is_active) VALUES
		('aaaaaaaa-0000-0000-0000-000000000001', '11111111-1111-1111-1111-111111111111', 'twitch', 'streamer_a', true)`)
	mustExec(`INSERT INTO users (username, auth_provider, access_token, token_expires_at, granted_scopes) VALUES
		('streamer_a', 'twitch', 'token-a', NOW() + INTERVAL '2 hours', ARRAY['user:read:chat','user:bot','channel:bot'])`)

	// Channel B: ADR-0016 case — no users row, linked credentials only (login case differs).
	mustExec(`INSERT INTO overlay_chat_sources (id, overlay_id, platform, channel_id, is_active) VALUES
		('aaaaaaaa-0000-0000-0000-000000000002', '11111111-1111-1111-1111-111111111111', 'twitch', 'streamer_b', true)`)
	mustExec(`INSERT INTO twitch_oauth_tokens (user_id, twitch_user_id, twitch_login, access_token, refresh_token, token_expires_at, granted_scopes) VALUES
		(gen_random_uuid(), '222', 'Streamer_B', 'token-b-linked', 'refresh-b', NOW() + INTERVAL '2 hours', ARRAY['user:read:chat','user:bot','channel:bot'])`)

	// Channel C: no credentials anywhere.
	mustExec(`INSERT INTO overlay_chat_sources (id, overlay_id, platform, channel_id, is_active) VALUES
		('aaaaaaaa-0000-0000-0000-000000000003', '11111111-1111-1111-1111-111111111111', 'twitch', 'streamer_c', true)`)

	// Channel D: users row WITHOUT chat scopes, but valid linked credentials WITH them —
	// the linked credential must win so the channel can be served via EventSub.
	mustExec(`INSERT INTO overlay_chat_sources (id, overlay_id, platform, channel_id, is_active) VALUES
		('aaaaaaaa-0000-0000-0000-000000000004', '11111111-1111-1111-1111-111111111111', 'twitch', 'streamer_d', true)`)
	mustExec(`INSERT INTO users (username, auth_provider, access_token, token_expires_at, granted_scopes) VALUES
		('streamer_d', 'twitch', 'token-d-login-only', NOW() + INTERVAL '2 hours', ARRAY['bits:read'])`)
	mustExec(`INSERT INTO twitch_oauth_tokens (user_id, twitch_user_id, twitch_login, access_token, refresh_token, token_expires_at, granted_scopes) VALUES
		(gen_random_uuid(), '444', 'streamer_d', 'token-d-linked', 'refresh-d', NOW() + INTERVAL '2 hours', ARRAY['user:read:chat','user:bot','channel:bot'])`)

	type row struct {
		token        *string
		hasChatScope bool
	}
	got := map[string]row{}

	rows, err := pool.Query(ctx, QueryActiveTwitchChannelCredentials)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var sourceID, channelID, overlayID string
		var accessToken *string
		var tokenExpiresAt *time.Time
		var hasChatScope bool
		if err := rows.Scan(&sourceID, &channelID, &overlayID, &accessToken, &tokenExpiresAt, &hasChatScope); err != nil {
			t.Fatalf("scan failed: %v", err)
		}
		got[channelID] = row{token: accessToken, hasChatScope: hasChatScope}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows error: %v", err)
	}

	if len(got) != 4 {
		t.Fatalf("expected 4 channels, got %d: %v", len(got), got)
	}

	a := got["streamer_a"]
	if !a.hasChatScope || a.token == nil || *a.token != "token-a" {
		t.Errorf("channel A: want users-row token with chat scope, got %+v", a)
	}

	b := got["streamer_b"]
	if !b.hasChatScope {
		t.Errorf("channel B (linked credentials only): has_chat_scope must be true")
	}
	if b.token == nil || *b.token != "token-b-linked" {
		t.Errorf("channel B: want linked token, got %+v", b)
	}

	c := got["streamer_c"]
	if c.hasChatScope || c.token != nil {
		t.Errorf("channel C: want no credentials, got %+v", c)
	}

	d := got["streamer_d"]
	if !d.hasChatScope {
		t.Errorf("channel D: linked chat-scoped credential must win over scope-less users row")
	}
	if d.token == nil || *d.token != "token-d-linked" {
		t.Errorf("channel D: want linked token, got %+v", d)
	}
}

func setupQueryTestDB(t *testing.T) (*pgxpool.Pool, func()) {
	ctx := context.Background()

	req := testcontainers.ContainerRequest{
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
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		t.Fatalf("Failed to start container: %v", err)
	}

	host, err := container.Host(ctx)
	if err != nil {
		t.Fatalf("Failed to get container host: %v", err)
	}
	port, err := container.MappedPort(ctx, "5432")
	if err != nil {
		t.Fatalf("Failed to get container port: %v", err)
	}

	pool, err := pgxpool.New(ctx, "postgres://testuser:testpass@"+host+":"+port.Port()+"/testdb?sslmode=disable")
	if err != nil {
		t.Fatalf("Failed to create connection pool: %v", err)
	}

	schema := `
		CREATE TABLE overlays (
			id UUID PRIMARY KEY,
			is_active BOOLEAN NOT NULL DEFAULT true
		);
		CREATE TABLE overlay_chat_sources (
			id UUID PRIMARY KEY,
			overlay_id UUID NOT NULL,
			platform VARCHAR(50) NOT NULL,
			channel_id VARCHAR(100) NOT NULL,
			is_active BOOLEAN NOT NULL DEFAULT true
		);
		CREATE TABLE users (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			username VARCHAR(100) NOT NULL,
			auth_provider VARCHAR(20) NOT NULL,
			access_token TEXT NOT NULL,
			token_expires_at TIMESTAMP NOT NULL,
			granted_scopes TEXT[] NOT NULL DEFAULT '{}'
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
			encryption_version INT NOT NULL DEFAULT 1
		);
	`
	if _, err := pool.Exec(ctx, schema); err != nil {
		t.Fatalf("Failed to create schema: %v", err)
	}

	return pool, func() {
		pool.Close()
		container.Terminate(ctx)
	}
}
