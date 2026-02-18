package replay

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// DeletionEvent represents a deletion event for replay buffer
// Matches Phase 1 deletion event structure from message-processor
type DeletionEvent struct {
	Type         string    `json:"type"`                    // "single", "batch", "clear"
	TargetUUID   string    `json:"target_uuid,omitempty"`   // For single deletions
	TargetUserID string    `json:"target_user_id,omitempty"` // For batch deletions
	Platform     string    `json:"platform"`
	Timestamp    time.Time `json:"timestamp"`
}

// DeletionReplayBuffer stores deletion events for replay on reconnect
type DeletionReplayBuffer interface {
	// Add stores deletion event with timestamp score
	Add(ctx context.Context, overlayID string, deletion *DeletionEvent) error

	// GetSince retrieves all deletion events after given timestamp (exclusive)
	GetSince(ctx context.Context, overlayID string, sinceTimestamp int64) ([]*DeletionEvent, error)

	// Prune removes events older than threshold (backup to TTL)
	Prune(ctx context.Context, overlayID string, olderThan int64) error
}

// RedisDeletionReplayBuffer implements DeletionReplayBuffer using Redis sorted sets
type RedisDeletionReplayBuffer struct {
	client *redis.Client
	ttl    time.Duration // 60 seconds default
}

// NewRedisDeletionReplayBuffer creates a new Redis-backed replay buffer
func NewRedisDeletionReplayBuffer(client *redis.Client, ttl time.Duration) *RedisDeletionReplayBuffer {
	return &RedisDeletionReplayBuffer{
		client: client,
		ttl:    ttl,
	}
}

// Add stores deletion event with timestamp score
func (b *RedisDeletionReplayBuffer) Add(ctx context.Context, overlayID string, deletion *DeletionEvent) error {
	key := fmt.Sprintf("replay:deletions:%s", overlayID)

	// Serialize to JSON
	data, err := json.Marshal(deletion)
	if err != nil {
		return fmt.Errorf("failed to marshal deletion event: %w", err)
	}

	// Use millisecond timestamp as score for precise ordering
	score := float64(deletion.Timestamp.UnixMilli())

	// ZADD + EXPIRE in pipeline to ensure atomicity
	pipe := b.client.Pipeline()
	pipe.ZAdd(ctx, key, redis.Z{
		Score:  score,
		Member: string(data),
	})
	pipe.Expire(ctx, key, b.ttl) // Refresh TTL on each add
	_, err = pipe.Exec(ctx)

	return err
}

// GetSince retrieves all deletion events after given timestamp (exclusive)
func (b *RedisDeletionReplayBuffer) GetSince(ctx context.Context, overlayID string, sinceTimestamp int64) ([]*DeletionEvent, error) {
	key := fmt.Sprintf("replay:deletions:%s", overlayID)

	// Query range: (sinceTimestamp, +inf) - exclusive lower bound to prevent duplicates
	results, err := b.client.ZRangeByScore(ctx, key, &redis.ZRangeBy{
		Min: fmt.Sprintf("(%d", sinceTimestamp), // Parenthesis = exclusive
		Max: "+inf",
	}).Result()

	if err == redis.Nil {
		return []*DeletionEvent{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query replay buffer: %w", err)
	}

	// Deserialize all events
	events := make([]*DeletionEvent, 0, len(results))
	for _, data := range results {
		var event DeletionEvent
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			// Log error but continue processing other events (resilient parsing)
			continue
		}
		events = append(events, &event)
	}

	return events, nil
}

// Prune removes events older than threshold (backup to automatic TTL)
func (b *RedisDeletionReplayBuffer) Prune(ctx context.Context, overlayID string, olderThan int64) error {
	key := fmt.Sprintf("replay:deletions:%s", overlayID)
	return b.client.ZRemRangeByScore(ctx, key, "-inf", fmt.Sprintf("%d", olderThan)).Err()
}
