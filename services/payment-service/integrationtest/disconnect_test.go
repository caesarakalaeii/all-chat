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

package integrationtest

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/caesar/all-chat/services/payment-service/entitlement"
	"github.com/caesar/all-chat/services/payment-service/handlers"
	"github.com/caesar/all-chat/services/payment-service/patreon"
	"github.com/caesar/all-chat/services/payment-service/repository"
	"github.com/caesar/all-chat/shared/premium"
)

// memberBody is a members:* webhook body for patreonUserID pledging cents.
func memberBody(patreonUserID string, cents int) string {
	return `{"data":{"type":"member","id":"m-` + patreonUserID + `","attributes":{"patron_status":"active_patron","currently_entitled_amount_cents":` +
		itoa(cents) + `,"last_charge_status":"Paid"},"relationships":{"user":{"data":{"type":"user","id":"` + patreonUserID + `"}}}}}`
}

// Disconnecting Patreon must unlink the membership from the account for good: a
// later webhook for the same patron must neither re-link the row nor bring premium
// back on the next recompute (which admin grants and override expiry trigger).
func TestDisconnectUnlinksMembership(t *testing.T) {
	if testing.Short() {
		t.Skip("requires Docker")
	}
	pool, cleanup := setupDB(t)
	defer cleanup()
	ctx := context.Background()
	rc := premium.NewRecomputer(pool, zap.NewNop())

	cases := []struct {
		name      string
		column    string // the subject column on both Patreon tables
		ctxKey    string // the gin context key the JWT middleware sets
		newID     func() string
		disconn   func(*handlers.StatusHandler) gin.HandlerFunc
		status    func(*handlers.StatusHandler) gin.HandlerFunc
		recompute func(context.Context, string) (bool, error)
		isPremium func(*testing.T, *pgxpool.Pool, string) bool
		cents     int
	}{
		{
			name: "streamer", column: "user_id", ctxKey: "user_id",
			newID:     func() string { return insertUser(t, pool, "disconnect") },
			disconn:   func(h *handlers.StatusHandler) gin.HandlerFunc { return h.Disconnect },
			status:    func(h *handlers.StatusHandler) gin.HandlerFunc { return h.Status },
			recompute: rc.Recompute, isPremium: dbPremium, cents: minCents,
		},
		{
			name: "viewer", column: "viewer_id", ctxKey: "viewer_id",
			newID:     func() string { return insertViewer(t, pool) },
			disconn:   func(h *handlers.StatusHandler) gin.HandlerFunc { return h.ViewerDisconnect },
			status:    func(h *handlers.StatusHandler) gin.HandlerFunc { return h.ViewerStatus },
			recompute: rc.RecomputeViewer, isPremium: dbViewerPremium, cents: viewerMinCents,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			subjectID := tc.newID()
			patreonUserID := "pu-dc-" + tc.name
			connectPatreon(t, pool, tc.column, subjectID, patreonUserID)

			rdb := redis.NewClient(&redis.Options{Addr: miniredis.RunT(t).Addr()})
			subRepo := repository.NewSubscriptionRepository(pool, zap.NewNop())
			tokenRepo := repository.NewTokenRepository(pool, nil, zap.NewNop())
			svc := entitlement.NewService(subRepo, rc, minCents, viewerMinCents, zap.NewNop())
			const secret = "whsec_disconnect"
			wh := handlers.NewWebhookHandler(secret, rdb, tokenRepo, svc, zap.NewNop())
			sh := handlers.NewStatusHandler(subRepo, tokenRepo, rc, zap.NewNop())

			gin.SetMode(gin.TestMode)
			router := gin.New()
			router.POST("/webhook", wh.Handle)
			asSubject := func(c *gin.Context) { c.Set(tc.ctxKey, subjectID); c.Next() }
			router.DELETE("/connection", asSubject, tc.disconn(sh))
			router.GET("/status", asSubject, tc.status(sh))

			serve := func(method, path, body, event string) *httptest.ResponseRecorder {
				req := httptest.NewRequest(method, path, strings.NewReader(body))
				if event != "" {
					req.Header.Set("X-Patreon-Event", event)
					req.Header.Set("X-Patreon-Signature", signMD5(secret, body))
				}
				w := httptest.NewRecorder()
				router.ServeHTTP(w, req)
				return w
			}

			require.Equal(t, http.StatusOK, serve(http.MethodPost, "/webhook", memberBody(patreonUserID, tc.cents), patreon.EventMembersCreate).Code)
			require.True(t, tc.isPremium(t, pool, subjectID), "precondition: the connected patron is premium")

			require.Equal(t, http.StatusOK, serve(http.MethodDelete, "/connection", "", "").Code)
			assert.False(t, tc.isPremium(t, pool, subjectID), "disconnect ends premium immediately")
			assert.Zero(t, linkedRows(t, pool, tc.column, subjectID), "disconnect unlinks the membership")
			var st map[string]any
			require.NoError(t, json.Unmarshal(serve(http.MethodGet, "/status", "", "").Body.Bytes(), &st))
			assert.Equal(t, false, st["connected"], "status reports the account as no longer connected")

			// Patreon keeps sending webhooks for the still-active patron.
			require.Equal(t, http.StatusOK, serve(http.MethodPost, "/webhook", memberBody(patreonUserID, tc.cents+100), patreon.EventMembersUpdate).Code)
			assert.Zero(t, linkedRows(t, pool, tc.column, subjectID), "a webhook after disconnect must not re-link the membership")

			got, err := tc.recompute(ctx, subjectID)
			require.NoError(t, err)
			assert.False(t, got, "a recompute after the webhook must not re-grant premium")
			assert.False(t, tc.isPremium(t, pool, subjectID))
		})
	}
}

