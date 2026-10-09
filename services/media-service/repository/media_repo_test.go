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

	"github.com/caesar/all-chat/services/media-service/models"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// recordedQuery is one statement the repository issued through the Querier
// seam.
type recordedQuery struct {
	sql  string
	args []any
}

// fakeQuerier stands in for the pgx pool at the repository's Querier seam.
// It answers every method from the canned responses the test sets, and
// records the SQL and arguments so tests can assert on the exact
// statement — including the owner scoping in DeleteByOwner, which lives
// only in the SQL and is therefore invisible to the handler tests (they
// mock the registry and would happily pass with a broken WHERE clause).
//
// Hand-rolled instead of pgxmock because pgxmock v4.9.0 does not compile
// against this module's pgx v5.11.0 (pgx.Rows grew a TypeMap method), and
// the sibling testcontainers convention would silently skip in the
// acceptance runner, which has no docker.
type fakeQuerier struct {
	queries []recordedQuery

	execTag pgconn.CommandTag
	execErr error

	row fakeRow

	rows fakeRows
}

func (f *fakeQuerier) Exec(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	f.queries = append(f.queries, recordedQuery{sql: sql, args: args})
	return f.execTag, f.execErr
}

func (f *fakeQuerier) QueryRow(_ context.Context, sql string, args ...any) pgx.Row {
	f.queries = append(f.queries, recordedQuery{sql: sql, args: args})
	return &f.row
}

func (f *fakeQuerier) Query(_ context.Context, sql string, args ...any) (pgx.Rows, error) {
	f.queries = append(f.queries, recordedQuery{sql: sql, args: args})
	return &f.rows, nil
}

// assertIssued fails the test unless the repository issued exactly one
// statement whose SQL contains wantSQLFragment (the load-bearing clause,
// e.g. the owner-scoped WHERE) and passed wantArgs positionally.
func (f *fakeQuerier) assertIssued(t *testing.T, wantSQLFragment string, wantArgs []any) {
	t.Helper()
	require.Len(t, f.queries, 1, "the repository must issue exactly one statement")
	q := f.queries[0]
	assert.Contains(t, q.sql, wantSQLFragment)
	assert.Equal(t, wantArgs, q.args, "arguments must be passed in the statement's placeholder order")
}

// fakeRow is the single-row result of QueryRow.
type fakeRow struct {
	values []any
}

func (r *fakeRow) Scan(dest ...any) error {
	return scanInto(r.values, dest)
}

// fakeRows is the multi-row result of Query. Only Next/Scan are meaningful
// for the repository; the rest satisfy pgx.Rows.
type fakeRows struct {
	rows [][]any
	next int
}

func (r *fakeRows) Next() bool {
	r.next++
	return r.next <= len(r.rows)
}

func (r *fakeRows) Scan(dest ...any) error {
	return scanInto(r.rows[r.next-1], dest)
}

func (r *fakeRows) Close()                                       {}
func (r *fakeRows) Err() error                                   { return nil }
func (r *fakeRows) CommandTag() pgconn.CommandTag                { return pgconn.CommandTag{} }
func (r *fakeRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (r *fakeRows) Values() ([]any, error)                       { return r.rows[r.next-1], nil }
func (r *fakeRows) RawValues() [][]byte                          { return nil }
func (r *fakeRows) Conn() *pgx.Conn                              { return nil }
func (r *fakeRows) TypeMap() *pgtype.Map                         { return nil }

// scanInto positionally assigns canned values into the destination
// pointers, mirroring pgx for the simple types the repository scans
// (strings, int64, time.Time). A type mismatch is an error, not a panic:
// it means the canned row no longer matches what the repository scans.
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
			return fmt.Errorf("scan: value %d is %T, destination is %T",
				i, values[i], d)
		}
		target.Elem().Set(value)
	}
	return nil
}

func TestCreate_InsertsPresignedRow(t *testing.T) {
	createdAt := time.Unix(1700000000, 0).UTC()
	db := &fakeQuerier{row: fakeRow{values: []any{"id-1", createdAt}}}

	obj := &models.MediaObject{
		UserID:      "user-1",
		ObjectKey:   "user-1/uuid1/airhorn.mp3",
		Filename:    "airhorn.mp3",
		ContentType: "audio/mpeg",
		SizeBytes:   1234,
	}
	repo := NewMediaRepository(db)
	require.NoError(t, repo.Create(context.Background(), obj))

	db.assertIssued(t, "INSERT INTO media_objects", []any{
		"user-1", "user-1/uuid1/airhorn.mp3", "airhorn.mp3", "audio/mpeg", int64(1234),
	})
	assert.Equal(t, "id-1", obj.ID, "Create must scan back the generated id")
	assert.Equal(t, createdAt, obj.CreatedAt, "Create must scan back created_at")
}

