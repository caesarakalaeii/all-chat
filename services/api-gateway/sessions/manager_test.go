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

package sessions

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

func TestParseSessionTime(t *testing.T) {
	tests := []struct {
		name      string
		timeStr   string
		fieldName string
		wantError bool
	}{
		{
			name:      "valid RFC3339 time",
			timeStr:   "2026-02-07T10:00:00Z",
			fieldName: "started_at",
			wantError: false,
		},
		{
			name:      "empty string",
			timeStr:   "",
			fieldName: "started_at",
			wantError: true,
		},
		{
			name:      "invalid format",
			timeStr:   "not-a-time",
			fieldName: "started_at",
			wantError: true,
		},
		{
			name:      "zero time",
			timeStr:   "0001-01-01T00:00:00Z",
			fieldName: "started_at",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseSessionTime(tt.timeStr, tt.fieldName)
			if tt.wantError {
				if err == nil {
					t.Errorf("parseSessionTime() expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("parseSessionTime() unexpected error: %v", err)
				}
				if result.IsZero() {
					t.Errorf("parseSessionTime() returned zero time for valid input")
				}
			}
		})
	}
}

func TestValidateStartedAt(t *testing.T) {
	now := time.Now().UTC()

	tests := []struct {
		name      string
		time      time.Time
		wantError bool
	}{
		{
			name:      "valid current time",
			time:      now,
			wantError: false,
		},
		{
			name:      "valid time 1 hour ago",
			time:      now.Add(-1 * time.Hour),
			wantError: false,
		},
		{
			name:      "zero time",
			time:      time.Time{},
			wantError: true,
		},
		{
			name:      "time before 2020",
			time:      time.Date(2019, 12, 31, 23, 59, 59, 0, time.UTC),
			wantError: true,
		},
		{
			name:      "time in future (2 hours)",
			time:      now.Add(2 * time.Hour),
			wantError: true,
		},
		{
			name:      "time at year 0001 (common parse error)",
			time:      time.Date(1, 1, 1, 0, 0, 0, 0, time.UTC),
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateStartedAt(tt.time)
			if tt.wantError {
				if err == nil {
					t.Errorf("validateStartedAt() expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("validateStartedAt() unexpected error: %v", err)
				}
			}
		})
	}
}

func TestRefreshTTLs(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	sm := NewSessionManager(client, nil, zap.NewNop(), time.Minute)
	ctx := context.Background()

	// An existing session with a short TTL should be extended to SessionTTL.
	mr.HSet(SessionKeyPrefix+"live", "started_at", time.Now().UTC().Format(time.RFC3339))
	mr.SetTTL(SessionKeyPrefix+"live", time.Minute)

	// A missing overlay must be a safe no-op (EXPIRE on a missing key returns
	// false without error and must not create the key).
	sm.RefreshTTLs(ctx, []string{"live", "missing"})

	if ttl := mr.TTL(SessionKeyPrefix + "live"); ttl != SessionTTL {
		t.Fatalf("expected TTL %v after refresh, got %v", SessionTTL, ttl)
	}
	if mr.Exists(SessionKeyPrefix + "missing") {
		t.Fatalf("missing overlay key should not have been created")
	}

	// Empty input is a no-op and must not panic or error.
	sm.RefreshTTLs(ctx, nil)
}

