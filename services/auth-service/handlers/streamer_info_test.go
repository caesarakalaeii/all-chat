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
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/caesar/all-chat/services/auth-service/repository"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

// streamerInfoStubRow satisfies pgx.Row; Scan always returns the configured error.
type streamerInfoStubRow struct{ err error }

func (r streamerInfoStubRow) Scan(dest ...interface{}) error { return r.err }

// stubStreamerInfoDB satisfies streamerInfoDB; QueryRow returns a row whose
// Scan yields queryRowErr (Query is unused by the paths under test).
type stubStreamerInfoDB struct{ queryRowErr error }

func (d *stubStreamerInfoDB) Query(ctx context.Context, sql string, args ...interface{}) (pgx.Rows, error) {
	return nil, nil
}

func (d *stubStreamerInfoDB) QueryRow(ctx context.Context, sql string, args ...interface{}) pgx.Row {
	return streamerInfoStubRow{err: d.queryRowErr}
}

func newStreamerInfoTestHandler(t *testing.T, queryRowErr error) (*gin.Engine, *observer.ObservedLogs) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	core, logs := observer.New(zapcore.DebugLevel)

	userRepo := new(MockUserRepository)
	// Force the username lookup to miss so the handler falls through to the
	// channel_id lookup branch we are exercising.
	userRepo.On("GetByUsername", mock.Anything, mock.Anything).Return(nil, repository.ErrUserNotFound)

	h := NewStreamerInfoHandler(zap.New(core), userRepo, &stubStreamerInfoDB{queryRowErr: queryRowErr})

	router := gin.New()
	router.GET("/streamer/:username", h.HandleGetStreamerInfo)
	return router, logs
}

// A normal "streamer not found" (pgx.ErrNoRows) must return 404 and log at
// debug level — not error — so the production logger attaches no stacktrace.
func TestHandleGetStreamerInfo_NotFound_LogsDebugReturns404(t *testing.T) {
	router, logs := newStreamerInfoTestHandler(t, pgx.ErrNoRows)

	req := httptest.NewRequest(http.MethodGet, "/streamer/ghost", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for not-found streamer, got %d", w.Code)
	}

	entries := logs.FilterMessage("Streamer not found by username or channel_id").All()
	if len(entries) != 1 {
		t.Fatalf("expected exactly 1 not-found log entry, got %d", len(entries))
	}
	if entries[0].Level != zapcore.DebugLevel {
		t.Errorf("not-found must log at debug (no stacktrace), got level %v", entries[0].Level)
	}
}

// A real database error (anything other than pgx.ErrNoRows) must surface as 500
// — not a misleading 404 — and be logged at error level.
func TestHandleGetStreamerInfo_DBError_LogsErrorReturns500(t *testing.T) {
	router, logs := newStreamerInfoTestHandler(t, errors.New("connection refused"))

	req := httptest.NewRequest(http.MethodGet, "/streamer/ghost", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 for real DB error, got %d", w.Code)
	}

	entries := logs.FilterMessage("Channel_id lookup query failed").All()
	if len(entries) != 1 || entries[0].Level != zapcore.ErrorLevel {
		t.Fatalf("expected exactly 1 error-level db-failure log, got %+v", entries)
	}
}