// Only an OAuth connection links a membership. A reconcile pass works from a token
// list read at the start of the pass, so it can still apply a membership for a
// subject that disconnected moments ago; that must not link the row or grant
// premium. Reconnecting (a token row again) links it.
func TestApplyLinksOnlyAConnectedSubject(t *testing.T) {
	if testing.Short() {
		t.Skip("requires Docker")
	}
	pool, cleanup := setupDB(t)
	defer cleanup()
	ctx := context.Background()

	userID := insertUser(t, pool, "stale")
	svc := entitlement.NewService(repository.NewSubscriptionRepository(pool, zap.NewNop()),
		premium.NewRecomputer(pool, zap.NewNop()), minCents, viewerMinCents, zap.NewNop())
	snap := &patreon.MembershipSnapshot{PatreonUserID: "pu-stale", PatronStatus: "active_patron", EntitledCents: minCents}

	_, isPrem, err := svc.Apply(ctx, snap, &userID, nil)
	require.NoError(t, err)
	assert.False(t, isPrem, "a subject without a Patreon connection gets no premium")
	assert.Zero(t, linkedRows(t, pool, "user_id", userID), "a subject without a Patreon connection is not linked")

	connectPatreon(t, pool, "user_id", userID, "pu-stale")
	_, isPrem, err = svc.Apply(ctx, snap, &userID, nil)
	require.NoError(t, err)
	assert.True(t, isPrem, "reconnecting links the membership again")
	assert.Equal(t, 1, linkedRows(t, pool, "user_id", userID))
}

// Rows left linked by a disconnect from before the fix (or by switching Patreon
// accounts) have no connection behind them. The reconcile pass deletes them and
// recomputes the subject, so premium they wrongly kept is revoked; a connected
// subject's row is untouched.
func TestPruneDisconnectedDeletesOrphanedMemberships(t *testing.T) {
	if testing.Short() {
		t.Skip("requires Docker")
	}
	pool, cleanup := setupDB(t)
	defer cleanup()
	ctx := context.Background()

	orphanUser := insertUser(t, pool, "orphan")
	orphanViewer := insertViewer(t, pool)
	connectedUser := insertUser(t, pool, "kept")
	connectPatreon(t, pool, "user_id", connectedUser, "pu-kept")
	_, err := pool.Exec(ctx, `
		INSERT INTO premium_subscriptions (user_id, viewer_id, product, provider_user_id, status, cents) VALUES
		    ($1, NULL, 'streamer', 'pu-orphan-u', 'active', 500),
		    (NULL, $2, 'viewer',   'pu-orphan-v', 'active', 500),
		    ($3, NULL, 'streamer', 'pu-kept',     'active', 500),
		    (NULL, NULL, 'streamer', 'pu-unlinked', 'active', 500)`,
		orphanUser, orphanViewer, connectedUser)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, "UPDATE users SET is_premium = TRUE WHERE id IN ($1, $2)", orphanUser, connectedUser)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, "UPDATE viewers SET is_premium = TRUE WHERE id = $1", orphanViewer)
	require.NoError(t, err)

	svc := entitlement.NewService(repository.NewSubscriptionRepository(pool, zap.NewNop()),
		premium.NewRecomputer(pool, zap.NewNop()), minCents, viewerMinCents, zap.NewNop())
	pruned, err := svc.PruneDisconnected(ctx)
	require.NoError(t, err)
	assert.Equal(t, 2, pruned)

	assert.Zero(t, linkedRows(t, pool, "user_id", orphanUser))
	assert.Zero(t, linkedRows(t, pool, "viewer_id", orphanViewer))
	assert.False(t, dbPremium(t, pool, orphanUser), "premium kept by an orphaned membership is revoked")
	assert.False(t, dbViewerPremium(t, pool, orphanViewer), "viewer premium kept by an orphaned membership is revoked")
	assert.Equal(t, 1, linkedRows(t, pool, "user_id", connectedUser), "a connected subject's membership is kept")
	assert.True(t, dbPremium(t, pool, connectedUser))
	var unlinked int
	require.NoError(t, pool.QueryRow(ctx, "SELECT COUNT(*) FROM premium_subscriptions WHERE provider_user_id = 'pu-unlinked'").Scan(&unlinked))
	assert.Equal(t, 1, unlinked, "a never-linked patron's row is left alone")

	pruned, err = svc.PruneDisconnected(ctx)
	require.NoError(t, err)
	assert.Zero(t, pruned, "a second pass finds nothing")
}

// connectPatreon stores a Patreon connection (token values are irrelevant here)
// for the subject in column ("user_id" or "viewer_id").
func connectPatreon(t *testing.T, pool *pgxpool.Pool, column, subjectID, patreonUserID string) {
	t.Helper()
	_, err := pool.Exec(context.Background(),
		`INSERT INTO patreon_oauth_tokens (`+column+`, patreon_user_id, access_token, refresh_token, token_expires_at)
		 VALUES ($1, $2, 'enc', 'enc', NOW() + INTERVAL '30 days')`, subjectID, patreonUserID)
	require.NoError(t, err)
}

// linkedRows counts the premium_subscriptions rows linked to the subject.
func linkedRows(t *testing.T, pool *pgxpool.Pool, column, subjectID string) int {
	t.Helper()
	var n int
	require.NoError(t, pool.QueryRow(context.Background(),
		"SELECT COUNT(*) FROM premium_subscriptions WHERE "+column+" = $1", subjectID).Scan(&n))
	return n
}
