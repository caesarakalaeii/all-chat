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
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/caesar/all-chat/services/engagement-service/announcer"
	"github.com/caesar/all-chat/services/engagement-service/models"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func timer(interval, minMsg *int, minsSinceFire float64, linesAtFire, lines int64) models.ChatTimer {
	last := time.Now().UTC().Add(-time.Duration(minsSinceFire * float64(time.Minute)))
	return models.ChatTimer{IntervalMinutes: interval, MinMessages: minMsg,
		LastFiredAt: &last, LinesAtLastFire: linesAtFire, FireCount: 42}
}

// The firing decision (ADR-0081): interval and/or message count; a messages-only
// timer also needs at least one minute between fires.
func TestTimerDueTruthTable(t *testing.T) {
	cases := []struct {
		name               string
		interval, minMsg   *int
		minsSinceFire      float64
		linesAtFire, lines int64
		want               bool
	}{
		// Interval only.
		{"interval not elapsed", new(5), nil, 2, 0, 0, false},
		{"interval elapsed", new(5), nil, 5, 0, 0, true},
		// Messages only.
		{"not enough lines", nil, new(3), 5, 10, 12, false},
		{"enough lines", nil, new(3), 1, 10, 14, true},
		{"enough lines but under the 60s floor", nil, new(3), 0.5, 10, 14, false},
		{"enough lines after the 60s floor", nil, new(3), 1.01, 10, 14, true},
		{"exactly at the threshold", nil, new(3), 1, 10, 13, true},
		// Both set: interval AND lines must both be satisfied.
		{"both: interval elapsed, not enough lines", new(10), new(3), 11, 10, 12, false},
		{"both: enough lines, interval not elapsed", new(10), new(3), 3, 10, 14, false},
		{"both: both satisfied", new(10), new(3), 11, 10, 14, true},
	}
	now := time.Now().UTC().Add(10 * time.Second)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := timerDue(timer(tc.interval, tc.minMsg, tc.minsSinceFire, tc.linesAtFire, tc.lines), now, tc.lines)
			assert.Equal(t, tc.want, got)
		})
	}
}

type fakeTimerStore struct {
	timers []models.EnabledChatTimer
	claims []uuid.UUID
}

func (f *fakeTimerStore) EnabledChatTimers(context.Context) ([]models.EnabledChatTimer, error) {
	return f.timers, nil
}

func (f *fakeTimerStore) ClaimChatTimer(_ context.Context, t models.ChatTimer, _ int64) bool {
	// Mirror the repo's conditional UPDATE: with an in-memory store the first
	// claimer wins and cannot win twice (each row is claimed at most once).
	for _, id := range f.claims {
		if id == t.ID {
			return false
		}
	}
	f.claims = append(f.claims, t.ID)
	return true
}

type fakeEcho struct{ texts []string }

func (f *fakeEcho) RegisterEcho(_ context.Context, _ uuid.UUID, text string) error {
	f.texts = append(f.texts, text)
	return nil
}

type sentTimer struct{ user, text, platform string }

type fakeTimerOutput struct {
	sent []sentTimer
}

func (f *fakeTimerOutput) Clip(_ context.Context, _ string) (string, error) { return "", nil }
func (f *fakeTimerOutput) BotSend(_ context.Context, userID, message, platform string, purpose announcer.SendPurpose) error {
	if purpose != announcer.PurposeTimer {
		return assert.AnError
	}
	f.sent = append(f.sent, sentTimer{userID, message, platform})
	return nil
}

// A due timer renders (no chatter context), claims, registers its echo and posts to
// twitch+kick (youtube only for a premium owner); a timer that is not due does
// nothing, and a second claimer gets nothing.
func TestTimerSchedulerFire(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rdb.Close()

	store := &fakeTimerStore{}
	out := &fakeTimerOutput{}
	echo := &fakeEcho{}
	s := NewTimerScheduler(rdb, store, out, echo, zap.NewNop(), time.Hour)

	due := timer(new(5), nil, 10, 0, 0)
	due.ID = uuid.New()
	due.UserID = ownerID
	due.Message = "gg {count}"
	pastHold := timer(nil, new(3), 1.01, 1, 4)
	pastHold.ID = uuid.New()
	pastHold.UserID = ownerID
	pastHold.Message = "still due"
	notDue := timer(nil, new(3), 0.5, 1, 4)
	notDue.ID = uuid.New()
	notDue.UserID = ownerID
	notDue.Message = "never"
	store.timers = []models.EnabledChatTimer{
		{Timer: due, Premium: true},
		{Timer: pastHold},
		{Timer: notDue},
	}
	require.NoError(t, rdb.Set(context.Background(), chattimerLinesKey(ownerID), 4, 0).Err())

	s.fireDue(context.Background())

	// Both due timers claimed: premium owner posts to twitch+kick+youtube, the
	// non-premium one to twitch+kick only. The not-due one posts nothing.
	require.Len(t, out.sent, 5)
	assert.Equal(t, sentTimer{user: ownerID.String(), text: "gg 43", platform: "twitch"}, out.sent[0])
	assert.Equal(t, sentTimer{user: ownerID.String(), text: "gg 43", platform: "kick"}, out.sent[1])
	assert.Equal(t, sentTimer{user: ownerID.String(), text: "gg 43", platform: "youtube"}, out.sent[2])
	assert.Contains(t, out.sent, sentTimer{user: ownerID.String(), text: "still due", platform: "twitch"})
	assert.Contains(t, out.sent, sentTimer{user: ownerID.String(), text: "still due", platform: "kick"})
	// Not-due timer posts nothing with its text.
	for _, s := range out.sent {
		assert.NotEqual(t, "never", s.text)
	}
	// The echo guard is registered for every posted text, so a keyword trigger inside
	// a timer post cannot fire on its echo.
	require.Len(t, echo.texts, 2)

	// Re-fire: nothing is due any more (already claimed).
	out.sent = nil
	s.fireDue(context.Background())
	assert.Empty(t, out.sent)
}
