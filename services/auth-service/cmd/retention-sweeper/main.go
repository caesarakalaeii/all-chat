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

// retention-sweeper deletes rows that are past the retention periods the privacy
// policy promises (ADR-0089). It runs as a Kubernetes CronJob every 15 minutes and
// exits; nothing else enforces these periods.
//
// Usage:
//
//	DATABASE_URL=postgres://... retention-sweeper [--batch-size=1000] [--batch-delay-ms=100]
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"
)

// rule is one retention period. where is constant SQL selecting the rows that are
// past it; it is never built from input.
type rule struct {
	table string
	where string
}

// rules is the whole retention policy enforced by this job. Every period here is
// quoted in the privacy policy, so a change here is a change to that promise.
var rules = []rule{
	// Messages viewers sent through All-Chat (migration 046).
	{"viewer_message_history", `created_at < NOW() - INTERVAL '1 hour'`},
	// YouTube API quota forensics (migration 016).
	{"youtube_quota_audit_log", `"timestamp" < NOW() - INTERVAL '30 days'`},
	// Alert-processor event log (migration 106). Write-only: list overlays rank
	// from Redis, nothing reads these rows back.
	{"alert_events", `created_at < NOW() - INTERVAL '30 days'`},
	// Moderation command audit log (migration 060).
	{"moderation_actions", `created_at < NOW() - INTERVAL '90 days'`},
	// Abandoned device-link requests (migration 088): pending rows hold no
	// credential once expired. A row that minted a device token is that token's
	// audit trail and is kept while the token exists.
	{"device_link_requests", `device_token_id IS NULL AND expires_at < NOW() - INTERVAL '1 hour'`},

	// OAuth credentials, 7 days after they died. An access token past its expiry is
	// NOT dead while a refresh token can renew it, and token-refresh-service pushes a
	// rejected token's expiry 30 days ahead instead of recording the rejection, so
	// expiry alone proves nothing for youtube_oauth_tokens, twitch_oauth_tokens,
	// patreon_oauth_tokens, mod_oauth_credentials, chat_bot_accounts, viewer_sessions
	// or users; those rows are also still needed for ownership, webhook mapping or bot
	// sends. Only the tables below have a rule that proves death (ADR-0089).
	//
	// Legacy TikTok tokens: nothing reads, writes or refreshes the table any more.
	{"tiktok_oauth_tokens", `expiry < NOW() - INTERVAL '7 days'`},
	// Legacy Kick listener rows: without a numeric kick_user_id no code path
	// (moderation, chat send) resolves or refreshes them, and the kick-listener only
	// reads unexpired rows. Linked credentials with kick_user_id are refreshed on use.
	{"kick_oauth_tokens", `kick_user_id IS NULL AND expiry < NOW() - INTERVAL '7 days'`},
	// Instagram has no refresh grant, so an expired token is dead. NULL expires_at
	// is a non-expiring Page-derived token and never matches.
	{"instagram_oauth_tokens", `expires_at < NOW() - INTERVAL '7 days'`},
}

type options struct {
	batchSize  int
	batchDelay time.Duration
	dbURL      string
}

func parseFlags(args []string, getenv func(string) string) (*options, error) {
	fs := flag.NewFlagSet("retention-sweeper", flag.ContinueOnError)
	batchSize := fs.Int("batch-size", 1000, "rows per DELETE; each batch is its own short transaction")
	batchDelayMs := fs.Int("batch-delay-ms", 100, "pause between full batches to throttle DB load")
	if err := fs.Parse(args); err != nil {
		return nil, err
	}
	if *batchSize < 1 {
		return nil, fmt.Errorf("--batch-size must be at least 1")
	}
	dbURL := getenv("DATABASE_URL")
	if dbURL == "" {
		return nil, fmt.Errorf("DATABASE_URL must be set")
	}
	return &options{
		batchSize:  *batchSize,
		batchDelay: time.Duration(*batchDelayMs) * time.Millisecond,
		dbURL:      dbURL,
	}, nil
}

// sweepTable deletes the rows matching r in batches until none are left. Each
// DELETE locks at most batchSize rows, so the first run's backlog never holds a
// long lock. SKIP LOCKED lets an overlapping run (or a writer touching a row)
// proceed without waiting; the locking subquery re-checks r.where against the
// newest row version, so a row updated out of the window in the meantime is kept.
func sweepTable(ctx context.Context, conn *pgx.Conn, r rule, o *options) (int64, error) {
	sql := fmt.Sprintf(`DELETE FROM %[1]s WHERE id IN (
		SELECT id FROM %[1]s WHERE %[2]s LIMIT $1 FOR UPDATE SKIP LOCKED)`, r.table, r.where)
	var total int64
	for {
		tag, err := conn.Exec(ctx, sql, o.batchSize)
		if err != nil {
			return total, err
		}
		total += tag.RowsAffected()
		if tag.RowsAffected() < int64(o.batchSize) {
			return total, nil
		}
		select {
		case <-ctx.Done():
			return total, ctx.Err()
		case <-time.After(o.batchDelay):
		}
	}
}

// connectBudget bounds how long the sweeper waits for its first connection. A new
// pod's IP reaches the Postgres NetworkPolicy allow-set a moment after the pod
// starts, so the first dial is refused; long-running services ride this out with
// shared/database's startup retry, and a one-shot job has to as well.
const connectBudget = time.Minute

func connect(ctx context.Context, cfg *pgx.ConnConfig, logger *zap.Logger) (*pgx.Conn, error) {
	ctx, cancel := context.WithTimeout(ctx, connectBudget)
	defer cancel()
	backoff := time.Second
	for attempt := 1; ; attempt++ {
		conn, err := pgx.ConnectConfig(ctx, cfg)
		if err == nil {
			return conn, nil
		}
		logger.Warn("database not reachable, retrying", zap.Int("attempt", attempt), zap.Duration("backoff", backoff), zap.Error(err))
		select {
		case <-ctx.Done():
			return nil, err
		case <-time.After(backoff):
		}
		backoff = min(backoff*2, 10*time.Second)
	}
}

func run(ctx context.Context, args []string, getenv func(string) string) int {
	logger, err := zap.NewProduction()
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "failed to build logger: %v\n", err)
		return 1
	}
	defer logger.Sync() //nolint:errcheck

	o, err := parseFlags(args, getenv)
	if err != nil {
		logger.Error("invalid configuration", zap.Error(err))
		return 1
	}

	cfg, err := pgx.ParseConfig(o.dbURL)
	if err != nil {
		logger.Error("invalid DATABASE_URL", zap.Error(err))
		return 1
	}
	cfg.RuntimeParams["application_name"] = "retention-sweeper"
	// One connection, not a pool: the deletes run one after another, and the
	// database has little connection headroom (ADR-0039).
	conn, err := connect(ctx, cfg, logger)
	if err != nil {
		logger.Error("database connection failed", zap.Error(err))
		return 1
	}
	defer conn.Close(context.Background())

	code := 0
	for _, r := range rules {
		start := time.Now()
		n, err := sweepTable(ctx, conn, r, o)
		fields := []zap.Field{zap.String("table", r.table), zap.Int64("rows_deleted", n), zap.Duration("duration", time.Since(start))}
		if err != nil {
			// Keep going: one broken table must not stop the others being cleaned.
			logger.Error("retention sweep failed", append(fields, zap.Error(err))...)
			code = 1
			continue
		}
		logger.Info("retention sweep", fields...)
	}
	return code
}

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()
	os.Exit(run(ctx, os.Args[1:], os.Getenv))
}
