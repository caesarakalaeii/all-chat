package clients

import (
	"context"

	"github.com/caesar/all-chat/services/emote-service/models"
)

// EmoteClient is the interface for fetching emotes from external providers
type EmoteClient interface {
	// FetchEmotes fetches emotes for a given channel
	FetchEmotes(ctx context.Context, channel string) ([]models.Emote, error)

	// Provider returns the provider name
	Provider() string
}
