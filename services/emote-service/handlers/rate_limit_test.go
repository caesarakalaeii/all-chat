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
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/caesar/all-chat/services/emote-service/clients"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap/zaptest"
)

// These tests lock in the failure policy added after the 2026-07-18/21 provider storms:
// a channel with no emotes on a provider must not be re-checked upstream on every
// request (negative caching), and a provider that 429s must not be hammered until its
// cooldown passes.

func doChannelRequest(t *testing.T, h *EmoteHandler, channel string) *httptest.ResponseRecorder {
	t.Helper()
	router := gin.New()
	router.GET("/emotes/channel/:channel", h.GetChannelEmotes)
	req, _ := http.NewRequest("GET", "/emotes/channel/"+channel, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

func TestGetChannelEmotes_NotFoundIsNegativeCached(t *testing.T) {
	gin.SetMode(gin.TestMode)

	client := &mockEmoteClient{
		err:      fmt.Errorf("bttv: no emotes for channel: %w", clients.ErrNotFound),
		provider: "bttv",
	}
	mockCache := newMockEmoteCache()
	h := NewEmoteHandler(map[string]EmoteClient{"bttv": client}, mockCache, zaptest.NewLogger(t), nil)

	w := doChannelRequest(t, h, "ddg")
	assert.Equal(t, http.StatusOK, w.Code)
	assert.EqualValues(t, 1, client.fetchCalls.Load(), "first request must hit the provider")
	assert.Contains(t, mockCache.negativeKeys, "bttv:ddg", "not-found must be negative-cached")

	w = doChannelRequest(t, h, "ddg")
	assert.Equal(t, http.StatusOK, w.Code)
	assert.EqualValues(t, 1, client.fetchCalls.Load(),
		"second request must be served from the negative cache, not the provider")
}

func TestGetChannelEmotes_RateLimitOpensCooldown(t *testing.T) {
	gin.SetMode(gin.TestMode)

	client := &mockEmoteClient{
		err:      &clients.RateLimitedError{Provider: "bttv"},
		provider: "bttv",
	}
	h := NewEmoteHandler(map[string]EmoteClient{"bttv": client}, newMockEmoteCache(), zaptest.NewLogger(t), nil)

	w := doChannelRequest(t, h, "ddg")
	assert.Equal(t, http.StatusOK, w.Code, "a rate-limited provider degrades, not fails, the aggregate response")
	assert.EqualValues(t, 1, client.fetchCalls.Load())

	// Different channel on the SAME provider: the cooldown is provider-wide, so no
	// upstream call happens.
	w = doChannelRequest(t, h, "otherchannel")
	assert.Equal(t, http.StatusOK, w.Code)
	assert.EqualValues(t, 1, client.fetchCalls.Load(),
		"requests during the cooldown must not hit the provider")
}

func TestGetChannelEmotes_CooldownExpires(t *testing.T) {
	gin.SetMode(gin.TestMode)

	client := &mockEmoteClient{
		err:      &clients.RateLimitedError{Provider: "bttv"},
		provider: "bttv",
	}
	h := NewEmoteHandler(map[string]EmoteClient{"bttv": client}, newMockEmoteCache(), zaptest.NewLogger(t), nil)
	h.rateLimitCooldown = 10 * time.Millisecond

	doChannelRequest(t, h, "ddg")
	assert.EqualValues(t, 1, client.fetchCalls.Load())

	time.Sleep(25 * time.Millisecond)

	doChannelRequest(t, h, "ddg")
	assert.EqualValues(t, 2, client.fetchCalls.Load(),
		"after the cooldown expires the provider is probed again")
}

func TestStartCooldown_HonorsRetryAfter(t *testing.T) {
	h := &EmoteHandler{
		logger:            zaptest.NewLogger(t),
		rateLimitCooldown: time.Minute,
		cooldownUntil:     make(map[string]time.Time),
	}

	// Retry-After longer than the default extends the window.
	h.startCooldown("bttv", &clients.RateLimitedError{Provider: "bttv", RetryAfter: 5 * time.Minute})
	h.cooldownMu.Lock()
	until := h.cooldownUntil["bttv"]
	h.cooldownMu.Unlock()
	assert.True(t, until.After(time.Now().Add(4*time.Minute)),
		"Retry-After beyond the default must extend the cooldown")
	assert.True(t, h.providerInCooldown("bttv"))
	assert.False(t, h.providerInCooldown("ffz"), "cooldowns are per provider")

	// A shorter/absent hint must never SHRINK an existing window.
	h.startCooldown("bttv", &clients.RateLimitedError{Provider: "bttv"})
	h.cooldownMu.Lock()
	after := h.cooldownUntil["bttv"]
	h.cooldownMu.Unlock()
	assert.False(t, after.Before(until), "a later 429 with no hint must not shorten the cooldown")
}
