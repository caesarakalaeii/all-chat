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

// Consumer durability tests (PR #524 review P1-1 + P2-6). The two highest-risk consumer
// behaviours — NOGROUP self-heal (P1-1) and conditional ack + PEL reclaim (H2/H3, P2-6)
// — had zero automated coverage; a refactor re-introducing unconditional XAck or dropping
// the NOGROUP branch would have shipped green. These exercise the real primitives against
// an in-process miniredis (already vendored), so they run in the normal unit suite.
package consumer

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	mpmodels "github.com/caesar/all-chat/services/message-processor/models"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func newMiniRedis(t *testing.T) (*miniredis.Miniredis, *redis.Client) {
	t.Helper()
	mr, err := miniredis.Run()
	require.NoError(t, err)
	t.Cleanup(mr.Close)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	return mr, rdb
}

func TestIsNoGroup(t *testing.T) {
	assert.False(t, isNoGroup(nil))
	assert.False(t, isNoGroup(errors.New("connection refused")))
	assert.True(t, isNoGroup(errors.New("NOGROUP No such key 's' or consumer group 'g' in XREADGROUP with GROUP option")))
}

// TestRecoverConsumerGroup proves the P1-1 self-heal: after a Redis reset drops the
// consumer group (the stream survives via XADD), a read returns a NOGROUP we detect, and
// recoverConsumerGroup recreates the group so processing resumes instead of the read loop
// spinning on NOGROUP forever while every vote/wager is silently dropped.
func TestRecoverConsumerGroup(t *testing.T) {
	_, rdb := newMiniRedis(t)
	ctx := context.Background()
	const stream, group = "engagement:commands", "engagement"

	require.NoError(t, rdb.XGroupCreateMkStream(ctx, stream, group, "$").Err())
	require.NoError(t, rdb.XAdd(ctx, &redis.XAddArgs{Stream: stream, Values: map[string]any{"n": "1"}}).Err())

	// Simulate the HA cutover / reset: the group vanishes.
	require.NoError(t, rdb.XGroupDestroy(ctx, stream, group).Err())

	// A (non-blocking) read on the dropped group is a NOGROUP that isNoGroup catches.
	_, err := rdb.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group: group, Consumer: "c", Streams: []string{stream, ">"}, Count: 1, Block: -1,
	}).Result()
	require.Error(t, err)
	assert.Truef(t, isNoGroup(err), "read on a dropped group must be a detectable NOGROUP: %v", err)

	// Recover — the group exists again and reads no longer NOGROUP.
	recoverConsumerGroup(ctx, rdb, stream, group, zap.NewNop())
	groups, err := rdb.XInfoGroups(ctx, stream).Result()
	require.NoError(t, err)
	require.Len(t, groups, 1, "the consumer group is recreated")

	_, err = rdb.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group: group, Consumer: "c", Streams: []string{stream, ">"}, Count: 1, Block: -1,
	}).Result()
	assert.Falsef(t, isNoGroup(err), "must not be NOGROUP after recovery: %v", err)
}

// TestDrainPEL_ConditionalAckAndReclaim proves H2/H3 (P2-6 a + c): drainPEL reclaims
// entries idle past MinIdle under a dead consumer, acks the ones its process handles, and
// leaves a transiently-failed entry PENDING for a later retry rather than dropping it.
func TestDrainPEL_ConditionalAckAndReclaim(t *testing.T) {
	mr, rdb := newMiniRedis(t)
	ctx := context.Background()
	const stream, group, dead, live = "s", "g", "deadpod", "livepod"

	require.NoError(t, rdb.XGroupCreateMkStream(ctx, stream, group, "0").Err())
	id1, err := rdb.XAdd(ctx, &redis.XAddArgs{Stream: stream, Values: map[string]any{"n": "1"}}).Result()
	require.NoError(t, err)
	_, err = rdb.XAdd(ctx, &redis.XAddArgs{Stream: stream, Values: map[string]any{"n": "2"}}).Result()
	require.NoError(t, err)

	// Pin the clock so delivery time is deterministic. miniredis PEL idle is measured
	// against effectiveNow() (SetTime), NOT FastForward (which only decrements TTLs).
	base := time.Now()
	mr.SetTime(base)

	// Deliver both to a now-"dead" consumer so they sit in its PEL, unacked.
	_, err = rdb.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group: group, Consumer: dead, Streams: []string{stream, ">"}, Count: 10, Block: -1,
	}).Result()
	require.NoError(t, err)

	// Advance the clock past the reclaim threshold so XAutoClaim will pick them up.
	mr.SetTime(base.Add(pelReclaimMinIdle + time.Minute))

	// process: transient failure on id1 (stays pending), success on id2 (acked).
	process := func(_ context.Context, msg redis.XMessage) error {
		if msg.ID == id1 {
			return errors.New("transient db error")
		}
		return nil
	}
	drainPEL(ctx, rdb, stream, group, live, zap.NewNop(), process)

	pending, err := rdb.XPendingExt(ctx, &redis.XPendingExtArgs{
		Stream: stream, Group: group, Start: "-", End: "+", Count: 10,
	}).Result()
	require.NoError(t, err)
	require.Len(t, pending, 1, "exactly the transiently-failed entry stays pending; the handled one is acked")
	assert.Equal(t, id1, pending[0].ID, "the still-pending entry is the one whose process errored")
	assert.Equal(t, live, pending[0].Consumer, "it was reclaimed by the live consumer for retry")
}

// TestCommandHandlePermanentlyUnprocessableAreAcked proves the ack side of the H2
// invariant (P2-6 b): a phantom entry, malformed JSON, and valid-but-not-a-command all
// return nil (→ the caller acks) and never touch the repo — so they can't wedge the
// stream. A nil repo here would panic if any of these reached a DB call.
func TestCommandHandlePermanentlyUnprocessableAreAcked(t *testing.T) {
	c := &CommandConsumer{log: zap.NewNop()} // repo/rdb deliberately nil: these paths must not use them
	ctx := context.Background()

	require.NoError(t, c.handle(ctx, redis.XMessage{ID: "1-0", Values: map[string]any{}}),
		"phantom/trimmed entry (no data field) is acked")
	require.NoError(t, c.handle(ctx, redis.XMessage{ID: "2-0",
		Values: map[string]any{mpmodels.FieldEngagementData: "{not valid json"}}),
		"malformed JSON is permanently unprocessable → acked")

	notCmd, err := json.Marshal(mpmodels.CommandJob{Text: "hello world", Platform: "twitch", ChannelID: "c", UserID: "u"})
	require.NoError(t, err)
	require.NoError(t, c.handle(ctx, redis.XMessage{ID: "3-0",
		Values: map[string]any{mpmodels.FieldEngagementData: string(notCmd)}}),
		"valid JSON that isn't a command is acked without a repo call")
}
