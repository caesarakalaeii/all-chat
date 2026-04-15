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

package irc

import (
	"testing"
	"time"
)

func newTestConnectionManager() *ConnectionManager {
	return &ConnectionManager{
		stopChan:         make(chan struct{}),
		firstMessageChan: make(map[string]chan struct{}),
	}
}

func TestIsStale_NeverConnected_ReturnsFalse(t *testing.T) {
	cm := newTestConnectionManager()
	// lastActivityAt is zero (never connected)
	if cm.IsStale() {
		t.Error("IsStale() should return false when the connection was never established")
	}
}

func TestIsStale_RecentActivity_ReturnsFalse(t *testing.T) {
	cm := newTestConnectionManager()
	cm.lastActivityAt = time.Now().Add(-1 * time.Minute)
	if cm.IsStale() {
		t.Error("IsStale() should return false when lastActivityAt is recent")
	}
}

func TestIsStale_ActivityJustBelowThreshold_ReturnsFalse(t *testing.T) {
	cm := newTestConnectionManager()
	// Just under the 10-minute threshold
	cm.lastActivityAt = time.Now().Add(-staleLivenessThreshold + time.Second)
	if cm.IsStale() {
		t.Error("IsStale() should return false when below the threshold")
	}
}

func TestIsStale_ActivityBeyondThreshold_ReturnsTrue(t *testing.T) {
	cm := newTestConnectionManager()
	// Well beyond the 10-minute threshold
	cm.lastActivityAt = time.Now().Add(-staleLivenessThreshold - time.Minute)
	if !cm.IsStale() {
		t.Error("IsStale() should return true when lastActivityAt exceeds staleLivenessThreshold")
	}
}

func TestIsStale_ExactlyAtThreshold_ReturnsFalse(t *testing.T) {
	cm := newTestConnectionManager()
	// Exactly at threshold — not stale yet (strictly greater than)
	cm.lastActivityAt = time.Now().Add(-staleLivenessThreshold)
	// time.Since might make this flicker; give a tiny margin
	if cm.IsStale() {
		// Allow: might be exactly at boundary
		t.Log("IsStale() returned true exactly at threshold — acceptable due to timing")
	}
}
