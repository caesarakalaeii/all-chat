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
