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

// Package quota reserves YouTube Data API quota for moderation bans against the SAME
// atomic counter the youtube-listener uses (the reserve/confirm/rollback_youtube_quota
// SQL functions, migration 008, on the shared youtube_quota_usage table). It is not the
// full youtube-listener Tracker — no in-memory state, no background reset/audit loops,
// just the three atomic SQL calls keyed on the current Pacific date (YouTube resets
// quota at midnight PT). This keeps an infrequent moderation ban (50 units) from pushing
// the listener over its daily limit without coupling to the Tracker's lifecycle.
package quota

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// QuotaCostBan is the Data API cost of liveChatBans.insert.
const QuotaCostBan = 50

// DefaultDailyLimit mirrors youtube-listener's default (QUOTA_LIMIT_DAILY) so a shared
// reservation check uses the same ceiling.
const DefaultDailyLimit = 1009000

// Querier is the subset of *pgxpool.Pool the reserver needs (kept small for testing).
type Querier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// Reserver implements reserve-confirm-rollback (ADR-0006) over the shared SQL functions.
type Reserver struct {
	db         Querier
	dailyLimit int
	loc        *time.Location
}

// NewReserver builds a reserver. A non-positive dailyLimit falls back to the default.
// If the Pacific tz database is unavailable it falls back to UTC (the daily boundary
// may then be a few hours off, which only matters for a ban issued near midnight PT).
func NewReserver(db Querier, dailyLimit int) *Reserver {
	if dailyLimit <= 0 {
		dailyLimit = DefaultDailyLimit
	}
	loc, err := time.LoadLocation("America/Los_Angeles")
	if err != nil {
		loc = time.UTC
	}
	return &Reserver{db: db, dailyLimit: dailyLimit, loc: loc}
}

// today is the current date in YouTube's reset timezone, formatted for the DATE param.
func (r *Reserver) today() string {
	return time.Now().In(r.loc).Format("2006-01-02")
}

// Reserve atomically reserves units for today. Returns false (no error) when the daily
// limit would be exceeded — the caller must not make the API call.
func (r *Reserver) Reserve(ctx context.Context, units int) (bool, error) {
	var ok bool
	if err := r.db.QueryRow(ctx, `SELECT reserve_youtube_quota($1, $2, $3)`, r.today(), units, r.dailyLimit).Scan(&ok); err != nil {
		return false, fmt.Errorf("reserve youtube quota: %w", err)
	}
	return ok, nil
}

// Confirm moves reserved units to used after a successful API call.
func (r *Reserver) Confirm(ctx context.Context, units int) error {
	if _, err := r.db.Exec(ctx, `SELECT confirm_youtube_quota($1, $2)`, r.today(), units); err != nil {
		return fmt.Errorf("confirm youtube quota: %w", err)
	}
	return nil
}

// Rollback releases reserved units after a failed API call.
func (r *Reserver) Rollback(ctx context.Context, units int) error {
	if _, err := r.db.Exec(ctx, `SELECT rollback_youtube_quota($1, $2)`, r.today(), units); err != nil {
		return fmt.Errorf("rollback youtube quota: %w", err)
	}
	return nil
}
