package normalizer

import (
	"strings"
	"testing"
	"time"

	"github.com/caesar/all-chat/services/message-processor/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// makeRawMsg creates a minimal valid RawChatMessage for YouTube with the given tags.
func makeRawMsg(tags map[string]string) *models.RawChatMessage {
	return &models.RawChatMessage{
		MessageID: "test-msg-id",
		Platform:  "youtube",
		ChannelID: "UCxxxxxx",
		UserID:    "UCyyyyyy",
		Username:  "TestUser",
		Text:      "test message",
		Timestamp: time.Now(),
		Tags:      tags,
	}
}

// findBadge finds the first badge with the given name in a slice of badges, or nil.
func findBadge(badges []models.Badge, name string) *models.Badge {
	for i := range badges {
		if badges[i].Name == name {
			return &badges[i]
		}
	}
	return nil
}

// TestYouTubeNormalizer_ExtractBadges_RealMemberURL tests that when badge_member_url is set,
// the normalized badge uses the real image URL instead of the SVG fallback.
// This test will FAIL until Plan 03 modifies extractBadges to check badge_member_url.
func TestYouTubeNormalizer_ExtractBadges_RealMemberURL(t *testing.T) {
	n := NewYouTubeNormalizer()
	raw := makeRawMsg(map[string]string{
		"badge_member_url":     "https://yt.img/member.png",
		"badge_member_tooltip": "6-Month Member",
		"is_sponsor":           "true",
	})

	unified, err := n.Normalize(raw, "overlay-1")
	require.NoError(t, err)

	badge := findBadge(unified.User.Badges, "member")
	require.NotNil(t, badge, "member badge should be present")
	assert.Equal(t, "https://yt.img/member.png", badge.IconURL, "should use real image URL, not SVG fallback")
	assert.Equal(t, "6-Month Member", badge.Version, "version should be the tooltip text")
}

// TestYouTubeNormalizer_ExtractBadges_SVGFallback tests that when badge_member_url is empty
// but is_sponsor is true, the badge uses the SVG fallback.
// This test should PASS immediately (existing behavior).
func TestYouTubeNormalizer_ExtractBadges_SVGFallback(t *testing.T) {
	n := NewYouTubeNormalizer()
	raw := makeRawMsg(map[string]string{
		"badge_member_url": "",
		"is_sponsor":       "true",
	})

	unified, err := n.Normalize(raw, "overlay-1")
	require.NoError(t, err)

	badge := findBadge(unified.User.Badges, "member")
	require.NotNil(t, badge, "member badge should be present when is_sponsor=true")
	assert.True(t, strings.HasPrefix(badge.IconURL, "data:image/svg+xml"), "should use SVG fallback when no real URL")
}

// TestYouTubeNormalizer_ExtractBadges_BackwardCompat tests that when badge_member_url is absent
// (old listener format) but is_sponsor is true, the member badge is still produced.
// This test should PASS immediately (existing behavior preserved).
func TestYouTubeNormalizer_ExtractBadges_BackwardCompat(t *testing.T) {
	n := NewYouTubeNormalizer()
	// Deliberately omit badge_member_url tag (old listener format)
	raw := makeRawMsg(map[string]string{
		"is_sponsor": "true",
	})

	unified, err := n.Normalize(raw, "overlay-1")
	require.NoError(t, err)

	badge := findBadge(unified.User.Badges, "member")
	require.NotNil(t, badge, "member badge should still be produced for old listener format")
	assert.True(t, strings.HasPrefix(badge.IconURL, "data:image/svg+xml"), "SVG fallback should be used")
}

// TestYouTubeNormalizer_ExtractBadges_OwnerBadge tests that is_owner=true produces
// an owner badge with SVG IconURL (regression test — YTBADGE-03).
// This test should PASS immediately (existing behavior).
func TestYouTubeNormalizer_ExtractBadges_OwnerBadge(t *testing.T) {
	n := NewYouTubeNormalizer()
	raw := makeRawMsg(map[string]string{
		"is_owner": "true",
	})

	unified, err := n.Normalize(raw, "overlay-1")
	require.NoError(t, err)

	badge := findBadge(unified.User.Badges, "owner")
	require.NotNil(t, badge, "owner badge should be present")
	assert.True(t, strings.HasPrefix(badge.IconURL, "data:image/svg+xml"), "owner badge should have SVG icon")
}

// TestYouTubeNormalizer_ExtractBadges_ModeratorBadge tests that is_moderator=true produces
// a moderator badge (regression test — YTBADGE-03).
// This test should PASS immediately (existing behavior).
func TestYouTubeNormalizer_ExtractBadges_ModeratorBadge(t *testing.T) {
	n := NewYouTubeNormalizer()
	raw := makeRawMsg(map[string]string{
		"is_moderator": "true",
	})

	unified, err := n.Normalize(raw, "overlay-1")
	require.NoError(t, err)

	badge := findBadge(unified.User.Badges, "moderator")
	require.NotNil(t, badge, "moderator badge should be present")
	assert.True(t, strings.HasPrefix(badge.IconURL, "data:image/svg+xml"), "moderator badge should have SVG icon")
}

// TestYouTubeNormalizer_ExtractBadges_MemberURLWithoutIsSponsor tests that badge_member_url
// alone (without is_sponsor) still produces a member badge.
// This test will FAIL until Plan 03 adds badge_member_url trigger logic.
func TestYouTubeNormalizer_ExtractBadges_MemberURLWithoutIsSponsor(t *testing.T) {
	n := NewYouTubeNormalizer()
	raw := makeRawMsg(map[string]string{
		"badge_member_url": "https://real.png",
		// Deliberately absent: "is_sponsor"
	})

	unified, err := n.Normalize(raw, "overlay-1")
	require.NoError(t, err)

	badge := findBadge(unified.User.Badges, "member")
	require.NotNil(t, badge, "member badge should be triggered by badge_member_url even without is_sponsor")
	assert.Equal(t, "https://real.png", badge.IconURL, "should use badge_member_url")
}
