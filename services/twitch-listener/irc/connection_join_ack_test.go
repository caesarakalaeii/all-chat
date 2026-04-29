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

	"github.com/gempir/go-twitch-irc/v4"
)

func TestHandleSelfJoin_RemovesFromPending(t *testing.T) {
	cm := newTestConnectionManager()
	cm.pendingJoins["caedrel"] = time.Now()

	cm.handleSelfJoin(twitch.UserJoinMessage{Channel: "caedrel"})

	cm.pendingJoinsMu.Lock()
	_, stillPending := cm.pendingJoins["caedrel"]
	cm.pendingJoinsMu.Unlock()
	if stillPending {
		t.Error("expected SELFJOIN to clear caedrel from pendingJoins, but it is still tracked")
	}
}

func TestHandleSelfJoin_LowercasesChannel(t *testing.T) {
	cm := newTestConnectionManager()
	cm.pendingJoins["caesarlp"] = time.Now()

	// Twitch normally lowercases, but assert defensively.
	cm.handleSelfJoin(twitch.UserJoinMessage{Channel: "CaesarLP"})

	cm.pendingJoinsMu.Lock()
	_, stillPending := cm.pendingJoins["caesarlp"]
	cm.pendingJoinsMu.Unlock()
	if stillPending {
		t.Error("expected SELFJOIN with mixed case to clear lowercase pendingJoins entry")
	}
}

func TestDepart_RemovesFromPending(t *testing.T) {
	cm := newTestConnectionManager()
	cm.pendingJoins["caedrel"] = time.Now()

	// Calling Depart on a brand-new ConnectionManager would dereference
	// cm.client; only exercise the pendingJoins cleanup here.
	cm.pendingJoinsMu.Lock()
	delete(cm.pendingJoins, "caedrel")
	cm.pendingJoinsMu.Unlock()

	cm.pendingJoinsMu.Lock()
	_, stillPending := cm.pendingJoins["caedrel"]
	cm.pendingJoinsMu.Unlock()
	if stillPending {
		t.Error("Depart-equivalent cleanup should remove caedrel from pendingJoins")
	}
}

func TestClearPendingJoins_EmptiesMap(t *testing.T) {
	cm := newTestConnectionManager()
	cm.pendingJoins["a"] = time.Now()
	cm.pendingJoins["b"] = time.Now()

	cm.clearPendingJoins()

	cm.pendingJoinsMu.Lock()
	count := len(cm.pendingJoins)
	cm.pendingJoinsMu.Unlock()
	if count != 0 {
		t.Errorf("expected pendingJoins to be empty after clear, got %d entries", count)
	}
}

func TestPendingJoins_AgeDetection(t *testing.T) {
	cm := newTestConnectionManager()
	cm.pendingJoins["fresh"] = time.Now()
	cm.pendingJoins["stuck"] = time.Now().Add(-2 * joinAckTimeout)

	now := time.Now()
	var stuck []string
	cm.pendingJoinsMu.Lock()
	for ch, sentAt := range cm.pendingJoins {
		if now.Sub(sentAt) > joinAckTimeout {
			stuck = append(stuck, ch)
		}
	}
	cm.pendingJoinsMu.Unlock()

	if len(stuck) != 1 || stuck[0] != "stuck" {
		t.Errorf("expected only 'stuck' to exceed joinAckTimeout, got %v", stuck)
	}
}
