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
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/caesar/all-chat/services/alert-processor/models"
	mpmodels "github.com/caesar/all-chat/services/message-processor/models"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The fake querier seam below is the same hand-rolled stub as
// media-service/repository/media_repo_test.go: pgxmock does not compile against
// pgx v5.11.0 (pgx.Rows grew a TypeMap method) and testcontainers silently skip
// in the acceptance runner, which has no docker.

type recordedQuery struct {
	sql  string
	args []any
}

type fakeQuerier struct {
	queries []recordedQuery

	execErr error

	row  fakeRow
	rows fakeRows
}

func (f *fakeQuerier) Exec(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	f.queries = append(f.queries, recordedQuery{sql: sql, args: args})
	return pgconn.CommandTag{}, f.execErr
}

func (f *fakeQuerier) QueryRow(_ context.Context, sql string, args ...any) pgx.Row {
	f.queries = append(f.queries, recordedQuery{sql: sql, args: args})
	return &f.row
}

func (f *fakeQuerier) Query(_ context.Context, sql string, args ...any) (pgx.Rows, error) {
	f.queries = append(f.queries, recordedQuery{sql: sql, args: args})
	if f.rows.err != nil {
		return nil, f.rows.err
	}
	return &f.rows, nil
}

func (f *fakeQuerier) assertIssued(t *testing.T, wantSQLFragment string, wantArgs []any) {
	t.Helper()
	require.Len(t, f.queries, 1, "the repository must issue exactly one statement")
	q := f.queries[0]
	assert.Contains(t, q.sql, wantSQLFragment)
	if wantArgs != nil {
		assert.Equal(t, wantArgs, q.args, "arguments must be passed in the statement's placeholder order")
	}
}

type fakeRow struct {
	values []any
}

func (r *fakeRow) Scan(dest ...any) error { return scanInto(r.values, dest) }

type fakeRows struct {
	rows [][]any
	next int
	err  error
}

func (r *fakeRows) Next() bool { r.next++; return r.next <= len(r.rows) }

func (r *fakeRows) Scan(dest ...any) error { return scanInto(r.rows[r.next-1], dest) }

func (r *fakeRows) Close()                                       {}
func (r *fakeRows) Err() error                                   { return r.err }
func (r *fakeRows) CommandTag() pgconn.CommandTag                { return pgconn.CommandTag{} }
func (r *fakeRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (r *fakeRows) Values() ([]any, error)                       { return r.rows[r.next-1], nil }
func (r *fakeRows) RawValues() [][]byte                          { return nil }
func (r *fakeRows) Conn() *pgx.Conn                              { return nil }
func (r *fakeRows) TypeMap() *pgtype.Map                         { return nil }

func scanInto(values, dest []any) error {
	if len(values) != len(dest) {
		return fmt.Errorf("scan: %d values into %d destinations", len(values), len(dest))
	}
	for i, d := range dest {
		if d == nil {
			continue
		}
		target := reflect.ValueOf(d)
		if target.Kind() != reflect.Pointer {
			return fmt.Errorf("scan: destination %d is not a pointer", i)
		}
		value := reflect.ValueOf(values[i])
		if !value.Type().AssignableTo(target.Elem().Type()) {
			return fmt.Errorf("scan: value %d is %T, destination is %T", i, values[i], d)
		}
		target.Elem().Set(value)
	}
	return nil
}

// TestFindAlertOverlays_RoutesOnlyAlertCapableOverlays states the core routing
// filter: a chat overlay subscribed to the same source must never receive alert
// events. The clause lives only in the SQL (the query has no in-process filter),
// so the assertion is on the statement text as well as the scanned rows.
func TestFindAlertOverlays_RoutesOnlyAlertCapableOverlays(t *testing.T) {
	db := &fakeQuerier{rows: fakeRows{rows: [][]any{
		{"overlay-a", "user-1", "alerts"},
		{"overlay-b", "user-2", "goal"},
		{"overlay-c", "user-3", "list"},
	}}}

	repo := NewRepository(db)
	overlays, err := repo.FindAlertOverlays(context.Background(), "twitch", "12345")
	require.NoError(t, err)
	require.Len(t, overlays, 3)

	assert.Equal(t, AlertOverlay{OverlayID: "overlay-a", UserID: "user-1", OverlayType: "alerts"}, overlays[0])
	assert.Equal(t, AlertOverlay{OverlayID: "overlay-b", UserID: "user-2", OverlayType: "goal"}, overlays[1])
	assert.Equal(t, AlertOverlay{OverlayID: "overlay-c", UserID: "user-3", OverlayType: "list"}, overlays[2])

	db.assertIssued(t, "o.overlay_type IN ('alerts','goal','list')", []any{"twitch", "12345"})
	// The fan-out must cover both source shapes, like the message-processor's
	// overlay_router UNION: direct sources plus accepted share_requests.
	assert.Contains(t, db.queries[0].sql, "UNION", "routing must include the shared_overlay fan-out branch")
	assert.Contains(t, db.queries[0].sql, "share_requests", "routing must join share_requests for the fan-out branch")
}

// TestFindAlertOverlays_PropagatesQueryError: a failed route lookup must fail
// the alert, not silently fan out to zero overlays (an undelivered alert that
// still got ACKed is lost forever).
func TestFindAlertOverlays_PropagatesQueryError(t *testing.T) {
	db := &fakeQuerier{rows: fakeRows{err: errors.New("db down")}}

	repo := NewRepository(db)
	overlays, err := repo.FindAlertOverlays(context.Background(), "twitch", "12345")
	assert.Error(t, err, "a routing query failure must propagate to the caller")
	assert.Nil(t, overlays)
}

// TestInsertAlertEvent_PersistsIdempotently states the write contract behind
// at-least-once re-delivery: ON CONFLICT (id) DO NOTHING, so a re-persisted
// alert is absorbed instead of double-written.
func TestInsertAlertEvent_PersistsIdempotently(t *testing.T) {
	db := &fakeQuerier{}
	repo := NewRepository(db)

	occurred := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	alert := &models.Alert{
		AlertID:    "alert-uuid",
		OverlayID:  "overlay-uuid",
		Platform:   "twitch",
		EventType:  "bits",
		EventData:  &mpmodels.EventInfo{Type: "bits", Tier: "high"},
		User:       models.AlertUser{ID: "user-1", Name: "SomeChatter"},
		OccurredAt: occurred,
	}

	require.NoError(t, repo.InsertAlertEvent(context.Background(), alert))

	db.assertIssued(t, "INSERT INTO alert_events", []any{
		"alert-uuid",
		"overlay-uuid",
		"twitch",
		"bits",
		"user-1",
		"SomeChatter",
		`{"type":"bits","tier":"high","duration":0,"is_update":false}`,
		occurred,
	})
	assert.Contains(t, db.queries[0].sql, "ON CONFLICT (id) DO NOTHING",
		"re-persisted alerts must be absorbed, not double-written")
}

// TestInsertAlertEvent_PropagatesWriteError: a failed persist must fail the
// alert so the consumer leaves it unacked and redelivers.
func TestInsertAlertEvent_PropagatesWriteError(t *testing.T) {
	db := &fakeQuerier{execErr: errors.New("db down")}
	repo := NewRepository(db)

	err := repo.InsertAlertEvent(context.Background(), &models.Alert{EventType: "bits"})
	assert.Error(t, err, "a persist failure must propagate so the alert is redelivered")
}
