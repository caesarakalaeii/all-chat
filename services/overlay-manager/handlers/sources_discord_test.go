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

package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/caesar/all-chat/services/overlay-manager/clients"
	"github.com/caesar/all-chat/services/overlay-manager/models"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

// The Discord source guard closes a cross-tenant escalation: before it existed, any
// authenticated user could attach an arbitrary Discord channel id to their own overlay.
// Because the shared bot is the actor for every Discord read (channel registry → the
// attacker's overlay receives that guild's chat), every moderation write (delete/timeout/ban
// bounded only by the bot's guild permissions), and every relay write (a provisioned webhook
// posting into the channel), the only thing binding a source to its rightful owner is this
// check. See ADR-0048.

type mockDiscordChannelResolver struct {
	guilds map[string]string // channelID → guildID
	err    error
}

func (m *mockDiscordChannelResolver) GuildIDForChannel(_ context.Context, channelID string) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	g, ok := m.guilds[channelID]
	if !ok {
		return "", clients.ErrDiscordChannelNotFound
	}
	return g, nil
}

type mockDiscordGuildOwnership struct {
	owned map[string]bool // userID|guildID → true
	err   error
}

func (m *mockDiscordGuildOwnership) UserOwnsGuild(_ context.Context, userID, guildID string) (bool, error) {
	if m.err != nil {
		return false, m.err
	}
	return m.owned[userID+"|"+guildID], nil
}

// discordTestHandler builds a SourcesHandler with the Discord guard wired and a capture hook
// on Create. db stays nil: the guard must never need it.
func discordTestHandler(resolver discordChannelGuildResolver, ownership discordGuildOwnership, captured **models.ChatSource) *SourcesHandler {
	h := &SourcesHandler{
		sourceRepo: &mockSourceRepository{
			createFunc: func(_ context.Context, s *models.ChatSource) error {
				if captured != nil {
					*captured = s
				}
				return nil
			},
		},
		overlayRepo: &mockOverlayRepository{
			getByIDAndUserIDFunc: func(_ context.Context, id, userID string) (*models.Overlay, error) {
				return &models.Overlay{ID: id, UserID: userID, Name: "Test Overlay"}, nil
			},
		},
		db:     nil,
		logger: zap.NewNop(),
	}
	h.SetDiscordGuard(resolver, ownership)
	return h
}

func postDiscordSource(h *SourcesHandler, body map[string]interface{}) *httptest.ResponseRecorder {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/overlays/:id/sources", func(c *gin.Context) {
		c.Set("user_id", "attacker")
		h.HandleAddSource(c)
	})
	bodyBytes, _ := json.Marshal(body)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/overlays/overlay-id/sources", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	return w
}

// A channel in a guild the caller never connected must be refused. This is the core of the
// vulnerability: the bot can see the guild, so without this check the source works.
func TestHandleAddSource_Discord_RejectsChannelInUnconnectedGuild(t *testing.T) {
	var captured *models.ChatSource
	h := discordTestHandler(
		&mockDiscordChannelResolver{guilds: map[string]string{"victim-chan": "victim-guild"}},
		&mockDiscordGuildOwnership{owned: map[string]bool{"attacker|attacker-guild": true}},
		&captured,
	)

	w := postDiscordSource(h, map[string]interface{}{
		"platform":   "discord",
		"channel_id": "victim-chan",
		"config":     map[string]interface{}{"guild_id": "victim-guild"},
	})

	assert.Equal(t, http.StatusForbidden, w.Code,
		"a channel in a guild the caller has not connected must be refused")
	assert.Nil(t, captured, "no source row may be created for an unowned guild")
}

// The legitimate flow must keep working: the frontend only offers channels from guilds the
// user connected, so the resolved guild is one they own.
func TestHandleAddSource_Discord_AllowsChannelInConnectedGuild(t *testing.T) {
	var captured *models.ChatSource
	h := discordTestHandler(
		&mockDiscordChannelResolver{guilds: map[string]string{"my-chan": "attacker-guild"}},
		&mockDiscordGuildOwnership{owned: map[string]bool{"attacker|attacker-guild": true}},
		&captured,
	)

	w := postDiscordSource(h, map[string]interface{}{
		"platform":   "discord",
		"channel_id": "my-chan",
		"config": map[string]interface{}{
			"guild_id":           "attacker-guild",
			"inbound_channel_id": "my-chan",
		},
	})

	assert.Equal(t, http.StatusCreated, w.Code)
	if assert.NotNil(t, captured) {
		assert.Equal(t, "discord", captured.Platform)
	}
}

// A guard that cannot run must refuse, not pass. Without the bot token there is no way to
// resolve the channel's guild, so Discord sources cannot be validated at all.
func TestHandleAddSource_Discord_FailsClosedWithoutGuard(t *testing.T) {
	var captured *models.ChatSource
	h := discordTestHandler(nil, nil, &captured)

	w := postDiscordSource(h, map[string]interface{}{
		"platform":   "discord",
		"channel_id": "any-chan",
	})

	assert.Equal(t, http.StatusServiceUnavailable, w.Code,
		"an unconfigured guard must fail closed, never allow")
	assert.Nil(t, captured)
}

