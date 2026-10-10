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

package consumer

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	mpmodels "github.com/caesar/all-chat/services/message-processor/models"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

// recordingHandler records every message the consumer hands over and can fail
// on demand, standing in for the processor pipeline.
type recordingHandler struct {
	calls []*mpmodels.RawChatMessage
	err   error
}

func (h *recordingHandler) Handle(_ context.Context, raw *mpmodels.RawChatMessage) error {
	h.calls = append(h.calls, raw)
	return h.err
}

func newTestConsumer(t *testing.T, handler Handler) (*miniredis.Miniredis, *redis.Client, *Consumer, *observer.ObservedLogs) {
	t.Helper()
	mr, err := miniredis.Run()
	require.NoError(t, err)
	t.Cleanup(mr.Close)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	core, logs := observer.New(zap.WarnLevel)
	c := New(rdb, zap.New(core), handler, "test-consumer")
	c.blockFor = 10 * time.Millisecond
	return mr, rdb, c, logs
}

// createGroup puts the stream into the state a running deployment is in:
// listeners have XADDed (the stream exists) and the group has been created.
func createGroup(t *testing.T, rdb *redis.Client) {
	t.Helper()
	require.NoError(t, rdb.XGroupCreateMkStream(context.Background(), StreamKey, ConsumerGroup, "0").Err())
}

func seed(t *testing.T, rdb *redis.Client, msg *mpmodels.RawChatMessage) string {
	t.Helper()
	data, err := json.Marshal(msg)
	require.NoError(t, err)
	id, err := rdb.XAdd(context.Background(), &redis.XAddArgs{
		Stream: StreamKey,
		Values: map[string]any{"data": string(data)},
	}).Result()
	require.NoError(t, err)
	return id
}

func seedChat(text string) *mpmodels.RawChatMessage {
	return &mpmodels.RawChatMessage{
		MessageID: "msg-" + text,
		Platform:  "twitch",
		ChannelID: "12345",
		UserID:    "u1",
		Username:  "somechatter",
		Text:      text,
		Timestamp: time.Now(),
	}
}

func pendingCount(t *testing.T, rdb *redis.Client, group string) int64 {
	t.Helper()
	pending, err := rdb.XPending(context.Background(), StreamKey, group).Result()
	require.NoError(t, err)
	return pending.Count
}

// counterValue reads one labelled counter off the default registry, the way a
// Prometheus scrape would. The metrics package's counter vars are unexported,
// so a test outside it cannot call WithLabelValues; asserting on the gathered
// registry is also the stronger form — it proves the value a scrape would
// actually return, not a child a test conjured by asking for it.
func counterValue(t *testing.T, name, platform, eventType string) float64 {
	t.Helper()
	families, err := prometheus.DefaultGatherer.Gather()
	require.NoError(t, err)
	for _, family := range families {
		if family.GetName() != name {
			continue
		}
		for _, metric := range family.GetMetric() {
			var gotPlatform, gotType string
			for _, label := range metric.GetLabel() {
				switch label.GetName() {
				case "platform":
					gotPlatform = label.GetValue()
				case "event_type":
					gotType = label.GetValue()
				}
			}
			if gotPlatform == platform && gotType == eventType {
				return metric.GetCounter().GetValue()
			}
		}
	}
	return 0
}

// TestProcessMessage_CountsConsumedEntries: the consumed counter is the only
// per-event-type volume signal the service has; dropping the RecordConsumed
// call keeps the pipeline correct while dashboards show a dead consumer.
// Chat lines count too — the consumed/routed delta is exactly the chat volume
// the alert path deliberately does not touch.
func TestProcessMessage_CountsConsumedEntries(t *testing.T) {
	handler := &recordingHandler{}
	_, rdb, c, _ := newTestConsumer(t, handler)
	createGroup(t, rdb)

	event := seedChat("")
	event.MessageID = "msg-1"
	event.EventType = "bits"
	event.EventData = map[string]interface{}{"badge_tier": 100}
	seed(t, rdb, event)
	seed(t, rdb, seedChat("hello"))

	const consumed = "alert_processor_events_consumed_total"
	beforeBits := counterValue(t, consumed, "twitch", "bits")
	beforeChat := counterValue(t, consumed, "twitch", "chat")

	require.NoError(t, c.readAndProcess(context.Background()))

	assert.Equal(t, beforeBits+1, counterValue(t, consumed, "twitch", "bits"),
		"an event entry must be counted as consumed")
	assert.Equal(t, beforeChat+1, counterValue(t, consumed, "twitch", "chat"),
		"a skipped chat line is still consumed; it must count")
}

