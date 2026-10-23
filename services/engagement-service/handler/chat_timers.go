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

package handler

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/caesar/all-chat/services/engagement-service/models"
	"github.com/caesar/all-chat/services/engagement-service/repository"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// maxChatTimersPerUser caps a streamer's timers; the scheduler loads and checks every
// enabled one on every tick.
const maxChatTimersPerUser = 20

// chatTimerStore is the slice of the repository the timers editor needs.
type chatTimerStore interface {
	ListChatTimers(ctx context.Context, userID uuid.UUID) ([]models.ChatTimer, error)
	CountChatTimers(ctx context.Context, userID uuid.UUID) (int, error)
	CreateChatTimer(ctx context.Context, t models.ChatTimer) (models.ChatTimer, error)
	UpdateChatTimer(ctx context.Context, t models.ChatTimer) (models.ChatTimer, error)
	DeleteChatTimer(ctx context.Context, userID, id uuid.UUID) error
	GetChatBotSetting(ctx context.Context, userID uuid.UUID) (string, error)
	SetChatBotSetting(ctx context.Context, userID uuid.UUID, sender string) error
}

// ChatTimers serves the timers editor and the bot-sender setting (ADR-0081). Both
// belong to the authenticated streamer; there is no overlay in the path.
type ChatTimers struct {
	store    chatTimerStore
	onChange func() // resyncs chatcmd:channels after a write
	log      *zap.Logger
}

// NewChatTimers creates the handler. onChange runs after every successful write.
func NewChatTimers(store chatTimerStore, onChange func(), log *zap.Logger) *ChatTimers {
	return &ChatTimers{store: store, onChange: onChange, log: log}
}

type chatTimerBody struct {
	Name            string `json:"name"`
	Message         string `json:"message"`
	IntervalMinutes *int   `json:"interval_minutes"`
	MinMessages     *int   `json:"min_messages"`
	Enabled         *bool  `json:"enabled"`
}

// timer validates the body into a ChatTimer, or returns the reason it is invalid.
// The same bounds exist as DB CHECK constraints; this turns them into a readable 400.
func (b chatTimerBody) timer(userID uuid.UUID) (models.ChatTimer, string) {
	name := strings.TrimSpace(b.Name)
	if strings.Trim(b.Message, " \t\n\r") == "" {
		return models.ChatTimer{}, "message must not be blank"
	}
	switch {
	case name == "" || len(name) > 50:
		return models.ChatTimer{}, "name must be 1 to 50 characters"
	case !utf8.ValidString(name) || !utf8.ValidString(b.Message):
		return models.ChatTimer{}, "name and message must be valid UTF-8"
	case len(b.Message) > 2000:
		return models.ChatTimer{}, "message must be at most 2000 characters"
	}
	if b.IntervalMinutes == nil && b.MinMessages == nil {
		return models.ChatTimer{}, "an interval or a minimum message count is required"
	}
	if b.IntervalMinutes != nil && (*b.IntervalMinutes < 5 || *b.IntervalMinutes > 1440) {
		return models.ChatTimer{}, "interval_minutes must be between 5 and 1440"
	}
	if b.MinMessages != nil && (*b.MinMessages < 1 || *b.MinMessages > 10000) {
		return models.ChatTimer{}, "min_messages must be between 1 and 10000"
	}
	enabled := b.Enabled == nil || *b.Enabled
	return models.ChatTimer{UserID: userID, Name: name, Message: b.Message,
		IntervalMinutes: b.IntervalMinutes, MinMessages: b.MinMessages, Enabled: enabled}, ""
}

// List returns the streamer's timers and the sender setting the editor shows.
func (h *ChatTimers) List(c *gin.Context) {
	userID, ok := streamerID(c)
	if !ok {
		return
	}
	timers, err := h.store.ListChatTimers(c.Request.Context(), userID)
	if err != nil {
		h.fail(c, "list chat timers", err)
		return
	}
	sender, err := h.store.GetChatBotSetting(c.Request.Context(), userID)
	if err != nil {
		h.fail(c, "chat bot setting", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"timers": timers, "sender": sender})
}

