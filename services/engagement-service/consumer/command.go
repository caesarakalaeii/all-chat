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

// Package consumer holds the engagement service's Redis consumers: a durable
// command stream (chat votes/wagers) and a best-effort earning Pub/Sub.
package consumer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/caesar/all-chat/services/engagement-service/publisher"
	"github.com/caesar/all-chat/services/engagement-service/repository"
	mpmodels "github.com/caesar/all-chat/services/message-processor/models"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

const consumerGroup = "engagement"

// handleTimeout bounds each per-message handle so a stuck row lock (Wager's SELECT
// … FOR UPDATE) or a slow query can't serialize the 64-message batch or wedge a
// consumer goroutine forever. Exceeding it returns a ctx error, which the loop
// treats as transient (leaves the entry pending for redelivery — safe, writes
// dedupe per (round, viewer)). Shared by the command and native consumers.
const handleTimeout = 5 * time.Second

// CommandConsumer drains engagement:commands, resolves the durable viewer, and
// applies votes/wagers to the overlay's live poll/prediction. Confirmations are
// intentionally silent on the platform (send is opt-in); feedback reaches the
// viewer via the broadcast tally and the web page's pull endpoint.
type CommandConsumer struct {
	rdb          *redis.Client
	repo         *repository.Repository
	pub          *publisher.Publisher
	log          *zap.Logger
	consumerName string
}

// NewCommandConsumer creates a CommandConsumer. consumerName should be unique per
// pod (e.g. hostname) so pending-entry ownership is per-instance.
func NewCommandConsumer(rdb *redis.Client, repo *repository.Repository, pub *publisher.Publisher, consumerName string, log *zap.Logger) *CommandConsumer {
	return &CommandConsumer{rdb: rdb, repo: repo, pub: pub, log: log, consumerName: consumerName}
}

// Run blocks consuming the command stream until ctx is cancelled.
func (c *CommandConsumer) Run(ctx context.Context) {
	// Create the group at the stream tail (MKSTREAM); ignore BUSYGROUP.
	if err := c.rdb.XGroupCreateMkStream(ctx, mpmodels.StreamEngagementCommands, consumerGroup, "$").Err(); err != nil &&
		!strings.Contains(err.Error(), "BUSYGROUP") {
		c.log.Warn("create engagement command group", zap.Error(err))
	}
	c.log.Info("engagement command consumer started", zap.String("consumer", c.consumerName))

	// Reclaim entries orphaned by a crashed/rescheduled pod (this consumer name is
	// per-pod), then keep sweeping so votes/wagers are never stranded (H3). The periodic
	// drain is tracked so Run doesn't return until it has stopped — the caller waits on Run
	// before closing the DB/Redis clients, so no drain is mid-XAutoClaim at close (P3-13).
	drainPEL(ctx, c.rdb, mpmodels.StreamEngagementCommands, consumerGroup, c.consumerName, c.log, c.safeHandle)
	var drainWG sync.WaitGroup
	drainWG.Add(1)
	go func() {
		defer drainWG.Done()
		periodicDrain(ctx, c.rdb, mpmodels.StreamEngagementCommands, consumerGroup, c.consumerName, c.log, c.safeHandle)
	}()
	defer drainWG.Wait()

	for {
		if ctx.Err() != nil {
			return
		}
		res, err := c.rdb.XReadGroup(ctx, &redis.XReadGroupArgs{
			Group:    consumerGroup,
			Consumer: c.consumerName,
			Streams:  []string{mpmodels.StreamEngagementCommands, ">"},
			Count:    64,
			Block:    readBlockTime,
		}).Result()
		if err != nil {
			if ctx.Err() != nil || errors.Is(err, redis.Nil) {
				continue
			}
			if isNoGroup(err) {
				// A Redis reset dropped the group; recreate it so votes/wagers aren't
				// silently dropped forever (P1-1). The next periodicDrain re-arms once
				// the group exists again.
				recoverConsumerGroup(ctx, c.rdb, mpmodels.StreamEngagementCommands, consumerGroup, c.log)
				continue
			}
			c.log.Warn("xreadgroup engagement commands", zap.Error(err))
			continue
		}
		for _, stream := range res {
			for _, msg := range stream.Messages {
				// Ack only when the message is fully handled or is permanently
				// unprocessable. A transient DB failure returns an error and leaves the
				// entry pending so a later read/drain retries it (H2) — RecordVote/Wager
				// dedupe per (round, viewer), so redelivery can't double-count.
				if err := c.safeHandle(ctx, msg); err != nil {
					c.log.Warn("engagement command deferred for retry", zap.String("id", msg.ID), zap.Error(err))
					continue
				}
				if err := c.rdb.XAck(ctx, mpmodels.StreamEngagementCommands, consumerGroup, msg.ID).Err(); err != nil {
					c.log.Warn("xack engagement command", zap.String("id", msg.ID), zap.Error(err))
				}
			}
		}
	}
}

