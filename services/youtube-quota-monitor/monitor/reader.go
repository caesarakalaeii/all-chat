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

package monitor

import (
	"context"
	"fmt"
	"time"

	"github.com/caesar/all-chat/shared/quota"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Snapshot is a point-in-time view of today's shared YouTube quota usage.
type Snapshot struct {
	Used       int     // confirmed units used today
	Reserved   int     // in-flight (reserved, not yet confirmed) units
	Available  int     // limit - used - reserved
	Limit      int     // the daily limit recorded on today's row
	Percentage float64 // (used + reserved) / limit * 100
}

// QuotaReader reads the current quota snapshot. Implemented by pgxReader against the
// shared youtube_quota_usage table; faked in tests.
type QuotaReader interface {
	Read(ctx context.Context) (Snapshot, error)
}

type pgxReader struct {
	db  *pgxpool.Pool
	loc *time.Location
}

// NewPgxReader builds a reader over the shared database. Quota resets at midnight PT,
// so it queries the row keyed on the current Pacific date.
func NewPgxReader(db *pgxpool.Pool) *pgxReader {
	return &pgxReader{db: db, loc: quota.Pacific()}
}

// Read returns today's quota snapshot via the get_youtube_quota_with_reserved SQL
// function (migration 008), which always returns exactly one row (zeros when no
// reservation has happened yet today). percentage is cast to float8 so it scans into
// a Go float64 — pgx v5 will not scan a raw NUMERIC into float64.
func (r *pgxReader) Read(ctx context.Context) (Snapshot, error) {
	today := time.Now().In(r.loc).Format("2006-01-02")
	var s Snapshot
	err := r.db.QueryRow(ctx,
		`SELECT used, reserved, available, limit_val, percentage::float8 FROM get_youtube_quota_with_reserved($1)`,
		today,
	).Scan(&s.Used, &s.Reserved, &s.Available, &s.Limit, &s.Percentage)
	if err != nil {
		return Snapshot{}, fmt.Errorf("read youtube quota: %w", err)
	}
	return s, nil
}
