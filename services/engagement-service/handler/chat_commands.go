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

	"github.com/caesar/all-chat/services/engagement-service/chatcmd"
	"github.com/caesar/all-chat/services/engagement-service/models"
	"github.com/caesar/all-chat/services/engagement-service/repository"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// maxChatCommandsPerUser caps a streamer's commands; every enabled one is matched
// against each message of their channels.
const maxChatCommandsPerUser = 100

type chatCommandStore interface {
	ListChatCommands(ctx context.Context, userID uuid.UUID) ([]models.ChatCommand, error)
	CountChatCommands(ctx context.Context, userID uuid.UUID) (int, error)
	CreateChatCommand(ctx context.Context, c models.ChatCommand) (models.ChatCommand, error)
	UpdateChatCommand(ctx context.Context, c models.ChatCommand) (models.ChatCommand, error)
	DeleteChatCommand(ctx context.Context, userID, id uuid.UUID) error
	OwnerStatus(ctx context.Context, userID uuid.UUID) (repository.ChatCommandOwnerStatus, error)
}

// ChatCommands serves the chat commands editor (ADR-0079). Commands belong to the
// authenticated streamer; there is no overlay in the path.
type ChatCommands struct {
	store    chatCommandStore
	onChange func()       // resyncs chatcmd:channels after a write
	fetch    *http.Client // {urlfetch} in previews
	log      *zap.Logger
}

// NewChatCommands creates the handler. onChange runs after every successful write.
func NewChatCommands(store chatCommandStore, onChange func(), log *zap.Logger) *ChatCommands {
	return &ChatCommands{store: store, onChange: onChange, fetch: chatcmd.FetchClient(), log: log}
}

type chatCommandBody struct {
	Kind            string `json:"kind"`
	Trigger         string `json:"trigger"`
	Response        string `json:"response"`
	CreateClip      bool   `json:"create_clip"`
	ImageURL        string `json:"image_url"`
	MinRole         string `json:"min_role"`
	CooldownSeconds int    `json:"cooldown_seconds"`
	Enabled         *bool  `json:"enabled"`
}

// command validates the body into a ChatCommand, or returns the reason it is invalid.
// The DB checks the same bounds; this turns them into a readable 400.
func (b chatCommandBody) command(userID uuid.UUID) (models.ChatCommand, string) {
	trigger := strings.TrimSpace(b.Trigger)
	image := strings.TrimSpace(b.ImageURL)
	switch {
	case b.Kind != models.TriggerCommand && b.Kind != models.TriggerKeyword:
		return models.ChatCommand{}, "kind must be command or keyword"
	case trigger == "" || len(trigger) > 100:
		return models.ChatCommand{}, "trigger must be 1 to 100 characters"
	case b.Kind == models.TriggerCommand && strings.ContainsAny(trigger, " \t\n"):
		return models.ChatCommand{}, "a command trigger is a single word, like !hello"
	case b.Kind == models.TriggerKeyword && utf8.RuneCountInString(trigger) < 2:
		return models.ChatCommand{}, "a keyword must be at least 2 characters"
	case len(b.Response) > 2000:
		return models.ChatCommand{}, "response must be at most 2000 characters"
	case len(image) > 1000:
		return models.ChatCommand{}, "image must be at most 1000 characters"
	case strings.TrimSpace(b.Response) == "" && !b.CreateClip && image == "":
		return models.ChatCommand{}, "a command needs a response, a clip or an image"
	case b.CooldownSeconds < 0 || b.CooldownSeconds > 86400:
		return models.ChatCommand{}, "cooldown must be between 0 and 86400 seconds"
	}
	if _, ok := chatcmd.ParseRole(b.MinRole); !ok {
		return models.ChatCommand{}, "min_role must be everyone, subscriber, vip, moderator or broadcaster"
	}
	enabled := b.Enabled == nil || *b.Enabled
	return models.ChatCommand{UserID: userID, Kind: b.Kind, Trigger: trigger, Response: b.Response,
		CreateClip: b.CreateClip, ImageURL: image, MinRole: b.MinRole, CooldownSeconds: b.CooldownSeconds, Enabled: enabled}, ""
}

func streamerID(c *gin.Context) (uuid.UUID, bool) {
	uid, ok := requireUser(c)
	if !ok {
		return uuid.Nil, false
	}
	id, err := uuid.Parse(uid)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid streamer identity"})
		return uuid.Nil, false
	}
	return id, true
}

// List returns the streamer's commands and the account status the editor shows
// (YouTube replies need premium; Twitch replies and clips need their grants).
func (h *ChatCommands) List(c *gin.Context) {
	userID, ok := streamerID(c)
	if !ok {
		return
	}
	cmds, err := h.store.ListChatCommands(c.Request.Context(), userID)
	if err != nil {
		h.fail(c, "list chat commands", err)
		return
	}
	status, err := h.store.OwnerStatus(c.Request.Context(), userID)
	if err != nil && !errors.Is(err, repository.ErrNotFound) {
		h.fail(c, "chat command owner status", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"commands": cmds, "status": status})
}

