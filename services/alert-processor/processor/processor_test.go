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

package processor

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/caesar/all-chat/services/alert-processor/leaderboard"
	"github.com/caesar/all-chat/services/alert-processor/models"
	"github.com/caesar/all-chat/services/alert-processor/publisher"
	"github.com/caesar/all-chat/services/alert-processor/repository"
	mpmodels "github.com/caesar/all-chat/services/message-processor/models"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// fakeStore stands in for the repository: canned routing results, recorded
// inserts, and an injectable insert failure. The Postgres seam itself is
// covered in the repository package's tests.
type fakeStore struct {
	overlays  []repository.AlertOverlay
	routeErr  error
	insertErr error
	// failInsertOverlay, when set, fails the persist for that one overlay —
	// how a multi-overlay fan-out is made to fail partway through.
	failInsertOverlay string
	inserts           []*models.Alert
}

func (f *fakeStore) FindAlertOverlays(_ context.Context, _, _ string) ([]repository.AlertOverlay, error) {
	if f.routeErr != nil {
		return nil, f.routeErr
	}
	return f.overlays, nil
}

func (f *fakeStore) InsertAlertEvent(_ context.Context, alert *models.Alert) error {
	if f.insertErr != nil {
		return f.insertErr
	}
	if f.failInsertOverlay != "" && alert.OverlayID == f.failInsertOverlay {
		return errors.New("db down")
	}
	f.inserts = append(f.inserts, alert)
	return nil
}

func bitsEvent() *mpmodels.RawChatMessage {
	return &mpmodels.RawChatMessage{
		MessageID: "msg-1",
		Platform:  "twitch",
		ChannelID: "12345",
		UserID:    "u1",
		Username:  "somechatter",
		Tags:      map[string]string{"display-name": "Some Chatter"},
		Timestamp: time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC),
		EventType: "bits",
		EventData: map[string]interface{}{"badge_tier": 100},
	}
}

func newHarness(t *testing.T, store *fakeStore) (*miniredis.Miniredis, *redis.Client, *Processor) {
	t.Helper()
	mr, err := miniredis.Run()
	require.NoError(t, err)
	t.Cleanup(mr.Close)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	return mr, rdb, New(store, publisher.New(rdb), leaderboard.New(rdb), zap.NewNop())
}

// collectAlerts subscribes to each overlay's alerts channel and returns a
// channel of (overlayID, payload) deliveries.
func collectAlerts(t *testing.T, rdb *redis.Client, overlayIDs []string) <-chan map[string]string {
	t.Helper()
	subs := make([]*redis.PubSub, 0, len(overlayIDs))
	for _, id := range overlayIDs {
		sub := rdb.Subscribe(context.Background(), publisher.Channel(id))
		t.Cleanup(func() { _ = sub.Close() })
		if _, err := sub.Receive(context.Background()); err != nil {
			t.Fatalf("subscribe %s: %v", id, err)
		}
		subs = append(subs, sub)
	}

	out := make(chan map[string]string, len(overlayIDs))
	for i, id := range overlayIDs {
		sub := subs[i]
		go func(id string) {
			msg, ok := <-sub.Channel()
			if ok {
				out <- map[string]string{"overlay": id, "payload": msg.Payload}
			}
		}(id)
	}
	return out
}

// TestHandle_FansOutToEveryAlertCapableOverlay: one routed event must produce
// one alert per overlay — alerts, goal and list overlays alike ('goal'
// overlays animate on the same events; only leaderboards are list-only). Each
// alert carries its own overlay id and its own deterministic alert id.
func TestHandle_FansOutToEveryAlertCapableOverlay(t *testing.T) {
	store := &fakeStore{overlays: []repository.AlertOverlay{
		{OverlayID: "overlay-a", OverlayType: "alerts"},
		{OverlayID: "overlay-b", OverlayType: "goal"},
		{OverlayID: "overlay-c", OverlayType: "list"},
	}}
	_, rdb, p := newHarness(t, store)

	deliveries := collectAlerts(t, rdb, []string{"overlay-a", "overlay-b", "overlay-c"})
	require.NoError(t, p.Handle(context.Background(), bitsEvent()))

	require.Len(t, store.inserts, 3, "every routed overlay must get its persisted alert")
	seenIDs := map[string]bool{}
	for i, insert := range store.inserts {
		wantOverlay := []string{"overlay-a", "overlay-b", "overlay-c"}[i]
		assert.Equal(t, wantOverlay, insert.OverlayID)
		assert.Equal(t, "bits", insert.EventType)
		assert.Equal(t, "twitch", insert.Platform)
		assert.Equal(t, bitsEvent().Timestamp, insert.OccurredAt)
		assert.Equal(t, models.AlertUser{ID: "u1", Name: "Some Chatter"}, insert.User)
		require.NotNil(t, insert.EventData, "the normalized event payload must be carried")
		assert.NotEmpty(t, insert.AlertID)
		assert.False(t, seenIDs[insert.AlertID], "each overlay's alert must have its own id")
		seenIDs[insert.AlertID] = true
	}

	got := map[string]string{}
	for i := 0; i < 3; i++ {
		select {
		case d := <-deliveries:
			got[d["overlay"]] = d["payload"]
		case <-time.After(2 * time.Second):
			t.Fatal("not every alert-capable overlay received the event")
		}
	}
	assert.Len(t, got, 3, "alerts, goal and list overlays must all receive the event")
}

