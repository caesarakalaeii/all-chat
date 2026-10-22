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
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/caesar/all-chat/services/engagement-service/announcer"
	"github.com/caesar/all-chat/services/engagement-service/chatcmd"
	"github.com/caesar/all-chat/services/engagement-service/models"
	"github.com/caesar/all-chat/services/engagement-service/repository"
	mpmodels "github.com/caesar/all-chat/services/message-processor/models"
	"github.com/caesar/all-chat/shared/alerts"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

const (
	chatCmdGroup = "chatcmd"
	// chatCmdWorkers bounds concurrent command runs. A run waits on auth-service (clip,
	// send), so one slow platform must not hold up every other channel's commands.
	chatCmdWorkers = 8
	chatCmdTimeout = 15 * time.Second
	// chatCmdCacheTTL is how stale a channel's command list may be. Every message of a
	// command channel is forwarded, so this keeps the DB at one query per channel per TTL.
	chatCmdCacheTTL = 5 * time.Second
	// echoTTL covers the round trip of a reply through the platform and back into chat.
	echoTTL = 60 * time.Second
	// seenTTL outlives the spread of one message's per-overlay copies (Kick).
	seenTTL = 5 * time.Minute

	clipFailedText = "(clip failed)"
)

type chatCmdStore interface {
	ChatCommandsForChannel(ctx context.Context, platform, channelID string) ([]repository.ChannelCommands, error)
	IncrementChatCommandUse(ctx context.Context, id uuid.UUID) (int64, error)
	OwnerStatus(ctx context.Context, userID uuid.UUID) (repository.ChatCommandOwnerStatus, error)
	AlertsOverlays(ctx context.Context, userID uuid.UUID) ([]uuid.UUID, error)
}

// chatOutput is auth-service's send and clip paths (announcer.Announcer).
type chatOutput interface {
	Send(ctx context.Context, userID, message, platform string) error
	Clip(ctx context.Context, userID string) (string, error)
}

type cachedCommands struct {
	at     time.Time
	owners []repository.ChannelCommands
}

// ChatCommandConsumer runs streamer-defined chat commands (ADR-0079) for the messages
// message-processor forwards on chatcmd:jobs. Delivery is at most once (NOACK reads):
// a lost command is a missed reply, while a redelivered one would answer twice.
type ChatCommandConsumer struct {
	rdb          *redis.Client
	store        chatCmdStore
	out          chatOutput
	fetch        *http.Client // {urlfetch}
	log          *zap.Logger
	consumerName string

	mu    sync.Mutex
	cache map[string]cachedCommands
}

// NewChatCommandConsumer creates the consumer. consumerName should be unique per pod.
func NewChatCommandConsumer(rdb *redis.Client, store chatCmdStore, out chatOutput, consumerName string, log *zap.Logger) *ChatCommandConsumer {
	return &ChatCommandConsumer{rdb: rdb, store: store, out: out, fetch: chatcmd.FetchClient(), log: log,
		consumerName: consumerName, cache: map[string]cachedCommands{}}
}

// Run blocks consuming chatcmd:jobs until ctx is cancelled, then waits for running
// commands to finish.
func (c *ChatCommandConsumer) Run(ctx context.Context) {
	if err := c.rdb.XGroupCreateMkStream(ctx, mpmodels.StreamChatCommands, chatCmdGroup, "$").Err(); err != nil &&
		!strings.Contains(err.Error(), "BUSYGROUP") {
		c.log.Warn("create chat command group", zap.Error(err))
	}
	c.log.Info("chat command consumer started", zap.String("consumer", c.consumerName))

	var wg sync.WaitGroup
	defer wg.Wait()
	sem := make(chan struct{}, chatCmdWorkers)
	for ctx.Err() == nil {
		res, err := c.rdb.XReadGroup(ctx, &redis.XReadGroupArgs{
			Group:    chatCmdGroup,
			Consumer: c.consumerName,
			Streams:  []string{mpmodels.StreamChatCommands, ">"},
			Count:    64,
			Block:    readBlockTime,
			NoAck:    true,
		}).Result()
		if err != nil {
			if ctx.Err() != nil || errors.Is(err, redis.Nil) {
				continue
			}
			if isNoGroup(err) {
				recoverConsumerGroup(ctx, c.rdb, mpmodels.StreamChatCommands, chatCmdGroup, c.log)
				continue
			}
			c.log.Warn("xreadgroup chat commands", zap.Error(err))
			time.Sleep(time.Second)
			continue
		}
		for _, stream := range res {
			for _, msg := range stream.Messages {
				raw, _ := msg.Values[mpmodels.FieldEngagementData].(string)
				var job mpmodels.CommandJob
				if err := json.Unmarshal([]byte(raw), &job); err != nil {
					continue
				}
				sem <- struct{}{}
				wg.Add(1)
				go func() {
					defer func() {
						if r := recover(); r != nil {
							c.log.Error("panic running chat command", zap.Any("panic", r))
						}
						<-sem
						wg.Done()
					}()
					// Detached from ctx so a shutdown lets a started reply finish.
					hctx, cancel := context.WithTimeout(context.Background(), chatCmdTimeout)
					defer cancel()
					c.handle(hctx, job)
				}()
			}
		}
	}
}

