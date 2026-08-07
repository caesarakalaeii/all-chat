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

package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The delegatable set is a closed allowlist, and this is the only place a grant's actions are
// admitted. ModerationScopesForActions downstream also accepts "engagement" (mapping to
// channel:read:polls / channel:read:predictions), so anything that reaches it unfiltered is a
// scope-widening hole. Normalising here is what keeps that from being reachable via a grant.
func TestNormalizeDelegatedActions_RejectsAnythingOutsideTheFourVerbs(t *testing.T) {
	for _, bad := range []string{"engagement", "send", "rediscover", "DELETE", "", "delete "} {
		t.Run(bad, func(t *testing.T) {
			_, err := NormalizeDelegatedActions([]string{bad})
			assert.Error(t, err, "%q must not be storable as a delegated action", bad)
		})
	}
}

func TestNormalizeDelegatedActions_AcceptsTheFourVerbs(t *testing.T) {
	got, err := NormalizeDelegatedActions([]string{"delete", "timeout", "ban", "unban"})
	require.NoError(t, err)
	assert.Equal(t, []string{"delete", "timeout", "ban", "unban"}, got)
}

func TestNormalizeDelegatedActions_DedupesAndOrdersCanonically(t *testing.T) {
	got, err := NormalizeDelegatedActions([]string{"ban", "delete", "ban", "timeout"})
	require.NoError(t, err)
	assert.Equal(t, []string{"delete", "timeout", "ban"}, got,
		"canonical order keeps the stored array, the API response and the UI in agreement")
}

func TestNormalizeDelegatedActions_EmptyMeansTheDefaultPair(t *testing.T) {
	got, err := NormalizeDelegatedActions(nil)
	require.NoError(t, err)
	assert.Equal(t, []string{"delete", "timeout"}, got,
		"the safe default is the pair a volunteer needs day to day; ban/unban stay opt-in")
	assert.Equal(t, DefaultDelegatedActions, got)
}

// An absent list means "use the default"; an explicitly empty one means "grant nothing", and
// those must not collapse into each other. Defaulting an explicit [] would hand out delete and
// timeout to a caller who asked for neither.
func TestNormalizeDelegatedActions_ExplicitlyEmptyIsNotTheDefault(t *testing.T) {
	_, err := NormalizeDelegatedActions([]string{})
	assert.ErrorIs(t, err, ErrNoActions,
		"an explicitly empty list must be an error, never silently widened to the default")
}

func TestNormalizeDelegatedPlatforms(t *testing.T) {
	t.Run("the four moderation platforms are delegatable", func(t *testing.T) {
		got, err := NormalizeDelegatedPlatforms([]string{"twitch", "kick", "youtube", "discord"})
		require.NoError(t, err)
		assert.ElementsMatch(t, []string{"twitch", "kick", "youtube", "discord"}, got)
	})

	t.Run("tiktok has no moderation API, so a leg for it is refused rather than dead", func(t *testing.T) {
		_, err := NormalizeDelegatedPlatforms([]string{"tiktok"})
		assert.Error(t, err)
	})

	// A shared_overlay source displays another streamer's chat. Owner-only authorization made it
	// unreachable by construction; a delegated grant must not be the thing that reopens it.
	t.Run("shared_overlay can never be delegated", func(t *testing.T) {
		_, err := NormalizeDelegatedPlatforms([]string{"shared_overlay"})
		assert.Error(t, err)
	})

	t.Run("nothing enabled is the fail-closed default", func(t *testing.T) {
		got, err := NormalizeDelegatedPlatforms(nil)
		require.NoError(t, err)
		assert.Empty(t, got, "a platform is never implicitly delegated — Discord least of all")
	})

	t.Run("duplicates collapse", func(t *testing.T) {
		got, err := NormalizeDelegatedPlatforms([]string{"twitch", "twitch"})
		require.NoError(t, err)
		assert.Equal(t, []string{"twitch"}, got)
	})
}

// Pre-binding an invite to a platform account is only honest where we can actually resolve the
// accepting user's id on that platform. Twitch is the only one today (the mod picker reads Helix);
// accepting a binding we cannot check would mean storing a constraint that silently does nothing.
func TestPreBindablePlatform(t *testing.T) {
	assert.True(t, PreBindablePlatform("twitch"))
	for _, p := range []string{"kick", "youtube", "discord", "tiktok", "", "shared_overlay"} {
		assert.False(t, PreBindablePlatform(p), "%q must not be accepted as a pre-binding", p)
	}
}

func TestModeratorsPerOverlayCap(t *testing.T) {
	assert.Equal(t, 10, ModeratorsPerOverlayCap,
		"the cap is enforced at invite time only, so changing it can never cut off a working mod team")
}
