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
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// newLingerTestManager builds a minimal Manager wired only with the fields
// publishConnectionEvent touches (redis client, logger, linger TTL). It deliberately
// avoids NewManager so the test does not start the heartbeat goroutine or need a DB.
func newLingerTestManager(t *testing.T, lingerTTL time.Duration) (*miniredis.Miniredis, *Manager) {
	t.Helper()
	mr, err := miniredis.Run()
	require.NoError(t, err)
	t.Cleanup(mr.Close)

	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })

	return mr, &Manager{
		logger:              zap.NewNop(),
		redisClient:         client,
		disconnectLingerTTL: lingerTTL,
	}
}

// TestPublishDisconnect_LingersConnectionKey verifies that on disconnect the
// overlay:connected key is retained (with a shortened TTL) rather than deleted, so the
// demand source-of-truth — and thus upstream chat capture — survives a brief reconnect.
func TestPublishDisconnect_LingersConnectionKey(t *testing.T) {
	mr, m := newLingerTestManager(t, 5*time.Minute)

	const overlayID = "overlay-caesarlp"
	key := "overlay:connected:" + overlayID
	require.NoError(t, mr.Set(key, "1"))

	m.publishConnectionEvent(context.Background(), overlayID, "disconnected")

	assert.True(t, mr.Exists(key), "connection key must survive disconnect so upstream capture lingers")

	ttl := mr.TTL(key)
	assert.Greater(t, ttl, time.Duration(0), "lingering key must carry a TTL so it expires for genuinely-gone overlays")
	assert.LessOrEqual(t, ttl, 5*time.Minute, "linger TTL must not exceed the configured window")
}

// TestPublishDisconnect_ImmediateDeleteWhenLingerDisabled verifies the opt-out:
// PUBSUB_LINGER_SECONDS=0 (linger TTL 0) reverts to the previous immediate-delete behavior.
func TestPublishDisconnect_ImmediateDeleteWhenLingerDisabled(t *testing.T) {
	mr, m := newLingerTestManager(t, 0)

	const overlayID = "overlay-caesarlp"
	key := "overlay:connected:" + overlayID
	require.NoError(t, mr.Set(key, "1"))

	m.publishConnectionEvent(context.Background(), overlayID, "disconnected")

	assert.False(t, mr.Exists(key), "with linger disabled the key must be deleted immediately on disconnect")
}
