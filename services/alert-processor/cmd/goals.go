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
	"strconv"
	"time"

	"github.com/caesar/all-chat/services/alert-processor/goals"
	"github.com/caesar/all-chat/services/moderation-service/tokens"
	"github.com/caesar/all-chat/shared/encryption"
	"github.com/caesar/all-chat/shared/quota"
	"github.com/caesar/all-chat/shared/youtubetoken"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// goalPollInterval is GOAL_POLL_SECONDS (default 60): how often a connected
// follower/subscriber goal is polled (ADR-0072).
func goalPollInterval() time.Duration {
	if n, err := strconv.Atoi(getEnv("GOAL_POLL_SECONDS", "")); err == nil && n > 0 {
		return time.Duration(n) * time.Second
	}
	return 60 * time.Second
}

// goalCounters builds one Counter per platform whose credentials are
// configured. Without the token cipher no owner token can be read, so no
// platform is polled; goal bars then show their last stored count.
func goalCounters(pool *pgxpool.Pool, log *zap.Logger) map[string]goals.Counter {
	counters := map[string]goals.Counter{}
	cipher, err := encryption.NewMultiKeyEncryptorFromEnvWithLogger(log)
	if err != nil {
		log.Warn("Goal polling disabled: token cipher unavailable (set TOKEN_ENCRYPTION_KEY_V1)", zap.Error(err))
		return counters
	}
	if id, secret := getEnv("TWITCH_CLIENT_ID", ""), getEnv("TWITCH_CLIENT_SECRET", ""); id != "" && secret != "" {
		counters["twitch"] = &goals.TwitchCounter{Tokens: tokens.NewTwitchSource(pool, cipher, id, secret), ClientID: id}
	} else {
		log.Info("Twitch follower goals not polled: TWITCH_CLIENT_ID/TWITCH_CLIENT_SECRET not set")
	}
	if id, secret := getEnv("YOUTUBE_CLIENT_ID", ""), getEnv("YOUTUBE_CLIENT_SECRET", ""); id != "" && secret != "" {
		limit, _ := strconv.Atoi(getEnv("YOUTUBE_QUOTA_LIMIT_DAILY", ""))
		counters["youtube"] = &goals.YouTubeCounter{
			Tokens: youtubetoken.NewYouTubeSource(pool, cipher, id, secret),
			Quota:  quota.NewReserver(pool, limit), // limit <= 0 falls back to quota.DefaultDailyLimit
		}
	} else {
		log.Info("YouTube subscriber goals not polled: YOUTUBE_CLIENT_ID/YOUTUBE_CLIENT_SECRET not set")
	}
	return counters
}
