package ports

import (
	"context"

	"github.com/caesar/all-chat/internal/emote-service/core/domain"
)

type EmoteService interface {
	GetGlobalEmotes(ctx context.Context, provider string) ([]domain.EmoteResponse, error)
	GetChannelEmotes(ctx context.Context, channel, provider string) ([]domain.EmoteResponse, error)
	RefreshCache(ctx context.Context) error
}
