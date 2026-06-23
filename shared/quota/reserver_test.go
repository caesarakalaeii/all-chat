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

package quota

import (
	"context"
	"errors"
	"regexp"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeRow struct {
	val     bool
	scanErr error
}

func (f fakeRow) Scan(dest ...any) error {
	if f.scanErr != nil {
		return f.scanErr
	}
	if len(dest) > 0 {
		if p, ok := dest[0].(*bool); ok {
			*p = f.val
		}
	}
	return nil
}

type fakeQuerier struct {
	reserveResult bool
	scanErr       error
	execErr       error
	sqls          []string
	args          [][]any
}

func (f *fakeQuerier) QueryRow(_ context.Context, sql string, args ...any) pgx.Row {
	f.sqls = append(f.sqls, sql)
	f.args = append(f.args, args)
	return fakeRow{val: f.reserveResult, scanErr: f.scanErr}
}

func (f *fakeQuerier) Exec(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	f.sqls = append(f.sqls, sql)
	f.args = append(f.args, args)
	return pgconn.CommandTag{}, f.execErr
}

var dateRE = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

func TestReserve_CallsSQLWithDateUnitsLimit(t *testing.T) {
	q := &fakeQuerier{reserveResult: true}
	r := NewReserver(q, 5000)

	ok, err := r.Reserve(context.Background(), QuotaCostBan)
	require.NoError(t, err)
	assert.True(t, ok)
	require.Len(t, q.args, 1)
	assert.Contains(t, q.sqls[0], "reserve_youtube_quota")
	assert.Regexp(t, dateRE, q.args[0][0], "first arg is the YYYY-MM-DD date")
	assert.Equal(t, 50, q.args[0][1], "units")
	assert.Equal(t, 5000, q.args[0][2], "daily limit")
}

func TestReserve_InsufficientReturnsFalse(t *testing.T) {
	q := &fakeQuerier{reserveResult: false}
	r := NewReserver(q, 100)
	ok, err := r.Reserve(context.Background(), QuotaCostBan)
	require.NoError(t, err)
	assert.False(t, ok, "an over-limit reservation returns false so the caller skips the API call")
}

func TestReserve_ScanErrorPropagates(t *testing.T) {
	q := &fakeQuerier{scanErr: errors.New("boom")}
	r := NewReserver(q, 100)
	ok, err := r.Reserve(context.Background(), QuotaCostYouTubeSend)
	require.Error(t, err)
	assert.False(t, ok)
	assert.Contains(t, err.Error(), "reserve youtube quota")
}

func TestConfirmAndRollback_CallTheirSQL(t *testing.T) {
	q := &fakeQuerier{}
	r := NewReserver(q, 0)

	require.NoError(t, r.Confirm(context.Background(), QuotaCostBan))
	require.NoError(t, r.Rollback(context.Background(), QuotaCostBan))
	require.Len(t, q.sqls, 2)
	assert.Contains(t, q.sqls[0], "confirm_youtube_quota")
	assert.Contains(t, q.sqls[1], "rollback_youtube_quota")
	assert.Equal(t, QuotaCostBan, q.args[0][1])
}

func TestConfirm_ExecErrorPropagates(t *testing.T) {
	q := &fakeQuerier{execErr: errors.New("db down")}
	r := NewReserver(q, 0)
	err := r.Confirm(context.Background(), QuotaCostYouTubeSend)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "confirm youtube quota")
}

func TestRollback_ExecErrorPropagates(t *testing.T) {
	q := &fakeQuerier{execErr: errors.New("db down")}
	r := NewReserver(q, 0)
	err := r.Rollback(context.Background(), QuotaCostYouTubeSend)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "rollback youtube quota")
}

func TestNewReserver_DefaultsDailyLimit(t *testing.T) {
	q := &fakeQuerier{reserveResult: true}
	r := NewReserver(q, 0) // non-positive → default
	_, err := r.Reserve(context.Background(), QuotaCostBan)
	require.NoError(t, err)
	assert.Equal(t, DefaultDailyLimit, q.args[0][2])
}
