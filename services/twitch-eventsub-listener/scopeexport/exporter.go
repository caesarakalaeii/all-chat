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

// Package scopeexport reports the IRC→EventSub migration backlog as a Prometheus gauge:
// distinct active Twitch channels grouped by whether their owner granted the EventSub
// chat scope (user:read:chat).
//
// Why this exists: the migration dashboard's other panels infer "still on IRC" from
// listener_messages_received_total. But IRC and EventSub activate asymmetrically —
// EventSub only subscribes/claims a channel while an overlay's WebSocket is live
// (demand-gated; see channels.Manager.reconcileChatLocked), whereas IRC joins every
// active source within the idle window regardless of demand. So a fully-migrated channel
// produces IRC traffic during the (frequent) windows its overlay is disconnected, and the
// `… unless on (channel_id) (listener_eventsub_chat_owned)` subtraction (an instant) does
// not remove it. The result: already-migrated channels and channels created after EventSub
// became the default both show up as "needs migration", and the panel never drains to zero.
//
// This gauge is derived from granted OAuth scope — a config fact, not chat traffic — so it
// is the authoritative "still needs to migrate" signal and the correct gate for the IRC
// listener sunset (ADR-0026): it is safe to enforce only once the unscoped_has_cred backlog
// is drained (unscoped_no_cred can never migrate; see below).
package scopeexport

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"go.uber.org/zap"
)

// RefreshInterval is how often the backlog is recomputed. The query is a small aggregate
// over overlay_chat_sources keyed on indexed login/username columns, so 60s is cheap even
// when every replica runs it.
const RefreshInterval = 60 * time.Second

// Migration states reported under the migration_state label. The split distinguishes the
// backlog that the in-overlay re-consent nudge (ADR-0026 warn phase) can actually clear
// from the backlog it cannot:
const (
	// stateScoped: owner granted user:read:chat → EventSub-capable. "Migrated".
	stateScoped = "scoped"
	// stateUnscopedHasCred: owner has a Twitch credential but it lacks chat scope →
	// re-adding the source (re-consent) migrates them. The warn-phase nudge targets these.
	stateUnscopedHasCred = "unscoped_has_cred"
	// stateUnscopedNoCred: no owner credential exists at all (third-party channel added as a
	// source, or an unlinked own-channel). EventSub's channel.chat.message subscription needs
	// the broadcaster's own authorized token, so these can NEVER move off IRC — they go dark
	// permanently at enforce and no in-product action by the connected user can fix them.
	stateUnscopedNoCred = "unscoped_no_cred"
)

// allStates is the fixed label set. Every cycle publishes all three so a state that drops
// to zero reports 0 rather than going stale at its last non-zero sample.
var allStates = []string{stateScoped, stateUnscopedHasCred, stateUnscopedNoCred}

// backlog reports the count of distinct active Twitch channels (active source + active
// overlay) in each migration state. (unscoped_has_cred + unscoped_no_cred) is the set that
// loses Twitch chat when the IRC listener is retired (ADR-0026).
var backlog = promauto.NewGaugeVec(prometheus.GaugeOpts{
	Name: "eventsub_twitch_migration_sources",
	Help: "Distinct active Twitch channels by IRC→EventSub migration state (scoped|unscoped_has_cred|unscoped_no_cred), derived from granted OAuth scope rather than chat traffic. unscoped_* is the set that loses Twitch chat when the IRC listener is retired (ADR-0026).",
}, []string{"service", "migration_state"})

