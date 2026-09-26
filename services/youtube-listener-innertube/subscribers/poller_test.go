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

package subscribers

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/caesar/all-chat/shared/youtubetoken"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// fakeAPI stands in for the Data API client.
type fakeAPI struct {
	subs  []Subscriber
	err   error
	calls int
	// when set, the first call returns err (401), subsequent calls return subs
	// after the poller refreshes.
	recoverAfterAuth bool
}

func (f *fakeAPI) ListRecentSubscribers(_ context.Context, _ string) ([]Subscriber, error) {
	f.calls++
	if f.err != nil && !(f.recoverAfterAuth && f.calls > 1) {
		return nil, f.err
	}
	return f.subs, nil
}

// fakeTokens stands in for the YouTube token source.
type fakeTokens struct {
	cred       *youtubetoken.YouTubeCredential
	resolveErr error
	refreshErr error
	refreshes  int
}

func (f *fakeTokens) ResolveByChannel(_ context.Context, _ string) (*youtubetoken.YouTubeCredential, error) {
	if f.resolveErr != nil {
		return nil, f.resolveErr
	}
	return f.cred, nil
}

func (f *fakeTokens) Refresh(_ context.Context, cred *youtubetoken.YouTubeCredential) error {
	f.refreshes++
	if f.refreshErr != nil {
		return f.refreshErr
	}
	cred.AccessToken = "refreshed-token"
	return nil
}

// fakeQuota records reserve/confirm/rollback.
type fakeQuota struct {
	reserveOK  bool
	reserved   int
	confirmed  int
	rolledBack int
}

func (f *fakeQuota) Reserve(_ context.Context, units int) (bool, error) {
	if !f.reserveOK {
		return false, nil
	}
	f.reserved += units
	return true, nil
}

func (f *fakeQuota) Confirm(_ context.Context, units int) error {
	f.confirmed += units
	return nil
}

func (f *fakeQuota) Rollback(_ context.Context, units int) error {
	f.rolledBack += units
	return nil
}

// fakeAnnouncer records announced events.
type fakeAnnouncer struct {
	events []Event
	err    error
}

func (f *fakeAnnouncer) AnnounceSubscriber(_ context.Context, ev Event) error {
	if f.err != nil {
		return f.err
	}
	f.events = append(f.events, ev)
	return nil
}

func newTestPoller(api SubscriberAPI, tokens TokenSource, q QuotaReserver, ann Announcer) *Poller {
	return NewPoller(
		"UC_test",
		tokens,
		api,
		q,
		ann,
		zap.NewNop(),
		NopMetrics(),
		PollerOptions{VideoID: "vid-1"},
	)
}

func liveCred() *youtubetoken.YouTubeCredential {
	return &youtubetoken.YouTubeCredential{
		AccessToken:  "live-token",
		RefreshToken: "refresh-token",
		ExpiresAt:    time.Now().Add(1 * time.Hour),
	}
}

// A new subscriber past the watermark is announced, and the watermark advances.
func TestPollOnce_AnnouncesNewSubscriber(t *testing.T) {
	older := time.Now().Add(-2 * time.Hour)
	newer := time.Now().Add(-5 * time.Minute)

	api := &fakeAPI{subs: []Subscriber{
		{SubscriptionID: "s2", Title: "NewFan", ChannelID: "UC_fan2", PublishedAt: newer},
		{SubscriptionID: "s1", Title: "OldFan", ChannelID: "UC_fan1", PublishedAt: older},
	}}
	tokens := &fakeTokens{cred: liveCred()}
	q := &fakeQuota{reserveOK: true}
	ann := &fakeAnnouncer{}

	p := newTestPoller(api, tokens, q, ann)
	p.watermark = older

	p.pollOnce(context.Background())

	require.Len(t, ann.events, 1, "only the subscriber newer than the watermark is announced")
	assert.Equal(t, "NewFan", ann.events[0].Title)
	assert.Equal(t, "UC_fan2", ann.events[0].UserID)
	assert.Equal(t, newer, p.watermark, "watermark advances to the newest publishedAt seen")
	assert.Equal(t, 1, q.reserved, "one quota unit reserved")
	assert.Equal(t, 1, q.confirmed, "quota confirmed after a successful call")
	assert.Equal(t, 0, q.rolledBack)
	assert.Equal(t, 0, p.decayIndex, "interval rearmed on activity")
}