// Create adds a timer. Gated by RequireEarlyAccess and RequirePremium(GateChatCommands)
// in main.go, like the command writes.
func (h *ChatTimers) Create(c *gin.Context) {
	userID, ok := streamerID(c)
	if !ok {
		return
	}
	t, ok := h.bind(c, userID)
	if !ok {
		return
	}
	n, err := h.store.CountChatTimers(c.Request.Context(), userID)
	if err != nil {
		h.fail(c, "count chat timers", err)
		return
	}
	if n >= maxChatTimersPerUser {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "you have reached the limit of 20 chat timers"})
		return
	}
	saved, err := h.store.CreateChatTimer(c.Request.Context(), t)
	if err != nil {
		h.writeErr(c, "create chat timer", err)
		return
	}
	h.onChange()
	c.JSON(http.StatusCreated, saved)
}

// Update replaces a timer the streamer owns. Gated like Create.
func (h *ChatTimers) Update(c *gin.Context) {
	userID, ok := streamerID(c)
	if !ok {
		return
	}
	id, ok := uuidParam(c, "tid")
	if !ok {
		return
	}
	t, ok := h.bind(c, userID)
	if !ok {
		return
	}
	t.ID = id
	saved, err := h.store.UpdateChatTimer(c.Request.Context(), t)
	if err != nil {
		h.writeErr(c, "update chat timer", err)
		return
	}
	h.onChange()
	c.JSON(http.StatusOK, saved)
}

// Delete removes a timer the streamer owns. Never gated: a lapsed premium must not
// trap a streamer with timers they can no longer remove.
func (h *ChatTimers) Delete(c *gin.Context) {
	userID, ok := streamerID(c)
	if !ok {
		return
	}
	id, ok := uuidParam(c, "tid")
	if !ok {
		return
	}
	if err := h.store.DeleteChatTimer(c.Request.Context(), userID, id); err != nil {
		h.writeErr(c, "delete chat timer", err)
		return
	}
	h.onChange()
	c.Status(http.StatusNoContent)
}

// GetSender returns the sender the streamer's command replies and timer posts use.
func (h *ChatTimers) GetSender(c *gin.Context) {
	userID, ok := streamerID(c)
	if !ok {
		return
	}
	sender, err := h.store.GetChatBotSetting(c.Request.Context(), userID)
	if err != nil {
		h.fail(c, "chat bot setting", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"sender": sender})
}

type senderBody struct {
	Sender string `json:"sender"`
}

// PutSender switches the streamer's sends between the All-Chat Bot and their own
// account. Gated by the chat-commands gate in main.go, like the command writes.
func (h *ChatTimers) PutSender(c *gin.Context) {
	userID, ok := streamerID(c)
	if !ok {
		return
	}
	var b senderBody
	if err := c.ShouldBindJSON(&b); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	if b.Sender != repository.SenderAllChatBot && b.Sender != repository.SenderSelf {
		c.JSON(http.StatusBadRequest, gin.H{"error": "sender must be bot or self"})
		return
	}
	if err := h.store.SetChatBotSetting(c.Request.Context(), userID, b.Sender); err != nil {
		h.fail(c, "set chat bot setting", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"sender": b.Sender})
}

func (h *ChatTimers) bind(c *gin.Context, userID uuid.UUID) (models.ChatTimer, bool) {
	var b chatTimerBody
	if err := c.ShouldBindJSON(&b); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return models.ChatTimer{}, false
	}
	t, reason := b.timer(userID)
	if reason != "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": reason})
		return models.ChatTimer{}, false
	}
	return t, true
}

func (h *ChatTimers) writeErr(c *gin.Context, what string, err error) {
	switch {
	case errors.Is(err, repository.ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "chat timer not found"})
	default:
		h.fail(c, what, err)
	}
}

func (h *ChatTimers) fail(c *gin.Context, what string, err error) {
	h.log.Error(what, zap.Error(err))
	c.JSON(http.StatusInternalServerError, gin.H{"error": what + " failed"})
}