// scopeBacklogQuery resolves, per distinct active Twitch channel, the best available
// credential's chat-scope flag and whether any credential exists at all, then buckets each
// channel into a migration state. The source/credential predicates are kept in lock-step
// with channels.QueryActiveTwitchChannelCredentials (ADR-0015/0016): same two credential
// origins (the channel owner's Twitch-login users row, or a linked twitch_oauth_tokens row),
// same "prefer chat-scoped AND unexpired, users row breaks ties" precedence, same active
// filters (ocs.is_active AND o.is_active). Counting distinct channels (not source rows)
// matches the per-channel granularity of the dashboard's channel_id panels.
const scopeBacklogQuery = `
SELECT
  CASE
    WHEN r.has_chat_scope THEN 'scoped'
    WHEN r.has_cred       THEN 'unscoped_has_cred'
    ELSE 'unscoped_no_cred'
  END AS migration_state,
  COUNT(*) AS n
FROM (
  SELECT DISTINCT LOWER(ocs.channel_id) AS login,
    COALESCE((
      SELECT c.has_chat_scope FROM (
        SELECT COALESCE('user:read:chat' = ANY(u.granted_scopes), false) AS has_chat_scope, u.token_expires_at, 1 AS pri
        FROM users u
        WHERE LOWER(u.username) = LOWER(ocs.channel_id) AND u.auth_provider = 'twitch'
        UNION ALL
        SELECT 'user:read:chat' = ANY(t.granted_scopes) AS has_chat_scope, t.token_expires_at, 2 AS pri
        FROM twitch_oauth_tokens t
        WHERE LOWER(t.twitch_login) = LOWER(ocs.channel_id)
      ) c
      ORDER BY (c.has_chat_scope AND c.token_expires_at > NOW()) DESC, c.pri ASC
      LIMIT 1
    ), false) AS has_chat_scope,
    (EXISTS (SELECT 1 FROM users u WHERE LOWER(u.username) = LOWER(ocs.channel_id) AND u.auth_provider = 'twitch')
     OR EXISTS (SELECT 1 FROM twitch_oauth_tokens t WHERE LOWER(t.twitch_login) = LOWER(ocs.channel_id))) AS has_cred
  FROM overlay_chat_sources ocs
  JOIN overlays o ON ocs.overlay_id = o.id
  WHERE ocs.platform = 'twitch' AND ocs.is_active = true AND o.is_active = true
) r
GROUP BY 1
`

// Export recomputes the migration backlog into the gauge every RefreshInterval until ctx is
// done. Safe to run on every replica: the value is identical across pods and collapses under
// `max by (migration_state)` in PromQL. A nil pool makes this a no-op (e.g. in tests). On a
// query error the gauge is left unchanged (not zeroed) so a transient DB blip can't make the
// backlog falsely flatline to zero and read as "migration complete".
func Export(ctx context.Context, db *pgxpool.Pool, service string, logger *zap.Logger) {
	if db == nil {
		return
	}
	syncOnce(ctx, db, service, logger) // publish immediately so the gauge isn't absent until the first tick
	ticker := time.NewTicker(RefreshInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			syncOnce(ctx, db, service, logger)
		}
	}
}

// syncOnce runs the backlog query once and publishes the result. It bounds the query with a
// timeout so a slow database can't wedge the loop, and on any failure returns without
// touching the gauge (see Export's contract).
func syncOnce(ctx context.Context, db *pgxpool.Pool, service string, logger *zap.Logger) {
	qctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	rows, err := db.Query(qctx, scopeBacklogQuery)
	if err != nil {
		logger.Warn("scope backlog exporter: query failed; leaving gauge unchanged", zap.Error(err))
		return
	}
	defer rows.Close()

	counts := make(map[string]float64, len(allStates))
	for rows.Next() {
		var state string
		var n int64
		if err := rows.Scan(&state, &n); err != nil {
			logger.Warn("scope backlog exporter: scan failed; leaving gauge unchanged", zap.Error(err))
			return
		}
		counts[state] = float64(n)
	}
	if err := rows.Err(); err != nil {
		logger.Warn("scope backlog exporter: row iteration failed; leaving gauge unchanged", zap.Error(err))
		return
	}

	setBacklog(service, counts)
}

// setBacklog publishes counts for every known state, defaulting absent states to 0. Split
// out from syncOnce so the gauge-publishing/zero-fill logic is unit-testable without a DB.
func setBacklog(service string, counts map[string]float64) {
	for _, state := range allStates {
		backlog.WithLabelValues(service, state).Set(counts[state])
	}
}
