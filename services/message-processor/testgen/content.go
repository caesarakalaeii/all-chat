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

package testgen

import (
	"fmt"
	"math/rand"

	"github.com/caesar/all-chat/services/message-processor/models"
)

// chatPlatforms is the set of platforms used for plain chat messages. Events
// are platform-specific and pick their own platform from the event template.
var chatPlatforms = []string{"twitch", "youtube", "kick", "tiktok", "discord"}

const (
	// testChannelID/testChannelName are the synthetic channel identity carried by
	// every generated message. They don't correspond to any real platform channel,
	// so external tools can recognize the test stream.
	testChannelID   = "test-channel"
	testChannelName = "Test Channel"

	// emoteFallbackChannel is a real channel whose emotes stand in for the
	// synthetic test channel during enrichment. The test channel doesn't exist on
	// any emote provider (7TV/BTTV/FFZ/Twitch), so resolving against it returns
	// nothing; caesarlp gives the stream realistic, resolvable emotes.
	emoteFallbackChannel = "caesarlp"
)

var fakeUsers = []string{
	"pixelpenguin", "noscope_nancy", "lurkmaster3000", "captain_clutch", "soggy_waffles",
	"emote_enjoyer", "ratio_andy", "midnight_owl", "gigachad_gary", "copium_addict",
	"vibe_checker", "404_username", "the_real_mvp", "spam_o_matic", "quiet_observer",
}

// voteColors keeps poll voters visually varied so vote-counters can be tested
// against differently-styled messages.
var nameColors = []string{
	"#FF4500", "#1E90FF", "#9ACD32", "#FF69B4", "#FFD700",
	"#00FFFF", "#FF6347", "#7FFF00", "#BA55D3", "#00FA9A",
}

// chatPhrases include common emote codes so the emote enricher has something to
// resolve, producing realistic enriched payloads.
var chatPhrases = []string{
	"that was insane PogChamp",
	"LULW he really did that",
	"first time? KEKW",
	"chat is this real",
	"W stream",
	"monkaS that was close",
	"gg go next",
	"OMEGALUL",
	"pog moment right there",
	"+1 to that",
	"hold the line",
	"sheeeesh",
	"clip it and ship it",
	"no shot bro",
	"absolute cinema",
}

// voteOptions are the poll-vote numbers interspersed into the stream.
var voteOptions = []string{"1", "2", "3", "4"}

func pick[T any](rng *rand.Rand, items []T) T {
	return items[rng.Intn(len(items))]
}

func (g *Generator) baseUser(rng *rand.Rand) models.UserInfo {
	name := pick(rng, fakeUsers)
	badges := []models.Badge{}
	if rng.Float64() < 0.3 {
		badges = append(badges, models.Badge{Name: "subscriber", Version: fmt.Sprintf("%d", 1+rng.Intn(36))})
	}
	if rng.Float64() < 0.1 {
		badges = append(badges, models.Badge{Name: "moderator", Version: "1"})
	}
	if rng.Float64() < 0.05 {
		badges = append(badges, models.Badge{Name: "vip", Version: "1"})
	}
	return models.UserInfo{
		ID:          fmt.Sprintf("test-%d", rng.Intn(1_000_000)),
		Username:    name,
		DisplayName: name,
		Badges:      badges,
		Color:       pick(rng, nameColors),
	}
}

func (g *Generator) newMessage(rng *rand.Rand, platform, text string) *models.UnifiedChatMessage {
	return &models.UnifiedChatMessage{
		ID:          fmt.Sprintf("testgen-%d", rng.Int63()),
		OverlayID:   g.overlayID,
		Platform:    platform,
		ChannelID:   testChannelID,
		ChannelName: testChannelName,
		User:        g.baseUser(rng),
		Message:     models.MessageInfo{Text: text},
		Metadata:    map[string]interface{}{"test_stream": true},
	}
}

// buildChat produces either a poll-vote message ("1".."4") or random chatter.
func (g *Generator) buildChat(rng *rand.Rand, voteRatio float64) *models.UnifiedChatMessage {
	platform := pick(rng, chatPlatforms)
	if rng.Float64() < voteRatio {
		msg := g.newMessage(rng, platform, pick(rng, voteOptions))
		msg.Metadata["vote"] = true
		return msg
	}
	return g.newMessage(rng, platform, pick(rng, chatPhrases))
}