// TestReadAndProcess_SkipsChatAndDeletions: pure chat (event_type empty or
// "chat") must never enter the alert path, and neither must moderation
// deletions — they are reflect-back plumbing for the chat pipeline, not alert
// content. Skipped entries are ACKed so they never accumulate in the PEL.
func TestReadAndProcess_SkipsChatAndDeletions(t *testing.T) {
	handler := &recordingHandler{}
	_, rdb, c, _ := newTestConsumer(t, handler)
	createGroup(t, rdb)

	emptyType := seedChat("hello")
	explicitChat := seedChat("hi")
	explicitChat.MessageID = "msg-2"
	explicitChat.EventType = "chat"
	deletion := seedChat("")
	deletion.MessageID = "msg-3"
	deletion.EventType = "message_deletion"
	deletion.EventData = map[string]interface{}{"deletion_type": "clear"}

	event := seedChat("")
	event.MessageID = "msg-4"
	event.EventType = "bits"
	event.EventData = map[string]interface{}{"badge_tier": 100}

	for _, msg := range []*mpmodels.RawChatMessage{emptyType, explicitChat, deletion, event} {
		seed(t, rdb, msg)
	}

	require.NoError(t, c.readAndProcess(context.Background()))

	require.Len(t, handler.calls, 1, "only the event may reach the handler")
	assert.Equal(t, "bits", handler.calls[0].EventType)
	assert.Equal(t, int64(0), pendingCount(t, rdb, ConsumerGroup), "skipped entries must be ACKed, not left pending")
}

// TestReadAndProcess_AcksOnlyAfterHandlerSucceeds: the at-least-once contract.
// A failed alert must stay in the PEL for redelivery — ACKing it would drop it
// forever, because Pub/Sub never re-sends.
func TestReadAndProcess_AcksOnlyAfterHandlerSucceeds(t *testing.T) {
	handler := &recordingHandler{err: errors.New("publish failed")}
	_, rdb, c, _ := newTestConsumer(t, handler)
	createGroup(t, rdb)

	event := seedChat("")
	event.EventType = "bits"
	event.EventData = map[string]interface{}{"badge_tier": 100}
	seed(t, rdb, event)

	require.NoError(t, c.readAndProcess(context.Background()),
		"a handler failure is left for redelivery, not a consume-loop error")
	require.Len(t, handler.calls, 1)
	assert.Equal(t, int64(1), pendingCount(t, rdb, ConsumerGroup), "a failed alert must remain pending for redelivery")

	// The redelivery (via reclaim) must reach the handler again and, once it
	// succeeds, ACK.
	handler.err = nil
	handler.calls = nil
	c.claimMinIdle = 0
	c.reclaimStale(context.Background())
	require.Len(t, handler.calls, 1, "the pending entry must be redelivered")
	assert.Equal(t, int64(0), pendingCount(t, rdb, ConsumerGroup), "a successfully processed alert must be ACKed")
}

// TestReadAndProcess_DropsUndecodableEntries: an entry whose payload is not a
// RawChatMessage can never become one. It must be ACKed and dropped — leaving
// it unacked would redeliver it forever.
func TestReadAndProcess_DropsUndecodableEntries(t *testing.T) {
	handler := &recordingHandler{}
	_, rdb, c, _ := newTestConsumer(t, handler)
	createGroup(t, rdb)

	id, err := rdb.XAdd(context.Background(), &redis.XAddArgs{
		Stream: StreamKey,
		Values: map[string]any{"data": "{not json"},
	}).Result()
	require.NoError(t, err)
	require.NotEmpty(t, id)

	require.NoError(t, c.readAndProcess(context.Background()))
	assert.Empty(t, handler.calls)
	assert.Equal(t, int64(0), pendingCount(t, rdb, ConsumerGroup), "undecodable entries must be ACKed and dropped")
}

// TestReadAndProcess_MissingStreamBacksOff: XREADGROUP validates the
// consumer group before honoring Block, so on a fresh deployment (chat:raw
// not yet created by any listener) the read returns NOGROUP immediately.
// Without an explicit backoff the consume loop busy-spins at full CPU issuing
// XREADGROUP + XGROUP CREATE pairs until the first chat message ever
// arrives — exactly the missing-stream scenario the service must tolerate.
// The NOGROUP path must wait out the block time instead, and it must not
// create the stream on its way past.
func TestReadAndProcess_MissingStreamBacksOff(t *testing.T) {
	handler := &recordingHandler{}
	mr, _, c, _ := newTestConsumer(t, handler)
	c.blockFor = 60 * time.Millisecond

	start := time.Now()
	require.NoError(t, c.readAndProcess(context.Background()),
		"a missing stream is a wait-and-retry state, not a consume-loop error")
	assert.GreaterOrEqual(t, time.Since(start), 60*time.Millisecond,
		"the NOGROUP path must back off instead of busy-spinning")
	assert.Empty(t, handler.calls)
	assert.False(t, mr.Exists(StreamKey), "the consumer must not create chat:raw itself")

	// Stop must not hang on the backoff: the wait is interruptible.
	c.Stop()
	start = time.Now()
	require.NoError(t, c.readAndProcess(context.Background()))
	assert.Less(t, time.Since(start), 60*time.Millisecond,
		"a stopped consumer must not sit out the backoff")
}