// TestHandle_LeaderboardOnlyForListOverlays: the running gift total feeds the
// 'list' overlay's board only — an 'alerts' overlay must not grow a board it
// never renders. Amount-less events (a 0-gift mystery gift) must not score.
func TestHandle_LeaderboardOnlyForListOverlays(t *testing.T) {
	store := &fakeStore{overlays: []repository.AlertOverlay{
		{OverlayID: "overlay-a", OverlayType: "alerts"},
		{OverlayID: "overlay-c", OverlayType: "list"},
	}}
	mr, _, p := newHarness(t, store)

	event := bitsEvent()
	event.EventType = "mystery_gift"
	event.EventData = map[string]interface{}{"gift_count": 5}

	require.NoError(t, p.Handle(context.Background(), event))

	score, err := mr.ZScore("overlay:overlay-c:leaderboard:gifts", "twitch:u1")
	require.NoError(t, err, "the list overlay's gifts board must carry the gift count")
	assert.InDelta(t, 5.0, score, 1e-9)

	assert.False(t, mr.Exists("overlay:overlay-a:leaderboard:gifts"),
		"non-list overlays must not grow leaderboards")

	// An event with no amount must not touch any board.
	event.MessageID = "msg-2"
	event.EventData = map[string]interface{}{"gift_count": 0}
	require.NoError(t, p.Handle(context.Background(), event))
	score, err = mr.ZScore("overlay:overlay-c:leaderboard:gifts", "twitch:u1")
	require.NoError(t, err)
	assert.InDelta(t, 5.0, score, 1e-9, "a zero-amount event must not change scores")
}

// TestHandle_LeaderboardScoreIsIdempotentOnRedelivery: the consumer group is
// at-least-once, so a routed alert is redelivered whenever a later step
// failed — here overlay B's persist fails after overlay A already scored.
// ZINCRBY is additive, so without an idempotency guard the redelivery would
// double-count A's amount permanently (until the board's 30-day TTL).
func TestHandle_LeaderboardScoreIsIdempotentOnRedelivery(t *testing.T) {
	store := &fakeStore{overlays: []repository.AlertOverlay{
		{OverlayID: "overlay-a", OverlayType: "list"},
		{OverlayID: "overlay-b", OverlayType: "list"},
	}}
	mr, _, p := newHarness(t, store)

	event := bitsEvent()
	event.EventType = "mystery_gift"
	event.EventData = map[string]interface{}{"gift_count": 5}

	// First delivery: overlay A scores, then overlay B's persist fails and the
	// entry stays pending for redelivery.
	store.failInsertOverlay = "overlay-b"
	assert.Error(t, p.Handle(context.Background(), event))

	// Redelivery: both overlays succeed.
	store.failInsertOverlay = ""
	require.NoError(t, p.Handle(context.Background(), event))

	for _, overlayID := range []string{"overlay-a", "overlay-b"} {
		score, err := mr.ZScore("overlay:"+overlayID+":leaderboard:gifts", "twitch:u1")
		require.NoError(t, err, "overlay %s's board must carry the gift", overlayID)
		assert.InDelta(t, 5.0, score, 1e-9,
			"overlay %s must carry the amount exactly once, not once per delivery", overlayID)
	}
}

// TestHandle_PersistFailureStopsBeforePublish: if history cannot be written,
// the alert must not be broadcast — overlays would animate on an event that
// does not exist, and the redelivery would animate them twice.
func TestHandle_PersistFailureStopsBeforePublish(t *testing.T) {
	store := &fakeStore{
		overlays:  []repository.AlertOverlay{{OverlayID: "overlay-a", OverlayType: "alerts"}},
		insertErr: errors.New("db down"),
	}
	_, rdb, p := newHarness(t, store)

	deliveries := collectAlerts(t, rdb, []string{"overlay-a"})
	err := p.Handle(context.Background(), bitsEvent())
	assert.Error(t, err, "a persist failure must propagate so the entry is redelivered")

	// A sentinel published after the failed Handle proves ordering: if the
	// alert had been published, it would arrive before the sentinel.
	require.NoError(t, rdb.Publish(context.Background(), publisher.Channel("overlay-a"), "sentinel").Err())
	select {
	case d := <-deliveries:
		assert.Equal(t, "sentinel", d["payload"], "nothing may be published before history is persisted")
	case <-time.After(2 * time.Second):
		t.Fatal("sentinel never arrived")
	}
}

