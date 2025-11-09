package services

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/caesar/all-chat/internal/emote-service/adapters/clients"
	"github.com/caesar/all-chat/internal/emote-service/core/domain"
	"github.com/caesar/all-chat/internal/emote-service/core/ports"
	"github.com/caesar/all-chat/pkg/logger"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

type emoteService struct {
	client      *clients.EmoteClient
	redisClient *redis.Client
}

func NewEmoteService(client *clients.EmoteClient, redisClient *redis.Client) ports.EmoteService {
	return &emoteService{
		client:      client,
		redisClient: redisClient,
	}
}

func (s *emoteService) GetGlobalEmotes(ctx context.Context, provider string) ([]domain.EmoteResponse, error) {
	// Try cache first
	cacheKey := fmt.Sprintf("emotes:global:%s", provider)
	cached, err := s.redisClient.Get(ctx, cacheKey).Result()
	if err == nil {
		var emotes []domain.EmoteResponse
		if err := json.Unmarshal([]byte(cached), &emotes); err == nil {
			logger.Debug("Cache hit for global emotes", zap.String("provider", provider))
			return emotes, nil
		}
	}

	// Fetch from API
	var rawEmotes []domain.Emote
	switch provider {
	case "7tv":
		rawEmotes, err = s.client.FetchSevenTVGlobal(ctx)
	case "bttv":
		rawEmotes, err = s.client.FetchBTTVGlobal(ctx)
	case "ffz":
		rawEmotes, err = s.client.FetchFFZGlobal(ctx)
	default:
		return nil, fmt.Errorf("unknown provider: %s", provider)
	}

	if err != nil {
		return nil, err
	}

	// Convert to response format
	emotes := make([]domain.EmoteResponse, len(rawEmotes))
	for i, e := range rawEmotes {
		emotes[i] = domain.EmoteResponse{
			Code:     e.Code,
			Provider: e.Provider,
			URL:      e.URL,
			Animated: e.Animated,
		}
	}

	// Cache for 1 hour
	data, _ := json.Marshal(emotes)
	s.redisClient.Set(ctx, cacheKey, data, time.Hour)

	logger.Info("Fetched global emotes", zap.String("provider", provider), zap.Int("count", len(emotes)))

	return emotes, nil
}

func (s *emoteService) GetChannelEmotes(ctx context.Context, channel, provider string) ([]domain.EmoteResponse, error) {
	// Try cache first
	cacheKey := fmt.Sprintf("emotes:channel:%s:%s", provider, channel)
	cached, err := s.redisClient.Get(ctx, cacheKey).Result()
	if err == nil {
		var emotes []domain.EmoteResponse
		if err := json.Unmarshal([]byte(cached), &emotes); err == nil {
			logger.Debug("Cache hit for channel emotes", zap.String("provider", provider), zap.String("channel", channel))
			return emotes, nil
		}
	}

	// Fetch from API
	var rawEmotes []domain.Emote
	switch provider {
	case "7tv":
		rawEmotes, err = s.client.FetchSevenTVChannel(ctx, channel)
	case "bttv":
		rawEmotes, err = s.client.FetchBTTVChannel(ctx, channel)
	case "ffz":
		rawEmotes, err = s.client.FetchFFZChannel(ctx, channel)
	default:
		return nil, fmt.Errorf("unknown provider: %s", provider)
	}

	if err != nil {
		logger.Warn("Failed to fetch channel emotes", zap.String("provider", provider), zap.String("channel", channel), zap.Error(err))
		return []domain.EmoteResponse{}, nil // Return empty instead of error
	}

	// Convert to response format
	emotes := make([]domain.EmoteResponse, len(rawEmotes))
	for i, e := range rawEmotes {
		emotes[i] = domain.EmoteResponse{
			Code:     e.Code,
			Provider: e.Provider,
			URL:      e.URL,
			Animated: e.Animated,
		}
	}

	// Cache for 15 minutes
	data, _ := json.Marshal(emotes)
	s.redisClient.Set(ctx, cacheKey, data, 15*time.Minute)

	logger.Info("Fetched channel emotes", zap.String("provider", provider), zap.String("channel", channel), zap.Int("count", len(emotes)))

	return emotes, nil
}

func (s *emoteService) RefreshCache(ctx context.Context) error {
	// Refresh global emotes for all providers
	providers := []string{"7tv", "bttv", "ffz"}
	for _, provider := range providers {
		_, err := s.GetGlobalEmotes(ctx, provider)
		if err != nil {
			logger.Error("Failed to refresh global emotes", zap.String("provider", provider), zap.Error(err))
		}
	}
	return nil
}
