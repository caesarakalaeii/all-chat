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

package main

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The bucket key holds the raw client IP, and the privacy policy promises it
// lives only as a counter for one window. These tests pin that the key always
// carries the window as its TTL.

const rawIPKey = "ip:203.0.113.42"

// dropExpire fails every Redis round trip that carries an EXPIRE before it
// reaches Redis, as a dropped connection or a pod killed mid-request would.
type dropExpire struct{}

var errDropped = errors.New("connection dropped")

func hasExpire(cmds []redis.Cmder) bool {
	for _, cmd := range cmds {
		if cmd.Name() == "expire" {
			return true
		}
	}
	return false
}

func (dropExpire) DialHook(next redis.DialHook) redis.DialHook { return next }

func (dropExpire) ProcessHook(next redis.ProcessHook) redis.ProcessHook {
	return func(ctx context.Context, cmd redis.Cmder) error {
		if hasExpire([]redis.Cmder{cmd}) {
			return errDropped
		}
		return next(ctx, cmd)
	}
}

func (dropExpire) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return func(ctx context.Context, cmds []redis.Cmder) error {
		if hasExpire(cmds) {
			return errDropped
		}
		return next(ctx, cmds)
	}
}

func TestRateLimiter_LostExpireLeavesNoKeyWithoutTTL(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	rdb.AddHook(dropExpire{})
	rl := newRedisRateLimiter(rdb, 60, time.Minute)

	_, _, err := rl.allow(context.Background(), rawIPKey)
	require.ErrorIs(t, err, errDropped)

	mr.FastForward(time.Minute)
	assert.False(t, mr.Exists("rate:"+rawIPKey), "the raw-IP key outlived its window")
}

func TestRateLimiter_KeyAlwaysExpiresAtWindowEnd(t *testing.T) {
	mr := miniredis.RunT(t)
	rl := newRedisRateLimiter(redis.NewClient(&redis.Options{Addr: mr.Addr()}), 60, 30*time.Second)
	// A key without a TTL, as the old INCR-then-EXPIRE code could leave behind.
	require.NoError(t, mr.Set("rate:"+rawIPKey, "5"))

	allowed, _, err := rl.allow(context.Background(), rawIPKey)
	require.NoError(t, err)
	assert.True(t, allowed)
	assert.Equal(t, 30*time.Second, mr.TTL("rate:"+rawIPKey))

	// Later hits keep the window fixed instead of pushing the expiry out.
	mr.FastForward(10 * time.Second)
	_, _, err = rl.allow(context.Background(), rawIPKey)
	require.NoError(t, err)
	assert.Equal(t, 20*time.Second, mr.TTL("rate:"+rawIPKey))
}
