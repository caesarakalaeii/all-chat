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
