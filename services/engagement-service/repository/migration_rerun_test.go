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

//go:build integration

// Regression test for PR #524 review finding P0-1: the migration runner replays every
// up-migration on each pod start (no applied-migrations table, see run-migrations.sh),
// so a GLOBAL-unique index created in 069/070 would be REBUILT on every re-run — and
// abort with duplicate_key once real data legitimately fans one Twitch round / one chat
// message across multiple overlays (ADR-0028/0030). That aborts the migration init
// container and gates every deploy/restart. The fix creates those indexes per-overlay /
// per-round directly in 069/070; this test seeds the cross-scope duplicates a fresh-DB
// verification never has, then re-applies the engagement migration batch and asserts it
// still succeeds. Run with: go test -tags=integration ./repository/...
package repository_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

func TestP0_1_MigrationRerunWithMultiOverlayData(t *testing.T) {
	psql, err := exec.LookPath("psql")
	if err != nil {
		t.Skip("requires psql on PATH to re-apply the migration batch")
	}
	pool := newTestDB(t)
	ctx := context.Background()

	// Two overlays sourcing the "same" Twitch channel, plus a viewer.
	ovA := seedOverlay(t, pool)
	ovB := seedOverlay(t, pool)
	viewer := seedViewer(t, pool)

	// (1) Cross-overlay duplicate on (source, external_id): one Twitch poll mirrored to
	// both overlays. Legal under the per-overlay index, illegal under a global one.
	// (2) Cross-round duplicate on source_message_id: the same chat message id on the
	// vote recorded for each overlay's poll.
	pollExt := "poll-ext-" + uuid.NewString()[:8]
	pollMsg := uuid.New()
	seedPollVote(t, pool, seedNativePoll(t, pool, ovA, pollExt), viewer, pollMsg)
	seedPollVote(t, pool, seedNativePoll(t, pool, ovB, pollExt), viewer, pollMsg)

	// (3) + (4) Prediction analogues (uniq_prediction_overlay_source_external /
	// uniq_pred_entry_msg_round).
	predExt := "pred-ext-" + uuid.NewString()[:8]
	predMsg := uuid.New()
	seedPredEntry(t, pool, seedNativePrediction(t, pool, ovA, predExt), viewer, predMsg)
	seedPredEntry(t, pool, seedNativePrediction(t, pool, ovB, predExt), viewer, predMsg)

	// Sanity: the duplicates actually exist (guards against a silently vacuous test if a
	// future schema change rejected them at insert time).
	var n int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT count(*) FROM polls WHERE source='twitch_native' AND external_id=$1`, pollExt).Scan(&n))
	require.Equal(t, 2, n, "both overlays' mirror rows must coexist under the per-overlay unique index")

	// Re-apply the engagement migration batch (069+) exactly as run-migrations.sh does.
	// On the pre-fix code this aborts rebuilding a global unique over the rows above; on
	// the fixed code every CREATE ... IF NOT EXISTS short-circuits on the per-scope name.
	host := envOr("DATABASE_HOST", "localhost")
	port := envOr("DATABASE_PORT", "5432")
	user := envOr("DATABASE_USER", "allchat")
	dbname := envOr("DATABASE_NAME", "allchat")
	env := append(os.Environ(), "PGPASSWORD="+envOr("DATABASE_PASSWORD", "allchat_dev_password"))

	files, err := filepath.Glob(filepath.Join(migrationsDir(t), "[0-9]*.sql"))
	require.NoError(t, err)
	sort.Strings(files)
	replayed := 0
	for _, f := range files {
		base := filepath.Base(f)
		if strings.HasSuffix(base, "_down.sql") {
			continue
		}
		// 3-digit zero-padded prefixes sort lexically; replay only the engagement batch
		// (the P0-1 landmine lives in 069+) to keep the blast radius tight.
		if base < "069" {
			continue
		}
		cmd := exec.Command(psql, "-h", host, "-p", port, "-U", user, "-d", dbname,
			"-v", "ON_ERROR_STOP=1", "-q", "-f", f)
		cmd.Env = env
		out, cerr := cmd.CombinedOutput()
		require.NoErrorf(t, cerr, "re-applying %s over multi-overlay data must succeed:\n%s", base, out)
		replayed++
	}
	require.Positive(t, replayed, "expected to re-apply at least the engagement migrations")
}

// --- fixtures ---

func seedNativePoll(t *testing.T, pool *pgxpool.Pool, overlayID uuid.UUID, externalID string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	require.NoError(t, pool.QueryRow(context.Background(),
		`INSERT INTO polls (overlay_id, source, external_id, question, state, allow_change)
		 VALUES ($1,'twitch_native',$2,'q','ACTIVE',false) RETURNING id`, overlayID, externalID).Scan(&id))
	return id
}

func seedPollVote(t *testing.T, pool *pgxpool.Pool, pollID, viewerID uuid.UUID, msgID uuid.UUID) {
	t.Helper()
	var optID uuid.UUID
	require.NoError(t, pool.QueryRow(context.Background(),
		`INSERT INTO poll_options (poll_id, idx, label) VALUES ($1,1,'a') RETURNING id`, pollID).Scan(&optID))
	_, err := pool.Exec(context.Background(),
		`INSERT INTO poll_votes (poll_id, viewer_id, option_id, platform, source_message_id)
		 VALUES ($1,$2,$3,'twitch',$4)`, pollID, viewerID, optID, msgID)
	require.NoError(t, err)
}

func seedNativePrediction(t *testing.T, pool *pgxpool.Pool, overlayID uuid.UUID, externalID string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	require.NoError(t, pool.QueryRow(context.Background(),
		`INSERT INTO predictions (overlay_id, source, external_id, title, state)
		 VALUES ($1,'twitch_native',$2,'t','LOCKED') RETURNING id`, overlayID, externalID).Scan(&id))
	return id
}

func seedPredEntry(t *testing.T, pool *pgxpool.Pool, predID, viewerID uuid.UUID, msgID uuid.UUID) {
	t.Helper()
	var outID uuid.UUID
	require.NoError(t, pool.QueryRow(context.Background(),
		`INSERT INTO prediction_outcomes (prediction_id, idx, label) VALUES ($1,1,'o') RETURNING id`, predID).Scan(&outID))
	_, err := pool.Exec(context.Background(),
		`INSERT INTO prediction_entries (prediction_id, viewer_id, outcome_id, amount, platform, source_message_id)
		 VALUES ($1,$2,$3,10,'twitch',$4)`, predID, viewerID, outID, msgID)
	require.NoError(t, err)
}

// --- helpers ---

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// migrationsDir resolves repo-root/migrations relative to this test file, so the test
// works regardless of the working directory `go test` runs it from.
func migrationsDir(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok, "runtime.Caller failed")
	// services/engagement-service/repository/<this file> -> repo root/migrations
	return filepath.Join(filepath.Dir(thisFile), "..", "..", "..", "migrations")
}
