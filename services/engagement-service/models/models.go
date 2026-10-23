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

// Package models holds the domain types for the engagement service: polls,
// predictions, the points ledger, and the WebSocket snapshot payloads published
// to overlays (issue #523).
package models

import (
	"time"

	"github.com/google/uuid"
)

// Source discriminates an All-Chat-native engagement from a mirrored Twitch one.
const (
	SourceAllChat      = "allchat"
	SourceTwitchNative = "twitch_native"
)

// Poll / prediction states.
const (
	PollActive = "ACTIVE"
	PollClosed = "CLOSED"

	PredCreated  = "CREATED"
	PredActive   = "ACTIVE"
	PredLocked   = "LOCKED"
	PredResolved = "RESOLVED"
	PredCanceled = "CANCELED"
)

// Poll is an All-Chat-native or mirrored poll.
type Poll struct {
	ID uuid.UUID `json:"id"`
	// OverlayID routes the broadcast (the publisher builds overlay:{id}:poll from
	// this Go field before marshaling) but is NEVER serialized: on a viewer WS
	// frame, the ?since= replay, or the public OBS/HTTP read it would leak the
	// overlay's bearer-capability id, which grants token-less OBS-feed access.
	OverlayID   uuid.UUID    `json:"-"`
	Source      string       `json:"source"`
	ExternalID  *string      `json:"external_id,omitempty"`
	Question    string       `json:"question"`
	State       string       `json:"state"`
	AllowChange bool         `json:"allow_change"`
	Options     []PollOption `json:"options"`
	CreatedAt   time.Time    `json:"created_at"`
	EndsAt      *time.Time   `json:"ends_at,omitempty"`
	ClosedAt    *time.Time   `json:"closed_at,omitempty"`
}

// PollOption is one choice on a poll, carrying its live vote count.
type PollOption struct {
	ID    uuid.UUID `json:"id"`
	Idx   int       `json:"idx"`
	Label string    `json:"label"`
	Votes int64     `json:"votes"`
}

// Prediction is an All-Chat-native or mirrored prediction.
type Prediction struct {
	ID uuid.UUID `json:"id"`
	// OverlayID routes the broadcast but is NEVER serialized — see Poll.OverlayID.
	OverlayID        uuid.UUID           `json:"-"`
	Source           string              `json:"source"`
	ExternalID       *string             `json:"external_id,omitempty"`
	Title            string              `json:"title"`
	State            string              `json:"state"`
	WinningOutcomeID *uuid.UUID          `json:"winning_outcome_id,omitempty"`
	Outcomes         []PredictionOutcome `json:"outcomes"`
	AutoLockAt       *time.Time          `json:"auto_lock_at,omitempty"`
	CreatedAt        time.Time           `json:"created_at"`
	LockedAt         *time.Time          `json:"locked_at,omitempty"`
	ResolvedAt       *time.Time          `json:"resolved_at,omitempty"`
}

// PredictionOutcome is one side of a prediction, with its live wagered pool.
type PredictionOutcome struct {
	ID       uuid.UUID `json:"id"`
	Idx      int       `json:"idx"`
	Label    string    `json:"label"`
	Color    *string   `json:"color,omitempty"`
	TotalPts int64     `json:"total_points"`
	Entrants int64     `json:"entrants"`
}

// EarnConfig is the per-overlay points earning configuration.
type EarnConfig struct {
	OverlayID      uuid.UUID `json:"overlay_id"`
	PointsName     string    `json:"points_name"`
	BitsMultiplier float64   `json:"bits_multiplier"`
	USDMultiplier  float64   `json:"usd_multiplier"`
	SubHigh        int64     `json:"sub_high"`
	SubMedium      int64     `json:"sub_medium"`
	SubLow         int64     `json:"sub_low"`
	GiftPerSub     int64     `json:"gift_per_sub"`
	ChatPerMinute  int64     `json:"chat_per_minute"`
	WatchPerMinute int64     `json:"watch_per_minute"`
	Enabled        bool      `json:"enabled"`
	// AnnounceOnStart posts the round question + numbered options + participate link
	// to chat when a poll/prediction opens. Opt-in (default false): it needs the
	// Twitch send scope (user:write:chat), reusing the moderation send path (ADR-0028).
	AnnounceOnStart bool `json:"announce_on_start"`
}

