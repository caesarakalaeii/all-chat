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

// Package integrationtest exercises payment-service against a real Postgres
// (testcontainers) running the actual migration set, plus an in-memory Redis. It
// validates the recompute SQL, the admin-override-preservation invariant, and the
// end-to-end signed-webhook -> premium -> premium-gated-endpoint loop.
//
// These tests require Docker and are skipped under `go test -short`.
package integrationtest

import (
	"context"
	"crypto/hmac"
	"crypto/md5" //nolint:gosec // tests compute the provider-mandated HMAC-MD5 webhook signature
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.uber.org/zap"

	"github.com/caesar/all-chat/services/payment-service/entitlement"
	"github.com/caesar/all-chat/services/payment-service/handlers"
	"github.com/caesar/all-chat/services/payment-service/patreon"
	"github.com/caesar/all-chat/services/payment-service/repository"
	"github.com/caesar/all-chat/shared/featuregates"
	"github.com/caesar/all-chat/shared/middleware"
	"github.com/caesar/all-chat/shared/premium"
)

const minCents = 500
const viewerMinCents = 200

func TestEntitlementApplyGrantsAndRevokes(t *testing.T) {
	if testing.Short() {
		t.Skip("requires Docker")
	}
	pool, cleanup := setupDB(t)
	defer cleanup()
	ctx := context.Background()

	userID := insertUser(t, pool, "apply")
	subRepo := repository.NewSubscriptionRepository(pool, zap.NewNop())
	svc := entitlement.NewService(subRepo, premium.NewRecomputer(pool, zap.NewNop()), minCents, viewerMinCents, zap.NewNop())

	// Active patron at threshold -> premium granted.
	_, isPrem, err := svc.Apply(ctx, &patreon.MembershipSnapshot{
		PatreonUserID: "pu-apply", PatronStatus: "active_patron", EntitledCents: 500,
	}, &userID, nil, nil)
	require.NoError(t, err)
	assert.True(t, isPrem)
	assert.True(t, dbPremium(t, pool, userID), "users.is_premium should be true after active membership")

	// Former patron -> premium revoked.
	_, isPrem, err = svc.Apply(ctx, &patreon.MembershipSnapshot{
		PatreonUserID: "pu-apply", PatronStatus: "former_patron",
	}, &userID, nil, nil)
	require.NoError(t, err)
	assert.False(t, isPrem)
	assert.False(t, dbPremium(t, pool, userID), "users.is_premium should be false after lapse")
}

// TestRecomputeTruthTableAndAdminOverride validates the real recompute SQL across
// the override × subscription matrix, including that an admin comp survives a lapse.
func TestRecomputeTruthTableAndAdminOverride(t *testing.T) {
	if testing.Short() {
		t.Skip("requires Docker")
	}
	pool, cleanup := setupDB(t)
	defer cleanup()
	ctx := context.Background()
	rc := premium.NewRecomputer(pool, zap.NewNop())

	tru, fals := true, false
	cases := []struct {
		name     string
		override *bool
		subSt    string // "" = no subscription row
		want     bool
	}{
		{"nil override + active sub", nil, "active", true},
		{"nil override + former sub", nil, "former", false},
		{"nil override + no sub", nil, "", false},
		{"admin grant + no sub (comp)", &tru, "", true},
		{"admin grant survives former sub", &tru, "former", true},
		{"admin deny beats active sub", &fals, "active", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			id := insertUser(t, pool, "rc-"+strings.ReplaceAll(tc.name, " ", "_"))
			_, err := pool.Exec(ctx, "UPDATE users SET premium_admin_override = $2 WHERE id = $1", id, tc.override)
			require.NoError(t, err)
			if tc.subSt != "" {
				_, err := pool.Exec(ctx,
					"INSERT INTO premium_subscriptions (user_id, provider, provider_user_id, status, cents) VALUES ($1,'patreon',$2,$3,500)",
					id, "pu-"+id, tc.subSt)
				require.NoError(t, err)
			}
			got, err := rc.Recompute(ctx, id)
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
			assert.Equal(t, tc.want, dbPremium(t, pool, id))
		})
	}
}

