// This file is part of All-Chat.
// Copyright (C) 2026 caesarakalaeii
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

package dedup

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/caesar/all-chat/shared/sendall"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

const (
	// TTL for deduplication keys (30 seconds should be enough)
	dedupTTL = 30 * time.Second

	// Key prefix for deduplication
	dedupPrefix = "dedup:message"

	// nativeDedupPrefix namespaces deduplication by platform-native message id.
	nativeDedupPrefix = "dedup:native"

	// nativeDedupTTL bounds how long a native message id is remembered. It only needs to cover
	// the windows where the same native message can be ingested twice: the brief IRC↔EventSub
	// handoff overlap (≤ one IRC sync) and Twitch webhook retries (seconds). 2 minutes is ample.
	nativeDedupTTL = 2 * time.Minute
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

// IsDuplicateNativeID reports whether a message with this platform-native message id has already
// been seen within nativeDedupTTL, atomically recording it if not (SETNX). Both the IRC parser and
// the EventSub webhook handler stamp the identical native Twitch message id into Tags["id"], so this
// collapses the unavoidable IRC↔EventSub handoff overlap (and Twitch webhook retries) to a single
// delivered message. An empty id is never a duplicate. Fails OPEN on a Redis error (treats the
// message as new) so a Redis blip can never drop a real message — at worst a duplicate slips through.
func (d *Deduplicator) IsDuplicateNativeID(ctx context.Context, platform, nativeID string) (bool, error) {
	if nativeID == "" {
		return false, nil
	}
	key := fmt.Sprintf("%s:%s:%s", nativeDedupPrefix, platform, nativeID)
	wasSet, err := d.client.SetNX(ctx, key, "1", nativeDedupTTL).Result()
	if err != nil {
		d.logger.Error("native-id dedup SetNX failed, failing open", zap.Error(err))
		return false, err
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

// IsDuplicateForOverlay checks if message is duplicate for specific overlay
// Includes overlayID in fingerprint to isolate deduplication per overlay
func (d *Deduplicator) IsDuplicateForOverlay(ctx context.Context, overlayID, platform, channelID, messageID, userID, text string, timestamp time.Time) (bool, error) {
	fingerprint := d.createFingerprintWithOverlay(overlayID, platform, channelID, messageID, userID, text, timestamp)

	key := fmt.Sprintf("%s:%s", dedupPrefix, fingerprint)

	// SetNX returns true if key was set (not duplicate), false if key exists (duplicate)
	wasSet, err := d.client.SetNX(ctx, key, "1", 5*time.Second).Result()
	if err != nil {
		d.logger.Error("Redis SetNX failed, failing open", zap.Error(err))
		return false, nil // Fail open - allow message through on error
	}

	return !wasSet, nil // If wasSet is false, key existed = duplicate
}

func (d *Deduplicator) createFingerprintWithOverlay(overlayID, platform, channelID, messageID, userID, text string, timestamp time.Time) string {
	// Truncate timestamp to second for time window
	truncatedTime := timestamp.Truncate(time.Second).Unix()

	// Include platform-specific message ID if available (empty string if not)
	message := fmt.Sprintf("%s|%s|%s|%s|%s|%s|%d", overlayID, platform, channelID, messageID, userID, text, truncatedTime)

	hash := sha256.Sum256([]byte(message))
	return hex.EncodeToString(hash[:])
}

// ClearForOverlay removes a message from the deduplication cache for a specific overlay (for testing/debugging)
func (d *Deduplicator) ClearForOverlay(ctx context.Context, overlayID, platform, channelID, messageID, userID, text string, timestamp time.Time) error {
	fingerprint := d.createFingerprintWithOverlay(overlayID, platform, channelID, messageID, userID, text, timestamp)
	key := fmt.Sprintf("%s:%s", dedupPrefix, fingerprint)

	return d.client.Del(ctx, key).Err()
}

// LookupSendAllGroup checks whether an incoming message matches a streamer "send to
// all" pre-registration written by auth-service just before fan-out (keyed on the
// platform, the streamer's platform-native sender id, and the normalized text). On a
// match it returns the shared group (id + full platform set) so the echo can be
// collapsed into one combined-pill message. Returns nil (treat as an ordinary message)
// when there is no registration, and fails to nil on any Redis/parse error.
func (d *Deduplicator) LookupSendAllGroup(ctx context.Context, platform, senderID, text string) (*sendall.Registration, error) {
	if senderID == "" || text == "" {
		return nil, nil
	}
	val, err := d.client.Get(ctx, sendall.Key(platform, senderID, text)).Result()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var reg sendall.Registration
	if err := json.Unmarshal([]byte(val), &reg); err != nil {
		return nil, err
	}
	if reg.GroupID == "" {
		return nil, nil
	}
	return &reg, nil
}

// ClaimSendAllOverlay reports whether THIS echo is the first of its send-to-all group
// to reach the given overlay (atomic SETNX). The winner publishes the combined-pill
// message; losing siblings are dropped so the overlay shows one message, not N. Fails
// open (returns true) on a Redis error so a blip never silently drops the message.
func (d *Deduplicator) ClaimSendAllOverlay(ctx context.Context, overlayID, groupID string) (bool, error) {
	wasSet, err := d.client.SetNX(ctx, sendall.PublishedKey(overlayID, groupID), "1", sendall.TTL).Result()
	if err != nil {
		d.logger.Error("sendall overlay claim SetNX failed, failing open", zap.Error(err))
		return true, err
	}
	return wasSet, nil
}