// Create adds a command. Gated by RequireEarlyAccess and RequirePremium(GateChatCommands) in main.go.
func (h *ChatCommands) Create(c *gin.Context) {
	userID, ok := streamerID(c)
	if !ok {
		return
	}
	cmd, ok := h.bind(c, userID)
	if !ok {
		return
	}
	n, err := h.store.CountChatCommands(c.Request.Context(), userID)
	if err != nil {
		h.fail(c, "count chat commands", err)
		return
	}
	if n >= maxChatCommandsPerUser {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "you have reached the limit of 100 chat commands"})
		return
	}
	saved, err := h.store.CreateChatCommand(c.Request.Context(), cmd)
	if err != nil {
		h.writeErr(c, "create chat command", err)
		return
	}
	h.onChange()
	c.JSON(http.StatusCreated, saved)
}

// Update replaces a command the streamer owns. Gated like Create.
func (h *ChatCommands) Update(c *gin.Context) {
	userID, ok := streamerID(c)
	if !ok {
		return
	}
	id, ok := uuidParam(c, "cid")
	if !ok {
		return
	}
	cmd, ok := h.bind(c, userID)
	if !ok {
		return
	}
	cmd.ID = id
	saved, err := h.store.UpdateChatCommand(c.Request.Context(), cmd)
	if err != nil {
		h.writeErr(c, "update chat command", err)
		return
	}
	h.onChange()
	c.JSON(http.StatusOK, saved)
}

// Delete removes a command the streamer owns. Never gated: a lapsed premium must not
// trap a streamer with commands they can no longer remove.
func (h *ChatCommands) Delete(c *gin.Context) {
	userID, ok := streamerID(c)
	if !ok {
		return
	}
	id, ok := uuidParam(c, "cid")
	if !ok {
		return
	}
	if err := h.store.DeleteChatCommand(c.Request.Context(), userID, id); err != nil {
		h.writeErr(c, "delete chat command", err)
		return
	}
	h.onChange()
	c.Status(http.StatusNoContent)
}

type previewBody struct {
	Response   string `json:"response"`
	Image      string `json:"image"`
	Input      string `json:"input"` // what the viewer typed after the command
	MinRole    string `json:"min_role"`
	CreateClip bool   `json:"create_clip"`
}

// Preview renders a response and image template against sample chat input, returning
// the line chat would see, the image the alerts overlay would show, and any {js},
// {urlfetch} or image errors, so the streamer can debug a template without firing it
// in their chat. {urlfetch} really fetches, bounded like a fired command and by the
// route's per-user rate limit.
func (h *ChatCommands) Preview(c *gin.Context) {
	if _, ok := streamerID(c); !ok {
		return
	}
	var b previewBody
	if err := c.ShouldBindJSON(&b); err != nil || len(b.Response) > 2000 || len(b.Image) > 1000 || len(b.Input) > 500 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "response (max 2000), image (max 1000) and input (max 500) are required"})
		return
	}
	role, _ := chatcmd.ParseRole(b.MinRole)
	input := strings.TrimSpace(b.Input)
	vars := chatcmd.Vars{User: "ExampleViewer", Args: strings.Fields(input), Query: input,
		Platform: "twitch", Channel: "yourchannel", Count: 1, Role: role.String(),
		Fetch: chatcmd.HTTPFetch(c.Request.Context(), h.fetch)}
	if b.CreateClip {
		vars.Clip = "https://clips.twitch.tv/ExampleClip"
	}
	out, errs := chatcmd.Render(b.Response, vars)
	image, imgErrs := chatcmd.ImageURL(b.Image, vars)
	errs = append(errs, imgErrs...)
	if errs == nil {
		errs = []string{}
	}
	c.JSON(http.StatusOK, gin.H{"output": chatcmd.Line(out), "image": image, "errors": errs})
}

func (h *ChatCommands) bind(c *gin.Context, userID uuid.UUID) (models.ChatCommand, bool) {
	var b chatCommandBody
	if err := c.ShouldBindJSON(&b); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return models.ChatCommand{}, false
	}
	cmd, reason := b.command(userID)
	if reason != "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": reason})
		return models.ChatCommand{}, false
	}
	return cmd, true
}

func (h *ChatCommands) writeErr(c *gin.Context, what string, err error) {
	switch {
	case errors.Is(err, repository.ErrDuplicateTrigger):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	case errors.Is(err, repository.ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "chat command not found"})
	default:
		h.fail(c, what, err)
	}
}

func (h *ChatCommands) fail(c *gin.Context, what string, err error) {
	h.log.Error(what, zap.Error(err))
	c.JSON(http.StatusInternalServerError, gin.H{"error": what + " failed"})
}
