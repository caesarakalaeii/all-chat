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

package classifier

import (
	"testing"

	"github.com/caesar/all-chat/services/message-processor/models"
	"github.com/stretchr/testify/assert"
)

func TestClassifyTwitchSubscription(t *testing.T) {
	tier, duration := ClassifyEvent("twitch", "subscription", nil)
	assert.Equal(t, "high", tier)
	assert.Equal(t, 30, duration)
}

func TestClassifyTwitchGiftSubscription(t *testing.T) {
	tier, duration := ClassifyEvent("twitch", "gift_subscription", nil)
	assert.Equal(t, "high", tier)
	assert.Equal(t, 30, duration)
}

func TestClassifyTwitchMysteryGift(t *testing.T) {
	tier, duration := ClassifyEvent("twitch", "mystery_gift", nil)
	assert.Equal(t, "high", tier)
	assert.Equal(t, 45, duration)
}

func TestClassifyTwitchRaid_Large(t *testing.T) {
	value := &models.EventValue{
		Amount:   5000,
		Currency: "viewers",
	}
	tier, duration := ClassifyEvent("twitch", "raid", value)
	assert.Equal(t, "high", tier)
	assert.Equal(t, 40, duration)
}

func TestClassifyTwitchRaid_Medium(t *testing.T) {
	value := &models.EventValue{
		Amount:   500,
		Currency: "viewers",
	}
	tier, duration := ClassifyEvent("twitch", "raid", value)
	assert.Equal(t, "high", tier)
	assert.Equal(t, 30, duration)
}

func TestClassifyTwitchRaid_Small(t *testing.T) {
	value := &models.EventValue{
		Amount:   50,
		Currency: "viewers",
	}
	tier, duration := ClassifyEvent("twitch", "raid", value)
	assert.Equal(t, "medium", tier)
	assert.Equal(t, 20, duration)
}

func TestClassifyTwitchBits_Large(t *testing.T) {
	value := &models.EventValue{
		Amount:   5000,
		Currency: "bits",
	}
	tier, duration := ClassifyEvent("twitch", "bits", value)
	assert.Equal(t, "high", tier)
	assert.Equal(t, 35, duration)
}

func TestClassifyTwitchBits_Medium(t *testing.T) {
	value := &models.EventValue{
		Amount:   500,
		Currency: "bits",
	}
	tier, duration := ClassifyEvent("twitch", "bits", value)
	assert.Equal(t, "medium", tier)
	assert.Equal(t, 20, duration)
}

func TestClassifyTwitchBits_Small(t *testing.T) {
	value := &models.EventValue{
		Amount:   50,
		Currency: "bits",
	}
	tier, duration := ClassifyEvent("twitch", "bits", value)
	assert.Equal(t, "low", tier)
	assert.Equal(t, 10, duration)
}

func TestClassifyTwitchChannelPoints(t *testing.T) {
	tier, duration := ClassifyEvent("twitch", "channel_points", nil)
	assert.Equal(t, "medium", tier)
	assert.Equal(t, 15, duration)
}

func TestClassifyYouTubeSuperChat_VeryLarge(t *testing.T) {
	value := &models.EventValue{
		Amount:   100000000, // $100 in micros
		Currency: "USD",
	}
	tier, duration := ClassifyEvent("youtube", "super_chat", value)
	assert.Equal(t, "high", tier)
	assert.Equal(t, 60, duration)
}

func TestClassifyYouTubeSuperChat_Large(t *testing.T) {
	value := &models.EventValue{
		Amount:   25000000, // $25 in micros
		Currency: "USD",
	}
	tier, duration := ClassifyEvent("youtube", "super_chat", value)
	assert.Equal(t, "high", tier)
	assert.Equal(t, 45, duration)
}

func TestClassifyYouTubeSuperChat_Medium(t *testing.T) {
	value := &models.EventValue{
		Amount:   7000000, // $7 in micros
		Currency: "USD",
	}
	tier, duration := ClassifyEvent("youtube", "super_chat", value)
	assert.Equal(t, "high", tier)
	assert.Equal(t, 30, duration)
}

func TestClassifyYouTubeSuperChat_Small(t *testing.T) {
	value := &models.EventValue{
		Amount:   3000000, // $3 in micros
		Currency: "USD",
	}
	tier, duration := ClassifyEvent("youtube", "super_chat", value)
	assert.Equal(t, "medium", tier)
	assert.Equal(t, 20, duration)
}