// handle fires, for every streamer whose overlays carry the message's channel, the
// first of their commands the message matches.
func (c *ChatCommandConsumer) handle(ctx context.Context, job mpmodels.CommandJob) {
	owners, err := c.commandsFor(ctx, job.Platform, job.ChannelID)
	if err != nil {
		c.log.Warn("load chat commands", zap.String("platform", job.Platform), zap.Error(err))
		return
	}
	role := chatcmd.HighestRole(job.Roles)
	for _, o := range owners {
		m, ok := chatcmd.Match(o.Commands, job.Text)
		if !ok {
			continue
		}
		if min, _ := chatcmd.ParseRole(m.Command.MinRole); !role.Allows(min) {
			continue
		}
		if !c.claim(ctx, o.UserID, m.Command.ID, m.Command.CooldownSeconds, job) {
			continue
		}
		c.run(ctx, o, m, job, role)
	}
}

func (c *ChatCommandConsumer) commandsFor(ctx context.Context, platform, channelID string) ([]repository.ChannelCommands, error) {
	key := mpmodels.ChatCommandChannelMember(platform, channelID)
	c.mu.Lock()
	hit, ok := c.cache[key]
	c.mu.Unlock()
	if ok && time.Since(hit.at) < chatCmdCacheTTL {
		return hit.owners, nil
	}
	owners, err := c.store.ChatCommandsForChannel(ctx, platform, channelID)
	if err != nil {
		return nil, err
	}
	c.mu.Lock()
	// ponytail: entries are overwritten, never evicted; bounded by the chat-command
	// channel count. Sweep stale keys here if that set ever grows large.
	c.cache[key] = cachedCommands{at: time.Now(), owners: owners}
	c.mu.Unlock()
	return owners, nil
}

// claim decides, in Redis so every replica agrees, whether this message may run the
// command: not an echo of the streamer's own reply, not a second copy of a message
// already handled, and not inside the command's cooldown.
func (c *ChatCommandConsumer) claim(ctx context.Context, owner, cmdID uuid.UUID, cooldown int, job mpmodels.CommandJob) bool {
	if n, err := c.rdb.Exists(ctx, echoKey(owner, job.Text)).Result(); err != nil || n > 0 {
		return false
	}
	if job.MessageID != "" {
		key := "chatcmd:seen:" + owner.String() + ":" + job.Platform + ":" + job.MessageID
		if ok, err := c.rdb.SetNX(ctx, key, 1, seenTTL).Result(); err != nil || !ok {
			return false
		}
	}
	if cooldown > 0 {
		ok, err := c.rdb.SetNX(ctx, "chatcmd:cd:"+cmdID.String(), 1, time.Duration(cooldown)*time.Second).Result()
		if err != nil || !ok {
			return false
		}
	}
	return true
}