// TestHandle_PublishFailurePropagates: a lost alert is never re-sent by
// anyone — only the redelivery recovers it, which requires the error to
// bubble to the consumer.
func TestHandle_PublishFailurePropagates(t *testing.T) {
	store := &fakeStore{overlays: []repository.AlertOverlay{{OverlayID: "overlay-a", OverlayType: "alerts"}}}
	mr, _, p := newHarness(t, store)
	mr.Close()

	err := p.Handle(context.Background(), bitsEvent())
	assert.Error(t, err, "a publish failure must propagate so the entry is redelivered")
	assert.Len(t, store.inserts, 1, "persist happens before publish and must have run")
}

// TestHandle_UnnormalizableEventIsDropped: normalization failures are
// deterministic (a pure function of the message), so retrying cannot help.
// The event must be dropped with a nil error — returning one would redeliver
// the same undecodable message forever, wedging the consumer group.
func TestHandle_UnnormalizableEventIsDropped(t *testing.T) {
	store := &fakeStore{overlays: []repository.AlertOverlay{{OverlayID: "overlay-a", OverlayType: "alerts"}}}
	_, _, p := newHarness(t, store)

	event := bitsEvent()
	event.Platform = "kick" // no event normalizer

	assert.NoError(t, p.Handle(context.Background(), event),
		"an event the normalizers cannot decode must be dropped, not retried")
	assert.Empty(t, store.inserts)
}

// TestHandle_NoOverlaysIsNoop: a message from a source no alert overlay
// subscribes to must be a silent pass — no writes, no publishes, no error.
func TestHandle_NoOverlaysIsNoop(t *testing.T) {
	store := &fakeStore{}
	_, _, p := newHarness(t, store)

	assert.NoError(t, p.Handle(context.Background(), bitsEvent()))
	assert.Empty(t, store.inserts)
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

// TestHandle_CountersTrackThePipeline: the routed/persisted/publish-error
// counters are the operational story of one event (persisted-before-published
// is visible as routed ≥ persisted). Dropping any Record* call keeps the
// pipeline correct while silently blinding dashboards — these assertions pin
// the wiring, not the counters themselves (metrics_test covers those).
func TestHandle_CountersTrackThePipeline(t *testing.T) {
	store := &fakeStore{overlays: []repository.AlertOverlay{
		{OverlayID: "overlay-a", OverlayType: "alerts"},
	}}
	_, rdb, p := newHarness(t, store)

	const (
		routed    = "alert_processor_events_routed_total"
		persisted = "alert_processor_events_persisted_total"
	)
	beforeRouted := counterValue(t, routed, "twitch", "bits")
	beforePersisted := counterValue(t, persisted, "twitch", "bits")
	beforePublishErrors := counterValue(t, "alert_processor_publish_errors_total", "twitch", "bits")

	deliveries := collectAlerts(t, rdb, []string{"overlay-a"})
	require.NoError(t, p.Handle(context.Background(), bitsEvent()))
	select {
	case <-deliveries:
	case <-time.After(2 * time.Second):
		t.Fatal("the alert never arrived")
	}

	assert.Equal(t, beforeRouted+1, counterValue(t, routed, "twitch", "bits"),
		"routing to an overlay must count one routed event")
	assert.Equal(t, beforePersisted+1, counterValue(t, persisted, "twitch", "bits"),
		"a successful persist must count one persisted event")
	assert.Equal(t, beforePublishErrors, counterValue(t, "alert_processor_publish_errors_total", "twitch", "bits"),
		"a successful publish must not count a publish error")
}

// TestHandle_PublishError_CountsPublishError: the publish-error counter is
// how an overlay population silently going blind becomes visible; the
// RecordPublishError call must sit on the failure path, not merely exist.
func TestHandle_PublishError_CountsPublishError(t *testing.T) {
	store := &fakeStore{overlays: []repository.AlertOverlay{
		{OverlayID: "overlay-a", OverlayType: "alerts"},
	}}
	mr, _, p := newHarness(t, store)
	mr.Close() // the publish must fail: Redis is gone

	const publishErrors = "alert_processor_publish_errors_total"
	before := counterValue(t, publishErrors, "twitch", "bits")

	assert.Error(t, p.Handle(context.Background(), bitsEvent()),
		"a publish failure must still propagate for redelivery")
	assert.Equal(t, before+1, counterValue(t, publishErrors, "twitch", "bits"),
		"a failed publish must count one publish error")
}

// TestHandle_RouteFailurePropagates: a failed routing lookup must redeliver,
// not silently fan out to zero overlays.
func TestHandle_RouteFailurePropagates(t *testing.T) {
	store := &fakeStore{routeErr: errors.New("db down")}
	_, _, p := newHarness(t, store)

	assert.Error(t, p.Handle(context.Background(), bitsEvent()))
}
