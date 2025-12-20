package dedup

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

const (
	// TTL for deduplication keys (30 seconds should be enough)
	dedupTTL = 30 * time.Second

	// Key prefix for deduplication
	dedupPrefix = "dedup:message"
)

// Deduplicator provides message deduplication using Redis
type Deduplicator struct {
	client *redis.Client
	logger *zap.Logger
}

// NewDeduplicator creates a new message deduplicator
func NewDeduplicator(client *redis.Client, logger *zap.Logger) *Deduplicator {
	return &Deduplicator{
		client: client,
		logger: logger,
	}
}

// IsDuplicate checks if a message is a duplicate
// Returns true if the message was seen recently (within TTL)
func (d *Deduplicator) IsDuplicate(ctx context.Context, platform, channelID, userID, text string, timestamp time.Time) (bool, error) {
	// Create fingerprint from message components
	fingerprint := d.createFingerprint(platform, channelID, userID, text, timestamp)

	// Try to set key with NX (only if not exists)
	key := fmt.Sprintf("%s:%s", dedupPrefix, fingerprint)
	wasSet, err := d.client.SetNX(ctx, key, "1", dedupTTL).Result()
	if err != nil {
		d.logger.Error("Failed to check duplicate",
			zap.String("fingerprint", fingerprint),
			zap.Error(err),
		)
		// On error, assume not duplicate (fail open)
		return false, err
	}

	// If wasSet is false, key already existed = duplicate
	if !wasSet {
		d.logger.Debug("Duplicate message detected",
			zap.String("platform", platform),
			zap.String("channel", channelID),
			zap.String("user", userID),
			zap.String("fingerprint", fingerprint[:16]+"..."),
		)
	}

	return !wasSet, nil
}

// createFingerprint creates a unique fingerprint for a message
// Format: SHA256(platform|channel|user|text|timestamp_truncated)
func (d *Deduplicator) createFingerprint(platform, channelID, userID, text string, timestamp time.Time) string {
	// Truncate timestamp to seconds to handle slight timing variations
	truncatedTime := timestamp.Truncate(time.Second).Unix()

	// Create message string
	message := fmt.Sprintf("%s|%s|%s|%s|%d",
		platform,
		channelID,
		userID,
		text,
		truncatedTime,
	)

	// Hash it
	hash := sha256.Sum256([]byte(message))
	return hex.EncodeToString(hash[:])
}

// MarkAsProcessed explicitly marks a message as processed (alternative to IsDuplicate)
func (d *Deduplicator) MarkAsProcessed(ctx context.Context, platform, channelID, userID, text string, timestamp time.Time) error {
	fingerprint := d.createFingerprint(platform, channelID, userID, text, timestamp)
	key := fmt.Sprintf("%s:%s", dedupPrefix, fingerprint)

	return d.client.Set(ctx, key, "1", dedupTTL).Err()
}

// Clear removes a message from the deduplication cache (for testing/debugging)
func (d *Deduplicator) Clear(ctx context.Context, platform, channelID, userID, text string, timestamp time.Time) error {
	fingerprint := d.createFingerprint(platform, channelID, userID, text, timestamp)
	key := fmt.Sprintf("%s:%s", dedupPrefix, fingerprint)

	return d.client.Del(ctx, key).Err()
}