// The second poll after an announcement must not re-announce: the watermark
// moved. This is the dedup regression test.
func TestPollOnce_DoesNotReannounce(t *testing.T) {
	at := time.Now().Add(-5 * time.Minute)
	api := &fakeAPI{subs: []Subscriber{{SubscriptionID: "s1", Title: "NewFan", ChannelID: "UC_fan1", PublishedAt: at}}}
	tokens := &fakeTokens{cred: liveCred()}
	q := &fakeQuota{reserveOK: true}
	ann := &fakeAnnouncer{}

	p := newTestPoller(api, tokens, q, ann)
	p.watermark = at.Add(-1 * time.Minute)

	p.pollOnce(context.Background())
	require.Len(t, ann.events, 1)
	p.pollOnce(context.Background())
	assert.Len(t, ann.events, 1, "the same subscriber must not be announced twice")
}

// Quiet channel decays the interval ladder; a new subscriber rearms it.
func TestDecayAndRearm(t *testing.T) {
	api := &fakeAPI{subs: nil}
	tokens := &fakeTokens{cred: liveCred()}
	q := &fakeQuota{reserveOK: true}
	ann := &fakeAnnouncer{}

	p := newTestPoller(api, tokens, q, ann)

	for i := 1; i < len(PollIntervalDecay); i++ {
		p.pollOnce(context.Background())
		assert.Equal(t, i, p.decayIndex, "decay rung %d", i)
	}
	// Cap: further quiet polls do not run off the ladder.
	p.pollOnce(context.Background())
	assert.Equal(t, len(PollIntervalDecay)-1, p.decayIndex, "decay caps at the last rung")

	// New subscriber rearms to rung 0.
	api.subs = []Subscriber{{SubscriptionID: "s9", Title: "WakeUp", ChannelID: "UC_f9", PublishedAt: time.Now()}}
	p.pollOnce(context.Background())
	assert.Equal(t, 0, p.decayIndex, "activity rearms the interval")
}

// A 401 mid-poll triggers one refresh and one retry; the retry's result is
// processed normally.
func TestPollOnce_401RefreshesAndRetries(t *testing.T) {
	at := time.Now().Add(-1 * time.Minute)
	api := &fakeAPI{
		err:              ErrUnauthorized,
		recoverAfterAuth: true,
		subs:             []Subscriber{{SubscriptionID: "s1", Title: "FreshFan", ChannelID: "UC_f1", PublishedAt: at}},
	}
	tokens := &fakeTokens{cred: liveCred()}
	q := &fakeQuota{reserveOK: true}
	ann := &fakeAnnouncer{}

	p := newTestPoller(api, tokens, q, ann)

	p.pollOnce(context.Background())

	assert.Equal(t, 1, tokens.refreshes, "a 401 triggers exactly one refresh")
	require.Len(t, ann.events, 1, "the retried poll's subscriber is announced")
	assert.Equal(t, "FreshFan", ann.events[0].Title)
}

// An invalid_grant during that refresh kills polling for the stream —
// token-refresh-service's permanently-failed marking owns the recovery.
func TestPollOnce_InvalidGrantDisablesPolling(t *testing.T) {
	api := &fakeAPI{err: ErrUnauthorized}
	tokens := &fakeTokens{cred: liveCred(), refreshErr: errors.New("invalid_grant: Token has been expired or revoked")}
	q := &fakeQuota{reserveOK: true}
	ann := &fakeAnnouncer{}

	p := newTestPoller(api, tokens, q, ann)

	p.pollOnce(context.Background())
	assert.True(t, p.tokenDead, "an invalid_grant marks the token dead")

	callsBefore := api.calls
	p.pollOnce(context.Background())
	assert.Equal(t, callsBefore, api.calls, "a dead token short-circuits further polls")
	assert.Empty(t, ann.events)
}

