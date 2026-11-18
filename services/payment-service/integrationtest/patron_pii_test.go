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
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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

// piiWebhookBody is a members:create payload shaped like Patreon's: the member
// itself carries the patron's email, name and note, and the included user and
// address resources carry the rest of their profile.
const piiWebhookBody = `{
	"data": {
		"type": "member", "id": "m-pii",
		"attributes": {
			"patron_status": "active_patron", "currently_entitled_amount_cents": 500, "last_charge_status": "Paid",
			"email": "fan@example.com", "full_name": "Ada Fan", "note": "private note"
		},
		"relationships": {
			"user": {"data": {"type": "user", "id": "pu-pii"}},
			"address": {"data": {"type": "address", "id": "addr-1"}}
		}
	},
	"included": [
		{"type": "user", "id": "pu-pii", "attributes": {
			"email": "fan@example.com", "full_name": "Ada Fan", "first_name": "Ada", "vanity": "adafan",
			"url": "https://www.patreon.com/adafan", "social_connections": {"discord": {"user_id": "424242"}}
		}},
		{"type": "address", "id": "addr-1", "attributes": {
			"addressee": "Ada Fan", "line_1": "1 Main Street", "city": "Springfield", "phone_number": "+15550100"
		}}
	]
}`

var piiValues = []string{"fan@example.com", "Ada Fan", "private note", "adafan", "424242", "Main Street", "+15550100"}

// The subscription row keeps the ids, status, tier and amounts entitlement is
// decided from. Nothing personal from the webhook body may reach it.
func TestWebhookPersistsNoPatronPII(t *testing.T) {
	if testing.Short() {
		t.Skip("requires Docker")
	}
	pool, cleanup := setupDB(t)
	defer cleanup()
	ctx := context.Background()

	userID := insertUser(t, pool, "pii")
	_, err := pool.Exec(ctx,
		`INSERT INTO patreon_oauth_tokens (user_id, patreon_user_id, access_token, refresh_token, token_expires_at)
		 VALUES ($1, 'pu-pii', 'enc', 'enc', NOW() + INTERVAL '30 days')`, userID)
	require.NoError(t, err)

	rdb := redis.NewClient(&redis.Options{Addr: miniredis.RunT(t).Addr()})
	svc := entitlement.NewService(repository.NewSubscriptionRepository(pool, zap.NewNop()),
		premium.NewRecomputer(pool, zap.NewNop()), minCents, viewerMinCents, zap.NewNop())
	const secret = "whsec_pii"
	wh := handlers.NewWebhookHandler(secret, rdb, repository.NewTokenRepository(pool, nil, zap.NewNop()), svc, zap.NewNop())
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/api/v1/webhooks/patreon", wh.Handle)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/patreon", strings.NewReader(piiWebhookBody))
	req.Header.Set("X-Patreon-Event", patreon.EventMembersCreate)
	req.Header.Set("X-Patreon-Signature", signMD5(secret, piiWebhookBody))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	assert.True(t, dbPremium(t, pool, userID), "the webhook must still grant premium")
	assertNoPatronPII(t, subscriptionRow(t, pool, "pu-pii"))
}

// Rows written before migration 120 hold the full webhook body in raw. The
// migration must remove it, keep the row itself, and survive the runner
// re-applying every migration on every pod start.
func TestMigration120RemovesStoredPatreonPayloads(t *testing.T) {
	if testing.Short() {
		t.Skip("requires Docker")
	}
	pool, cleanup := setupDB(t)
	defer cleanup()
	ctx := context.Background()

	applyMigrationFile(t, pool, "120_drop_patreon_raw_payload_down.sql")
	_, err := pool.Exec(ctx,
		`INSERT INTO premium_subscriptions (provider_user_id, status, cents, raw) VALUES ('pu-old', 'active', 500, $1)`,
		piiWebhookBody)
	require.NoError(t, err)

	runMigrations(t, pool)
	runMigrations(t, pool)

	assertNoPatronPII(t, subscriptionRow(t, pool, "pu-old"))
	var status string
	require.NoError(t, pool.QueryRow(ctx,
		"SELECT status FROM premium_subscriptions WHERE provider_user_id = 'pu-old'").Scan(&status))
	assert.Equal(t, "active", status)
}

func subscriptionRow(t *testing.T, pool *pgxpool.Pool, patreonUserID string) string {
	t.Helper()
	var row string
	require.NoError(t, pool.QueryRow(context.Background(),
		"SELECT to_jsonb(s)::text FROM premium_subscriptions s WHERE provider_user_id = $1", patreonUserID).Scan(&row))
	return row
}

func assertNoPatronPII(t *testing.T, row string) {
	t.Helper()
	for _, v := range piiValues {
		assert.NotContains(t, row, v)
	}
}

func applyMigrationFile(t *testing.T, pool *pgxpool.Pool, name string) {
	t.Helper()
	sql, err := os.ReadFile(filepath.Join("..", "..", "..", "migrations", name))
	require.NoError(t, err)
	_, err = pool.Exec(context.Background(), string(sql))
	require.NoErrorf(t, err, "migration %s", name)
}
