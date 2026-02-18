package registry

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/caesar/all-chat/services/message-processor/models"
	"github.com/redis/go-redis/v9"
)

// DeletionBuffer stores deletion events for messages not yet in registry
type DeletionBuffer interface {
	Add(ctx context.Context, platform, channelID, platformMsgID string, event *models.RawChatMessage) error
	Get(ctx context.Context, platform, channelID, platformMsgID string) (*models.RawChatMessage, error)
	Remove(ctx context.Context, platform, channelID, platformMsgID string) error
}

// RedisDeletionBuffer implements DeletionBuffer using Redis hashes
type RedisDeletionBuffer struct {
	client *redis.Client
	ttl    time.Duration // 60 seconds per requirements
}

// NewRedisDeletionBuffer creates a new Redis-backed deletion buffer
func NewRedisDeletionBuffer(client *redis.Client, ttl time.Duration) *RedisDeletionBuffer {
	return &RedisDeletionBuffer{
		client: client,
		ttl:    ttl,
	}
}

// Add stores a deletion event in buffer with TTL
func (b *RedisDeletionBuffer) Add(ctx context.Context, platform, channelID, platformMsgID string, event *models.RawChatMessage) error {
	key := fmt.Sprintf("msgid:deletion_buffer:%s:%s:%s", platform, channelID, platformMsgID)

	// Serialize event to JSON
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal deletion event: %w", err)
	}

	// Store with TTL (60 seconds)
	if err := b.client.Set(ctx, key, data, b.ttl).Err(); err != nil {
		return fmt.Errorf("failed to store deletion event: %w", err)
	}

	return nil
}

// Get retrieves a buffered deletion event (returns nil if not found)
func (b *RedisDeletionBuffer) Get(ctx context.Context, platform, channelID, platformMsgID string) (*models.RawChatMessage, error) {
	key := fmt.Sprintf("msgid:deletion_buffer:%s:%s:%s", platform, channelID, platformMsgID)

	data, err := b.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return nil, nil // Not found - not an error
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get deletion event: %w", err)
	}

	var event models.RawChatMessage
	if err := json.Unmarshal([]byte(data), &event); err != nil {
		return nil, fmt.Errorf("failed to unmarshal deletion event: %w", err)
	}

	return &event, nil
}

// Remove deletes a buffered deletion event
func (b *RedisDeletionBuffer) Remove(ctx context.Context, platform, channelID, platformMsgID string) error {
	key := fmt.Sprintf("msgid:deletion_buffer:%s:%s:%s", platform, channelID, platformMsgID)
	return b.client.Del(ctx, key).Err()
}
