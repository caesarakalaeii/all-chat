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
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"go.uber.org/zap"
)

// countingResolver counts GetUserIDByLogin calls to verify caching.
type countingResolver struct {
	calls atomic.Int64
	id    string
	err   error
}

func (r *countingResolver) GetUserIDByLogin(_ context.Context, _ string) (string, error) {
	r.calls.Add(1)
	return r.id, r.err
}

// TestResolveBroadcasterID_CachesPositive verifies a successful login→id resolution is
// cached, so repeated syncs don't re-hit the Twitch API for the same channel.
func TestResolveBroadcasterID_CachesPositive(t *testing.T) {
	r := &countingResolver{id: "12345"}
	m := NewManager(nil, zap.NewNop(), r, nil, time.Minute)

	for i := 0; i < 3; i++ {
		id, err := m.resolveBroadcasterID(context.Background(), "caesarlp")
		if err != nil || id != "12345" {
			t.Fatalf("resolve %d: id=%q err=%v", i, id, err)
		}
	}
	if got := r.calls.Load(); got != 1 {
		t.Fatalf("resolver called %d times, want 1 (positive cache)", got)
	}
}

// TestResolveBroadcasterID_NegativeCache verifies an unresolvable login (deleted account)
// is negatively cached, so it isn't re-resolved every sync.
func TestResolveBroadcasterID_NegativeCache(t *testing.T) {
	r := &countingResolver{err: errors.New("user not found")}
	m := NewManager(nil, zap.NewNop(), r, nil, time.Minute)

	for i := 0; i < 3; i++ {
		if _, err := m.resolveBroadcasterID(context.Background(), "deleteduser"); err == nil {
			t.Fatalf("expected error on resolve %d", i)
		}
	}
	if got := r.calls.Load(); got != 1 {
		t.Fatalf("resolver called %d times, want 1 (negative cache)", got)
	}
}
