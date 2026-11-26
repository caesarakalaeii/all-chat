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
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
	"go.uber.org/zap"
)

const (
	// stickyDebounce is how long the reposter waits after the last message in a
	// channel before it moves the sticky. Reposting per message would both hit the
	// Discord rate limit during a busy conversation and leave the channel flickering.
	stickyDebounce = 5 * time.Second

	// stickyScanLimit is how far back the restart sweep looks for an orphaned copy.
	// Discord caps a single history page at 100; 50 is well inside that and covers
	// the messages that can realistically arrive while a pod restarts.
	stickyScanLimit = 50

	// stickyBotPlaceholder is replaced with the bot's mention at post time. The
	// configured text cannot contain the bot's ID because it is not known until the
	// gateway connects.
	stickyBotPlaceholder = "{bot}"
)

// stickyAPI is the slice of *discordgo.Session the reposter uses, so tests can
// substitute a fake. The signatures match the session's methods exactly, which is
// what lets the session be passed in directly.
type stickyAPI interface {
	ChannelMessageSend(channelID, content string, options ...discordgo.RequestOption) (*discordgo.Message, error)
	ChannelMessageDelete(channelID, messageID string, options ...discordgo.RequestOption) error
	ChannelMessages(channelID string, limit int, beforeID, afterID, aroundID string, options ...discordgo.RequestOption) ([]*discordgo.Message, error)
}

// stickyDeps are the reposter's dependencies.
type stickyDeps struct {
	API        stickyAPI
	ChannelIDs []string
	Text       string
	// BotID resolves the bot's own user ID; it is empty until the gateway is ready.
	BotID func() string
	// Debounce overrides stickyDebounce. Tests set it to keep themselves fast.
	Debounce time.Duration
	Log      *zap.Logger
}

// stickyReposter keeps a configured message at the bottom of configured channels.
// All state is in memory: the deployment runs a single replica, and losing it on a
// restart only costs one extra history scan (see cleanUpOrphans).
type stickyReposter struct {
	api      stickyAPI
	channels map[string]struct{}
	text     string
	botID    func() string
	debounce time.Duration
	log      *zap.Logger

	mu     sync.Mutex
	timers map[string]*time.Timer // channel ID -> armed repost timer
	lastID map[string]string      // channel ID -> ID of the sticky currently posted

	// cycles serialises reposts per channel. Without it a timer that fires while the
	// previous repost is still waiting on Discord would run a second delete-and-post
	// against the same last-ID, leaving one of the two stickies orphaned.
	cycles *serialQueues
}

func newStickyReposter(d stickyDeps) *stickyReposter {
	channels := make(map[string]struct{}, len(d.ChannelIDs))
	for _, id := range d.ChannelIDs {
		channels[id] = struct{}{}
	}
	debounce := d.Debounce
	if debounce <= 0 {
		debounce = stickyDebounce
	}
	return &stickyReposter{
		api:      d.API,
		channels: channels,
		text:     d.Text,
		botID:    d.BotID,
		debounce: debounce,
		log:      d.Log,
		timers:   make(map[string]*time.Timer),
		lastID:   make(map[string]string),
		cycles:   newSerialQueues(),
	}
}

// enabled reports whether the feature is configured. Emptying either env var turns
// it off, which is the documented way to take the sticky down.
func (s *stickyReposter) enabled() bool {
	return len(s.channels) > 0 && s.text != ""
}

// onMessage notes a message from someone other than the bot and (re)arms the repost
// timer for its channel. It returns immediately so the gateway handler is never
// blocked, and is a no-op for channels the sticky is not configured for.
func (s *stickyReposter) onMessage(channelID string) {
	if !s.enabled() {
		return
	}
	if _, ok := s.channels[channelID]; !ok {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if t := s.timers[channelID]; t != nil {
		t.Stop()
	}
	s.timers[channelID] = time.AfterFunc(s.debounce, func() {
		s.cycles.enqueue(channelID, func() { s.repost(channelID) })
	})
}

// stop cancels armed timers so shutdown does not post a sticky on the way out. A
// cycle already queued or running is left to finish: it only writes to Discord, and
// interrupting it between the delete and the post is what would lose the sticky.
func (s *stickyReposter) stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, t := range s.timers {
		t.Stop()
		delete(s.timers, id)
	}
}

