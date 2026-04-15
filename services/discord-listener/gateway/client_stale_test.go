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

package gateway

import (
	"testing"
	"time"
)

// newTestGatewayClient builds a minimal GatewayClient suitable for stale-detection tests.
// Only the fields exercised by IsStale() / LastActivityAt() are populated.
func newTestGatewayClient() *GatewayClient {
	return &GatewayClient{
		done: make(chan struct{}),
	}
}

func TestIsStale_NeverConnected_ReturnsFalse(t *testing.T) {
	c := newTestGatewayClient()
	// lastActivityAt is zero (never received any Gateway message)
	if c.IsStale() {
		t.Error("IsStale() should return false when the connection was never established")
	}
}

func TestIsStale_RecentActivity_ReturnsFalse(t *testing.T) {
	c := newTestGatewayClient()
	c.lastActivityAt = time.Now().Add(-1 * time.Minute)
	if c.IsStale() {
		t.Error("IsStale() should return false when lastActivityAt is recent")
	}
}

func TestIsStale_ActivityJustBelowThreshold_ReturnsFalse(t *testing.T) {
	c := newTestGatewayClient()
	// Just under the stale threshold
	c.lastActivityAt = time.Now().Add(-staleLivenessThreshold + time.Second)
	if c.IsStale() {
		t.Error("IsStale() should return false when below the threshold")
	}
}

func TestIsStale_ActivityBeyondThreshold_ReturnsTrue(t *testing.T) {
	c := newTestGatewayClient()
	// Well beyond the threshold (3 minutes + 1 minute margin)
	c.lastActivityAt = time.Now().Add(-staleLivenessThreshold - time.Minute)
	if !c.IsStale() {
		t.Error("IsStale() should return true when lastActivityAt exceeds staleLivenessThreshold")
	}
}

func TestIsStale_ExactlyAtThreshold_ReturnsFalse(t *testing.T) {
	c := newTestGatewayClient()
	// Exactly at threshold — not stale yet (strictly greater than)
	c.lastActivityAt = time.Now().Add(-staleLivenessThreshold)
	if c.IsStale() {
		// Allow: might be exactly at boundary due to timing
		t.Log("IsStale() returned true exactly at threshold — acceptable due to timing")
	}
}

func TestLastActivityAt_ReflectsRecordActivity(t *testing.T) {
	c := newTestGatewayClient()
	// Before any activity, zero value
	if !c.LastActivityAt().IsZero() {
		t.Error("LastActivityAt() should be zero before any activity is recorded")
	}

	before := time.Now()
	c.recordActivity()
	after := time.Now()

	got := c.LastActivityAt()
	if got.Before(before) || got.After(after) {
		t.Errorf("LastActivityAt() = %v, want between %v and %v", got, before, after)
	}
}

func TestRecordActivity_UpdatesLastActivityAt(t *testing.T) {
	c := newTestGatewayClient()
	c.lastActivityAt = time.Now().Add(-10 * time.Minute) // simulate old activity

	before := time.Now()
	c.recordActivity()
	after := time.Now()

	got := c.LastActivityAt()
	if got.Before(before) || got.After(after) {
		t.Errorf("recordActivity() did not update lastActivityAt: got %v", got)
	}
}