func (c *ChatCommandConsumer) run(ctx context.Context, o repository.ChannelCommands, m chatcmd.Matched, job mpmodels.CommandJob, role chatcmd.Role) {
	cmd := m.Command
	owner := o.UserID.String()
	count, err := c.store.IncrementChatCommandUse(ctx, cmd.ID)
	if err != nil {
		c.log.Warn("count chat command use", zap.Error(err))
		count = cmd.UseCount + 1
	}
	user := job.DisplayName
	if user == "" {
		user = job.Username
	}
	vars := chatcmd.Vars{User: user, Args: m.Args, Query: m.Query, Platform: job.Platform,
		Channel: job.ChannelID, Count: count, Role: role.String(), Fetch: chatcmd.HTTPFetch(ctx, c.fetch)}

	if cmd.CreateClip {
		url, err := c.out.Clip(ctx, owner)
		if err != nil {
			c.log.Info("chat command clip failed", zap.String("user", owner), zap.Error(err))
			url = clipFailedText
		}
		vars.Clip = url
	}

	// A reply goes only to the streamer's own channel on a platform with a send path.
	sendable := o.Owned && announcer.SendablePlatforms[job.Platform]
	if sendable && job.Platform == "youtube" {
		// A YouTube send spends Data API quota, which is a premium capability.
		st, err := c.store.OwnerStatus(ctx, o.UserID)
		sendable = err == nil && st.IsPremium
	}
	var reply string
	if cmd.Response != "" && (sendable || cmd.ImageURL != "") {
		text, _ := chatcmd.Render(cmd.Response, vars)
		reply = chatcmd.Line(text)
	}
	if sendable && reply != "" {
		c.send(ctx, o.UserID, reply, job.Platform)
	}
	if cmd.ImageURL != "" {
		c.showImage(ctx, o.UserID, cmd, job, vars, reply)
	}
}

func (c *ChatCommandConsumer) send(ctx context.Context, ownerID uuid.UUID, text, platform string) {
	if err := c.rdb.Set(ctx, echoKey(ownerID, text), 1, echoTTL).Err(); err != nil {
		return // without the echo guard a self-matching reply could loop
	}
	if err := c.out.Send(ctx, ownerID.String(), text, platform); err != nil {
		c.log.Info("chat command reply failed", zap.String("user", ownerID.String()), zap.String("platform", platform), zap.Error(err))
	}
}

// showImage puts the command's image on every alerts overlay of the streamer as a
// chat_command alert (ADR-0080). Like a clip it fires from any chat in the
// streamer's overlays: it shows on their own stream, not in someone else's chat.
func (c *ChatCommandConsumer) showImage(ctx context.Context, ownerID uuid.UUID, cmd models.ChatCommand, job mpmodels.CommandJob, vars chatcmd.Vars, reply string) {
	overlays, err := c.store.AlertsOverlays(ctx, ownerID)
	if err != nil {
		c.log.Warn("chat command alerts overlays", zap.String("user", ownerID.String()), zap.Error(err))
		return
	}
	if len(overlays) == 0 {
		return // no overlay to show it on: skip the render and its fetches
	}
	img, _ := chatcmd.ImageURL(cmd.ImageURL, vars)
	if img == "" {
		return
	}
	msg := reply
	if msg == "" {
		msg = job.Text
	}
	a := alerts.Alert{AlertID: uuid.NewString(), Platform: job.Platform, EventType: alerts.ChatCommandEvent,
		EventData: &alerts.EventData{Type: alerts.ChatCommandEvent,
			Value: &alerts.Value{Amount: float64(vars.Count), Currency: "uses", DisplayText: cmd.Trigger}},
		User: alerts.User{ID: job.UserID, Name: vars.User}, Message: msg, ImageURL: img, OccurredAt: time.Now().UTC()}
	for _, id := range overlays {
		a.OverlayID = id.String()
		payload, _ := json.Marshal(a)
		if err := c.rdb.Publish(ctx, "overlay:"+a.OverlayID+":alerts", payload).Err(); err != nil {
			c.log.Warn("publish chat command alert", zap.String("overlay", a.OverlayID), zap.Error(err))
		}
	}
}

// echoKey identifies a reply text the streamer's account just posted, normalized the
// way chatcmd.Line and the platforms normalize it.
func echoKey(owner uuid.UUID, text string) string {
	sum := sha256.Sum256([]byte(strings.ToLower(chatcmd.Line(text))))
	return "chatcmd:echo:" + owner.String() + ":" + hex.EncodeToString(sum[:12])
}
