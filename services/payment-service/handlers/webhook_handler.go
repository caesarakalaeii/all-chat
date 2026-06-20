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

package handlers

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"time"

	"github.com/caesar/all-chat/services/payment-service/entitlement"
	"github.com/caesar/all-chat/services/payment-service/patreon"
	"github.com/caesar/all-chat/services/payment-service/repository"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// webhookDedupTTL is how long a processed webhook's dedup marker is retained.
const webhookDedupTTL = 24 * time.Hour

// WebhookHandler receives Patreon membership webhooks.
type WebhookHandler struct {
	secret      string
	redis       *redis.Client
	tokenRepo   *repository.TokenRepository
	entitlement *entitlement.Service
	logger      *zap.Logger
}

// NewWebhookHandler builds a WebhookHandler.
func NewWebhookHandler(secret string, rdb *redis.Client, tokenRepo *repository.TokenRepository, ent *entitlement.Service, logger *zap.Logger) *WebhookHandler {
	return &WebhookHandler{secret: secret, redis: rdb, tokenRepo: tokenRepo, entitlement: ent, logger: logger}
}

// Handle verifies, deduplicates, and applies a Patreon membership webhook.
func (h *WebhookHandler) Handle(c *gin.Context) {
	ctx := c.Request.Context()

	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read body"})
		return
	}

	// HMAC-MD5 over the raw body (Patreon's scheme). Constant-time inside.
	if !patreon.VerifyWebhookSignature(h.secret, body, c.GetHeader("X-Patreon-Signature")) {
		h.logger.Warn("Patreon webhook signature verification failed")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid signature"})
		return
	}

	event := c.GetHeader("X-Patreon-Event")
	if !patreon.IsMemberEvent(event) {
		// Acknowledge events we don't act on (posts:*, etc.) so Patreon stops retrying.
		c.JSON(http.StatusOK, gin.H{"status": "ignored", "event": event})
		return
	}

	// Idempotency: skip if we've already processed this exact body. The marker is
	// set only AFTER successful processing, so a failed attempt is retried.
	dedupKey := "patreon:webhook:" + sha256Hex(body)
	if exists, _ := h.redis.Exists(ctx, dedupKey).Result(); exists > 0 {
		c.JSON(http.StatusOK, gin.H{"status": "duplicate"})
		return
	}

	snap, err := patreon.ParseMemberEvent(body)
	if err != nil {
		h.logger.Error("Failed to parse Patreon webhook", zap.String("event", event), zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}

	// Resolve the all-chat user via the stored Patreon connection. A patron who
	// never connected through our flow has no row — store the subscription anyway
	// (user_id nil); it gets linked when they connect or on the next reconcile.
	var userID *string
	if uid, found, lookupErr := h.tokenRepo.GetByPatreonUserID(ctx, snap.PatreonUserID); lookupErr != nil {
		h.logger.Error("Failed to resolve user for webhook", zap.Error(lookupErr))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "lookup failed"})
		return
	} else if found {
		userID = &uid
	}

	status, isPremium, err := h.entitlement.Apply(ctx, snap, userID, body)
	if err != nil {
		h.logger.Error("Failed to apply Patreon webhook", zap.String("event", event), zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to apply"})
		return
	}

	// Mark processed (best-effort) only after success.
	if err := h.redis.Set(ctx, dedupKey, 1, webhookDedupTTL).Err(); err != nil {
		h.logger.Warn("Failed to set webhook dedup marker", zap.Error(err))
	}

	h.logger.Info("Processed Patreon webhook",
		zap.String("event", event),
		zap.String("patreon_user_id", snap.PatreonUserID),
		zap.String("status", status),
		zap.Bool("is_premium", isPremium))
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