// DefaultEarnConfig returns the built-in defaults used when an overlay has no
// points_earn_config row yet. Mirrors the column defaults in migration 067 (kept
// in sync by the enabled-default migration). Points earning is OPT-IN: Enabled
// defaults false so no overlay silently accrues points (and names a currency)
// before the streamer turns it on.
func DefaultEarnConfig(overlayID uuid.UUID) EarnConfig {
	return EarnConfig{
		OverlayID:       overlayID,
		PointsName:      "Points",
		BitsMultiplier:  1,
		USDMultiplier:   100,
		SubHigh:         500,
		SubMedium:       300,
		SubLow:          150,
		GiftPerSub:      150,
		ChatPerMinute:   5,
		WatchPerMinute:  2,
		Enabled:         false,
		AnnounceOnStart: false,
	}
}

// Balance is a viewer's point balance within one overlay's economy.
type Balance struct {
	ViewerID  uuid.UUID `json:"-"`
	OverlayID uuid.UUID `json:"-"`
	Balance   int64     `json:"balance"`
}

// --- WebSocket snapshot payloads (published to overlay:{id}:poll / :prediction) ---

// PollSnapshot is the aggregate poll payload broadcast to an overlay. It carries
// no per-viewer data (broadcast is public). State conveys active vs ended.
type PollSnapshot struct {
	Poll       Poll  `json:"poll"`
	TotalVotes int64 `json:"total_votes"`
}

// PredictionSnapshot is the aggregate prediction payload broadcast to an overlay.
type PredictionSnapshot struct {
	Prediction Prediction `json:"prediction"`
	TotalPts   int64      `json:"total_points"`
}

// ViewerEngagement is the private per-viewer payload returned by the pull-first
// GET /viewers/me/engagement endpoint (balance + this viewer's current vote/wager).
type ViewerEngagement struct {
	PointsName    string     `json:"points_name"`
	Balance       int64      `json:"balance"`
	VotedOptionID *uuid.UUID `json:"voted_option_id,omitempty"`
	WagerOutcome  *uuid.UUID `json:"wager_outcome_id,omitempty"`
	WagerAmount   int64      `json:"wager_amount,omitempty"`
}

// --- Chat commands (ADR-0079) ---

// Chat command trigger kinds: a command fires when the message's first word equals
// the trigger ("!hello"); a keyword fires when the trigger appears anywhere in it.
const (
	TriggerCommand = "command"
	TriggerKeyword = "keyword"
)

// ChatCommand is one streamer-defined command or keyword trigger. It belongs to a
// user, not an overlay, and fires on that user's own channels.
type ChatCommand struct {
	ID              uuid.UUID `json:"id"`
	UserID          uuid.UUID `json:"-"`
	Kind            string    `json:"kind"`
	Trigger         string    `json:"trigger"`
	Response        string    `json:"response"`
	CreateClip      bool      `json:"create_clip"`
	ImageURL        string    `json:"image_url"` // template; shown on alerts overlays (ADR-0080)
	MinRole         string    `json:"min_role"`
	CooldownSeconds int       `json:"cooldown_seconds"`
	Enabled         bool      `json:"enabled"`
	UseCount        int64     `json:"use_count"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// ChatTimer is one streamer-defined recurring chat message (ADR-0081). It fires on
// the owner's own channels when its trigger is due: an interval elapsed since it
// last fired, and/or at least MinMessages chat lines since then (both set = every
// interval, but only when enough lines arrived). The reply is sent by the sender
// the owner chose in ChatBotSettings (auth-service resolves which account posts).
type ChatTimer struct {
	ID              uuid.UUID  `json:"id"`
	UserID          uuid.UUID  `json:"-"`
	Name            string     `json:"name"`
	Message         string     `json:"message"`
	IntervalMinutes *int       `json:"interval_minutes"` // nil = no interval (messages only)
	MinMessages     *int       `json:"min_messages"`     // nil = no line threshold (interval only)
	Enabled         bool       `json:"enabled"`
	LastFiredAt     *time.Time `json:"last_fired_at"`
	LinesAtLastFire int64      `json:"-"`
	FireCount       int64      `json:"fire_count"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

// EnabledChatTimer is one enabled timer as the ADR-0081 scheduler loads them per
// tick: the timer row plus the owner's premium flag (YouTube sends spend Data API
// quota, so only a premium owner's timers ask YouTube).
type EnabledChatTimer struct {
	Timer   ChatTimer `json:"timer"`
	Premium bool      `json:"premium"`
}
