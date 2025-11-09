package ports

import (
	"context"

	"github.com/caesar/all-chat/internal/chat-listener/core/domain"
)

// ChatService defines the interface for chat message processing
type ChatService interface {
	// Start begins listening to all active channels
	Start(ctx context.Context) error

	// Stop gracefully shuts down the service
	Stop() error

	// RefreshChannels reloads the list of active channels
	RefreshChannels(ctx context.Context) error

	// ProcessMessage enriches and publishes a chat message
	ProcessMessage(ctx context.Context, channel string, rawMessage interface{}) error
}

// ChannelRepository defines the interface for accessing channel data
type ChannelRepository interface {
	// GetActiveChannels retrieves all channels that should be monitored
	GetActiveChannels(ctx context.Context) ([]domain.ActiveChannel, error)
}

// EmoteClient defines the interface for fetching emotes
type EmoteClient interface {
	// GetChannelEmotes retrieves all emotes for a channel from all enabled providers
	GetChannelEmotes(ctx context.Context, channel string, enable7TV, enableBTTV, enableFFZ bool) ([]domain.Emote, error)
}

// Publisher defines the interface for publishing messages to Redis
type Publisher interface {
	// Publish sends a message to a Redis channel
	Publish(ctx context.Context, channel string, message interface{}) error
}

// IRCClient defines the interface for interacting with Twitch IRC
type IRCClient interface {
	// Connect establishes connection to Twitch IRC
	Connect() error

	// Disconnect closes the connection
	Disconnect() error

	// Join joins the specified channels
	Join(channels ...string)

	// Part leaves the specified channels
	Part(channels ...string)

	// OnMessage registers a callback for incoming messages
	OnMessage(callback func(channel, user, message string, tags map[string]string))
}
