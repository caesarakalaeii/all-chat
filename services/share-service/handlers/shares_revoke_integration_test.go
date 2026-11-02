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

// Share revocation and expiry against a Postgres with every migration applied. CI runs
// them in the test-engagement-integration job; locally point TEST_DATABASE_URL at a
// migrated database and run: go test -tags=integration ./handlers/
package handlers

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/caesar/all-chat/services/share-service/repository"
	"github.com/caesar/all-chat/shared/auth"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func testDSN() string {
	if v := os.Getenv("TEST_DATABASE_URL"); v != "" {
		return v
	}
	return "postgres://allchat:allchat_dev_password@localhost:5432/allchat?sslmode=disable"
}

// shareWorld is one sender overlay shared with two recipients, each of whom added the
// share as a source the way overlay-manager's AddSource stores it: platform
// 'shared_overlay', channel_id = the SENDER OVERLAY id.
type shareWorld struct {
	pool                *pgxpool.Pool
	sender, r1, r2      string // user ids
	senderOverlay       string
	r1Source, r2Source  string // the recipients' shared_overlay source ids
	share1, share2      string // sender -> r1, sender -> r2, both accepted
	userIDs, overlayIDs []string
}

func newShareWorld(t *testing.T) *shareWorld {
	t.Helper()
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, testDSN())
	require.NoError(t, err, "integration tests need a migrated Postgres (TEST_DATABASE_URL)")
	w := &shareWorld{pool: pool}
	t.Cleanup(func() {
		bg := context.Background()
		_, _ = pool.Exec(bg, `DELETE FROM share_requests WHERE sender_user_id = ANY($1::uuid[])`, w.userIDs)
		_, _ = pool.Exec(bg, `DELETE FROM overlays WHERE id = ANY($1::uuid[])`, w.overlayIDs)
		_, _ = pool.Exec(bg, `DELETE FROM users WHERE id = ANY($1::uuid[])`, w.userIDs)
		pool.Close()
	})

	w.sender = w.user(t, "sender")
	w.r1 = w.user(t, "r1")
	w.r2 = w.user(t, "r2")
	w.senderOverlay = w.overlay(t, w.sender)
	w.share1 = w.acceptedShare(t, w.r1)
	w.share2 = w.acceptedShare(t, w.r2)
	w.r1Source = w.sharedSource(t, w.overlay(t, w.r1))
	w.r2Source = w.sharedSource(t, w.overlay(t, w.r2))
	return w
}

func (w *shareWorld) user(t *testing.T, name string) string {
	t.Helper()
	tag := fmt.Sprintf("%s_%d", name, time.Now().UnixNano())
	var id string
	require.NoError(t, w.pool.QueryRow(context.Background(), `
		INSERT INTO users (twitch_id, username, display_name, access_token, refresh_token, token_expires_at)
		VALUES ($1, $1, $1, 'a', 'r', NOW() + INTERVAL '1 hour') RETURNING id`, tag).Scan(&id))
	w.userIDs = append(w.userIDs, id)
	return id
}

func (w *shareWorld) overlay(t *testing.T, userID string) string {
	t.Helper()
	var id string
	require.NoError(t, w.pool.QueryRow(context.Background(),
		`INSERT INTO overlays (user_id, name) VALUES ($1, 'share test') RETURNING id`, userID).Scan(&id))
	w.overlayIDs = append(w.overlayIDs, id)
	return id
}

func (w *shareWorld) acceptedShare(t *testing.T, recipient string) string {
	t.Helper()
	var id string
	require.NoError(t, w.pool.QueryRow(context.Background(), `
		INSERT INTO share_requests (sender_user_id, sender_overlay_id, recipient_user_id, status, responded_at)
		VALUES ($1, $2, $3, 'accepted', NOW()) RETURNING id`, w.sender, w.senderOverlay, recipient).Scan(&id))
	return id
}

