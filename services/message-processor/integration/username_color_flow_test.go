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

package integration

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/caesar/all-chat/services/message-processor/enricher"
	"github.com/caesar/all-chat/services/message-processor/models"
	"github.com/caesar/all-chat/services/message-processor/normalizer"
	"github.com/jackc/pgx/v5"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// Username-colour resolution across the real pipeline (ADR-0047):
//
//	raw chat:raw payload -> TwitchNormalizer -> ViewerBadgeEnricher -> wire fields
//
// This is the closest thing to an end-to-end test the colour chain admits. A true
// browser-level E2E is not possible here: the last hop is a CSS custom-property
// fallback (`var(--chat-username-color, <auto_color>)`) that only resolves at paint
// time in a real overlay, against per-overlay visual settings that live in the
// frontend. That hop is covered by the unit tests for `resolveUsernameColor`.
//
// The raw payloads below are verbatim shapes observed on production `chat:raw`
// (2026-08-01), including the two real cases that motivated the change: a chatter
// who set a Twitch colour and one who never did.

// fakeCosmeticsRow returns a canned viewer_cosmetics lookup result.
type fakeCosmeticsRow struct {
	viewerID  string
	nameColor *string
	err       error
}

func (r *fakeCosmeticsRow) Scan(dest ...interface{}) error {
	if r.err != nil {
		return r.err
	}
	// Column order must match the enricher's SELECT: viewer_id, name_color,
	// name_gradient, avatar_frame_url, avatar_flair_url, is_admin, is_premium,
	// twitch_username.
	*dest[0].(*string) = r.viewerID
	*dest[1].(**string) = r.nameColor
	*dest[2].(*[]byte) = nil
	*dest[3].(*string) = ""
	*dest[4].(*string) = ""
	*dest[5].(*bool) = false
	*dest[6].(*bool) = false
	*dest[7].(*string) = ""
	return nil
}

// fakeCosmeticsDB serves one canned row for every lookup.
type fakeCosmeticsDB struct{ row *fakeCosmeticsRow }

func (d *fakeCosmeticsDB) QueryRow(_ context.Context, _ string, _ ...interface{}) pgx.Row {
	return d.row
}

// runColorPipeline normalizes a raw Twitch payload and runs viewer enrichment,
// returning the user block exactly as it would be published to the overlay.
func runColorPipeline(t *testing.T, raw *models.RawChatMessage, row *fakeCosmeticsRow) models.UserInfo {
	t.Helper()

	mr := miniredis.RunT(t)
	redisClient := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = redisClient.Close() })

	unified, err := normalizer.NewTwitchNormalizer().Normalize(raw, "overlay-1")
	require.NoError(t, err, "normalize should succeed")

	e := enricher.NewViewerBadgeEnricher(redisClient, &fakeCosmeticsDB{row: row}, zap.NewNop())
	require.NoError(t, e.Enrich(context.Background(), unified), "enrich should succeed")

	return unified.User
}

func rawTwitchMsg(userID, username, color string) *models.RawChatMessage {
	tags := map[string]string{
		"display-name": username,
		"id":           "a3fe22e0-e8d9-4019-8b71-245a073eb8a8",
		"mod":          "0",
		"subscriber":   "1",
		"turbo":        "0",
	}
	if color != "" {
		tags["color"] = color
	}
	return &models.RawChatMessage{
		MessageID: "af83bc4e-7333-4e25-941b-0a3478f9dc34",
		Platform:  "twitch",
		ChannelID: "swordmaid",
		UserID:    userID,
		Username:  username,
		Text:      "that was it!",
		Timestamp: time.Now(),
		Tags:      tags,
	}
}

// The bug as reported: a chatter's own Twitch colour must survive the pipeline.
func TestUsernameColorFlow_TwitchSetColorIsHonored(t *testing.T) {
	notAViewer := &fakeCosmeticsRow{err: pgx.ErrNoRows}
	user := runColorPipeline(t, rawTwitchMsg("133335206", "djsubwoofer_kaz", "#DAA520"), notAViewer)

	assert.Equal(t, "#DAA520", user.Color, "the Twitch-set colour must reach the overlay verbatim")
	assert.NotEmpty(t, user.AutoColor, "auto colour is still published as a fallback")
	assert.NotEqual(t, user.Color, user.AutoColor,
		"auto colour must be a separate field, never overwrite the authoritative one")
}

// A chatter who never picked a Twitch colour leaves Color empty, so the streamer's
// "Username color" setting gets its chance before the auto palette.
func TestUsernameColorFlow_NoTwitchColorLeavesRoomForStreamerSetting(t *testing.T) {
	notAViewer := &fakeCosmeticsRow{err: pgx.ErrNoRows}
	user := runColorPipeline(t, rawTwitchMsg("1507827809", "purinyumyum", ""), notAViewer)

	assert.Empty(t, user.Color,
		"no platform colour must leave Color empty — a filled Color silently disables the streamer setting")
	assert.Equal(t, enricher.AutoColor("twitch:1507827809"), user.AutoColor,
		"auto colour is deterministic and keyed by platform:userID for unregistered chatters")
}

// A registered viewer's manually chosen All-Chat colour outranks their Twitch colour.
func TestUsernameColorFlow_AllChatColorBeatsTwitchColor(t *testing.T) {
	chosen := "#ff6600"
	viewer := &fakeCosmeticsRow{viewerID: "3f1b2c4d-0000-4000-8000-00000000abcd", nameColor: &chosen}
	user := runColorPipeline(t, rawTwitchMsg("133335206", "djsubwoofer_kaz", "#DAA520"), viewer)

	assert.Equal(t, chosen, user.Color,
		"a viewer who manually picked an All-Chat colour overrides the platform colour")
}

// A registered viewer who has NOT picked a colour keeps their Twitch colour, and
// their auto-colour is keyed by viewer UUID so it matches on their other platforms.
func TestUsernameColorFlow_RegisteredViewerWithoutChoiceKeepsTwitchColor(t *testing.T) {
	const viewerUUID = "3f1b2c4d-0000-4000-8000-00000000abcd"
	viewer := &fakeCosmeticsRow{viewerID: viewerUUID, nameColor: nil}
	user := runColorPipeline(t, rawTwitchMsg("133335206", "djsubwoofer_kaz", "#DAA520"), viewer)

	assert.Equal(t, "#DAA520", user.Color,
		"having an All-Chat account must not by itself replace the Twitch colour")
	assert.Equal(t, enricher.AutoColor(viewerUUID), user.AutoColor,
		"registered viewers key the auto colour off the viewer UUID for cross-platform consistency")
}