// safeHandle runs handle under a recover so one poison message can't kill the
// consumer goroutine (they run unsupervised — a panic would silently stop consuming
// for the pod's life). A recovered panic is treated as permanent (returns nil → ack):
// it would recur on every redelivery, so leaving it pending would poison the PEL.
func (c *CommandConsumer) safeHandle(ctx context.Context, msg redis.XMessage) (err error) {
	defer func() {
		if r := recover(); r != nil {
			c.log.Error("panic handling engagement command", zap.Any("panic", r), zap.String("id", msg.ID))
			err = nil
		}
	}()
	// Per-message deadline: covers both the live loop and the PEL-drain path (both
	// call safeHandle). defer cancel() is scoped to this call, not a loop, so it
	// does not leak.
	hctx, cancel := context.WithTimeout(ctx, handleTimeout)
	defer cancel()
	return c.handle(hctx, msg)
}

// handle applies a chat command. It returns nil when the message is fully handled
// or is permanently unprocessable (so the caller acks), and a non-nil error only on
// a transient DB/tx failure (so the caller leaves it pending for retry, H2). Business
// rejections (closed round, bad option, insufficient, already-wagered, native) are
// NOT errors — the chat path must never wedge the stream on an ordinary user mistake.
func (c *CommandConsumer) handle(ctx context.Context, msg redis.XMessage) error {
	raw, ok := msg.Values[mpmodels.FieldEngagementData].(string)
	if !ok {
		return nil // phantom/trimmed entry — nothing to do
	}
	var job mpmodels.CommandJob
	if err := json.Unmarshal([]byte(raw), &job); err != nil {
		c.log.Warn("unmarshal command job", zap.Error(err))
		return nil // malformed JSON never parses — permanent, ack
	}

	kind, idx, amount, ok := parseCommand(job.Text)
	if !ok {
		return nil // not a command
	}

	overlays, err := c.repo.OverlaysForChannel(ctx, job.Platform, job.ChannelID)
	if err != nil {
		return fmt.Errorf("overlays for channel: %w", err) // transient DB error → retry
	}
	if len(overlays) == 0 {
		return nil // no overlay sources this channel — nothing to do
	}
	viewerID, err := c.repo.GetOrCreateViewerByPlatform(ctx, job.Platform, job.UserID)
	if err != nil {
		return fmt.Errorf("resolve viewer for command: %w", err) // transient → retry
	}
	srcMsgID := parseUUIDOrNil(job.MessageID)

	// Accumulate transient errors across overlays: a DB failure on one overlay must
	// leave the entry pending, but a business rejection or no-active-round on another
	// must not. Redelivery is safe because the writes dedupe per (round, viewer) and
	// the replay index is scoped per round (created directly in migrations 069/070), so
	// re-running an already-applied overlay is a no-op rather than a poison unique_violation.
	var retryErr error
	for _, overlayID := range overlays {
		switch kind {
		case cmdVote:
			poll, err := c.repo.GetActivePoll(ctx, overlayID)
			if err != nil {
				if !errors.Is(err, repository.ErrNotFound) {
					retryErr = errors.Join(retryErr, fmt.Errorf("get active poll: %w", err))
				}
				continue // no active poll on this overlay
			}
			// msg.ID's epoch-ms is the monotonic ordering token: a 5m-drained redelivery
			// of an older vote can't revert a newer vote change (P3-3).
			accepted, err := c.repo.RecordVote(ctx, poll.ID, viewerID, overlayID, idx, job.Platform, srcMsgID, streamEntryMillis(msg.ID))
			if err != nil {
				retryErr = errors.Join(retryErr, fmt.Errorf("record chat vote: %w", err))
				continue
			}
			if accepted {
				if updated, err := c.repo.GetPoll(ctx, poll.ID); err == nil {
					c.pub.PublishPoll(ctx, updated)
				}
			}
		case cmdWager:
			pred, err := c.repo.GetActivePrediction(ctx, overlayID)
			if err != nil {
				if !errors.Is(err, repository.ErrNotFound) {
					retryErr = errors.Join(retryErr, fmt.Errorf("get active prediction: %w", err))
				}
				continue
			}
			// msg.ID (the Redis stream entry id) is the round-independent replay token:
			// stable across redelivery/reclaim, so a redelivered wager can't double-debit
			// even if a new round has since opened on this overlay (P2-1).
			res, err := c.repo.Wager(ctx, pred.ID, viewerID, overlayID, idx, amount, job.Platform, srcMsgID, msg.ID)
			if err != nil {
				retryErr = errors.Join(retryErr, fmt.Errorf("record chat wager: %w", err))
				continue
			}
			if res.Accepted {
				if updated, err := c.repo.GetPrediction(ctx, pred.ID); err == nil {
					c.pub.PublishPrediction(ctx, updated)
				}
			} else if res.Reason != "" {
				// Not an error (typo'd amount, already bet, broke, or a mirrored Twitch
				// round). Debug-log so operators can see why chat bets aren't landing
				// without spamming Warn at chat volume (L-U1).
				c.log.Debug("chat wager rejected",
					zap.String("reason", res.Reason), zap.String("platform", job.Platform),
					zap.String("user", job.UserID), zap.Int64("amount", amount))
			}
		}
	}
	return retryErr
}

