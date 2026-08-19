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

// XREADGROUP BLOCK regression guard (issue #546). Both blocking consumers passed a bare
// `Block: 5000` to a time.Duration field, i.e. 5000 NANOseconds. go-redis emits
// int64(Block/time.Millisecond), so that truncated to "BLOCK 0" — block forever — while
// feeding the same 5000ns to setReadTimeout, which cmdTimeout turns into a 5000ns+10s
// socket deadline (the t == 0 early-return that disables the deadline for a genuine
// block-forever is missed, because 5000ns is not 0). Redis waited indefinitely, the
// client gave up at ~10s, and every idle read logged `i/o timeout` and tore down the
// connection — ~12 warns/min/pod.
//
// These tests assert on the argument that actually reaches Redis, captured by a
// redis.Hook, rather than on log output or elapsed wall time (both flaky). The hook
// needs no live server, so the guard runs under `go test -short` and the CI gate really
// checks the fix.
package consumer

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// argCapture records the args of every XREADGROUP the client emits.
type argCapture struct {
	calls chan []any
}

func newArgCapture() *argCapture { return &argCapture{calls: make(chan []any, 64)} }

func (a *argCapture) DialHook(next redis.DialHook) redis.DialHook { return next }

func (a *argCapture) ProcessHook(next redis.ProcessHook) redis.ProcessHook {
	return func(ctx context.Context, cmd redis.Cmder) error {
		if cmd.Name() == "xreadgroup" {
			args := append([]any(nil), cmd.Args()...)
			select {
			case a.calls <- args:
			default:
			}
		}
		return next(ctx, cmd)
	}
}

func (a *argCapture) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return next
}

// next returns the args of the next captured XREADGROUP, failing if none arrives.
func (a *argCapture) next(t *testing.T) []any {
	t.Helper()
	select {
	case args := <-a.calls:
		return args
	case <-time.After(5 * time.Second):
		t.Fatal("no XREADGROUP was emitted")
		return nil
	}
}

// blockArg returns the value following the "block" token in an XREADGROUP arg list.
func blockArg(t *testing.T, args []any) (int64, bool) {
	t.Helper()
	for i, a := range args {
		s, ok := a.(string)
		if !ok || s != "block" || i+1 >= len(args) {
			continue
		}
		v, ok := args[i+1].(int64)
		require.Truef(t, ok, "block argument should be an int64, got %T", args[i+1])
		return v, true
	}
	return 0, false
}

// TestReadBlockTimeIsFiveSeconds pins the constant itself. Both call sites and the
// emitted-arg assertions below agree on this value; message-processor's ReadBlockTime is
// the same 5s.
func TestReadBlockTimeIsFiveSeconds(t *testing.T) {
	assert.Equal(t, 5*time.Second, readBlockTime,
		"readBlockTime must be a real 5s time.Duration, not a bare integer")

	// The conversion go-redis applies when building the command. A bare 5000 would
	// land here as 5000ns and truncate to 0 — "block forever".
	assert.EqualValues(t, 5000, int64(readBlockTime/time.Millisecond),
		"readBlockTime must serialise to 5000 milliseconds, not 0")
}

// deadListener is an address that accepts TCP connections and then says nothing. It lets
// the consumer's XREADGROUP be built and dispatched (so the hook captures the real args)
// without needing a Redis server or a handler — the read fails, the loop logs and we stop
// it. No message is ever delivered, so the nil repo is never touched.
func deadListener(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			t.Cleanup(func() { _ = conn.Close() })
		}
	}()
	return ln.Addr().String()
}

// runUntilXReadGroup starts run in the background and returns the args of the first
// XREADGROUP it emits, then cancels it.
func runUntilXReadGroup(t *testing.T, run func(context.Context, *redis.Client)) []any {
	t.Helper()
	capture := newArgCapture()
	rdb := redis.NewClient(&redis.Options{
		Addr: deadListener(t),
		// Keep the failing handshake/read from stalling the test; the hook has
		// already captured the args by the time the read times out.
		DialTimeout:  time.Second,
		ReadTimeout:  200 * time.Millisecond,
		WriteTimeout: time.Second,
		MaxRetries:   -1,
	})
	rdb.AddHook(capture)
	t.Cleanup(func() { _ = rdb.Close() })

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		run(ctx, rdb)
	}()

	args := capture.next(t)
	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("consumer did not stop after context cancellation")
	}
	return args
}

// TestCommandConsumerEmitsBlock5000 proves the command consumer's live read loop tells
// Redis to block for 5000ms. Before the fix this emitted "block 0" — block forever —
// against a ~10s client-side deadline.
func TestCommandConsumerEmitsBlock5000(t *testing.T) {
	args := runUntilXReadGroup(t, func(ctx context.Context, rdb *redis.Client) {
		NewCommandConsumer(rdb, nil, nil, "test-pod", zap.NewNop()).Run(ctx)
	})

	block, ok := blockArg(t, args)
	require.Truef(t, ok, "XREADGROUP must carry a BLOCK argument: %v", args)
	assert.EqualValues(t, 5000, block,
		"BLOCK must be 5000ms; 0 means block forever and desynchronises from the read deadline")
}

// TestNativeConsumerEmitsBlock5000 is the same guard for the native-mirror consumer,
// which had the identical bug with Count: 32.
func TestNativeConsumerEmitsBlock5000(t *testing.T) {
	args := runUntilXReadGroup(t, func(ctx context.Context, rdb *redis.Client) {
		NewNativeConsumer(rdb, nil, nil, "test-pod", zap.NewNop()).Run(ctx)
	})

	block, ok := blockArg(t, args)
	require.Truef(t, ok, "XREADGROUP must carry a BLOCK argument: %v", args)
	assert.EqualValues(t, 5000, block,
		"BLOCK must be 5000ms; 0 means block forever and desynchronises from the read deadline")
}
