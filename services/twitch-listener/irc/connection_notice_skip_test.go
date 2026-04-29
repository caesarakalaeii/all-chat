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
	"sync"
	"testing"
	"time"

	"github.com/caesar/all-chat/shared/metrics"
	"github.com/gempir/go-twitch-irc/v4"
)

// sharedTestMetrics is initialized once across the package: NewListenerMetrics
// registers Prometheus collectors with the global registry, so a fresh instance
// per test panics with "duplicate metrics collector registration attempted".
var (
	sharedTestMetricsOnce sync.Once
	sharedTestMetrics     *metrics.ListenerMetrics
)

// newTestConnectionManagerWithMetrics returns a ConnectionManager primed for
// handleNotice tests — handleNotice calls metrics.RecordError, which requires
// a non-nil *metrics.ListenerMetrics.
func newTestConnectionManagerWithMetrics() *ConnectionManager {
	sharedTestMetricsOnce.Do(func() {
		sharedTestMetrics = metrics.NewListenerMetrics("twitch", "twitch-listener-test")
	})
	cm := newTestConnectionManager()
	cm.metrics = sharedTestMetrics
	return cm
}

func TestIsJoinBanned_AbsentChannel_ReturnsFalse(t *testing.T) {
	cm := newTestConnectionManager()
	if cm.isJoinBanned("anychan") {
		t.Error("an unseen channel must not be reported as banned")
	}
}

func TestIsJoinBanned_PermanentEntry_ReturnsTrue(t *testing.T) {
	cm := newTestConnectionManager()
	cm.bannedChannels["kashashgaming"] = time.Time{} // zero = permanent

	if !cm.isJoinBanned("kashashgaming") {
		t.Error("permanent entry (zero time) must always be reported as banned")
	}

	// And the entry must NOT be evicted by a read — permanent stays permanent.
	cm.bannedChannelsMu.Lock()
	_, stillThere := cm.bannedChannels["kashashgaming"]
	cm.bannedChannelsMu.Unlock()
	if !stillThere {
		t.Error("permanent entries must survive isJoinBanned reads")
	}
}

func TestIsJoinBanned_NotYetExpired_ReturnsTrue(t *testing.T) {
	cm := newTestConnectionManager()
	cm.bannedChannels["caedrel"] = time.Now().Add(5 * time.Minute)

	if !cm.isJoinBanned("caedrel") {
		t.Error("entry within its backoff window must be reported as banned")
	}
}

func TestIsJoinBanned_Expired_EvictsAndReturnsFalse(t *testing.T) {
	cm := newTestConnectionManager()
	cm.bannedChannels["caedrel"] = time.Now().Add(-1 * time.Second)

	if cm.isJoinBanned("caedrel") {
		t.Error("expired entry must allow re-attempt (return false)")
	}

	cm.bannedChannelsMu.Lock()
	_, stillThere := cm.bannedChannels["caedrel"]
	cm.bannedChannelsMu.Unlock()
	if stillThere {
		t.Error("expired entry must be evicted on read so the map doesn't grow forever")
	}
}

func TestHandleNotice_MsgBanned_MarksPermanentAndClearsPending(t *testing.T) {
	cm := newTestConnectionManagerWithMetrics()
	cm.pendingJoins["kashashgaming"] = time.Now()

	cm.handleNotice(twitch.NoticeMessage{
		Channel: "kashashgaming",
		MsgID:   twitchMsgIDBanned,
		Message: "You are permanently banned from talking in kashashgaming.",
	})

	cm.bannedChannelsMu.Lock()
	until, ok := cm.bannedChannels["kashashgaming"]
	cm.bannedChannelsMu.Unlock()
	if !ok || !until.IsZero() {
		t.Errorf("msg_banned must record a permanent (zero-time) skip entry; got ok=%v until=%s", ok, until)
	}

	cm.pendingJoinsMu.Lock()
	_, stillPending := cm.pendingJoins["kashashgaming"]
	cm.pendingJoinsMu.Unlock()
	if stillPending {
		t.Error("msg_banned must clear pendingJoins so the watchdog stops triggering reconnects")
	}
}

