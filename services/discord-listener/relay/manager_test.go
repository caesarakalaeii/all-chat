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

package relay

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

// captureCallsPoster records calls to Post for assertion in tests.
type captureCallsPoster struct {
	calls []postCall
}

type postCall struct {
	webhookURL string
	payload    RelayPayload
}

func (p *captureCallsPoster) Post(_ context.Context, webhookURL string, msg RelayPayload) error {
	p.calls = append(p.calls, postCall{webhookURL: webhookURL, payload: msg})
	return nil
}

// stubRepository returns a fixed slice of relayConfig entries.
type stubRepository struct {
	configs []relayConfig
}

func (r *stubRepository) GetRelayConfigs(_ context.Context) ([]relayConfig, error) {
	return r.configs, nil
}

func (r *stubRepository) GetPendingRelayConfigs(_ context.Context) ([]pendingRelayConfig, error) {
	return nil, nil
}

func (r *stubRepository) StoreWebhookURL(_ context.Context, _, _ string) error {
	return nil
}

func TestRelayManager_DiscordPlatformFiltered(t *testing.T) {
	poster := &captureCallsPoster{}
	mgr := &Manager{
		poster: poster,
	}

	err := mgr.HandleMessage(context.Background(), "discord", "someuser", "", "", "hello", "overlay-123", "https://webhook.url")
	assert.NoError(t, err)
	assert.Len(t, poster.calls, 0, "platform=discord must never call poster.Post")
}

func TestRelayManager_NonDiscordRelayed(t *testing.T) {
	poster := &captureCallsPoster{}
	mgr := &Manager{
		poster: poster,
	}

	webhookURL := "https://discord.com/api/webhooks/123/token"
	err := mgr.HandleMessage(context.Background(), "twitch", "alice", "Alice", "https://avatar.url/img.png", "hello", "overlay-123", webhookURL)
	assert.NoError(t, err)

	if assert.Len(t, poster.calls, 1, "platform=twitch must call poster.Post exactly once") {
		assert.Equal(t, webhookURL, poster.calls[0].webhookURL)
		assert.Equal(t, "Alice [Twitch]", poster.calls[0].payload.Username)
		assert.Equal(t, "hello", poster.calls[0].payload.Content)
		assert.Equal(t, "https://avatar.url/img.png", poster.calls[0].payload.AvatarURL)
	}
}

func TestRelayManager_FallsBackToUsernameWhenDisplayNameEmpty(t *testing.T) {
	poster := &captureCallsPoster{}
	mgr := &Manager{
		poster: poster,
	}

	webhookURL := "https://discord.com/api/webhooks/123/token"
	err := mgr.HandleMessage(context.Background(), "kick", "bobuser", "", "", "hi", "overlay-1", webhookURL)
	assert.NoError(t, err)

	if assert.Len(t, poster.calls, 1) {
		assert.Equal(t, "bobuser [Kick]", poster.calls[0].payload.Username)
	}
}

func TestFlushBatch_SingleMessage_PostsIndividually(t *testing.T) {
	poster := &captureCallsPoster{}
	mgr := &Manager{poster: poster}

	batch := []RelayPayload{
		{Content: "hello", Username: "alice [Kick]", AvatarURL: "https://avatar.url"},
	}
	mgr.flushBatch(context.Background(), "https://webhook/url", batch, "overlay-1")

	if assert.Len(t, poster.calls, 1) {
		assert.Equal(t, "alice [Kick]", poster.calls[0].payload.Username)
		assert.Equal(t, "hello", poster.calls[0].payload.Content)
		assert.Equal(t, "https://avatar.url", poster.calls[0].payload.AvatarURL)
	}
}

func TestFlushBatch_MultipleMessages_BatchesIntoCombinedPost(t *testing.T) {
	poster := &captureCallsPoster{}
	mgr := &Manager{poster: poster}

	batch := []RelayPayload{
		{Content: "hello", Username: "alice [Kick]"},
		{Content: "world", Username: "bob [Twitch]"},
		{Content: "sup", Username: "charlie [Kick]"},
	}
	mgr.flushBatch(context.Background(), "https://webhook/url", batch, "overlay-1")

	if assert.Len(t, poster.calls, 1) {
		assert.Equal(t, batchWebhookUsername, poster.calls[0].payload.Username)
		assert.Contains(t, poster.calls[0].payload.Content, "**alice [Kick]**: hello")
		assert.Contains(t, poster.calls[0].payload.Content, "**bob [Twitch]**: world")
		assert.Contains(t, poster.calls[0].payload.Content, "**charlie [Kick]**: sup")
		assert.Empty(t, poster.calls[0].payload.AvatarURL, "batched posts should not have avatar")
	}
}

func TestFlushBatch_SplitsWhenContentExceedsLimit(t *testing.T) {
	poster := &captureCallsPoster{}
	mgr := &Manager{poster: poster}

	// Build messages that together exceed maxBatchContentLen
	longText := ""
	for i := 0; i < 200; i++ {
		longText += "x"
	}
	var batch []RelayPayload
	for i := 0; i < 20; i++ {
		batch = append(batch, RelayPayload{
			Content:  longText,
			Username: "user [Kick]",
		})
	}
	mgr.flushBatch(context.Background(), "https://webhook/url", batch, "overlay-1")

	assert.Greater(t, len(poster.calls), 1, "should split into multiple POSTs when content exceeds limit")
	for _, call := range poster.calls {
		assert.LessOrEqual(t, len(call.payload.Content), maxBatchContentLen+200, "each POST should stay near the content limit")
	}
}

func TestFlushBatch_EmptyBatch_NoOp(t *testing.T) {
	poster := &captureCallsPoster{}
	mgr := &Manager{poster: poster}

	mgr.flushBatch(context.Background(), "https://webhook/url", nil, "overlay-1")
	assert.Len(t, poster.calls, 0)
}

func TestParseRelayPayload_DiscordFiltered(t *testing.T) {
	mgr := &Manager{}
	payload := `{"platform":"discord","user":{"username":"bot"},"message":{"text":"hi"}}`
	_, ok := mgr.parseRelayPayload(payload, "overlay-1")
	assert.False(t, ok, "discord messages should be filtered")
}

func TestParseRelayPayload_KickParsed(t *testing.T) {
	mgr := &Manager{}
	payload := `{"platform":"kick","user":{"username":"alice","display_name":"Alice","avatar_url":"https://av.url"},"message":{"text":"hello"}}`
	rp, ok := mgr.parseRelayPayload(payload, "overlay-1")
	assert.True(t, ok)
	assert.Equal(t, "Alice [Kick]", rp.Username)
	assert.Equal(t, "hello", rp.Content)
	assert.Equal(t, "https://av.url", rp.AvatarURL)
}