func TestCountByUser_CountsWithTheCallerScoped(t *testing.T) {
	db := &fakeQuerier{row: fakeRow{values: []any{2}}}

	repo := NewMediaRepository(db)
	count, err := repo.CountByUser(context.Background(), "user-1")
	require.NoError(t, err)
	assert.Equal(t, 2, count)

	db.assertIssued(t, "WHERE user_id = $1", []any{"user-1"})
}

func TestListByUser_ListsWithTheCallerScopedAndScansRows(t *testing.T) {
	db := &fakeQuerier{rows: fakeRows{rows: [][]any{
		{"id-1", "user-1", "user-1/uuid1/a.mp3", "a.mp3", "audio/mpeg", int64(10), time.Unix(1000, 0).UTC()},
		{"id-2", "user-1", "user-1/uuid2/b.png", "b.png", "image/png", int64(20), time.Unix(2000, 0).UTC()},
	}}}

	repo := NewMediaRepository(db)
	media, err := repo.ListByUser(context.Background(), "user-1")
	require.NoError(t, err)
	require.Len(t, media, 2)
	assert.Equal(t, "id-1", media[0].ID)
	assert.Equal(t, "user-1/uuid1/a.mp3", media[0].ObjectKey)
	assert.Equal(t, "audio/mpeg", media[0].ContentType)
	assert.Equal(t, int64(10), media[0].SizeBytes)
	assert.Equal(t, "id-2", media[1].ID)
	assert.Equal(t, "image/png", media[1].ContentType)

	db.assertIssued(t, "WHERE user_id = $1", []any{"user-1"})
}

func TestListByUser_ReturnsNoRowsWithoutError(t *testing.T) {
	db := &fakeQuerier{}

	repo := NewMediaRepository(db)
	media, err := repo.ListByUser(context.Background(), "user-1")
	require.NoError(t, err)
	assert.Empty(t, media, "no rows must come back as an empty slice, not an error")
}

// TestDeleteByOwner_DeletesOnlyTheOwnersRow pins the owner check to the
// SQL: the DELETE must scope on BOTH user_id and object_key, in that
// argument order. This is the test that catches a WHERE clause regression,
// which would let any authenticated user delete another user's registry
// row and trigger a MinIO removal of their object.
func TestDeleteByOwner_DeletesOnlyTheOwnersRow(t *testing.T) {
	db := &fakeQuerier{execTag: pgconn.NewCommandTag("DELETE 1")}

	repo := NewMediaRepository(db)
	deleted, err := repo.DeleteByOwner(context.Background(), "user-1", "user-1/uuid1/a.mp3")
	require.NoError(t, err)
	assert.True(t, deleted, "a row owned by the caller must be reported as deleted")

	db.assertIssued(t, "DELETE FROM media_objects WHERE user_id = $1 AND object_key = $2",
		[]any{"user-1", "user-1/uuid1/a.mp3"})
}

func TestDeleteByOwner_ReportsNothingDeletedForOtherUsersRows(t *testing.T) {
	// Same key shape a caller would send for another user's object: the
	// row exists but belongs to someone else, so the scoped DELETE affects
	// zero rows and the handler answers 404.
	db := &fakeQuerier{execTag: pgconn.NewCommandTag("DELETE 0")}

	repo := NewMediaRepository(db)
	deleted, err := repo.DeleteByOwner(context.Background(), "user-1", "user-2/uuid2/b.png")
	require.NoError(t, err)
	assert.False(t, deleted, "no row of the caller's must be reported as deleted")
}

func TestDeleteByOwner_ReturnsTheQueryError(t *testing.T) {
	db := &fakeQuerier{execErr: errors.New("db down")}

	repo := NewMediaRepository(db)
	deleted, err := repo.DeleteByOwner(context.Background(), "user-1", "user-1/uuid1/a.mp3")
	require.Error(t, err)
	assert.False(t, deleted)
}
