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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.uber.org/zap"
)

// setupHeartbeatTestDB starts a throwaway Postgres with just the columns
// heartbeatActiveSources touches. updated_at is seeded via NOW() - interval so all
// timestamps are on the DB clock (no Go/DB timezone skew), and assertions compare the
// stored value before vs after the heartbeat rather than against a Go time.
func setupHeartbeatTestDB(t *testing.T) (*pgxpool.Pool, func()) {
	t.Helper()
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
	require.NoError(t, err)

	host, err := container.Host(ctx)
	require.NoError(t, err)
	port, err := container.MappedPort(ctx, "5432")
	require.NoError(t, err)

	pool, err := pgxpool.New(ctx, "postgres://testuser:testpass@"+host+":"+port.Port()+"/testdb?sslmode=disable")
	require.NoError(t, err)

	_, err = pool.Exec(ctx, `
		CREATE TABLE overlay_chat_sources (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			overlay_id UUID NOT NULL DEFAULT gen_random_uuid(),
			platform VARCHAR(50) NOT NULL,
			channel_id VARCHAR(100) NOT NULL,
			is_active BOOLEAN NOT NULL DEFAULT true,
			updated_at TIMESTAMP NOT NULL DEFAULT NOW()
		);
	`)
	require.NoError(t, err)

	return pool, func() {
		pool.Close()
		_ = container.Terminate(ctx)
	}
}

func insertStaleSource(t *testing.T, pool *pgxpool.Pool, platform, channelID string, active bool) {
	t.Helper()
	_, err := pool.Exec(context.Background(),
		`INSERT INTO overlay_chat_sources (platform, channel_id, is_active, updated_at)
		 VALUES ($1, $2, $3, NOW() - interval '48 hours')`,
		platform, channelID, active)
	require.NoError(t, err)
}

func updatedAtOf(t *testing.T, pool *pgxpool.Pool, channelID string) time.Time {
	t.Helper()
	var ts time.Time
	require.NoError(t, pool.QueryRow(context.Background(),
		`SELECT updated_at FROM overlay_chat_sources WHERE channel_id = $1`, channelID).Scan(&ts))
	return ts
}

// TestHeartbeatActiveSources_RefreshesOnlyDeliveredActiveTwitchSources verifies the
// EventSub listener heartbeat refreshes updated_at for exactly the Twitch sources this
// pod actually delivers chat for (ChatActive), and leaves everything else untouched — so
// the source-manager cleanup never reaps a live source, without resurrecting inactive
// ones or touching channels this pod does not serve.
func TestHeartbeatActiveSources_RefreshesOnlyDeliveredActiveTwitchSources(t *testing.T) {
	pool, cleanup := setupHeartbeatTestDB(t)
	defer cleanup()

	insertStaleSource(t, pool, "twitch", "caesarlp", true)      // active + delivered -> refresh
	insertStaleSource(t, pool, "twitch", "someoneelse", true)   // active but not delivered here -> leave
	insertStaleSource(t, pool, "twitch", "inactivechan", false) // inactive -> must not resurrect

	before := map[string]time.Time{
		"caesarlp":     updatedAtOf(t, pool, "caesarlp"),
		"someoneelse":  updatedAtOf(t, pool, "someoneelse"),
		"inactivechan": updatedAtOf(t, pool, "inactivechan"),
	}

	m := &Manager{
		db:       pool,
		logger:   zap.NewNop(),
		isLeader: func() bool { return true },
		channels: map[string]*Channel{
			// BroadcasterName carries mixed case; the heartbeat must match the lowercased
			// channel_id stored in overlay_chat_sources.
			"111": {BroadcasterID: "111", BroadcasterName: "CaesarLP", ChatActive: true},
			// A tracked channel WITHOUT a live chat subscription must not be heartbeated.
			"222": {BroadcasterID: "222", BroadcasterName: "someoneelse", ChatActive: false},
		},
	}

	m.heartbeatActiveSources(context.Background())

	assert.True(t, updatedAtOf(t, pool, "caesarlp").After(before["caesarlp"]),
		"delivered active twitch source must be heartbeated")
	assert.Equal(t, before["someoneelse"], updatedAtOf(t, pool, "someoneelse"),
		"a twitch source this pod does not deliver must not be heartbeated")
	assert.Equal(t, before["inactivechan"], updatedAtOf(t, pool, "inactivechan"),
		"an inactive source must not be resurrected by the heartbeat")
}

// TestHeartbeatActiveSources_SkippedOnNonLeader verifies a standby pod (not the EventSub
// leader) never heartbeats — only the leader holds real subscriptions and delivers chat,
// so a standby refreshing updated_at would keep a source alive that nothing is serving.
func TestHeartbeatActiveSources_SkippedOnNonLeader(t *testing.T) {
	pool, cleanup := setupHeartbeatTestDB(t)
	defer cleanup()

	insertStaleSource(t, pool, "twitch", "caesarlp", true)
	before := updatedAtOf(t, pool, "caesarlp")

	m := &Manager{
		db:       pool,
		logger:   zap.NewNop(),
		isLeader: func() bool { return false },
		channels: map[string]*Channel{"111": {BroadcasterName: "caesarlp", ChatActive: true}},
	}

	m.heartbeatActiveSources(context.Background())

	assert.Equal(t, before, updatedAtOf(t, pool, "caesarlp"),
		"a standby (non-leader) pod must not heartbeat sources")
}
