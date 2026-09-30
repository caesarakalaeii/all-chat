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

package discord

import (
	"context"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/caesar/all-chat/services/support-bot/access"
	"github.com/caesar/all-chat/services/support-bot/llm"
	"go.uber.org/zap"
)

// busyReply is what the user sees while their request is parked.
const busyReply = "Our provider is experiencing high traffic — your request is queued and will be answered as soon as capacity frees up."

// busyExpiryReply is sent when a parked request outlived its TTL.
const busyExpiryReply = "Sorry, the provider stayed busy for too long and I could not get your question answered. Please ask again later."

// parkedRequest is a question whose agent run hit KindBusy (the gateway pauses
// hobby traffic while production load is high). It keeps the full transcript, so
// resuming continues the same session — tool results and partial reasoning are
// not lost and side effects are never repeated.
type parkedRequest struct {
	uid       string
	channelID string
	mode      access.Mode
	messages  []llm.Message // transcript at the pause point
	enqueued  time.Time
}

// busyQueue parks requests the LLM refused with 429 and retries them on a single
// background worker, every cfg.BusyRetryDelay. A request older than
// cfg.BusyQueueTTL is dropped with busyExpiryReply instead of being retried
// forever.
type busyQueue struct {
	bot  *Bot
	mu   sync.Mutex
	q    []*parkedRequest
	wake chan struct{}
}

func newBusyQueue(bot *Bot) *busyQueue {
	return &busyQueue{bot: bot, wake: make(chan struct{}, 1)}
}

// park adds a request, wakes the worker, and tells the user their ask is queued.
// A nil session (tests) skips the notification.
func (bq *busyQueue) park(s *discordgo.Session, r *parkedRequest) {
	r.enqueued = time.Now()
	bq.mu.Lock()
	bq.q = append(bq.q, r)
	n := len(bq.q)
	bq.mu.Unlock()
	select {
	case bq.wake <- struct{}{}:
	default:
	}
	if s != nil {
		_, _ = s.ChannelMessageSend(r.channelID, busyReply)
	}
	bq.bot.log.Info("request parked: llm busy",
		zap.String("channel", r.channelID), zap.Int("queued", n))
}

// run drives the worker until ctx is cancelled. It wakes immediately when a new
// request is parked and otherwise ticks at the retry interval.
func (bq *busyQueue) run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-bq.wake:
		case <-time.After(bq.bot.cfg.BusyRetryDelay):
		}
		bq.drainOnce(ctx)
	}
}

// drainOnce retries every parked request once, in FIFO order. Still-busy ones are
// re-queued (at the back) unless their TTL expired.
func (bq *busyQueue) drainOnce(ctx context.Context) {
	bq.mu.Lock()
	pending := bq.q
	bq.q = nil
	bq.mu.Unlock()

	for _, r := range pending {
		if ctx.Err() != nil {
			bq.requeue(r)
			continue
		}
		if time.Since(r.enqueued) > bq.bot.cfg.BusyQueueTTL {
			bq.bot.resumeParked(ctx, r, true)
			continue
		}
		bq.bot.resumeParked(ctx, r, false)
	}
}

// requeue puts a request back without notifying the user.
func (bq *busyQueue) requeue(r *parkedRequest) {
	bq.mu.Lock()
	bq.q = append(bq.q, r)
	bq.mu.Unlock()
}

// snapshot returns the queued requests (test/inspection helper).
func (bq *busyQueue) snapshot() []*parkedRequest {
	bq.mu.Lock()
	defer bq.mu.Unlock()
	return bq.q
}
