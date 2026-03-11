package jobs

import (
	"github.com/caesar/all-chat/services/share-service/repository"
	"go.uber.org/zap"
)

// StreamEndEvent represents a stream lifecycle end event received from Redis.
// Used to expire shares with expiry_option='this_stream' for the given user.
type StreamEndEvent struct {
	Platform      string `json:"platform"`
	UserID        string `json:"user_id"`
	BroadcasterID string `json:"broadcaster_id"`
}

// LifecycleSubscriber listens for stream lifecycle events and expires
// shares with expiry_option='this_stream' when a stream ends.
// Wave 2 implementation: full Redis Pub/Sub subscription logic.
type LifecycleSubscriber struct {
	repo   *repository.ShareRepository
	logger *zap.SugaredLogger
}

// NewLifecycleSubscriber creates a new lifecycle subscriber stub.
// The redisClient parameter is reserved for Wave 2 Redis integration.
func NewLifecycleSubscriber(redisClient interface{}, repo *repository.ShareRepository, logger *zap.SugaredLogger) *LifecycleSubscriber {
	return &LifecycleSubscriber{
		repo:   repo,
		logger: logger,
	}
}
