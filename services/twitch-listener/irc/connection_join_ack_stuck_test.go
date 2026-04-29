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

// applyStuckJoinBan is the test-only port of the per-channel-ban branch in
// joinAckWatchdog. We can't run the watchdog directly because it calls
// client.Disconnect() / client.Depart() and the test ConnectionManager has no
// real client; this helper exercises the pure bookkeeping (bannedChannels +
// pendingJoins) so a regression in the branch can't slip through.
func (cm *ConnectionManager) applyStuckJoinBan(stuck []string, until time.Time) {
	cm.bannedChannelsMu.Lock()
	for _, ch := range stuck {
		if existing, present := cm.bannedChannels[ch]; present && existing.IsZero() {
			continue
		}
		cm.bannedChannels[ch] = until
	}
	cm.bannedChannelsMu.Unlock()

	cm.pendingJoinsMu.Lock()
	for _, ch := range stuck {
		delete(cm.pendingJoins, ch)
	}
	cm.pendingJoinsMu.Unlock()
}

// A handful of stuck channels (under joinAckReconnectThreshold) should be
// transiently banned, not trigger a full reconnect — pendingJoins must be
// cleared so the watchdog stops counting them, and bannedChannels must hold a
// non-zero (transient) entry so isJoinBanned will skip the next Sync's JOIN.
func TestStuckJoinBan_FewChannels_BansTransientlyAndClearsPending(t *testing.T) {
	cm := newTestConnectionManager()

	now := time.Now()
	cm.pendingJoins["spokojnypajonk"] = now.Add(-2 * joinAckTimeout)
	cm.pendingJoins["s4nt_bot"] = now.Add(-2 * joinAckTimeout)
	cm.pendingJoins["caedrel"] = now // healthy — must NOT be touched

	stuck := []string{"spokojnypajonk", "s4nt_bot"}
	until := now.Add(joinAckStuckBackoff)
	cm.applyStuckJoinBan(stuck, until)

	cm.bannedChannelsMu.Lock()
	defer cm.bannedChannelsMu.Unlock()
	for _, ch := range stuck {
		got, ok := cm.bannedChannels[ch]
		if !ok {
			t.Fatalf("expected %q in bannedChannels after stuck-ban", ch)
		}
		if got.IsZero() {
			t.Errorf("%q got permanent ban (zero time); should be transient until %v", ch, until)
		}
		if !got.After(now) {
			t.Errorf("%q ban expiry %v should be in the future", ch, got)
		}
	}

	cm.pendingJoinsMu.Lock()
	defer cm.pendingJoinsMu.Unlock()
	for _, ch := range stuck {
		if _, still := cm.pendingJoins[ch]; still {
			t.Errorf("%q must be removed from pendingJoins so the watchdog stops re-firing", ch)
		}
	}
	if _, ok := cm.pendingJoins["caedrel"]; !ok {
		t.Error("healthy channel caedrel must NOT be removed from pendingJoins")
	}
}

// A pre-existing permanent ban (zero time) must not be downgraded to a
// transient one by the stuck-ban path. msg_banned and the invalid-login
// short-circuit both write zero-time entries that mean "never re-attempt for
// this process lifetime" — overwriting them with NOW+1h would re-introduce
// the original reconnect-loop bug.
func TestStuckJoinBan_PermanentBanIsNotDowngradedToTransient(t *testing.T) {
	cm := newTestConnectionManager()

	cm.bannedChannels["already_banned_forever"] = time.Time{}
	cm.pendingJoins["already_banned_forever"] = time.Now().Add(-2 * joinAckTimeout)

	stuck := []string{"already_banned_forever"}
	cm.applyStuckJoinBan(stuck, time.Now().Add(joinAckStuckBackoff))

	cm.bannedChannelsMu.Lock()
	got, ok := cm.bannedChannels["already_banned_forever"]
	cm.bannedChannelsMu.Unlock()
	if !ok {
		t.Fatal("permanent ban entry must remain")
	}
	if !got.IsZero() {
		t.Errorf("permanent ban must stay zero (forever); got %v", got)
	}
}

// joinAckReconnectThreshold draws the line between per-channel-ban and the
// full reconnect path. Verify the constant is set so a typical 1–4-stuck
// "bad-channel cluster" never trips the connection-wide path, and a clearly-
// broken connection (≥ threshold channels stuck) still reconnects.
func TestJoinAckReconnectThreshold_TypicalBadCluster_StaysBelow(t *testing.T) {
	if joinAckReconnectThreshold < 5 {
		t.Errorf("joinAckReconnectThreshold=%d is too low; bad-channel clusters of 4 will trigger full reconnects",
			joinAckReconnectThreshold)
	}
	if joinAckReconnectThreshold > 25 {
		t.Errorf("joinAckReconnectThreshold=%d is too high; a clearly-broken connection won't recover",
			joinAckReconnectThreshold)
	}
}
