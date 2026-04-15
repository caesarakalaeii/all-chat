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

// Package redisutil provides in-memory Redis test helpers for demand subscriber tests.
// It is intentionally separate from the testutil package so service-level smoke tests
// can import testutil without pulling in the miniredis dependency.
package redisutil

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

// StartTestRedis starts an in-memory miniredis server for tests.
// The server is NOT automatically closed — caller must close it explicitly before goleak.VerifyNone.
// Returns the miniredis server and its address (host:port).
func StartTestRedis(t *testing.T) (*miniredis.Miniredis, string) {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("failed to start miniredis: %v", err)
	}
	return mr, mr.Addr()
}

// StartTestRedisWithClient starts miniredis and returns both the server and a connected go-redis client.
// The caller must explicitly close the client and server (in that order) before goleak.VerifyNone fires,
// to allow miniredis goroutines to drain cleanly.
func StartTestRedisWithClient(t *testing.T) (*miniredis.Miniredis, *redis.Client) {
	t.Helper()
	mr, addr := StartTestRedis(t)
	rc := redis.NewClient(&redis.Options{
		Addr:        addr,
		DialTimeout: 2 * time.Second,
		ReadTimeout: 2 * time.Second,
	})
	if err := rc.Ping(context.Background()).Err(); err != nil {
		mr.Close()
		t.Fatalf("failed to connect to test Redis at %s: %v", addr, err)
	}
	return mr, rc
}

// NewTestRedisClient creates a go-redis client connected to the given address.
// The client is automatically closed when the test ends.
func NewTestRedisClient(t *testing.T, addr string) *redis.Client {
	t.Helper()
	rc := redis.NewClient(&redis.Options{
		Addr:        addr,
		DialTimeout: 2 * time.Second,
		ReadTimeout: 2 * time.Second,
	})
	if err := rc.Ping(context.Background()).Err(); err != nil {
		t.Fatalf("failed to connect to test Redis at %s: %v", addr, err)
	}
	t.Cleanup(func() { rc.Close() })
	return rc
}
