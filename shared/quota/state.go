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

// Package quota is the canonical YouTube Data API quota toolkit shared across
// services. It provides three things over the shared youtube_quota_usage table
// (migration 008):
//
//   - Reserver: atomic reserve-confirm-rollback accounting (ADR-0006), used by
//     every service that spends quota (moderation bans, auth-service sends).
//   - State machine: maps a usage percentage to a QuotaState, plus the QuotaEvent
//     wire format and a Notifier that publishes events to the "quota:alerts" Redis
//     channel the discord-bot renders.
//
// The youtube-quota-monitor service combines them: it reads the table and drives
// the state machine + Notifier + Prometheus metrics, since the quota-based
// youtube-listener that historically owned this is no longer deployed (ADR-0023).
package quota

import (
	"os"
	"strconv"
	"time"
)

const (
	// DefaultDailyQuota is the YouTube Data API daily limit (units/day).
	DefaultDailyQuota = 1009000
	// DefaultDailyLimit is an alias used by the Reserver's limit fallback.
	DefaultDailyLimit = DefaultDailyQuota

	// Quota cost per YouTube Data API operation.
	//
	// videos.list and search.list are from Google's published cost table. The three liveChat*
	// costs are NOT: that table documents no rows for the live-streaming write methods, so these
	// are inherited estimates, and the word "official" that used to head this block was a
	// provenance claim the repo could not support (ADR-0048). They are deliberately left on the
	// HIGH side — over-reserving only under-uses the daily allowance, while under-reserving would
	// let real usage run past a limit we believe we are respecting. The real numbers need a
	// Cloud Console measurement over a project-day.
	QuotaCostLiveChatMessages = 5   // liveChatMessages.stream (listener poll)
	QuotaCostVideos           = 1   // videos.list
	QuotaCostSearch           = 100 // search.list
	QuotaCostBan              = 50  // liveChatBans.insert — every ban type, so timeout costs the same
	QuotaCostYouTubeSend      = 5   // liveChatMessages.insert (auth-service send)

	// Default state thresholds, as lower-bound percentages of the daily limit.
	DefaultHealthyThreshold   = 70.0  // [0,70)   HEALTHY
	DefaultDegradedThreshold  = 85.0  // [70,85)  DEGRADED
	DefaultCriticalThreshold  = 95.0  // [85,95)  CRITICAL
	DefaultExhaustedThreshold = 100.0 // [95,100) EXHAUSTED ; [100,inf) DEPLETED
)

// QuotaState represents the current state of quota usage.
type QuotaState string

const (
	QuotaStateHealthy   QuotaState = "HEALTHY"   // normal operation
	QuotaStateDegraded  QuotaState = "DEGRADED"  // reduce low-priority operations
	QuotaStateCritical  QuotaState = "CRITICAL"  // active polling only
	QuotaStateExhausted QuotaState = "EXHAUSTED" // slow everything down
	QuotaStateDepleted  QuotaState = "DEPLETED"  // hard block all requests
)

// Thresholds are the lower-bound usage percentages for each elevated state.
type Thresholds struct {
	Healthy   float64
	Degraded  float64
	Critical  float64
	Exhausted float64
}

// DefaultThresholds reads QUOTA_*_THRESHOLD env vars, falling back to the
// documented defaults (70/85/95/100).
func DefaultThresholds() Thresholds {
	return Thresholds{
		Healthy:   getEnvAsFloat("QUOTA_HEALTHY_THRESHOLD", DefaultHealthyThreshold),
		Degraded:  getEnvAsFloat("QUOTA_DEGRADED_THRESHOLD", DefaultDegradedThreshold),
		Critical:  getEnvAsFloat("QUOTA_CRITICAL_THRESHOLD", DefaultCriticalThreshold),
		Exhausted: getEnvAsFloat("QUOTA_EXHAUSTED_THRESHOLD", DefaultExhaustedThreshold),
	}
}

// CalculateState maps a usage percentage to a QuotaState using the documented,
// contiguous ranges: HEALTHY [0,healthy) DEGRADED [healthy,degraded)
// CRITICAL [degraded,critical) EXHAUSTED [critical,exhausted) DEPLETED [exhausted,inf).
func CalculateState(percentage float64, t Thresholds) QuotaState {
	switch {
	case percentage >= t.Exhausted:
		return QuotaStateDepleted
	case percentage >= t.Critical:
		return QuotaStateExhausted
	case percentage >= t.Degraded:
		return QuotaStateCritical
	case percentage >= t.Healthy:
		return QuotaStateDegraded
	default:
		return QuotaStateHealthy
	}
}

// Pacific is the timezone where YouTube quota resets at midnight (America/Los_Angeles).
// Falls back to UTC if the tz database is unavailable (the daily boundary may then be
// a few hours off, which only matters for operations near midnight PT).
func Pacific() *time.Location {
	loc, err := time.LoadLocation("America/Los_Angeles")
	if err != nil {
		return time.UTC
	}
	return loc
}

func getEnvAsFloat(key string, def float64) float64 {
	if v := os.Getenv(key); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return def
}
