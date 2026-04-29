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
)

// Join("شوشو") used to add the channel to pendingJoins, where it would age
// out at joinAckTimeout (180s) and force a full IRC reconnect — Twitch never
// sends SELFJOIN nor NOTICE for non-login-shaped names. Join() must now
// short-circuit before touching the wire and add a permanent bannedChannels
// entry so the watchdog never sees the name again.
func TestJoin_InvalidLogin_PermanentlyBannedNotPending(t *testing.T) {
	cm := newTestConnectionManager()

	cm.Join("شوشو")

	cm.pendingJoinsMu.Lock()
	_, isPending := cm.pendingJoins["شوشو"]
	cm.pendingJoinsMu.Unlock()
	if isPending {
		t.Error("Join() with an invalid login must not add to pendingJoins (would trigger joinAckWatchdog)")
	}

	cm.bannedChannelsMu.Lock()
	until, banned := cm.bannedChannels["شوشو"]
	cm.bannedChannelsMu.Unlock()
	if !banned {
		t.Fatal("Join() with an invalid login must add the channel to bannedChannels")
	}
	if !until.IsZero() {
		t.Errorf("invalid-login bans must be permanent (zero time); got expiry=%v", until)
	}
}

// A second Join() call for the same invalid login must be idempotent — i.e.
// must not blow up with a nil-client deref (test cm has no client) and must
// not overwrite the permanent ban with a different value.
func TestJoin_InvalidLogin_IsIdempotent(t *testing.T) {
	cm := newTestConnectionManager()

	cm.Join("一代鹹魚")
	cm.Join("一代鹹魚")

	cm.bannedChannelsMu.Lock()
	until, banned := cm.bannedChannels["一代鹹魚"]
	cm.bannedChannelsMu.Unlock()
	if !banned || !until.IsZero() {
		t.Errorf("repeated Join must keep permanent ban; got banned=%v until=%v", banned, until)
	}
}
