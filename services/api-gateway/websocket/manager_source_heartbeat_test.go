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

package websocket

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/caesar/all-chat/services/api-gateway/sessions"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// execCall records a single Exec invocation against the DB spy.
type execCall struct {
	sql  string
	args []any
}

// execSpy is a dbExecer that records calls instead of touching a real database,
// so the manager's best-effort idle-tracking writes can be asserted without a
// live Postgres (the api-gateway module has no testcontainers harness).
type execSpy struct {
	mu    sync.Mutex
	calls []execCall
	err   error
}

func (s *execSpy) Exec(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls = append(s.calls, execCall{sql: sql, args: args})
	return pgconn.CommandTag{}, s.err
}

func (s *execSpy) sourceHeartbeatCall() *execCall {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.calls {
		if strings.Contains(s.calls[i].sql, "overlay_chat_sources") {
			return &s.calls[i]
		}
	}
	return nil
}

// TestBumpActiveSourcesUpdatedAt_HeartbeatsActiveSources verifies the heartbeat
// issues an updated_at-only refresh scoped to ACTIVE sources of the given overlays.
// This is the write that keeps a connected overlay's Twitch (EventSub) source out of
// the source-manager cleanup's 24h stale window — the root cause of chat silently
// dropping on always-open overlays.
func TestBumpActiveSourcesUpdatedAt_HeartbeatsActiveSources(t *testing.T) {
	spy := &execSpy{}
	m := &Manager{logger: zap.NewNop(), db: spy}

	m.bumpActiveSourcesUpdatedAt(context.Background(), "overlay-a", "overlay-b")

	require.Len(t, spy.calls, 1)
	call := spy.calls[0]
	assert.Contains(t, call.sql, "UPDATE overlay_chat_sources")
	assert.Contains(t, call.sql, "updated_at = NOW()")
	// Only active sources — the cleanup only reaps is_active=true rows, and we must
	// not resurrect intentionally-inactive ones.
	assert.Contains(t, call.sql, "is_active = true")
	// updated_at-only write must NOT touch is_active (migration 059 keeps the NOTIFY
	// trigger from firing on updated_at-only changes, avoiding demand-refresh storms).
	assert.NotContains(t, call.sql, "SET is_active")
	require.Len(t, call.args, 1)
	assert.ElementsMatch(t, []string{"overlay-a", "overlay-b"}, call.args[0])
}

// TestBumpActiveSourcesUpdatedAt_NoOpWhenEmpty verifies no query is issued when there
// are no connected overlays (avoids a pointless full-table scan every heartbeat tick).
func TestBumpActiveSourcesUpdatedAt_NoOpWhenEmpty(t *testing.T) {
	spy := &execSpy{}
	m := &Manager{logger: zap.NewNop(), db: spy}

	m.bumpActiveSourcesUpdatedAt(context.Background())

	assert.Empty(t, spy.calls)
}

// TestRefreshConnectionTTLs_HeartbeatsConnectedOverlaySources verifies the periodic
// heartbeat tick refreshes the source updated_at for every demand-bearing connected
// overlay — the mechanism that prevents the reported "Twitch chat stops until refresh"
// incident on 24/7 overlays.
func TestRefreshConnectionTTLs_HeartbeatsConnectedOverlaySources(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer func() { _ = client.Close() }()

	spy := &execSpy{}
	m := &Manager{
		logger:           zap.NewNop(),
		redisClient:      client,
		db:               spy,
		pools:            map[string]*Pool{"o1": NewPool("o1", zap.NewNop()), "o2": NewPool("o2", zap.NewNop())},
		noDemandOverlays: map[string]bool{},
		connectionTTL:    10 * time.Minute,
		// refreshConnectionTTLs calls sessionManager.RefreshTTLs; wire a real one
		// (backed by the test miniredis) so the heartbeat exercises the actual
		// session-TTL refresh instead of dereferencing a nil manager. RefreshTTLs
		// only touches redis, so a nil DB pool is fine here.
		sessionManager: sessions.NewSessionManager(client, nil, zap.NewNop(), 0),
	}

	m.refreshConnectionTTLs()

	call := spy.sourceHeartbeatCall()
	require.NotNil(t, call, "expected a source updated_at heartbeat for connected overlays")
	ids, ok := call.args[0].([]string)
	require.True(t, ok)
	assert.ElementsMatch(t, []string{"o1", "o2"}, ids)
}

// TestRefreshConnectionTTLs_SkipsDemandFreeOverlays verifies demand-free
// (viewerParticipant/engagement-only) overlays are NOT heartbeated — they must not
// keep upstream capture alive, and the source heartbeat must follow the same demand
// gate as the overlay:connected key.
func TestRefreshConnectionTTLs_SkipsDemandFreeOverlays(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer func() { _ = client.Close() }()

	spy := &execSpy{}
	m := &Manager{
		logger:           zap.NewNop(),
		redisClient:      client,
		db:               spy,
		pools:            map[string]*Pool{"live": NewPool("live", zap.NewNop()), "participant": NewPool("participant", zap.NewNop())},
		noDemandOverlays: map[string]bool{"participant": true},
		connectionTTL:    10 * time.Minute,
		sessionManager:   sessions.NewSessionManager(client, nil, zap.NewNop(), 0),
	}

	m.refreshConnectionTTLs()

	call := spy.sourceHeartbeatCall()
	require.NotNil(t, call)
	ids, ok := call.args[0].([]string)
	require.True(t, ok)
	assert.Equal(t, []string{"live"}, ids, "demand-free overlays must be excluded from the source heartbeat")
}