// TestEnsureGroup_ColdStartDeliversBufferedEvents: ensureGroup creates the
// group at offset "0", not "$" — the listeners may already have buffered
// events into chat:raw before this service's first deployment, and a "$" cold
// start would silently skip every one of them. This drives the production
// creation path; the other tests create the group themselves (via
// XGroupCreateMkStream) and so never pin the offset production code uses.
func TestEnsureGroup_ColdStartDeliversBufferedEvents(t *testing.T) {
	handler := &recordingHandler{}
	_, rdb, c, _ := newTestConsumer(t, handler)

	bits := seedChat("")
	bits.MessageID = "msg-1"
	bits.EventType = "bits"
	bits.EventData = map[string]interface{}{"badge_tier": 100}
	follow := seedChat("")
	follow.MessageID = "msg-2"
	follow.EventType = "follow"
	for _, msg := range []*mpmodels.RawChatMessage{bits, follow} {
		seed(t, rdb, msg)
	}

	require.NoError(t, c.ensureGroup(context.Background()))
	require.NoError(t, c.readAndProcess(context.Background()))

	require.Len(t, handler.calls, 2,
		"events buffered before the group existed must be delivered, not skipped by a '$' cold start")
}

// TestConsumerGroup_NamedAsSpecified: the spec pins the group name to
// "alert-processors" (distinct from the message-processor's group). Every
// other test and the production code reference the constant, so a typo'd or
// renamed constant would keep the suite green while breaking the documented
// group identity — this pins the literal.
func TestConsumerGroup_NamedAsSpecified(t *testing.T) {
	assert.Equal(t, "alert-processors", ConsumerGroup)
}

// TestEnsureGroup_ToleratesMissingStream: in a fresh deployment the alert
// processor can start before any listener has XADDed to chat:raw. It must not
// create the stream itself (that is the listeners' job) and must not crash —
// the failure is logged once and the group is created lazily once the stream
// appears.
func TestEnsureGroup_ToleratesMissingStream(t *testing.T) {
	handler := &recordingHandler{}
	mr, rdb, c, logs := newTestConsumer(t, handler)

	err := c.ensureGroup(context.Background())
	assert.Error(t, err, "group creation on a missing stream must fail")
	assert.False(t, mr.Exists(StreamKey), "the consumer must not create chat:raw itself")

	err = c.ensureGroup(context.Background())
	assert.Error(t, err, "group creation keeps failing until the stream exists")
	assert.Equal(t, 1, logs.Len(), "the create-group failure must be logged once, not retried noisily")

	// A listener creates the stream with the first message; the lazy retry now
	// succeeds and is a no-op on later calls (BUSYGROUP).
	seed(t, rdb, seedChat("hello"))
	require.NoError(t, c.ensureGroup(context.Background()))
	require.NoError(t, c.ensureGroup(context.Background()))
	assert.Equal(t, 1, logs.Len(), "success after the logged failure must not add noise")
}

// TestReclaimStale_RedeliversOnlyOwnGroupPending: reclaim must move entries
// this group has not ACKed back to the handler, and must leave entries the
// other consumer group (message-processor's) still owns alone.
func TestReclaimStale_RedeliversOnlyOwnGroupPending(t *testing.T) {
	handler := &recordingHandler{}
	_, rdb, c, _ := newTestConsumer(t, handler)

	event := seedChat("")
	event.EventType = "bits"
	seed(t, rdb, event)

	createGroup(t, rdb)
	require.NoError(t, rdb.XGroupCreateMkStream(context.Background(), StreamKey, "message-processor", "0").Err())

	// The message-processor group reads the entry; our group has not seen it.
	_, err := rdb.XReadGroup(context.Background(), &redis.XReadGroupArgs{
		Group:    "message-processor",
		Consumer: "mp-pod",
		Streams:  []string{StreamKey, ">"},
		Count:    10,
	}).Result()
	require.NoError(t, err)

	// Our group reads it and the handler fails: it is ours pending now.
	handler.err = errors.New("boom")
	require.NoError(t, c.readAndProcess(context.Background()))
	assert.Equal(t, int64(1), pendingCount(t, rdb, ConsumerGroup))

	handler.err = nil
	handler.calls = nil
	c.claimMinIdle = 0
	c.reclaimStale(context.Background())
	require.Len(t, handler.calls, 1, "our stale pending entry must be reclaimed")
	assert.Equal(t, int64(0), pendingCount(t, rdb, ConsumerGroup))
	assert.Equal(t, int64(1), pendingCount(t, rdb, "message-processor"),
		"another consumer group's pending entry must be left for that group")
}