func TestClassifyYouTubeSuperChat_VerySmall(t *testing.T) {
	value := &models.EventValue{
		Amount:   1000000, // $1 in micros
		Currency: "USD",
	}
	tier, duration := ClassifyEvent("youtube", "super_chat", value)
	assert.Equal(t, "low", tier)
	assert.Equal(t, 10, duration)
}

func TestClassifyYouTubeNewSponsor(t *testing.T) {
	tier, duration := ClassifyEvent("youtube", "new_sponsor", nil)
	assert.Equal(t, "high", tier)
	assert.Equal(t, 30, duration)
}

func TestClassifyYouTubeMemberMilestone_LongTime(t *testing.T) {
	value := &models.EventValue{
		Amount:   36, // 36 months = 3 years
		Currency: "months",
	}
	tier, duration := ClassifyEvent("youtube", "member_milestone", value)
	assert.Equal(t, "high", tier)
	assert.Equal(t, 35, duration)
}

func TestClassifyYouTubeMemberMilestone_OneYear(t *testing.T) {
	value := &models.EventValue{
		Amount:   12, // 1 year
		Currency: "months",
	}
	tier, duration := ClassifyEvent("youtube", "member_milestone", value)
	assert.Equal(t, "high", tier)
	assert.Equal(t, 30, duration)
}

func TestClassifyYouTubeMembershipGift_Many(t *testing.T) {
	value := &models.EventValue{
		Amount:   15,
		Currency: "gifts",
	}
	tier, duration := ClassifyEvent("youtube", "membership_gift", value)
	assert.Equal(t, "high", tier)
	assert.Equal(t, 40, duration)
}

func TestClassifyTikTokGift_Large(t *testing.T) {
	value := &models.EventValue{
		Amount:   5000,
		Currency: "diamonds",
	}
	tier, duration := ClassifyEvent("tiktok", "gift", value)
	assert.Equal(t, "high", tier)
	assert.Equal(t, 35, duration)
}

func TestClassifyTikTokGift_Medium(t *testing.T) {
	value := &models.EventValue{
		Amount:   500,
		Currency: "diamonds",
	}
	tier, duration := ClassifyEvent("tiktok", "gift", value)
	assert.Equal(t, "medium", tier)
	assert.Equal(t, 20, duration)
}

func TestClassifyTikTokGift_Small(t *testing.T) {
	value := &models.EventValue{
		Amount:   50,
		Currency: "diamonds",
	}
	tier, duration := ClassifyEvent("tiktok", "gift", value)
	assert.Equal(t, "low", tier)
	assert.Equal(t, 10, duration)
}

func TestClassifyTikTokFollow(t *testing.T) {
	tier, duration := ClassifyEvent("tiktok", "follow", nil)
	assert.Equal(t, "medium", tier)
	assert.Equal(t, 15, duration)
}

func TestClassifyTikTokLikeAggregate_Many(t *testing.T) {
	value := &models.EventValue{
		Amount:   150,
		Currency: "likes",
	}
	tier, duration := ClassifyEvent("tiktok", "like_aggregate", value)
	assert.Equal(t, "medium", tier)
	assert.Equal(t, 12, duration)
}

func TestClassifyTikTokLikeAggregate_Few(t *testing.T) {
	value := &models.EventValue{
		Amount:   25,
		Currency: "likes",
	}
	tier, duration := ClassifyEvent("tiktok", "like_aggregate", value)
	assert.Equal(t, "low", tier)
	assert.Equal(t, 8, duration)
}

func TestClassifyUnknownPlatform(t *testing.T) {
	tier, duration := ClassifyEvent("unknown", "event", nil)
	assert.Equal(t, "medium", tier)
	assert.Equal(t, 15, duration)
}

func TestClassifyUnknownEventType(t *testing.T) {
	tier, duration := ClassifyEvent("twitch", "unknown_event", nil)
	assert.Equal(t, "medium", tier)
	assert.Equal(t, 15, duration)
}

func TestClassifyListenerDeprecationNotice(t *testing.T) {
	tier, duration := ClassifyEvent("system", "listener_deprecation_notice", nil)
	assert.Equal(t, "high", tier)
	assert.Equal(t, 60, duration)
}
