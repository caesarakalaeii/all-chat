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
	"net/http"
	"time"

	"go.uber.org/zap"
)

// readinessHandler reports ready only while both dependencies answer a ping:
// the stream, the publishes and the leaderboards all live in Redis, and the
// routing and persistence live in Postgres, so a pod missing either can only
// fail on every alert it is handed. Extracted from main's gin closure — as a
// plain http.HandlerFunc taking ping functions — so the unavailable-database
// and unavailable-Redis branches are unit-testable; the deployment manifest's
// readinessProbe routes traffic on this verdict, so an unconditional 200 must
// not survive a refactor unnoticed. The database is checked first: a dead
// database must not also cost a Redis round-trip before the probe gives up.
func readinessHandler(dbPing func(context.Context) error, redisPing func(context.Context) error, log *zap.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		if err := dbPing(ctx); err != nil {
			log.Error("Health check failed: database unavailable", zap.Error(err))
			respondReadiness(w, http.StatusServiceUnavailable, "database connection failed")
			return
		}
		if err := redisPing(ctx); err != nil {
			log.Error("Health check failed: redis unavailable", zap.Error(err))
			respondReadiness(w, http.StatusServiceUnavailable, "redis connection failed")
			return
		}
		respondReadiness(w, http.StatusOK, "")
	}
}

func respondReadiness(w http.ResponseWriter, code int, reason string) {
	body := map[string]string{"status": "ready"}
	if reason != "" {
		body = map[string]string{"status": "unavailable", "reason": reason}
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	// The status line is already out; a failed body write cannot change the
	// probe's verdict, and the connection is the client's to retry.
	_ = json.NewEncoder(w).Encode(body)
}
