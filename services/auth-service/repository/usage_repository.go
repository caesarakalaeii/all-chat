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

package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// rowQuerier is the slice of pgx used by UsageRepository. Narrowing it to
// QueryRow keeps the repository usable with a *pgxpool.Pool, a transaction, or
// a stub in tests.
type rowQuerier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// ActiveUserCounts holds the rolling-window active-user counts that define
// "actual product usage": distinct streamers whose overlay was genuinely
// connected within the window (DAU / WAU / MAU).
type ActiveUserCounts struct {
	Day   int `json:"day"`
	Week  int `json:"week"`
	Month int `json:"month"`
}

// UsageRepository answers product-usage questions over the operational tables.
// It is the single home of the "active user" definition so the admin dashboard
// and the Prometheus usage sampler can never drift apart.
type UsageRepository struct {
	db rowQuerier
}

// NewUsageRepository creates a usage repository over the given querier.
func NewUsageRepository(db rowQuerier) *UsageRepository {
	return &UsageRepository{db: db}
}

// activeUserCountsQuery counts distinct owners of a recently-connected overlay
// per rolling window in one round trip.
//
// Why overlays.last_connected_at defines activity:
//   - api-gateway bumps it on every demand-bearing WebSocket attach AND on each
//     heartbeat tick (~2min) while the overlay stays attached, so it tracks real
//     overlay usage rather than a login. Viewer "participate" sockets are
//     demand-free and deliberately do NOT bump it (see websocket.Manager).
//   - logins are not a usable proxy: users.updated_at is polluted by the daily
//     automated token refresh.
//
// Two exclusions keep the number honest:
//   - banned users never count as active.
//   - `last_connected_at > created_at` filters overlays that were created but
//     never opened. Migration 052 gave the column DEFAULT NOW(), so a fresh
//     overlay row starts with last_connected_at exactly equal to created_at
//     (same statement, same NOW()); any real attach bumps it strictly later.
//     Without this, merely creating an overlay would count as 30 days of usage
//     and the metric would just re-tell the sign-up story.
//
// The outer 30d bound keeps the scan to the window that feeds every bucket and
// lets idx_overlays_last_connected_at do the work.
const activeUserCountsQuery = `
	SELECT
		COUNT(DISTINCT o.user_id) FILTER (WHERE o.last_connected_at >= NOW() - INTERVAL '24 hours'),
		COUNT(DISTINCT o.user_id) FILTER (WHERE o.last_connected_at >= NOW() - INTERVAL '7 days'),
		COUNT(DISTINCT o.user_id) FILTER (WHERE o.last_connected_at >= NOW() - INTERVAL '30 days')
	FROM overlays o
	JOIN users u ON u.id = o.user_id
	WHERE u.is_banned = false
	  AND o.last_connected_at > o.created_at
	  AND o.last_connected_at >= NOW() - INTERVAL '30 days'
`

// ActiveUserCounts returns the number of distinct streamers who actually used an
// overlay in the last 24 hours, 7 days and 30 days.
func (r *UsageRepository) ActiveUserCounts(ctx context.Context) (ActiveUserCounts, error) {
	var counts ActiveUserCounts
	if err := r.db.QueryRow(ctx, activeUserCountsQuery).Scan(&counts.Day, &counts.Week, &counts.Month); err != nil {
		return ActiveUserCounts{}, fmt.Errorf("failed to count active users: %w", err)
	}
	return counts, nil
}