func TestHandleNotice_MsgChannelSuspended_MarksPermanent(t *testing.T) {
	cm := newTestConnectionManagerWithMetrics()

	cm.handleNotice(twitch.NoticeMessage{
		Channel: "deadchan",
		MsgID:   twitchMsgIDChannelSuspended,
		Message: "Channel suspended.",
	})

	if !cm.isJoinBanned("deadchan") {
		t.Error("msg_channel_suspended must short-circuit future Join() calls")
	}
}

func TestHandleNotice_MsgRoomNotFound_MarksPermanent(t *testing.T) {
	cm := newTestConnectionManagerWithMetrics()

	cm.handleNotice(twitch.NoticeMessage{
		Channel: "typo_chan",
		MsgID:   twitchMsgIDRoomNotFound,
		Message: "No room found.",
	})

	if !cm.isJoinBanned("typo_chan") {
		t.Error("msg_room_not_found must short-circuit future Join() calls")
	}
}

func TestHandleNotice_ConcurrentChannelLimit_MarksTransient(t *testing.T) {
	cm := newTestConnectionManagerWithMetrics()
	before := time.Now()

	cm.handleNotice(twitch.NoticeMessage{
		Channel: "byrakeru",
		MsgID:   twitchMsgIDConcurrentChannelLimit,
		Message: "You are connected to too many chat channels.",
	})

	cm.bannedChannelsMu.Lock()
	until, ok := cm.bannedChannels["byrakeru"]
	cm.bannedChannelsMu.Unlock()
	if !ok {
		t.Fatal("msg_concurrent_channel_limit_reached must populate bannedChannels")
	}
	if until.IsZero() {
		t.Error("cap rejection must be transient (non-zero backoff time)")
	}
	expectedAtLeast := before.Add(concurrentChannelLimitBackoff - 5*time.Second)
	expectedAtMost := before.Add(concurrentChannelLimitBackoff + 5*time.Second)
	if until.Before(expectedAtLeast) || until.After(expectedAtMost) {
		t.Errorf("backoff window outside expected ~%s; got until=%s (now was %s)",
			concurrentChannelLimitBackoff, until, before)
	}
}

func TestHandleNotice_UnrelatedMsgID_DoesNotPopulateSkipList(t *testing.T) {
	cm := newTestConnectionManagerWithMetrics()
	cm.pendingJoins["chan_in_flight"] = time.Now()

	cm.handleNotice(twitch.NoticeMessage{
		Channel: "chan_in_flight",
		MsgID:   "host_off", // unrelated NOTICE — has nothing to do with JOINs
		Message: "Exited host mode.",
	})

	cm.bannedChannelsMu.Lock()
	_, banned := cm.bannedChannels["chan_in_flight"]
	cm.bannedChannelsMu.Unlock()
	if banned {
		t.Error("non-JOIN-rejection NOTICEs must not populate the skip list")
	}

	cm.pendingJoinsMu.Lock()
	_, stillPending := cm.pendingJoins["chan_in_flight"]
	cm.pendingJoinsMu.Unlock()
	if !stillPending {
		t.Error("non-JOIN-rejection NOTICEs must not clear pendingJoins")
	}
}

func TestHandleNotice_EmptyChannel_DoesNotMutate(t *testing.T) {
	cm := newTestConnectionManagerWithMetrics()

	cm.handleNotice(twitch.NoticeMessage{
		Channel: "",
		MsgID:   twitchMsgIDBanned,
		Message: "garbled",
	})

	cm.bannedChannelsMu.Lock()
	_, hasEmpty := cm.bannedChannels[""]
	count := len(cm.bannedChannels)
	cm.bannedChannelsMu.Unlock()
	if hasEmpty || count != 0 {
		t.Errorf("a NOTICE with no parseable channel must be a no-op; got count=%d hasEmpty=%v", count, hasEmpty)
	}
}
