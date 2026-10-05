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

// Package repository owns the alert-processor's Postgres queries: which
// alert-capable overlays an event routes to, and the alert_events write.
package repository

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Querier is the slice of *pgxpool.Pool the repository needs. Declared as a
// seam so tests can stub the database (see media-service/repository for why
// this is hand-rolled rather than pgxmock).
type Querier interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// AlertOverlay is one overlay an event routes to. OverlayType is carried so the
// processor can decide what else the overlay earns: only 'list' overlays feed
// leaderboards, while all three types receive the alert event itself.
type AlertOverlay struct {
	OverlayID   string
	UserID      string
	OverlayType string
}

// Repository is the Postgres implementation of the alert-processor's queries.
type Repository struct {
	db Querier
}

// NewRepository creates the alert-processor repository.
func NewRepository(db Querier) *Repository {
	return &Repository{db: db}
}