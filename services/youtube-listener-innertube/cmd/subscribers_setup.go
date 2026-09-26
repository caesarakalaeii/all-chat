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

package main

import (
	"context"
	"fmt"
	"os"
	"strconv"

	"github.com/caesar/all-chat/services/youtube-listener-innertube/publisher"
	"github.com/caesar/all-chat/services/youtube-listener-innertube/streams"
	"github.com/caesar/all-chat/services/youtube-listener-innertube/subscribers"
	"github.com/caesar/all-chat/shared/database"
	"github.com/caesar/all-chat/shared/encryption"
	"github.com/caesar/all-chat/shared/listener"
	"github.com/caesar/all-chat/shared/quota"
	"github.com/caesar/all-chat/shared/youtubetoken"
	"go.uber.org/zap"
)

// setupSubscribers wires the subscriber-alert loop into the stream manager.
// All credentials are env-gated: DATABASE_* (a pool), TOKEN_ENCRYPTION_KEY_V1
// (the cipher), and YOUTUBE_CLIENT_ID/SECRET (the refresh grant). Missing any
// one disables the feature with a single warning — chat ingestion is untouched.
//
// The quota reserver is deliberately required too: subscriber polls are the
// listener's first intentional official-API spend, and every unit goes
// through the shared youtube_quota_usage accounting (ADR-0006/0023).
func setupSubscribers(
	streamManager *streams.Manager,
	streamPublisher *publisher.StreamPublisher,
	_ interface{}, // redisClient reserved for future watermark persistence
	logger *zap.Logger,
) {
	clientID := listener.Env("YOUTUBE_CLIENT_ID", "")
	clientSecret := listener.Env("YOUTUBE_CLIENT_SECRET", "")
	cipher, cipherErr := encryption.NewMultiKeyEncryptorFromEnvWithLogger(logger)

	if clientID == "" || clientSecret == "" {
		logger.Info("Subscriber alerts disabled: YOUTUBE_CLIENT_ID/YOUTUBE_CLIENT_SECRET not set")
		return
	}
	if cipherErr != nil {
		logger.Warn("Subscriber alerts disabled: token cipher unavailable (set TOKEN_ENCRYPTION_KEY_V1)", zap.Error(cipherErr))
		return
	}

	dbPassword := listener.Env("DATABASE_PASSWORD", "")
	if dbPassword == "" {
		logger.Info("Subscriber alerts disabled: DATABASE_PASSWORD not set")
		return
	}
	connString := fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=disable",
		listener.Env("DATABASE_USER", "allchat"),
		dbPassword,
		listener.Env("DATABASE_HOST", "localhost"),
		listener.Env("DATABASE_PORT", "5432"),
		listener.Env("DATABASE_NAME", "allchat"),
	)
	pool, err := database.NewPostgresPool(connString)
	if err != nil {
		logger.Warn("Subscriber alerts disabled: database unavailable", zap.Error(err))
		return
	}

	quotaLimit := DefaultYouTubeQuotaLimit()
	reserver := quota.NewReserver(pool, quotaLimit)
	tokenSource := youtubetoken.NewYouTubeSource(pool, cipher, clientID, clientSecret)

	announcer := &subscribers.PublisherAnnouncer{
		Pub: &rawPublishAdapter{pub: streamPublisher},
	}
	runner := subscribers.NewRunner(tokenSource, subscribers.NewClient(), reserver, announcer, logger, subscribers.NewMetrics(nil))

	streamManager.SetSubscriberRunner(runner)
	logger.Info("Subscriber alerts enabled",
		zap.Int("quota_limit_daily", quotaLimit),
	)
}

// rawPublishAdapter bridges the subscribers.RawPublisher interface to
// StreamPublisher.PublishRaw.
type rawPublishAdapter struct {
	pub *publisher.StreamPublisher
}

func (a *rawPublishAdapter) PublishRaw(ctx context.Context, payload []byte) error {
	return a.pub.PublishRaw(ctx, payload)
}

// DefaultYouTubeQuotaLimit reads YOUTUBE_QUOTA_LIMIT_DAILY, falling back to
// the shared default (1,009,000 units/day).
func DefaultYouTubeQuotaLimit() int {
	if v := os.Getenv("YOUTUBE_QUOTA_LIMIT_DAILY"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return quota.DefaultDailyLimit
}

