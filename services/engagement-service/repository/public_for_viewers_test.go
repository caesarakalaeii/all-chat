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

package repository_test

import (
	"context"
	"testing"

	"github.com/caesar/all-chat/services/engagement-service/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestP3_4_OverlayPublicForViewers: the predicate the direct overlay-id viewer endpoints
// now gate on must return true only when the overlay is active AND public-for-viewers AND
// the owner is not banned — so toggling public-for-viewers OFF stops participation.
func TestP3_4_OverlayPublicForViewers(t *testing.T) {
	pool := newTestDB(t)
	repo := repository.New(pool)
	ctx := context.Background()
	overlay := seedOverlay(t, pool)

	set := func(active, public bool) {
		_, err := pool.Exec(ctx,
			`UPDATE overlays SET is_active = $2, is_public_for_viewers = $3 WHERE id = $1`,
			overlay, active, public)
		require.NoError(t, err)
	}

	set(true, true)
	ok, err := repo.OverlayPublicForViewers(ctx, overlay)
	require.NoError(t, err)
	assert.True(t, ok, "active + public-for-viewers accepts participation")

	set(true, false)
	ok, err = repo.OverlayPublicForViewers(ctx, overlay)
	require.NoError(t, err)
	assert.False(t, ok, "toggling public-for-viewers off stops participation (P3-4)")

	set(false, true)
	ok, err = repo.OverlayPublicForViewers(ctx, overlay)
	require.NoError(t, err)
	assert.False(t, ok, "an inactive overlay accepts no participation")
}
