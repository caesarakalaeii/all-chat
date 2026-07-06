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

// Package repository holds every database query and transaction the engagement
// service runs against the shared All-Chat database (issue #523). Per ADR-0004
// the handful of authorization queries it needs are duplicated here rather than
// shared through an interface package; they mirror the moderation service.
package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/caesar/all-chat/services/engagement-service/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// sharedOverlayPlatform is excluded from engagement: a shared overlay mirrors
// another streamer's chat and must not drive that streamer's polls/points.
const sharedOverlayPlatform = "shared_overlay"

// Sentinel errors surfaced to handlers/consumers so they can map to HTTP codes
// or silently drop (the chat path must never spam platform chat).
var (
	ErrInsufficientBalance = errors.New("insufficient balance")
	ErrNotFound            = errors.New("not found")
	ErrConflict            = errors.New("conflict: an active engagement already exists")
	ErrWrongState          = errors.New("wrong state for this transition")
	ErrAlreadyWagered      = errors.New("viewer already wagered on this prediction")
	ErrNativeNoPoints      = errors.New("twitch-native engagements do not use all-chat points")
)

// dbtx is the subset of pgxpool.Pool / pgx.Tx used by the ledger primitive, so a
// credit/debit can run either standalone (its own tx) or inside a larger tx
// (wager/payout). Both the pool and a transaction satisfy it.
type dbtx interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// Repository runs the engagement service's queries over a pgx pool.
type Repository struct {
	db *pgxpool.Pool
}

// New creates a Repository over the given pool.
func New(db *pgxpool.Pool) *Repository { return &Repository{db: db} }

// Pool exposes the underlying pool for health checks.
func (r *Repository) Pool() *pgxpool.Pool { return r.db }

// --- Authorization (mirrors moderation-service/repository) ---

// VerifyOverlayOwnership reports whether userID owns overlayID.
func (r *Repository) VerifyOverlayOwnership(ctx context.Context, overlayID, userID string) (bool, error) {
	const q = `SELECT EXISTS(SELECT 1 FROM overlays WHERE id = $1 AND user_id = $2)`
	var exists bool
	if err := r.db.QueryRow(ctx, q, overlayID, userID).Scan(&exists); err != nil {
		return false, fmt.Errorf("verify overlay ownership: %w", err)
	}
	return exists, nil
}

// OverlayOwner returns the user id that owns overlayID, or ErrNotFound. Used by the
// announcer to send the round announcement as the overlay's streamer.
func (r *Repository) OverlayOwner(ctx context.Context, overlayID uuid.UUID) (string, error) {
	var userID string
	err := r.db.QueryRow(ctx, `SELECT user_id FROM overlays WHERE id = $1`, overlayID).Scan(&userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("overlay owner: %w", err)
	}
	return userID, nil
}

// IsUserPremium reports whether the streamer user has premium access.
func (r *Repository) IsUserPremium(ctx context.Context, userID string) (bool, error) {
	const q = `SELECT is_premium FROM users WHERE id = $1`
	var premium bool
	if err := r.db.QueryRow(ctx, q, userID).Scan(&premium); err != nil {
		return false, fmt.Errorf("check user premium: %w", err)
	}
	return premium, nil
}

// ChannelRef is a platform source channel on an overlay.
type ChannelRef struct {
	Platform  string
	ChannelID string
}