// TestWebhookGrantsPremiumThenGate is the end-to-end check: a signed Patreon
// webhook flips users.is_premium, and a premium-gated endpoint flips 403 -> 200 ->
// 403 accordingly. Only Patreon's network boundary is faked (we POST a synthetic,
// correctly-signed webhook to the real handler).
func TestWebhookGrantsPremiumThenGate(t *testing.T) {
	if testing.Short() {
		t.Skip("requires Docker")
	}
	pool, cleanup := setupDB(t)
	defer cleanup()
	ctx := context.Background()

	userID := insertUser(t, pool, "e2e")
	// Link patreon user -> all-chat user (token values are irrelevant to this path).
	_, err := pool.Exec(ctx,
		`INSERT INTO patreon_oauth_tokens (user_id, patreon_user_id, access_token, refresh_token, token_expires_at)
		 VALUES ($1, $2, 'enc', 'enc', NOW() + INTERVAL '30 days')`, userID, "pu-e2e")
	require.NoError(t, err)

	// Seed the 'sharing' premium gate so RequirePremium actually enforces premium.
	_, err = pool.Exec(ctx,
		"INSERT INTO feature_gates (feature_key, is_premium) VALUES ($1, TRUE) ON CONFLICT (feature_key) DO UPDATE SET is_premium = TRUE",
		featuregates.GateSharing)
	require.NoError(t, err)

	// Real webhook handler with miniredis + real repos/recompute.
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	subRepo := repository.NewSubscriptionRepository(pool, zap.NewNop())
	tokenRepo := repository.NewTokenRepository(pool, nil, zap.NewNop()) // nil cipher: GetByPatreonUserID needs no decryption
	svc := entitlement.NewService(subRepo, premium.NewRecomputer(pool, zap.NewNop()), minCents, viewerMinCents, zap.NewNop())
	const secret = "whsec_e2e"
	wh := handlers.NewWebhookHandler(secret, rdb, tokenRepo, svc, zap.NewNop())

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/api/v1/webhooks/patreon", wh.Handle)

	// A premium-gated endpoint guarded by the real RequirePremium middleware,
	// acting as our linked user.
	gates := featuregates.NewFeatureGateCache(pool, rdb, zap.NewNop())
	require.NoError(t, gates.Start(ctx))
	router.GET("/gated",
		func(c *gin.Context) { c.Set("user_id", userID); c.Next() },
		middleware.RequirePremium(pool, gates, featuregates.GateSharing, zap.NewNop()),
		func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) },
	)

	postWebhook := func(event, body string) int {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/patreon", strings.NewReader(body))
		req.Header.Set("X-Patreon-Event", event)
		req.Header.Set("X-Patreon-Signature", signMD5(secret, body))
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		return w.Code
	}
	getGated := func() int {
		req := httptest.NewRequest(http.MethodGet, "/gated", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		return w.Code
	}

	// Baseline: not premium -> gate denies.
	assert.Equal(t, http.StatusForbidden, getGated(), "should be 403 before subscribing")

	activeBody := `{"data":{"type":"member","id":"m1","attributes":{"patron_status":"active_patron","currently_entitled_amount_cents":500,"last_charge_status":"Paid"},"relationships":{"user":{"data":{"type":"user","id":"pu-e2e"}}}}}`
	assert.Equal(t, http.StatusOK, postWebhook(patreon.EventMembersCreate, activeBody))
	assert.True(t, dbPremium(t, pool, userID), "active webhook should grant premium")
	assert.Equal(t, http.StatusOK, getGated(), "gate should allow after subscribing")

	// A bad signature must be rejected and must not change state.
	badReq := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/patreon", strings.NewReader(activeBody))
	badReq.Header.Set("X-Patreon-Event", patreon.EventMembersUpdate)
	badReq.Header.Set("X-Patreon-Signature", "deadbeef")
	badW := httptest.NewRecorder()
	router.ServeHTTP(badW, badReq)
	assert.Equal(t, http.StatusUnauthorized, badW.Code)

	formerBody := `{"data":{"type":"member","id":"m1","attributes":{"patron_status":"former_patron"},"relationships":{"user":{"data":{"type":"user","id":"pu-e2e"}}}}}`
	assert.Equal(t, http.StatusOK, postWebhook(patreon.EventMembersDelete, formerBody))
	assert.False(t, dbPremium(t, pool, userID), "former webhook should revoke premium")
	assert.Equal(t, http.StatusForbidden, getGated(), "gate should deny after lapse")
}