// TestEnsureSession_ConcurrentCallsCreateExactlyOneSession is the regression
// test for the two-WebSockets-at-once race: both callers used to pass the
// exists-check before either wrote the hash, and the second INSERT left an
// orphaned ACTIVE stream_sessions row that nothing ever completes. The claim
// guarantees exactly one well-formed hash and (via the nil pool) at most one
// DB insert; the per-caller values are interchangeable, so the assertions
// check shape, not which caller won.
func TestEnsureSession_ConcurrentCallsCreateExactlyOneSession(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer client.Close()
	// db is nil on purpose and LOAD-BEARING: a live Postgres is out of reach
	// here (no testcontainers in this module), and on pre-fix code every
	// goroutine that wins the exists-check race reaches sm.db.Exec — the nil
	// pool is what panics and fails the test. The Redis hash below proves
	// what the winner wrote; do not swap db for a mock without asserting
	// insert counts instead.
	sm := NewSessionManager(client, nil, zap.NewNop(), time.Minute)
	ctx := context.Background()

	const callers = 16
	errs := make([]error, callers)
	var wg sync.WaitGroup
	for i := range callers {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			errs[i] = sm.EnsureSession(ctx, "race-overlay")
		}(i)
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Fatalf("caller %d: %v", i, err)
		}
	}
	key := SessionKeyPrefix + "race-overlay"
	// Exactly one well-formed session hash must exist (see the doc comment:
	// per-caller values are interchangeable, so shape is the contract). A
	// second writer would leave extra fields or clobber state; a loser that
	// reached the DB path would have panicked on the nil pool above.
	fields, err := client.HGetAll(ctx, key).Result()
	if err != nil {
		t.Fatalf("session hash must be readable: %v", err)
	}
	if len(fields) != 4 {
		t.Fatalf("session hash must have exactly 4 fields (session_id, started_at, state, event_count), got %d: %v", len(fields), fields)
	}
	sessionID := fields["session_id"]
	if sessionID == "" {
		t.Fatal("session_id must be set by the claim winner")
	}
	state := fields["state"]
	if state != "ACTIVE" {
		t.Fatalf("state must be the winner's ACTIVE, got %q", state)
	}
	if fields["started_at"] == "" || fields["event_count"] != "0" {
		t.Fatalf("winner's started_at and event_count must survive: %v", fields)
	}
}

// TestEnsureSession_ReleasesClaimWhenPipelineFails guards the cleanup path:
// the claim wins, then the session-hash pipeline fails (here: Redis errors
// are injected between HSetNX and the pipeline via a go-redis hook, since
// miniredis cannot fail a command on demand for the second call only). The
// half-written key must be deleted, and the next call must be able to claim
// and complete a fresh session. Without the release Del the key would strand
// forever with session_id and no TTL — the exact stuck state the pipeline-
// failure cleanup exists to prevent.
func TestEnsureSession_ReleasesClaimWhenPipelineFails(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer client.Close()
	sm := NewSessionManager(client, nil, zap.NewNop(), time.Minute)
	ctx := context.Background()
	key := SessionKeyPrefix + "fail-overlay"

	var failPipeline atomic.Bool
	client.AddHook(&releaseTestHook{fail: &failPipeline})

	// First call: claim succeeds, then the pipeline fails.
	failPipeline.Store(true)
	if err := sm.EnsureSession(ctx, "fail-overlay"); err == nil {
		t.Fatal("expected error when session pipeline fails")
	}
	if mr.Exists(key) {
		t.Fatal("half-written session key must be deleted after pipeline failure")
	}

	// Second call: Redis works again; a full session must be claimable.
	failPipeline.Store(false)
	if err := sm.EnsureSession(ctx, "fail-overlay"); err != nil {
		t.Fatalf("expected retry to succeed after cleanup, got %v", err)
	}
	if !mr.Exists(key) {
		t.Fatal("session key must exist after successful retry")
	}
	if state := mr.HGet(key, "state"); state != "ACTIVE" {
		t.Fatalf("retried session must be ACTIVE, got %q", state)
	}
}

// releaseTestHook fails the first pipeline after the HSetNX claim so the
// cleanup path runs. EnsureSession issues: Exists, HGet, HSetNX, then a
// pipeline (HSet x3 + Expire). Failing every PipelineExec call while the
// flag is set deterministically breaks only the pipeline.
type releaseTestHook struct {
	fail *atomic.Bool
}

func (h *releaseTestHook) DialHook(next redis.DialHook) redis.DialHook {
	return next
}

func (h *releaseTestHook) ProcessHook(next redis.ProcessHook) redis.ProcessHook {
	return next
}

func (h *releaseTestHook) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return func(ctx context.Context, cmds []redis.Cmder) error {
		if h.fail.Load() {
			return fmt.Errorf("injected pipeline failure")
		}
		return next(ctx, cmds)
	}
}