// eventTemplate describes how to build one fake platform event.
type eventTemplate struct {
	platform string
	text     string
	event    *models.EventInfo
}

// buildEvent emits a representative platform event with a valid EventInfo so
// downstream consumers can exercise their event-handling paths.
func (g *Generator) buildEvent(rng *rand.Rand) *models.UnifiedChatMessage {
	templates := eventTemplates(rng)
	tmpl := templates[rng.Intn(len(templates))]
	msg := g.newMessage(rng, tmpl.platform, tmpl.text)
	msg.Event = tmpl.event
	msg.Metadata["event"] = true
	return msg
}

// eventTemplates returns the catalogue of fake events. Built per-call so random
// amounts vary between events.
func eventTemplates(rng *rand.Rand) []eventTemplate {
	months := 1 + rng.Intn(48)
	bits := (1 + rng.Intn(50)) * 100
	raiders := 5 + rng.Intn(500)
	gifts := 1 + rng.Intn(20)
	usd := float64(1+rng.Intn(99)) + 0.99

	return []eventTemplate{
		{
			platform: "twitch",
			text:     "just subscribed!",
			event: &models.EventInfo{
				Type:     "subscription",
				Tier:     "medium",
				Duration: 8,
				Value:    &models.EventValue{Amount: 1, Currency: "months", DisplayText: "Tier 1 sub"},
			},
		},
		{
			platform: "twitch",
			text:     fmt.Sprintf("resubscribed for %d months!", months),
			event: &models.EventInfo{
				Type:     "resubscription",
				Tier:     "medium",
				Duration: 8,
				Value:    &models.EventValue{Amount: float64(months), Currency: "months", DisplayText: fmt.Sprintf("%d months", months)},
			},
		},
		{
			platform: "twitch",
			text:     fmt.Sprintf("gifted %d subs!", gifts),
			event: &models.EventInfo{
				Type:     "mystery_gift",
				Tier:     "high",
				Duration: 10,
				Value:    &models.EventValue{Amount: float64(gifts), Currency: "gifts", DisplayText: fmt.Sprintf("%d gifted subs", gifts)},
			},
		},
		{
			platform: "twitch",
			text:     fmt.Sprintf("Cheer%d great stream!", bits),
			event: &models.EventInfo{
				Type:     "bits",
				Tier:     "medium",
				Duration: 8,
				Value:    &models.EventValue{Amount: float64(bits), Currency: "bits", DisplayText: fmt.Sprintf("%d bits", bits)},
			},
		},
		{
			platform: "twitch",
			text:     fmt.Sprintf("is raiding with %d viewers!", raiders),
			event: &models.EventInfo{
				Type:     "raid",
				Tier:     "high",
				Duration: 10,
				Value:    &models.EventValue{Amount: float64(raiders), Currency: "viewers", DisplayText: fmt.Sprintf("%d raiders", raiders)},
			},
		},
		{
			platform: "twitch",
			text:     "redeemed Highlight My Message",
			event: &models.EventInfo{
				Type:     "channel_points",
				Tier:     "low",
				Duration: 6,
				Value:    &models.EventValue{Amount: 1000, Currency: "points", DisplayText: "1000 points"},
				Metadata: map[string]interface{}{"reward_title": "Highlight My Message"},
			},
		},
		{
			platform: "youtube",
			text:     "Thanks for the stream!",
			event: &models.EventInfo{
				Type:     "super_chat",
				Tier:     "high",
				Duration: 10,
				Value:    &models.EventValue{Amount: usd, Currency: "USD", DisplayText: fmt.Sprintf("$%.2f", usd)},
			},
		},
		{
			platform: "youtube",
			text:     "became a member!",
			event: &models.EventInfo{
				Type:     "new_sponsor",
				Tier:     "medium",
				Duration: 8,
				Value:    &models.EventValue{Amount: 1, Currency: "months", DisplayText: "New member"},
			},
		},
		{
			platform: "tiktok",
			text:     "sent a Rose!",
			event: &models.EventInfo{
				Type:     "gift",
				Tier:     "low",
				Duration: 6,
				Value:    &models.EventValue{Amount: float64(1 + rng.Intn(100)), Currency: "coins", DisplayText: "Rose"},
			},
		},
		{
			platform: "tiktok",
			text:     "followed the stream!",
			event: &models.EventInfo{
				Type:     "follow",
				Tier:     "low",
				Duration: 5,
			},
		},
	}
}