// No linked credential row: one log, dead for the stream, no DB hammering.
func TestPollOnce_NoCredentialDisablesPolling(t *testing.T) {
	api := &fakeAPI{}
	tokens := &fakeTokens{resolveErr: youtubetoken.ErrNoCredential}
	q := &fakeQuota{reserveOK: true}
	ann := &fakeAnnouncer{}

	p := newTestPoller(api, tokens, q, ann)

	p.pollOnce(context.Background())
	assert.True(t, p.tokenDead, "ErrNoCredential marks the token dead")
	assert.Equal(t, 0, api.calls, "no API call is made without a credential")
	assert.Equal(t, 0, q.reserved, "no quota is spent")
}

// Quota exhausted: the poll is skipped, no error state, nothing announced.
func TestPollOnce_QuotaExhaustedSkips(t *testing.T) {
	api := &fakeAPI{subs: []Subscriber{{SubscriptionID: "s1", Title: "Fan", ChannelID: "UC_f1", PublishedAt: time.Now()}}}
	tokens := &fakeTokens{cred: liveCred()}
	q := &fakeQuota{reserveOK: false}
	ann := &fakeAnnouncer{}

	p := newTestPoller(api, tokens, q, ann)
	p.pollOnce(context.Background())

	assert.Equal(t, 0, api.calls, "no API call without a reservation")
	assert.Empty(t, ann.events)
	assert.Equal(t, 0, p.decayIndex, "a quota skip neither decays nor rearms")
}

// A non-401 API error rolls the reservation back and decays.
func TestPollOnce_APIErrorRollsBack(t *testing.T) {
	api := &fakeAPI{err: errors.New("subscribers: http 500: boom")}
	tokens := &fakeTokens{cred: liveCred()}
	q := &fakeQuota{reserveOK: true}
	ann := &fakeAnnouncer{}

	p := newTestPoller(api, tokens, q, ann)
	p.pollOnce(context.Background())

	assert.Equal(t, 1, q.rolledBack, "a failed call rolls the reservation back")
	assert.Equal(t, 0, q.confirmed)
	assert.Equal(t, 1, p.decayIndex, "an errored poll still decays")
	assert.Empty(t, ann.events)
}

// The event message carries the full chat:raw subscriber contract.
func TestBuildEventMessage_Contract(t *testing.T) {
	at := time.Now()
	ev := Event{
		ChannelID: "UC_chan",
		StreamID:  "vid-9",
		Title:     "CoolViewer",
		UserID:    "UC_cool",
		AvatarURL: "https://example.com/a.png",
		SubAt:     at,
	}
	now := time.Now()

	msg := BuildEventMessage(ev, now)

	assert.Equal(t, "youtube", msg.Platform)
	assert.Equal(t, "subscriber", msg.EventType)
	assert.Equal(t, "CoolViewer", msg.Username)
	assert.Equal(t, "CoolViewer just subscribed", msg.Text)
	assert.Equal(t, "CoolViewer", msg.Tags["display_name"])
	assert.Equal(t, "https://example.com/a.png", msg.Tags["profile_image"])
	assert.Equal(t, "CoolViewer", msg.EventData["subscriber_title"])
	assert.Equal(t, "UC_cool", msg.EventData["subscriber_channel_id"])
	assert.Equal(t, "https://example.com/a.png", msg.EventData["subscriber_avatar_url"])
	assert.NotEmpty(t, msg.MessageID)
}

// Missing title degrades to "Someone", never an empty announcement.
func TestBuildEventMessage_FallbackTitle(t *testing.T) {
	ev := Event{ChannelID: "UC_chan", UserID: "UC_x", SubAt: time.Now()}
	msg := BuildEventMessage(ev, time.Now())
	assert.Equal(t, "Someone", msg.Username)
	assert.Equal(t, "Someone just subscribed", msg.Text)
	assert.Equal(t, "Someone", msg.EventData["subscriber_title"])
}
