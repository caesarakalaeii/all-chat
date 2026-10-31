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
	"github.com/caesar/all-chat/services/engagement-service/announcer"
	"github.com/caesar/all-chat/services/engagement-service/chatcmd"
	"github.com/caesar/all-chat/services/engagement-service/models"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"time"
)

// minuteFloor is how long a messages-only timer waits between fires, so a busy
// chat cannot turn a 3-message minimum into a spam flood.
const minuteFloor = time.Minute

// timerStore is the slice of the repository the scheduler needs.
type timerStore interface {
	EnabledChatTimers(ctx context.Context) ([]models.EnabledChatTimer, error)
	ClaimChatTimer(ctx context.Context, t models.ChatTimer, lines int64) bool
}

// timerEcho bridges a timer post to the chat-command consumer's echo guard so a
// keyword trigger inside a timer's own message cannot fire on its echo. The
// ChatCommandConsumer implements it.
type timerEcho interface {
	RegisterEcho(ctx context.Context, owner uuid.UUID, text string) error
}

// TimerScheduler fires streamer-defined chat timers (ADR-0081). It runs on every
// replica rather than behind a leader: the claim is a conditional UPDATE on the
// last_fired_at the firing decision saw, so of N replicas ticking the same timer
// exactly one wins and N-1 claims return nothing.
type TimerScheduler struct {
	rdb   *redis.Client
	store timerStore
	out   chatOutput
	echo  timerEcho
	log   *zap.Logger
	tick  time.Duration
}

// NewTimerScheduler creates the scheduler Run under bgCtx. tick is 15s in
// production; it is a parameter so a test can shrink it.
func NewTimerScheduler(rdb *redis.Client, store timerStore, out chatOutput, echo timerEcho, log *zap.Logger, tick time.Duration) *TimerScheduler {
	return &TimerScheduler{rdb: rdb, store: store, out: out, echo: echo, log: log, tick: tick}
}

// timerDue is the firing decision, as a pure function. The trigger is "interval
// elapsed since last fire" and/or "min_messages lines arrived since last fire".
//   - interval only: the interval elapsed.
//   - messages only: enough lines arrived, AND at least minuteFloor elapsed —
//     one burst must not fire a low minimum repeatedly within seconds.
//   - both: the interval elapsed AND enough lines arrived (the product decision:
//     every X minutes, but only when at least Y messages arrived since last fire).
func timerDue(t models.ChatTimer, now time.Time, lines int64) bool {
	last := time.Time{}
	if t.LastFiredAt != nil {
		last = *t.LastFiredAt
	}
	elapsed := now.Sub(last)
	intervalElapsed := t.IntervalMinutes != nil && elapsed >= time.Duration(*t.IntervalMinutes)*time.Minute
	linesEnough := linesEnvelope(t, lines)
	if t.IntervalMinutes != nil {
		return intervalElapsed && linesEnough
	}
	return linesEnough && elapsed >= minuteFloor
}

// linesEnvelope is the message-count half of the decision: satisfied when the timer
// has no minimum, else when enough lines arrived since the last fire.
func linesEnvelope(t models.ChatTimer, lines int64) bool {
	return t.MinMessages == nil || lines-t.LinesAtLastFire >= int64(*t.MinMessages)
}

// chatTimerPlatforms lists what a timer post asks for. Twitch and Kick always run;
// YouTube spends Data API quota per send, so only a premium owner's timer asks it.
// auth-service skips platforms without a sendable streamer identity, like announce.
func chatTimerPlatforms(premium bool) []string {
	if premium {
		return []string{"twitch", "kick", "youtube"}
	}
	return []string{"twitch", "kick"}
}

// Run ticks every interval until ctx ends.
func (s *TimerScheduler) Run(ctx context.Context) {
	ticker := time.NewTicker(s.tick)
	defer ticker.Stop()
	for {
		s.fireDue(ctx)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// fireDue runs one tick: load the enabled timers, fire those whose decision says
// due, claiming each against the DB so a fleet of replicas fires exactly once.
func (s *TimerScheduler) fireDue(ctx context.Context) {
	timers, err := s.store.EnabledChatTimers(ctx)
	if err != nil {
		if ctx.Err() == nil {
			s.log.Warn("load enabled chat timers", zap.Error(err))
		}
		return
	}
	now := time.Now().UTC()
	for _, et := range timers {
		lines, _ := s.rdb.Get(ctx, chattimerLinesKey(et.Timer.UserID)).Int64()
		if !timerDue(et.Timer, now, lines) {
			continue
		}
		if !s.store.ClaimChatTimer(ctx, et.Timer, lines) {
			continue // another replica claimed it this tick
		}
		s.sendTimer(ctx, et, et.Timer.FireCount+1)
	}
}

// sendTimer renders the claimed timer and posts it. The template renders with no
// chatter context; {count} is the timer's fire count (this fire included), so a
// streamer can show how many times it has run.
func (s *TimerScheduler) sendTimer(ctx context.Context, et models.EnabledChatTimer, count int64) {
	t := et.Timer
	vars := chatcmd.Vars{Count: count, Platform: "twitch", Role: "everyone",
		Fetch: chatcmd.HTTPFetch(ctx, chatcmd.FetchClient())}
	rendered, _ := chatcmd.Render(t.Message, vars)
	text := chatcmd.Line(rendered)
	if text == "" {
		return
	}
	// Like a command reply, register the text under the echo guard so a keyword
	// trigger inside a timer's own message cannot fire on its echo.
	if s.echo != nil {
		if err := s.echo.RegisterEcho(ctx, t.UserID, text); err != nil {
			s.log.Warn("register chat timer echo", zap.String("owner", t.UserID.String()), zap.Error(err))
		}
	}
	for _, p := range chatTimerPlatforms(et.Premium) {
		if err := s.out.BotSend(ctx, t.UserID.String(), text, p, announcer.PurposeTimer); err != nil {
			s.log.Info("chat timer send failed", zap.String("owner", t.UserID.String()),
				zap.String("platform", p), zap.Error(err))
		}
	}
}