func (w *shareWorld) sharedSource(t *testing.T, overlayID string) string {
	t.Helper()
	var id string
	require.NoError(t, w.pool.QueryRow(context.Background(), `
		INSERT INTO overlay_chat_sources (overlay_id, platform, channel_id, channel_name, is_active)
		VALUES ($1, 'shared_overlay', $2, 'share test', true) RETURNING id`, overlayID, w.senderOverlay).Scan(&id))
	return id
}

func (w *shareWorld) sourceActive(t *testing.T, sourceID string) bool {
	t.Helper()
	var active bool
	require.NoError(t, w.pool.QueryRow(context.Background(),
		`SELECT is_active FROM overlay_chat_sources WHERE id = $1`, sourceID).Scan(&active))
	return active
}

func (w *shareWorld) revoke(shareID, callerUserID string) int {
	gin.SetMode(gin.TestMode)
	keys := auth.NewKeyChain(map[string][]byte{"k1": []byte("0123456789abcdef0123456789abcdef")}, nil, "k1")
	h := NewShareHandler(repository.NewShareRepository(w.pool, zap.NewNop()), nil, w.pool, zap.NewNop(), nil, keys)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/shares/"+shareID+"/revoke", nil)
	c.Params = gin.Params{{Key: "id", Value: shareID}}
	c.Set("user_id", callerUserID)
	h.RevokeShareRequest(c)
	return rec.Code
}

// Revoking a share switches off exactly that recipient's shared_overlay source. The
// source stores the sender overlay id, not the share id, so a revoke keyed on the share
// id matched nothing and the recipient kept the sender's chat.
func TestRevokeShareRequest_DeactivatesRecipientSource(t *testing.T) {
	w := newShareWorld(t)

	assert.Equal(t, http.StatusForbidden, w.revoke(w.share1, w.r2), "a non-participant cannot revoke")
	assert.True(t, w.sourceActive(t, w.r1Source))

	require.Equal(t, http.StatusOK, w.revoke(w.share1, w.r1), "the recipient can revoke")
	assert.False(t, w.sourceActive(t, w.r1Source), "revoked share must switch off the recipient's source")
	assert.True(t, w.sourceActive(t, w.r2Source), "another recipient's share of the same overlay is untouched")

	assert.Equal(t, http.StatusConflict, w.revoke(w.share1, w.r1), "a revoked share is not active")
	assert.Equal(t, http.StatusNotFound, w.revoke("00000000-0000-0000-0000-000000000000", w.r1))

	require.Equal(t, http.StatusOK, w.revoke(w.share2, w.sender), "the sender can revoke")
	assert.False(t, w.sourceActive(t, w.r2Source))
}

// A recipient holding a second accepted share of the same overlay keeps the source:
// it is still entitled to the chat.
func TestRevokeShareRequest_KeepsSourceWhileAnotherShareIsAccepted(t *testing.T) {
	w := newShareWorld(t)
	w.acceptedShare(t, w.r1)

	require.Equal(t, http.StatusOK, w.revoke(w.share1, w.sender))
	assert.True(t, w.sourceActive(t, w.r1Source))
}

// Expiry (custom timer and this_stream) goes through ExpireAcceptedShare and must switch
// the source off the same way revocation does.
func TestExpireAcceptedShare_DeactivatesRecipientSource(t *testing.T) {
	w := newShareWorld(t)
	repo := repository.NewShareRepository(w.pool, zap.NewNop())

	require.NoError(t, repo.ExpireAcceptedShare(context.Background(), w.share1))
	assert.False(t, w.sourceActive(t, w.r1Source), "expired share must switch off the recipient's source")
	assert.True(t, w.sourceActive(t, w.r2Source))

	_, err := w.pool.Exec(context.Background(), `
		UPDATE share_requests SET expiry_option = 'custom', share_expires_at = NOW() - INTERVAL '1 minute'
		WHERE id = $1`, w.share2)
	require.NoError(t, err)
	n, err := repo.ExpireTimedAcceptedShares(context.Background())
	require.NoError(t, err)
	assert.GreaterOrEqual(t, n, 1)
	assert.False(t, w.sourceActive(t, w.r2Source), "the timed expiry job must switch the source off too")
}