// A transient Discord outage must not be mistaken for "unowned" (403) or let the source
// through — it is a 503 so the caller retries.
func TestHandleAddSource_Discord_FailsClosedOnResolverError(t *testing.T) {
	var captured *models.ChatSource
	h := discordTestHandler(
		&mockDiscordChannelResolver{err: errors.New("discord: unexpected status 500")},
		&mockDiscordGuildOwnership{},
		&captured,
	)

	w := postDiscordSource(h, map[string]interface{}{
		"platform":   "discord",
		"channel_id": "any-chan",
	})

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	assert.Nil(t, captured)
}

// Likewise a database failure on the ownership lookup must fail closed.
func TestHandleAddSource_Discord_FailsClosedOnOwnershipError(t *testing.T) {
	var captured *models.ChatSource
	h := discordTestHandler(
		&mockDiscordChannelResolver{guilds: map[string]string{"c": "g"}},
		&mockDiscordGuildOwnership{err: errors.New("db down")},
		&captured,
	)

	w := postDiscordSource(h, map[string]interface{}{
		"platform":   "discord",
		"channel_id": "c",
	})

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	assert.Nil(t, captured)
}

// An unknown channel id is a client error, distinguishable from "you do not own it".
func TestHandleAddSource_Discord_RejectsUnknownChannel(t *testing.T) {
	var captured *models.ChatSource
	h := discordTestHandler(
		&mockDiscordChannelResolver{guilds: map[string]string{}},
		&mockDiscordGuildOwnership{},
		&captured,
	)

	w := postDiscordSource(h, map[string]interface{}{
		"platform":   "discord",
		"channel_id": "does-not-exist",
	})

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Nil(t, captured)
}

// The client-supplied guild_id is never trusted as the binding, but if it contradicts what
// Discord reports, the request is a mistake (or an attempt) and is refused rather than
// silently stored — a wrong guild_id would mislabel the source in the UI.
func TestHandleAddSource_Discord_RejectsGuildIDMismatch(t *testing.T) {
	var captured *models.ChatSource
	h := discordTestHandler(
		&mockDiscordChannelResolver{guilds: map[string]string{"my-chan": "attacker-guild"}},
		&mockDiscordGuildOwnership{owned: map[string]bool{"attacker|attacker-guild": true}},
		&captured,
	)

	w := postDiscordSource(h, map[string]interface{}{
		"platform":   "discord",
		"channel_id": "my-chan",
		"config":     map[string]interface{}{"guild_id": "some-other-guild"},
	})

	assert.Equal(t, http.StatusBadRequest, w.Code,
		"a config guild_id that disagrees with Discord must be refused")
	assert.Nil(t, captured)
}

// inbound_channel_id is what actually gets written to the discord:channels:{id} registry, so
// it is the read-path target and must be validated even when channel_id is legitimate.
func TestHandleAddSource_Discord_ValidatesInboundChannelID(t *testing.T) {
	var captured *models.ChatSource
	h := discordTestHandler(
		&mockDiscordChannelResolver{guilds: map[string]string{
			"my-chan":     "attacker-guild",
			"victim-chan": "victim-guild",
		}},
		&mockDiscordGuildOwnership{owned: map[string]bool{"attacker|attacker-guild": true}},
		&captured,
	)

	w := postDiscordSource(h, map[string]interface{}{
		"platform":   "discord",
		"channel_id": "my-chan",
		"config": map[string]interface{}{
			"guild_id":           "attacker-guild",
			"inbound_channel_id": "victim-chan",
		},
	})

	assert.Equal(t, http.StatusForbidden, w.Code,
		"inbound_channel_id is the registry key and must be owned too")
	assert.Nil(t, captured)
}

// relay_channel_id is an outbound write target: discord-listener provisions a webhook and
// posts chat into it, so an unowned value turns All-Chat into a spam relay.
func TestHandleAddSource_Discord_ValidatesRelayChannelID(t *testing.T) {
	var captured *models.ChatSource
	h := discordTestHandler(
		&mockDiscordChannelResolver{guilds: map[string]string{
			"my-chan":     "attacker-guild",
			"victim-chan": "victim-guild",
		}},
		&mockDiscordGuildOwnership{owned: map[string]bool{"attacker|attacker-guild": true}},
		&captured,
	)

	w := postDiscordSource(h, map[string]interface{}{
		"platform":   "discord",
		"channel_id": "my-chan",
		"config": map[string]interface{}{
			"guild_id":         "attacker-guild",
			"relay_enabled":    true,
			"relay_channel_id": "victim-chan",
		},
	})

	assert.Equal(t, http.StatusForbidden, w.Code,
		"relay_channel_id is an outbound write target and must be owned")
	assert.Nil(t, captured)
}

