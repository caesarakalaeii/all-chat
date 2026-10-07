// This file is part of All-Chat.
// Copyright (C) 2026 caesarakalaeii
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published
// by the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// TestReadinessHandler_ReportsEachDependency: the deployment manifest's
// readinessProbe routes traffic on this handler's verdict, so it must actually
// ping both dependencies — an unconditional 200 would let a pod with a dead
// database or a dead Redis take alert traffic it can only fail on. The database
// is checked first, and a dead database must not also cost a Redis round-trip.
func TestReadinessHandler_ReportsEachDependency(t *testing.T) {
	cases := []struct {
		name           string
		dbErr          error
		redisErr       error
		wantStatus     int
		wantReason     string
		redisWasPinged bool
	}{
		{"ready when both answer", nil, nil, http.StatusOK, "", true},
		{"database down is not ready", errors.New("db down"), nil, http.StatusServiceUnavailable, "database connection failed", false},
		{"redis down is not ready", nil, errors.New("redis down"), http.StatusServiceUnavailable, "redis connection failed", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dbPinged, redisPinged := false, false
			handler := readinessHandler(
				func(context.Context) error { dbPinged = true; return tc.dbErr },
				func(context.Context) error { redisPinged = true; return tc.redisErr },
				zap.NewNop(),
			)

			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/health/ready", nil))

			assert.Equal(t, tc.wantStatus, rec.Code)
			require.True(t, dbPinged, "the database must always be pinged")
			assert.Equal(t, tc.redisWasPinged, redisPinged,
				"redis must be pinged exactly when the database answered")

			var body map[string]string
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body), "the response must be JSON")
			if tc.wantReason == "" {
				assert.Equal(t, "ready", body["status"])
			} else {
				assert.Equal(t, "unavailable", body["status"])
				assert.Equal(t, tc.wantReason, body["reason"],
					"the reason must say which dependency is down")
			}
		})
	}
}