type cmdKind int

const (
	cmdNone cmdKind = iota
	cmdVote
	cmdWager
)

// parseCommand extracts a vote or wager from a chat message. Grammar:
//   - "!vote N" / "!v N"                → vote for option N
//   - "!predict N amount" / "!bet N …"  → wager `amount` on outcome N
//   - a bare single integer "N"         → vote for option N (low-friction shortcut)
//
// The bare-number shortcut only fires when the whole trimmed message is one small
// integer, so ordinary chat that merely contains a number is not a vote. The
// consumer additionally requires an active poll before treating it as one.
func parseCommand(text string) (kind cmdKind, idx int, amount int64, ok bool) {
	fields := strings.Fields(strings.TrimSpace(text))
	if len(fields) == 0 {
		return cmdNone, 0, 0, false
	}

	if strings.HasPrefix(fields[0], "!") {
		switch strings.ToLower(strings.TrimPrefix(fields[0], "!")) {
		case "vote", "v":
			if len(fields) >= 2 && digitsOnly(fields[1]) {
				if n, err := strconv.Atoi(fields[1]); err == nil && n >= 1 {
					return cmdVote, n, 0, true
				}
			}
		case "predict", "bet", "p":
			if len(fields) >= 3 && digitsOnly(fields[1]) && digitsOnly(fields[2]) {
				n, err1 := strconv.Atoi(fields[1])
				amt, err2 := strconv.ParseInt(fields[2], 10, 64)
				if err1 == nil && err2 == nil && n >= 1 && amt > 0 {
					return cmdWager, n, amt, true
				}
			}
		}
		return cmdNone, 0, 0, false
	}

	// Bare single integer → poll vote shortcut.
	if len(fields) == 1 && digitsOnly(fields[0]) {
		if n, err := strconv.Atoi(fields[0]); err == nil && n >= 1 && n <= 99 {
			return cmdVote, n, 0, true
		}
	}
	return cmdNone, 0, 0, false
}

// digitsOnly reports whether s is a non-empty run of ASCII digits. strconv.Atoi /
// ParseInt accept a leading '+'/'-', so "+1" would parse as option 1 — but "+1" is one
// of the most common chat-agreement idioms, and counting it (or "-1", "+2") as a vote /
// wager option / amount silently mis-tallies a poll (P2-5). Requiring ASCII digits keeps
// "+1"/"-2" as ordinary chat while a bare "1" still votes.
func digitsOnly(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

// parseUUIDOrNil returns a *uuid.UUID if s is a valid UUID (e.g. a Twitch message
// id), else nil so the source_message_id replay-dedup guard is simply skipped on
// platforms whose message ids aren't UUIDs (the per-viewer PK still dedupes).
func parseUUIDOrNil(s string) *uuid.UUID {
	id, err := uuid.Parse(s)
	if err != nil {
		return nil
	}
	return &id
}

// streamEntryMillis extracts the epoch-millisecond component of a Redis stream entry id
// ("<ms>-<seq>") as a monotonic per-message ordering token for the poll-vote seq guard
// (P3-3). Returns 0 on a malformed id, which sorts oldest so it never wins a change guard.
func streamEntryMillis(id string) int64 {
	ms := id
	if i := strings.IndexByte(id, '-'); i >= 0 {
		ms = id[:i]
	}
	n, err := strconv.ParseInt(ms, 10, 64)
	if err != nil {
		return 0
	}
	return n
}