// TestViewerEntitlementApplyGrantsAndRevokes mirrors the streamer apply test for the
// viewer product (ADR-0019): an active patron at the cheaper viewer threshold flips
// viewers.is_premium, and a lapse revokes it.
func TestViewerEntitlementApplyGrantsAndRevokes(t *testing.T) {
	if testing.Short() {
		t.Skip("requires Docker")
	}
	pool, cleanup := setupDB(t)
	defer cleanup()
	ctx := context.Background()

	viewerID := insertViewer(t, pool)
	subRepo := repository.NewSubscriptionRepository(pool, zap.NewNop())
	svc := entitlement.NewService(subRepo, premium.NewRecomputer(pool, zap.NewNop()), minCents, viewerMinCents, zap.NewNop())

	// Active patron at the viewer threshold -> viewer premium granted.
	_, isPrem, err := svc.Apply(ctx, &patreon.MembershipSnapshot{
		PatreonUserID: "pv-apply", PatronStatus: "active_patron", EntitledCents: viewerMinCents,
	}, nil, &viewerID, nil)
	require.NoError(t, err)
	assert.True(t, isPrem)
	assert.True(t, dbViewerPremium(t, pool, viewerID), "viewers.is_premium should be true after active viewer membership")

	// Below the viewer threshold -> not premium.
	_, isPrem, err = svc.Apply(ctx, &patreon.MembershipSnapshot{
		PatreonUserID: "pv-apply", PatronStatus: "active_patron", EntitledCents: viewerMinCents - 1,
	}, nil, &viewerID, nil)
	require.NoError(t, err)
	assert.False(t, isPrem)
	assert.False(t, dbViewerPremium(t, pool, viewerID), "below-threshold pledge should not grant viewer premium")

	// Former patron -> premium revoked.
	_, isPrem, err = svc.Apply(ctx, &patreon.MembershipSnapshot{
		PatreonUserID: "pv-apply", PatronStatus: "former_patron",
	}, nil, &viewerID, nil)
	require.NoError(t, err)
	assert.False(t, isPrem)
	assert.False(t, dbViewerPremium(t, pool, viewerID), "viewers.is_premium should be false after lapse")
}

// TestRecomputeViewerInheritanceAndOverride validates RecomputeViewer's derivation:
// inheritance from a linked premium streamer grants the badge, the tri-state admin
// override wins both ways, and a force-grant works with no subscription.
func TestRecomputeViewerInheritanceAndOverride(t *testing.T) {
	if testing.Short() {
		t.Skip("requires Docker")
	}
	pool, cleanup := setupDB(t)
	defer cleanup()
	ctx := context.Background()
	rc := premium.NewRecomputer(pool, zap.NewNop())

	// (a) Inheritance: viewer linked to a premium streamer is premium with no sub.
	premUser := insertUser(t, pool, "inh")
	_, err := pool.Exec(ctx, "UPDATE users SET is_premium = TRUE WHERE id = $1", premUser)
	require.NoError(t, err)
	vInherit := insertViewer(t, pool)
	linkViewerSessionToUser(t, pool, vInherit, premUser, "twitch", "tw-inh")

	got, err := rc.RecomputeViewer(ctx, vInherit)
	require.NoError(t, err)
	assert.True(t, got, "viewer linked to a premium streamer inherits the badge")
	assert.True(t, dbViewerPremium(t, pool, vInherit))

	// (b) Admin force-deny overrides inheritance.
	_, err = pool.Exec(ctx, "UPDATE viewers SET premium_admin_override = FALSE WHERE id = $1", vInherit)
	require.NoError(t, err)
	got, err = rc.RecomputeViewer(ctx, vInherit)
	require.NoError(t, err)
	assert.False(t, got, "admin force-deny beats inheritance")
	assert.False(t, dbViewerPremium(t, pool, vInherit))

	// (c) Admin force-grant with no sub and no inheritance.
	vComp := insertViewer(t, pool)
	_, err = pool.Exec(ctx, "UPDATE viewers SET premium_admin_override = TRUE WHERE id = $1", vComp)
	require.NoError(t, err)
	got, err = rc.RecomputeViewer(ctx, vComp)
	require.NoError(t, err)
	assert.True(t, got, "admin force-grant grants viewer premium")
	assert.True(t, dbViewerPremium(t, pool, vComp))
}