// lastStickyID returns the ID of the sticky the reposter believes is currently
// posted in a channel, or empty when it does not know of one.
func (s *stickyReposter) lastStickyID(channelID string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastID[channelID]
}

// repost moves the sticky to the bottom of one channel: remove the old copy, post a
// new one, remember its ID.
func (s *stickyReposter) repost(channelID string) {
	botID := s.botID()
	if botID == "" {
		// The gateway has not reported our identity yet, so {bot} cannot be rendered.
		// Skipping is better than posting a message with a broken mention in it; the
		// next message in the channel arms another cycle.
		s.log.Warn("sticky skipped: bot ID not known yet", zap.String("channel", channelID))
		return
	}
	text := renderSticky(s.text, botID)

	s.mu.Lock()
	previous := s.lastID[channelID]
	delete(s.timers, channelID)
	s.mu.Unlock()

	if previous != "" {
		s.deleteSticky(channelID, previous)
	} else {
		s.cleanUpOrphans(channelID, botID, text)
	}

	msg, err := s.api.ChannelMessageSend(channelID, text)
	if err != nil {
		// Forget the old ID either way: it has just been deleted, and keeping it would
		// make the next cycle delete a message that no longer exists.
		s.setLastID(channelID, "")
		s.log.Warn("failed to post sticky", zap.String("channel", channelID), zap.Error(err))
		return
	}
	s.setLastID(channelID, msg.ID)
}

func (s *stickyReposter) setLastID(channelID, messageID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if messageID == "" {
		delete(s.lastID, channelID)
		return
	}
	s.lastID[channelID] = messageID
}

// deleteSticky removes one of our own stickies. A message somebody already deleted
// by hand is the expected case, not an error.
func (s *stickyReposter) deleteSticky(channelID, messageID string) {
	err := s.api.ChannelMessageDelete(channelID, messageID)
	if err == nil || isUnknownMessage(err) {
		return
	}
	s.log.Warn("failed to delete previous sticky",
		zap.String("channel", channelID), zap.String("message", messageID), zap.Error(err))
}

// cleanUpOrphans deletes stickies left in a channel by a previous process. The last
// posted ID lives in memory only, so after a restart the bot would otherwise post a
// second copy on top of the one already there.
func (s *stickyReposter) cleanUpOrphans(channelID, botID, text string) {
	msgs, err := s.api.ChannelMessages(channelID, stickyScanLimit, "", "", "")
	if err != nil {
		// Without the history we cannot tell an orphan from an ordinary message. Post
		// anyway: a duplicate reminder is a smaller problem than no reminder.
		s.log.Warn("sticky orphan scan failed", zap.String("channel", channelID), zap.Error(err))
		return
	}
	for _, msg := range msgs {
		if msg.Author == nil || msg.Author.ID != botID || msg.Content != text {
			continue
		}
		s.deleteSticky(channelID, msg.ID)
	}
}

// renderSticky substitutes the bot's mention for every {bot} placeholder.
func renderSticky(text, botID string) string {
	return strings.ReplaceAll(text, stickyBotPlaceholder, "<@"+botID+">")
}

// isUnknownMessage reports whether err says the message is already gone. Discord
// answers 404 with code 10008; the body is not always decodable, so the status is
// checked as well.
func isUnknownMessage(err error) bool {
	var restErr *discordgo.RESTError
	if !errors.As(err, &restErr) {
		return false
	}
	if restErr.Message != nil && restErr.Message.Code == discordgo.ErrCodeUnknownMessage {
		return true
	}
	return restErr.Response != nil && restErr.Response.StatusCode == http.StatusNotFound
}