// Non-Discord platforms must be unaffected by the guard, including when it is unconfigured.
func TestHandleAddSource_NonDiscord_UnaffectedByGuard(t *testing.T) {
	var captured *models.ChatSource
	h := discordTestHandler(nil, nil, &captured)

	w := postDiscordSource(h, map[string]interface{}{
		"platform":   "twitch",
		"channel_id": "xqc",
	})

	assert.Equal(t, http.StatusCreated, w.Code)
	assert.NotNil(t, captured)
}

// The internal auto-add endpoint exists for the OAuth callback (twitch/youtube/kick) and has
// no legitimate Discord caller. It writes no registry key, but a row alone is enough to make
// a channel moderatable, so it must refuse Discord outright.
func TestHandleAddSourceAuto_RejectsDiscord(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var captured *models.ChatSource
	h := discordTestHandler(nil, nil, &captured)

	router := gin.New()
	router.POST("/internal/overlays/:id/sources/auto", func(c *gin.Context) {
		c.Set("user_id", "attacker")
		h.HandleAddSourceAuto(c)
	})

	body, _ := json.Marshal(map[string]interface{}{
		"platform":   "discord",
		"channel_id": "victim-chan",
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/internal/overlays/overlay-id/sources/auto", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code,
		"the auto endpoint must not create Discord sources")
	assert.Nil(t, captured)
}

// patchConfig drives HandleUpdateSourceConfig with a given stored source.
func patchConfig(h *SourcesHandler, stored *models.ChatSource, config map[string]interface{}) *httptest.ResponseRecorder {
	gin.SetMode(gin.TestMode)
	h.sourceRepo = &mockSourceRepository{
		getByIDFunc: func(_ context.Context, _ string) (*models.ChatSource, error) {
			return stored, nil
		},
	}
	router := gin.New()
	router.PATCH("/overlays/:id/sources/:source_id/config", func(c *gin.Context) {
		c.Set("user_id", "attacker")
		h.HandleUpdateSourceConfig(c)
	})
	body, _ := json.Marshal(map[string]interface{}{"config": config})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PATCH", "/overlays/overlay-id/sources/source-id/config", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	return w
}

// The relay target can also be set after the fact, so the PATCH path needs the same guard.
func TestHandleUpdateSourceConfig_Discord_ValidatesRelayChannel(t *testing.T) {
	h := discordTestHandler(
		&mockDiscordChannelResolver{guilds: map[string]string{
			"my-chan":     "attacker-guild",
			"victim-chan": "victim-guild",
		}},
		&mockDiscordGuildOwnership{owned: map[string]bool{"attacker|attacker-guild": true}},
		nil,
	)

	w := patchConfig(h, &models.ChatSource{
		ID: "source-id", OverlayID: "overlay-id", Platform: "discord", ChannelID: "my-chan",
	}, map[string]interface{}{
		"relay_enabled":    true,
		"relay_channel_id": "victim-chan",
	})

	assert.Equal(t, http.StatusForbidden, w.Code,
		"a relay target added by PATCH must be validated like one added at create time")
}

// A primary channel deleted on Discord's side must not strand the owner: reconfiguring the
// source (e.g. turning a relay off) validates only the channels named in the request.
func TestHandleUpdateSourceConfig_Discord_AllowsWhenStoredChannelIsGone(t *testing.T) {
	h := discordTestHandler(
		// "gone-chan" is absent from the resolver, i.e. deleted on Discord.
		&mockDiscordChannelResolver{guilds: map[string]string{"my-chan": "attacker-guild"}},
		&mockDiscordGuildOwnership{owned: map[string]bool{"attacker|attacker-guild": true}},
		nil,
	)

	w := patchConfig(h, &models.ChatSource{
		ID: "source-id", OverlayID: "overlay-id", Platform: "discord", ChannelID: "gone-chan",
	}, map[string]interface{}{
		"relay_enabled": false,
	})

	assert.Equal(t, http.StatusOK, w.Code,
		"a deleted primary channel must not block turning a relay off")
}

// A source id from someone else's overlay must not be patchable just because the caller owns
// the overlay named in the path — UpdateConfig was previously keyed on source_id alone.
func TestHandleUpdateSourceConfig_RejectsSourceFromAnotherOverlay(t *testing.T) {
	h := discordTestHandler(
		&mockDiscordChannelResolver{},
		&mockDiscordGuildOwnership{},
		nil,
	)

	w := patchConfig(h, &models.ChatSource{
		ID: "source-id", OverlayID: "someone-elses-overlay", Platform: "twitch", ChannelID: "xqc",
	}, map[string]interface{}{"stream_select": "first_found"})

	assert.Equal(t, http.StatusNotFound, w.Code,
		"the source must belong to the overlay in the path")
}