// TestViewerWebhookGrantsPremium is the viewer end-to-end: a connected viewer's
// signed Patreon webhook flips viewers.is_premium on and then off.
func TestViewerWebhookGrantsPremium(t *testing.T) {
	if testing.Short() {
		t.Skip("requires Docker")
	}
	pool, cleanup := setupDB(t)
	defer cleanup()
	ctx := context.Background()

	viewerID := insertViewer(t, pool)
	// Link patreon account -> viewer (token values irrelevant to this path).
	_, err := pool.Exec(ctx,
		`INSERT INTO patreon_oauth_tokens (viewer_id, patreon_user_id, access_token, refresh_token, token_expires_at)
		 VALUES ($1, $2, 'enc', 'enc', NOW() + INTERVAL '30 days')`, viewerID, "pv-e2e")
	require.NoError(t, err)

	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	subRepo := repository.NewSubscriptionRepository(pool, zap.NewNop())
	tokenRepo := repository.NewTokenRepository(pool, nil, zap.NewNop())
	svc := entitlement.NewService(subRepo, premium.NewRecomputer(pool, zap.NewNop()), minCents, viewerMinCents, zap.NewNop())
	const secret = "whsec_viewer"
	wh := handlers.NewWebhookHandler(secret, rdb, tokenRepo, svc, zap.NewNop())

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/api/v1/webhooks/patreon", wh.Handle)

	postWebhook := func(event, body string) int {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/patreon", strings.NewReader(body))
		req.Header.Set("X-Patreon-Event", event)
		req.Header.Set("X-Patreon-Signature", signMD5(secret, body))
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		return w.Code
	}

	assert.False(t, dbViewerPremium(t, pool, viewerID), "should not be premium before subscribing")

	// Active patron at the (cheaper) viewer threshold -> viewer premium.
	activeBody := `{"data":{"type":"member","id":"mv","attributes":{"patron_status":"active_patron","currently_entitled_amount_cents":200,"last_charge_status":"Paid"},"relationships":{"user":{"data":{"type":"user","id":"pv-e2e"}}}}}`
	assert.Equal(t, http.StatusOK, postWebhook(patreon.EventMembersCreate, activeBody))
	assert.True(t, dbViewerPremium(t, pool, viewerID), "active viewer webhook should grant viewer premium")

	formerBody := `{"data":{"type":"member","id":"mv","attributes":{"patron_status":"former_patron"},"relationships":{"user":{"data":{"type":"user","id":"pv-e2e"}}}}}`
	assert.Equal(t, http.StatusOK, postWebhook(patreon.EventMembersDelete, formerBody))
	assert.False(t, dbViewerPremium(t, pool, viewerID), "former viewer webhook should revoke viewer premium")
}

// TestOneSubjectConstraint asserts the ADR-0019 invariant that a subscription is
// anchored to a user XOR a viewer, never both.
func TestOneSubjectConstraint(t *testing.T) {
	if testing.Short() {
		t.Skip("requires Docker")
	}
	pool, cleanup := setupDB(t)
	defer cleanup()
	ctx := context.Background()

	userID := insertUser(t, pool, "both")
	viewerID := insertViewer(t, pool)
	_, err := pool.Exec(ctx,
		`INSERT INTO premium_subscriptions (user_id, viewer_id, product, provider, provider_user_id, status, cents)
		 VALUES ($1, $2, 'viewer', 'patreon', 'pu-both', 'active', 500)`, userID, viewerID)
	require.Error(t, err, "a subscription anchored to both a user and a viewer must violate the one-subject CHECK")
}

