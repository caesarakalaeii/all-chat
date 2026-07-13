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

package repository_test

import (
	"context"
	"testing"
	"time"

	"github.com/caesar/all-chat/services/token-refresh-service/repository"
	"golang.org/x/oauth2"
)

// TestRecoveryWindowSQL verifies that the SQL constant used in GetExpiring* queries
// includes a 48-hour recovery window lower bound, not an unbounded expired clause.
//
// These tests do not hit a real database. They verify that the query strings embedded
// in the repository functions do NOT contain the unbounded form:
//
//	OR (token_expires_at < NOW())
//
// and DO contain the bounded form:
//
//	BETWEEN NOW() - INTERVAL '48 hours' AND NOW()
//
// This is a compile-time / source-level guard so that if someone reverts the fix
// the test suite will immediately flag the regression.
func TestGetExpiringUserTokens_QueryHasBoundedRecoveryWindow(t *testing.T) {
	// The exported constant is used to verify the SQL without a live DB.
	q := repository.QueryGetExpiringUserTokens
	assertNoBoundedExpiredClause(t, "GetExpiringUserTokens", q)
	assertBoundedRecoveryWindow(t, "GetExpiringUserTokens", q)
}

func TestGetExpiringViewerTokens_QueryHasBoundedRecoveryWindow(t *testing.T) {
	q := repository.QueryGetExpiringViewerTokens
	assertNoBoundedExpiredClause(t, "GetExpiringViewerTokens", q)
	assertBoundedRecoveryWindow(t, "GetExpiringViewerTokens", q)
}

func TestGetExpiringYouTubeTokens_QueryHasBoundedRecoveryWindow(t *testing.T) {
	q := repository.QueryGetExpiringYouTubeTokens
	assertNoBoundedExpiredClause(t, "GetExpiringYouTubeTokens", q)
	assertBoundedRecoveryWindow(t, "GetExpiringYouTubeTokens", q)
}

// TestGetExpiringUserTokens_PrioritisesActiveChatScope guards the anti-starvation
// fix: active EventSub streamers (tokens carrying user:read:chat) must sort ahead of
// dormant tokens so a synchronized herd of expiring tokens cannot push a live
// channel's token out of the LIMITed batch and let it lapse mid-stream.
func TestGetExpiringUserTokens_PrioritisesActiveChatScope(t *testing.T) {
	q := repository.QueryGetExpiringUserTokens
	if !contains(q, "user:read:chat") {
		t.Errorf("GetExpiringUserTokens: query no longer prioritises active chat-scoped streamers — expected an ORDER BY keyed on user:read:chat")
	}
}

// TestExpiringQueries_LimitIsParameterised guards that the per-type per-batch cap is
// wired to the configurable batch size ($N) rather than hardcoded to 100, which
// previously truncated the hourly herd and starved whatever sorted last.
func TestExpiringQueries_LimitIsParameterised(t *testing.T) {
	queries := map[string]string{
		"GetExpiringUserTokens":       repository.QueryGetExpiringUserTokens,
		"GetExpiringViewerTokens":     repository.QueryGetExpiringViewerTokens,
		"GetExpiringYouTubeTokens":    repository.QueryGetExpiringYouTubeTokens,
		"GetExpiringTwitchLinkTokens": repository.QueryGetExpiringTwitchLinkTokens,
	}
	for name, q := range queries {
		if contains(q, "LIMIT 100") {
			t.Errorf("%s: query still hardcodes LIMIT 100 — wire it to the configurable batch size", name)
		}
		if !contains(q, "LIMIT $") {
			t.Errorf("%s: query LIMIT is not parameterised ($N) — expected a bind-parameter limit", name)
		}
	}
}

// TestMarkUserTokenPermanentlyFailed_ExistsOnRepository verifies that the repository
// exposes the MarkUserTokenPermanentlyFailed method. Compilation of this file is the
// assertion — if the method does not exist the package will not compile.
func TestMarkUserTokenPermanentlyFailed_MethodExists(t *testing.T) {
	type permanentFailMarker interface {
		MarkUserTokenPermanentlyFailed(ctx context.Context, id string, suppressDuration time.Duration) error
	}

	// Verify *TokenRepository satisfies the interface.
	// The blank identifier means we only care about compilation, not runtime.
	var _ permanentFailMarker = (*repository.TokenRepository)(nil)
}

func TestMarkViewerTokenPermanentlyFailed_MethodExists(t *testing.T) {
	type permanentFailMarker interface {
		MarkViewerTokenPermanentlyFailed(ctx context.Context, sessionID string, suppressDuration time.Duration) error
	}
	var _ permanentFailMarker = (*repository.TokenRepository)(nil)
}

func TestMarkYouTubeTokenPermanentlyFailed_MethodExists(t *testing.T) {
	type permanentFailMarker interface {
		MarkYouTubeTokenPermanentlyFailed(ctx context.Context, userID, channelID string, suppressDuration time.Duration) error
	}
	var _ permanentFailMarker = (*repository.TokenRepository)(nil)
}

// ---- helpers ----------------------------------------------------------------

func assertNoBoundedExpiredClause(t *testing.T, name, query string) {
	t.Helper()
	// The old unbounded form that must NOT appear in the query.
	forbidden := "OR (token_expires_at < NOW())"
	if contains(query, forbidden) {
		t.Errorf("%s: query still contains unbounded expired clause %q — update it to use the 48-hour recovery window", name, forbidden)
	}
	// Also catch the expiry column name used by youtube_oauth_tokens table.
	forbiddenYT := "OR (expiry < NOW())"
	if contains(query, forbiddenYT) {
		t.Errorf("%s: query still contains unbounded expired clause %q — update it to use the 48-hour recovery window", name, forbiddenYT)
	}
}

func assertBoundedRecoveryWindow(t *testing.T, name, query string) {
	t.Helper()
	needle := "48 hours"
	if !contains(query, needle) {
		t.Errorf("%s: query does not contain 48-hour recovery window — expected %q in query", name, needle)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// --- linked Twitch credentials (ADR-0016, twitch_oauth_tokens) ---

func TestGetExpiringTwitchLinkTokens_QueryHasBoundedRecoveryWindow(t *testing.T) {
	q := repository.QueryGetExpiringTwitchLinkTokens
	assertNoBoundedExpiredClause(t, "GetExpiringTwitchLinkTokens", q)
	assertBoundedRecoveryWindow(t, "GetExpiringTwitchLinkTokens", q)
}

func TestUpdateTwitchLinkTokens_MethodExists(t *testing.T) {
	type updater interface {
		UpdateTwitchLinkTokens(ctx context.Context, userID, twitchLogin string, token *oauth2.Token) error
	}
	var _ updater = (*repository.TokenRepository)(nil)
}

func TestMarkTwitchLinkTokenPermanentlyFailed_MethodExists(t *testing.T) {
	type permanentFailMarker interface {
		MarkTwitchLinkTokenPermanentlyFailed(ctx context.Context, userID, twitchLogin string, suppressDuration time.Duration) error
	}
	var _ permanentFailMarker = (*repository.TokenRepository)(nil)
}
