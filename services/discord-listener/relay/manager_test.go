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
	channelID string
	content   string
}

func (p *captureCallsPoster) Post(_ context.Context, channelID, content string) error {
	p.calls = append(p.calls, postCall{channelID: channelID, content: content})
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

	err := mgr.HandleMessage(context.Background(), "discord", "someuser", "hello", "overlay-123", "relay-chan-id")
	assert.NoError(t, err)
	assert.Len(t, poster.calls, 0, "platform=discord must never call poster.Post")
}

func TestRelayManager_NonDiscordRelayed(t *testing.T) {
	poster := &captureCallsPoster{}
	mgr := &Manager{
		poster: poster,
	}

	relayChannelID := "relay-chan-999"
	err := mgr.HandleMessage(context.Background(), "twitch", "alice", "hello", "overlay-123", relayChannelID)
	assert.NoError(t, err)

	if assert.Len(t, poster.calls, 1, "platform=twitch must call poster.Post exactly once") {
		assert.Equal(t, relayChannelID, poster.calls[0].channelID)
		assert.Equal(t, "🟣 alice: hello", poster.calls[0].content)
	}
}