// TestRecomputeBetaTesterGrantsPremium validates the ADR-0020 input to the real
// recompute SQL: a beta-tester is premium with no subscription, an admin force-deny
// still beats it, and revoking the flag reverts to following the subscription.
func TestRecomputeBetaTesterGrantsPremium(t *testing.T) {
	if testing.Short() {
		t.Skip("requires Docker")
	}
	pool, cleanup := setupDB(t)
	defer cleanup()
	ctx := context.Background()
	rc := premium.NewRecomputer(pool, zap.NewNop())

	id := insertUser(t, pool, "beta")

	// (a) Beta-tester with no subscription and no override is premium.
	_, err := pool.Exec(ctx, "UPDATE users SET is_beta_tester = TRUE WHERE id = $1", id)
	require.NoError(t, err)
	got, err := rc.Recompute(ctx, id)
	require.NoError(t, err)
	assert.True(t, got, "a beta-tester is premium without any subscription")
	assert.True(t, dbPremium(t, pool, id))

	// (b) Admin force-deny beats beta-tester (override always wins).
	_, err = pool.Exec(ctx, "UPDATE users SET premium_admin_override = FALSE WHERE id = $1", id)
	require.NoError(t, err)
	got, err = rc.Recompute(ctx, id)
	require.NoError(t, err)
	assert.False(t, got, "admin force-deny overrides beta-tester premium")
	assert.False(t, dbPremium(t, pool, id))

	// (c) Revoking beta-tester (and clearing the override) reverts to the
	//     subscription, of which there is none -> not premium.
	_, err = pool.Exec(ctx, "UPDATE users SET is_beta_tester = FALSE, premium_admin_override = NULL WHERE id = $1", id)
	require.NoError(t, err)
	got, err = rc.Recompute(ctx, id)
	require.NoError(t, err)
	assert.False(t, got, "after revoking beta-tester with no sub, not premium")
	assert.False(t, dbPremium(t, pool, id))
}

// TestEarlyAccessGate is the end-to-end check for ADR-0020's early-access gate: the
// real RequireEarlyAccess middleware, backed by a real feature_gates row and the
// FeatureGateCache loaded from DB, admits a beta-tester and denies a plain user;
// graduating the gate (early_access=FALSE) opens it to everyone authenticated.
func TestEarlyAccessGate(t *testing.T) {
	if testing.Short() {
		t.Skip("requires Docker")
	}
	pool, cleanup := setupDB(t)
	defer cleanup()
	ctx := context.Background()

	betaUser := insertUser(t, pool, "beta-gate")
	_, err := pool.Exec(ctx, "UPDATE users SET is_beta_tester = TRUE WHERE id = $1", betaUser)
	require.NoError(t, err)
	plainUser := insertUser(t, pool, "plain-gate")

	const earlyKey = "beta-feature"
	_, err = pool.Exec(ctx,
		"INSERT INTO feature_gates (feature_key, is_premium, early_access) VALUES ($1, FALSE, TRUE) ON CONFLICT (feature_key) DO UPDATE SET early_access = TRUE",
		earlyKey)
	require.NoError(t, err)

	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})

	gin.SetMode(gin.TestMode)
	// serve mounts a throwaway router gated by RequireEarlyAccess acting as userID,
	// using the given cache, and returns the status code.
	serve := func(gates *featuregates.FeatureGateCache, userID string) int {
		r := gin.New()
		r.GET("/x",
			func(c *gin.Context) { c.Set("user_id", userID); c.Next() },
			middleware.RequireEarlyAccess(pool, gates, earlyKey, zap.NewNop()),
			func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) },
		)
		req := httptest.NewRequest(http.MethodGet, "/x", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w.Code
	}

	// early_access = TRUE: beta-tester passes, plain user is denied.
	gates := featuregates.NewFeatureGateCache(pool, rdb, zap.NewNop())
	require.NoError(t, gates.Start(ctx))
	assert.Equal(t, http.StatusOK, serve(gates, betaUser), "beta-tester passes the early-access gate")
	assert.Equal(t, http.StatusForbidden, serve(gates, plainUser), "non-beta user is denied the early-access feature")

	// Graduate the feature: a fresh cache (deterministic initial reload) sees
	// early_access = FALSE and the gate opens to any authenticated user.
	_, err = pool.Exec(ctx, "UPDATE feature_gates SET early_access = FALSE WHERE feature_key = $1", earlyKey)
	require.NoError(t, err)
	graduated := featuregates.NewFeatureGateCache(pool, rdb, zap.NewNop())
	require.NoError(t, graduated.Start(ctx))
	assert.Equal(t, http.StatusOK, serve(graduated, plainUser), "graduated feature is open to all authenticated users")
}

// ---- helpers -------------------------------------------------------------------

func signMD5(secret, body string) string {
	mac := hmac.New(md5.New, []byte(secret))
	mac.Write([]byte(body))
	return hex.EncodeToString(mac.Sum(nil))
}

