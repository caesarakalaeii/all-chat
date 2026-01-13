package streams

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// TokenStore persists streamList page tokens for reconnects.
type TokenStore struct {
	redis  *redis.Client
	logger *zap.Logger
	ttl    time.Duration
}

func NewTokenStore(redis *redis.Client, logger *zap.Logger) *TokenStore {
	return &TokenStore{
		redis:  redis,
		logger: logger,
		ttl:    6 * time.Hour,
	}
}

func (s *TokenStore) Get(ctx context.Context, liveChatID string) (string, error) {
	key := fmt.Sprintf("youtube:streamlist:token:%s", liveChatID)
	token, err := s.redis.Get(ctx, key).Result()
	if err == redis.Nil {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("failed to load streamList token: %w", err)
	}
	return token, nil
}

func (s *TokenStore) Set(ctx context.Context, liveChatID, token string) error {
	if token == "" {
		return nil
	}
	key := fmt.Sprintf("youtube:streamlist:token:%s", liveChatID)
	if err := s.redis.Set(ctx, key, token, s.ttl).Err(); err != nil {
		return fmt.Errorf("failed to save streamList token: %w", err)
	}
	return nil
}

func (s *TokenStore) Clear(ctx context.Context, liveChatID string) {
	key := fmt.Sprintf("youtube:streamlist:token:%s", liveChatID)
	if err := s.redis.Del(ctx, key).Err(); err != nil {
		s.logger.Warn("Failed to clear streamList token",
			zap.String("live_chat_id", liveChatID),
			zap.Error(err),
		)
	}
}
