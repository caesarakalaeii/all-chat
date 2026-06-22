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

package handlers

import (
	"context"
	"errors"
	"testing"

	"github.com/caesar/all-chat/shared/quota"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// quotaFakeRow / quotaFakeQuerier are a minimal quota.Querier for exercising the
// YouTube send quota path without a real database.
type quotaFakeRow struct {
	ok  bool
	err error
}

func (r quotaFakeRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	if len(dest) > 0 {
		if p, ok := dest[0].(*bool); ok {
			*p = r.ok
		}
	}
	return nil
}

type quotaFakeQuerier struct {
	reserveOK  bool
	reserveErr error
	execErr    error
	sqls       []string
}

func (f *quotaFakeQuerier) QueryRow(_ context.Context, sql string, _ ...any) pgx.Row {
	f.sqls = append(f.sqls, sql)
	return quotaFakeRow{ok: f.reserveOK, err: f.reserveErr}
}

func (f *quotaFakeQuerier) Exec(_ context.Context, sql string, _ ...any) (pgconn.CommandTag, error) {
	f.sqls = append(f.sqls, sql)
	return pgconn.CommandTag{}, f.execErr
}

func newQuotaTestHandler(q quota.Querier) *ChatSendHandler {
	h := &ChatSendHandler{log: zap.NewNop()}
	if q != nil {
		h.ytQuota = quota.NewReserver(q, 100)
	}
	return h
}

func TestReserveYouTubeSendQuota_NilReserverFailsOpen(t *testing.T) {
	h := newQuotaTestHandler(nil)
	assert.True(t, h.reserveYouTubeSendQuota(context.Background(), quota.QuotaCostYouTubeSend),
		"no reserver configured must fail open so a send is never blocked by accounting")
	// settle must be a no-op (no panic) when unconfigured
	h.settleYouTubeSendQuota(context.Background(), quota.QuotaCostYouTubeSend, true)
}

func TestReserveYouTubeSendQuota_ExhaustedBlocks(t *testing.T) {
	q := &quotaFakeQuerier{reserveOK: false}
	h := newQuotaTestHandler(q)
	assert.False(t, h.reserveYouTubeSendQuota(context.Background(), quota.QuotaCostYouTubeSend),
		"a genuinely exhausted quota blocks the send")
	require.Len(t, q.sqls, 1)
	assert.Contains(t, q.sqls[0], "reserve_youtube_quota")
}

func TestReserveYouTubeSendQuota_DBErrorFailsOpen(t *testing.T) {
	q := &quotaFakeQuerier{reserveErr: errors.New("db down")}
	h := newQuotaTestHandler(q)
	assert.True(t, h.reserveYouTubeSendQuota(context.Background(), quota.QuotaCostYouTubeSend),
		"a DB error must fail open — a quota hiccup must not block a streamer's own chat")
}

func TestSettleYouTubeSendQuota_ConfirmsAndRolls(t *testing.T) {
	q := &quotaFakeQuerier{reserveOK: true}
	h := newQuotaTestHandler(q)

	h.settleYouTubeSendQuota(context.Background(), quota.QuotaCostYouTubeSend, true)
	require.Len(t, q.sqls, 1)
	assert.Contains(t, q.sqls[0], "confirm_youtube_quota")

	h.settleYouTubeSendQuota(context.Background(), quota.QuotaCostYouTubeSend, false)
	require.Len(t, q.sqls, 2)
	assert.Contains(t, q.sqls[1], "rollback_youtube_quota")
}