func dbPremium(t *testing.T, pool *pgxpool.Pool, userID string) bool {
	t.Helper()
	var p bool
	require.NoError(t, pool.QueryRow(context.Background(), "SELECT is_premium FROM users WHERE id = $1", userID).Scan(&p))
	return p
}

func dbViewerPremium(t *testing.T, pool *pgxpool.Pool, viewerID string) bool {
	t.Helper()
	var p bool
	require.NoError(t, pool.QueryRow(context.Background(), "SELECT is_premium FROM viewers WHERE id = $1", viewerID).Scan(&p))
	return p
}

// insertViewer creates a bare durable viewer identity and returns its id.
func insertViewer(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()
	var id string
	require.NoError(t, pool.QueryRow(context.Background(), "INSERT INTO viewers DEFAULT VALUES RETURNING id").Scan(&id))
	return id
}

// linkViewerSessionToUser creates a viewer_sessions row tying (platform,
// platformUserID) to both the durable viewer and a streamer users account, so
// RecomputeViewer's inheritance term can resolve the linked streamer.
func linkViewerSessionToUser(t *testing.T, pool *pgxpool.Pool, viewerID, userID, platform, platformUserID string) {
	t.Helper()
	_, err := pool.Exec(context.Background(),
		`INSERT INTO viewer_sessions (platform, platform_user_id, username, display_name, access_token, token_expires_at, viewer_id, user_id)
		 VALUES ($1, $2, 'u', 'u', 'enc', NOW() + INTERVAL '1 day', $3, $4)`,
		platform, platformUserID, viewerID, userID)
	require.NoError(t, err)
}

var userSeq int

// insertUser creates a minimal user and returns its id. The label is ignored for
// the stored username (kept short to fit varchar(50)); uniqueness comes from a
// per-process sequence counter.
func insertUser(t *testing.T, pool *pgxpool.Pool, label string) string {
	t.Helper()
	userSeq++
	uniq := itoa(userSeq)
	var id string
	err := pool.QueryRow(context.Background(), `
		INSERT INTO users (twitch_id, auth_provider, username, display_name, access_token, refresh_token, token_expires_at)
		VALUES ($1, 'twitch', $2, $2, 'a', 'r', NOW() + INTERVAL '4 hours')
		RETURNING id`, "tw-"+uniq, "u-"+uniq).Scan(&id)
	require.NoError(t, err)
	return id
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var b []byte
	for i > 0 {
		b = append([]byte{byte('0' + i%10)}, b...)
		i /= 10
	}
	return string(b)
}

func setupDB(t *testing.T) (*pgxpool.Pool, func()) {
	t.Helper()
	ctx := context.Background()
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "postgres:16-alpine",
			ExposedPorts: []string{"5432/tcp"},
			Env: map[string]string{
				"POSTGRES_USER":     "testuser",
				"POSTGRES_PASSWORD": "testpass",
				"POSTGRES_DB":       "testdb",
			},
			WaitingFor: wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).WithStartupTimeout(60 * time.Second),
		},
		Started: true,
	})
	require.NoError(t, err)

	host, err := container.Host(ctx)
	require.NoError(t, err)
	port, err := container.MappedPort(ctx, "5432")
	require.NoError(t, err)

	pool, err := pgxpool.New(ctx, "postgres://testuser:testpass@"+host+":"+port.Port()+"/testdb?sslmode=disable")
	require.NoError(t, err)

	runMigrations(t, pool)

	return pool, func() {
		pool.Close()
		_ = container.Terminate(ctx)
	}
}

// runMigrations applies migrations/[0-9]*.sql in order, skipping *_down.sql — the
// same selection as scripts/run-migrations.sh.
func runMigrations(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	dir := filepath.Join("..", "..", "..", "migrations")
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)

	var names []string
	for _, e := range entries {
		n := e.Name()
		if e.IsDir() || !strings.HasSuffix(n, ".sql") || strings.HasSuffix(n, "_down.sql") {
			continue
		}
		if n[0] < '0' || n[0] > '9' {
			continue
		}
		names = append(names, n)
	}
	sort.Strings(names)
	require.NotEmpty(t, names, "no migrations found")

	ctx := context.Background()
	for _, n := range names {
		sql, err := os.ReadFile(filepath.Join(dir, n))
		require.NoError(t, err)
		_, err = pool.Exec(ctx, string(sql))
		require.NoErrorf(t, err, "migration %s", n)
	}
}
