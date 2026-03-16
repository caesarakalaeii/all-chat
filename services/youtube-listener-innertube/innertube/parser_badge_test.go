package innertube

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestExtractBadgesRich_MemberBadgeURL tests that a badge with CustomThumbnail containing 2 thumbnails
// returns the index 1 URL as badge_member_url and tooltip as badge_member_tooltip.
// NOTE: extractBadgesRich does not yet exist — this test will fail to compile until Plan 02.
func TestExtractBadgesRich_MemberBadgeURL(t *testing.T) {
	badges := []AuthorBadge{
		{
			LiveChatAuthorBadgeRenderer: LiveChatAuthorBadgeRenderer{
				CustomThumbnail: Thumbnails{
					Thumbnails: []Thumbnail{
						{URL: "https://yt.example.com/badge_32.png", Width: 32, Height: 32},
						{URL: "https://yt.example.com/badge_48.png", Width: 48, Height: 48},
					},
				},
				Tooltip: "3-Month Member",
			},
		},
	}

	memberURL, memberTooltip, badgeTooltips := extractBadgesRich(badges)

	assert.Equal(t, "https://yt.example.com/badge_48.png", memberURL, "should return index 1 URL")
	assert.Equal(t, "3-Month Member", memberTooltip, "should return tooltip")
	assert.Contains(t, badgeTooltips, "3-Month Member", "tooltip should be in badge slice")
}

// TestExtractBadgesRich_MemberBadgeSingleThumbnail tests that a badge with only 1 thumbnail
// returns index 0 URL without panicking.
func TestExtractBadgesRich_MemberBadgeSingleThumbnail(t *testing.T) {
	badges := []AuthorBadge{
		{
			LiveChatAuthorBadgeRenderer: LiveChatAuthorBadgeRenderer{
				CustomThumbnail: Thumbnails{
					Thumbnails: []Thumbnail{
						{URL: "https://yt.example.com/badge_32.png", Width: 32, Height: 32},
					},
				},
				Tooltip: "Member",
			},
		},
	}

	memberURL, memberTooltip, badgeTooltips := extractBadgesRich(badges)

	assert.Equal(t, "https://yt.example.com/badge_32.png", memberURL, "should return index 0 URL (no panic)")
	assert.Equal(t, "Member", memberTooltip)
	assert.Contains(t, badgeTooltips, "Member")
}

// TestExtractBadgesRich_SystemBadge tests that a badge with Icon only (no CustomThumbnail)
// results in empty badge_member_url but tooltip still returned in badgeTooltips.
func TestExtractBadgesRich_SystemBadge(t *testing.T) {
	badges := []AuthorBadge{
		{
			LiveChatAuthorBadgeRenderer: LiveChatAuthorBadgeRenderer{
				Icon:    &IconData{IconType: "MODERATOR"},
				Tooltip: "Moderator",
			},
		},
	}

	memberURL, memberTooltip, badgeTooltips := extractBadgesRich(badges)

	assert.Empty(t, memberURL, "icon-only badge should have empty member URL")
	assert.Empty(t, memberTooltip, "icon-only badge should have empty member tooltip")
	assert.Contains(t, badgeTooltips, "Moderator", "tooltip should still be in badge slice")
}

// TestExtractBadgesRich_NoBadges tests that an empty badge slice returns zero/nil values.
func TestExtractBadgesRich_NoBadges(t *testing.T) {
	memberURL, memberTooltip, badgeTooltips := extractBadgesRich([]AuthorBadge{})

	assert.Empty(t, memberURL)
	assert.Empty(t, memberTooltip)
	assert.Empty(t, badgeTooltips)
}

// TestParseTextMessage_MemberBadgeTagsSet tests that ParseMessages with a text message that has a
// membership badge results in msg.Tags["badge_member_url"] non-empty and
// msg.Tags["badge_member_tooltip"] set to the tooltip string.
func TestParseTextMessage_MemberBadgeTagsSet(t *testing.T) {
	channelID := "UC_test_channel"

	actions := []ChatAction{
		{
			AddChatItemAction: &AddChatItemAction{
				Item: ChatItem{
					LiveChatTextMessageRenderer: &LiveChatTextMessageRenderer{
						Message: MessageContent{
							Runs: []MessageRun{
								{Text: "Hello from a member!"},
							},
						},
						AuthorName:              SimpleText{SimpleText: "TestMember"},
						AuthorExternalChannelID: "UC123",
						TimestampUsec:           "1640000000000000",
						AuthorBadges: []AuthorBadge{
							{
								LiveChatAuthorBadgeRenderer: LiveChatAuthorBadgeRenderer{
									CustomThumbnail: Thumbnails{
										Thumbnails: []Thumbnail{
											{URL: "https://yt.example.com/badge_32.png", Width: 32, Height: 32},
											{URL: "https://yt.example.com/badge_48.png", Width: 48, Height: 48},
										},
									},
									Tooltip: "3-Month Member",
								},
							},
						},
					},
				},
			},
		},
	}

	messages, err := ParseMessages(actions, channelID)
	assert.NoError(t, err)
	assert.Len(t, messages, 1)

	msg := messages[0]
	assert.Equal(t, "https://yt.example.com/badge_48.png", msg.Tags["badge_member_url"], "badge_member_url should be index 1 thumbnail URL")
	assert.Equal(t, "3-Month Member", msg.Tags["badge_member_tooltip"], "badge_member_tooltip should be set")
	assert.Contains(t, msg.Tags["badges"], "3-Month Member", "badges tag should still contain tooltip (backward compat)")
}