// OverlaysForChannel returns the overlay ids that carry (platform, channelID) as
// a source. A single channel may feed several overlays. shared_overlay excluded.
func (r *Repository) OverlaysForChannel(ctx context.Context, platform, channelID string) ([]uuid.UUID, error) {
	// channel_id is stored with the streamer's original casing, but callers pass a
	// canonical lowercase login (the native-mirror producer lowercases the Twitch
	// broadcaster login, ADR-0029). Compare case-insensitively so a source stored as
	// "CaesarLP" still matches "caesarlp"; a functional index on LOWER(channel_id)
	// (migration 071) keeps this cheap.
	const q = `SELECT overlay_id FROM overlay_chat_sources
	           WHERE platform = $1 AND LOWER(channel_id) = LOWER($2) AND platform <> $3`
	rows, err := r.db.Query(ctx, q, platform, channelID, sharedOverlayPlatform)
	if err != nil {
		return nil, fmt.Errorf("overlays for channel: %w", err)
	}
	defer rows.Close()
	var out []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan overlay id: %w", err)
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// SourceChannelsForOverlay lists the non-shared source channels of an overlay, so
// the publisher can flag each as having an active engagement (for the hot path).
func (r *Repository) SourceChannelsForOverlay(ctx context.Context, overlayID uuid.UUID) ([]ChannelRef, error) {
	const q = `SELECT platform, channel_id FROM overlay_chat_sources
	           WHERE overlay_id = $1 AND platform <> $2`
	rows, err := r.db.Query(ctx, q, overlayID, sharedOverlayPlatform)
	if err != nil {
		return nil, fmt.Errorf("source channels: %w", err)
	}
	defer rows.Close()
	var out []ChannelRef
	for rows.Next() {
		var c ChannelRef
		if err := rows.Scan(&c.Platform, &c.ChannelID); err != nil {
			return nil, fmt.Errorf("scan channel ref: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// GetOrCreateViewerByPlatform resolves the durable viewer_id for (platform,
// platformUserID), creating the viewer + platform identity if absent. This is the
// universal identity path for chat-command voters (no login). It re-implements the
// auth-service resolver (ADR-0004: duplicate the small query rather than couple
// services) and is race-safe: a concurrent create loses the UNIQUE(platform,
// platform_user_id) insert and we re-read the winner (leaving a harmless orphan
// viewers row in the rare race).
func (r *Repository) GetOrCreateViewerByPlatform(ctx context.Context, platform, platformUserID string) (uuid.UUID, error) {
	var viewerID uuid.UUID
	err := r.db.QueryRow(ctx,
		`SELECT viewer_id FROM viewer_platform_identities WHERE platform = $1 AND platform_user_id = $2`,
		platform, platformUserID).Scan(&viewerID)
	if err == nil {
		return viewerID, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, fmt.Errorf("lookup viewer identity: %w", err)
	}

	var newID uuid.UUID
	if err := r.db.QueryRow(ctx, `INSERT INTO viewers DEFAULT VALUES RETURNING id`).Scan(&newID); err != nil {
		return uuid.Nil, fmt.Errorf("insert viewer: %w", err)
	}
	tag, err := r.db.Exec(ctx,
		`INSERT INTO viewer_platform_identities (viewer_id, platform, platform_user_id)
		 VALUES ($1, $2, $3) ON CONFLICT (platform, platform_user_id) DO NOTHING`,
		newID, platform, platformUserID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("insert viewer identity: %w", err)
	}
	if tag.RowsAffected() == 0 {
		// Lost the race: another create won. Re-read the winner.
		if err := r.db.QueryRow(ctx,
			`SELECT viewer_id FROM viewer_platform_identities WHERE platform = $1 AND platform_user_id = $2`,
			platform, platformUserID).Scan(&viewerID); err != nil {
			return uuid.Nil, fmt.Errorf("re-read viewer identity after race: %w", err)
		}
		return viewerID, nil
	}
	// Best-effort cosmetics row so downstream lookups don't miss (non-fatal).
	_, _ = r.db.Exec(ctx, `INSERT INTO viewer_cosmetics (viewer_id) VALUES ($1) ON CONFLICT DO NOTHING`, newID)
	return newID, nil
}

// --- Points ledger + balance ---

// applyLedger inserts an idempotent points_transactions row and moves the
// materialized balance using q (pool or tx). Returns applied=false (no error) when
// dedupKey already exists. For a debit (delta<0) it guards balance>=|delta| and
// returns ErrInsufficientBalance when it cannot debit. The debit path MUST run
// inside a transaction so an insufficient debit rolls back the ledger insert.
func (r *Repository) applyLedger(ctx context.Context, q dbtx, viewerID, overlayID uuid.UUID, delta int64, reason, refType string, refID *uuid.UUID, dedupKey string) (bool, error) {
	tag, err := q.Exec(ctx,
		`INSERT INTO points_transactions (viewer_id, overlay_id, delta, reason, ref_type, ref_id, dedup_key)
		 VALUES ($1,$2,$3,$4,$5,$6,$7) ON CONFLICT (dedup_key) DO NOTHING`,
		viewerID, overlayID, delta, reason, refType, refID, dedupKey)
	if err != nil {
		return false, fmt.Errorf("insert points txn: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return false, nil // idempotent: this delta was already applied
	}
	if delta >= 0 {
		if _, err := q.Exec(ctx,
			`INSERT INTO viewer_points (viewer_id, overlay_id, balance) VALUES ($1,$2,$3)
			 ON CONFLICT (viewer_id, overlay_id)
			 DO UPDATE SET balance = viewer_points.balance + EXCLUDED.balance, updated_at = NOW()`,
			viewerID, overlayID, delta); err != nil {
			return false, fmt.Errorf("credit balance: %w", err)
		}
		return true, nil
	}
	tag2, err := q.Exec(ctx,
		`UPDATE viewer_points SET balance = balance + $3, updated_at = NOW()
		 WHERE viewer_id = $1 AND overlay_id = $2 AND balance >= $4`,
		viewerID, overlayID, delta, -delta)
	if err != nil {
		return false, fmt.Errorf("debit balance: %w", err)
	}
	if tag2.RowsAffected() == 0 {
		return false, ErrInsufficientBalance
	}
	return true, nil
}

// AwardPoints applies a standalone credit/debit in its own transaction. Used by
// the earn engine and heartbeat path. Idempotent via dedupKey.
func (r *Repository) AwardPoints(ctx context.Context, viewerID, overlayID uuid.UUID, delta int64, reason, refType string, refID *uuid.UUID, dedupKey string) (bool, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return false, fmt.Errorf("begin award tx: %w", err)
	}
	defer tx.Rollback(ctx)
	applied, err := r.applyLedger(ctx, tx, viewerID, overlayID, delta, reason, refType, refID, dedupKey)
	if err != nil {
		return false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return false, fmt.Errorf("commit award tx: %w", err)
	}
	return applied, nil
}

// GetBalance returns the viewer's balance in an overlay economy (0 if none).
func (r *Repository) GetBalance(ctx context.Context, viewerID, overlayID uuid.UUID) (int64, error) {
	var bal int64
	err := r.db.QueryRow(ctx,
		`SELECT balance FROM viewer_points WHERE viewer_id = $1 AND overlay_id = $2`,
		viewerID, overlayID).Scan(&bal)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("get balance: %w", err)
	}
	return bal, nil
}

// --- Earn config ---

// GetEarnConfig returns the overlay's earn config, or built-in defaults if the
// overlay has no row yet (read stays cheap; defaults are not persisted on read).
func (r *Repository) GetEarnConfig(ctx context.Context, overlayID uuid.UUID) (models.EarnConfig, error) {
	var c models.EarnConfig
	c.OverlayID = overlayID
	err := r.db.QueryRow(ctx,
		`SELECT points_name, bits_multiplier, usd_multiplier, sub_high, sub_medium, sub_low,
		        gift_per_sub, chat_per_minute, watch_per_minute, enabled, announce_on_start
		 FROM points_earn_config WHERE overlay_id = $1`, overlayID).
		Scan(&c.PointsName, &c.BitsMultiplier, &c.USDMultiplier, &c.SubHigh, &c.SubMedium, &c.SubLow,
			&c.GiftPerSub, &c.ChatPerMinute, &c.WatchPerMinute, &c.Enabled, &c.AnnounceOnStart)
	if errors.Is(err, pgx.ErrNoRows) {
		return models.DefaultEarnConfig(overlayID), nil
	}
	if err != nil {
		return c, fmt.Errorf("get earn config: %w", err)
	}
	return c, nil
}

// UpsertEarnConfig writes the overlay's earn config (owner-only).
func (r *Repository) UpsertEarnConfig(ctx context.Context, c models.EarnConfig) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO points_earn_config
		   (overlay_id, points_name, bits_multiplier, usd_multiplier, sub_high, sub_medium, sub_low,
		    gift_per_sub, chat_per_minute, watch_per_minute, enabled, announce_on_start, updated_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,NOW())
		 ON CONFLICT (overlay_id) DO UPDATE SET
		   points_name=EXCLUDED.points_name, bits_multiplier=EXCLUDED.bits_multiplier,
		   usd_multiplier=EXCLUDED.usd_multiplier, sub_high=EXCLUDED.sub_high,
		   sub_medium=EXCLUDED.sub_medium, sub_low=EXCLUDED.sub_low, gift_per_sub=EXCLUDED.gift_per_sub,
		   chat_per_minute=EXCLUDED.chat_per_minute, watch_per_minute=EXCLUDED.watch_per_minute,
		   enabled=EXCLUDED.enabled, announce_on_start=EXCLUDED.announce_on_start, updated_at=NOW()`,
		c.OverlayID, c.PointsName, c.BitsMultiplier, c.USDMultiplier, c.SubHigh, c.SubMedium, c.SubLow,
		c.GiftPerSub, c.ChatPerMinute, c.WatchPerMinute, c.Enabled, c.AnnounceOnStart)
	if err != nil {
		return fmt.Errorf("upsert earn config: %w", err)
	}
	return nil
}
