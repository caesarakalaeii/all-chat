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

package normalizer

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/caesar/all-chat/services/message-processor/classifier"
	"github.com/caesar/all-chat/services/message-processor/models"
)

// maxTwitchGifs bounds chat GIFs surfaced per message (defence in depth; a single
// message realistically carries one). Matches the Discord attachment cap.
const maxTwitchGifs = 4

// TwitchNormalizer normalizes Twitch raw messages to unified format
type TwitchNormalizer struct{}

// NewTwitchNormalizer creates a new Twitch normalizer
func NewTwitchNormalizer() *TwitchNormalizer {
	return &TwitchNormalizer{}
}

// Normalize converts a RawChatMessage to UnifiedChatMessage
func (n *TwitchNormalizer) Normalize(raw *models.RawChatMessage, overlayID string) (*models.UnifiedChatMessage, error) {
	if raw.Platform != "twitch" {
		return nil, fmt.Errorf("unsupported platform: %s", raw.Platform)
	}

	if err := validateChannelID(raw.ChannelID); err != nil {
		return nil, fmt.Errorf("invalid channel ID: %w", err)
	}

	// Extract user info from tags
	userInfo := n.extractUserInfo(raw)

	// Extract Twitch native emotes from tags (positions/codes against the original text)
	emotes := n.extractTwitchEmotes(raw)

	// Extract chat GIFs from the "gifs" tag (ADR-0037). Twitch replaces the bracketed alt
	// caption with the GIF, so strip that span from the visible text and re-anchor any
	// first-party emote offsets to the stripped text. When nothing is stripped, extractTwitchGifs
	// returns raw.Text unchanged, so `text` is always the correct body.
	attachments, text, removed := extractTwitchGifs(raw.Text, raw.Tags["gifs"])
	if len(removed) > 0 {
		emotes = remapEmotePositions(emotes, removed)
	}

	// Create unified message
	unified := &models.UnifiedChatMessage{
		ID:          raw.MessageID,
		OverlayID:   overlayID,
		Platform:    "twitch",
		ChannelID:   raw.ChannelID,
		ChannelName: raw.ChannelID, // Twitch uses channel name as ID
		User:        userInfo,
		Message: models.MessageInfo{
			Text:        text,
			Emotes:      emotes,
			Attachments: attachments,
		},
		Timestamp: raw.Timestamp,
		Metadata:  n.extractMetadata(raw),
	}

	return unified, nil
}

// extractUserInfo extracts user information from tags
func (n *TwitchNormalizer) extractUserInfo(raw *models.RawChatMessage) models.UserInfo {
	tags := raw.Tags

	// Extract badges with URLs
	badges := make([]models.Badge, 0)
	if badgesStr, ok := tags["badges"]; ok && badgesStr != "" {
		// Format: "subscriber/12,moderator/1"
		badgePairs := strings.Split(badgesStr, ",")
		for _, pair := range badgePairs {
			parts := strings.Split(pair, "/")
			if len(parts) >= 2 {
				name := parts[0]
				version := parts[1]

				// Don't set placeholder URLs - let the badge enricher populate them
				// The old CDN format https://static-cdn.jtvnw.net/badges/v1/{name}/{version}/1 returns 404
				badges = append(badges, models.Badge{
					Name:    name,
					Version: version,
					IconURL: "", // Will be enriched by badge enricher
				})
			}
		}
	}

	// Extract source badges for shared chat (if present)
	sourceBadges := make([]models.Badge, 0)
	if sourceBadgesStr, ok := tags["source-badges"]; ok && sourceBadgesStr != "" {
		// Format: "subscriber/12,moderator/1"
		badgePairs := strings.Split(sourceBadgesStr, ",")
		for _, pair := range badgePairs {
			parts := strings.Split(pair, "/")
			if len(parts) >= 2 {
				name := parts[0]
				version := parts[1]

				sourceBadges = append(sourceBadges, models.Badge{
					Name:    name,
					Version: version,
					IconURL: "", // Will be enriched by badge enricher using source channel
				})
			}
		}
	}

	// Get display name (fallback to username)
	displayName := tags["display-name"]
	if displayName == "" {
		displayName = raw.Username
	}

	// Extract source user ID for shared chat
	sourceUserID := tags["source-id"]

	return models.UserInfo{
		ID:           raw.UserID,
		Username:     raw.Username,
		DisplayName:  displayName,
		AvatarURL:    "", // Will be enriched by avatar enricher
		Badges:       badges,
		Color:        tags["color"],
		SourceBadges: sourceBadges,
		SourceUserID: sourceUserID,
	}
}

// extractTwitchEmotes extracts native Twitch emotes from IRC tags
func (n *TwitchNormalizer) extractTwitchEmotes(raw *models.RawChatMessage) []models.Emote {
	emotesStr, ok := raw.Tags["emotes"]
	if !ok || emotesStr == "" {
		return []models.Emote{}
	}

	// Parse emotes tag format: "25:0-4,12-16/1902:6-10"
	// Format: emoteID:start-end,start-end/emoteID:start-end
	emotes := make([]models.Emote, 0)

	emoteParts := strings.Split(emotesStr, "/")
	for _, part := range emoteParts {
		// Split emote ID from positions
		idAndPos := strings.Split(part, ":")
		if len(idAndPos) != 2 {
			continue
		}

		emoteID := idAndPos[0]
		positionsStr := idAndPos[1]

		// Parse positions
		positions := make([][]int, 0)
		posPairs := strings.Split(positionsStr, ",")
		for _, posPair := range posPairs {
			startEnd := strings.Split(posPair, "-")
			if len(startEnd) != 2 {
				continue
			}

			start, err1 := strconv.Atoi(startEnd[0])
			end, err2 := strconv.Atoi(startEnd[1])
			if err1 == nil && err2 == nil {
				positions = append(positions, []int{start, end})
			}
		}

		if len(positions) > 0 {
			// Extract emote code from message text using first position
			// IRC positions are inclusive, so end position is already the last character
			code := ""
			if len(positions) > 0 && positions[0][0] < len(raw.Text) && positions[0][1]+1 <= len(raw.Text) {
				code = raw.Text[positions[0][0] : positions[0][1]+1]
			}

			emotes = append(emotes, models.Emote{
				Code:      code,
				Provider:  "twitch",
				URL:       fmt.Sprintf("https://static-cdn.jtvnw.net/emoticons/v2/%s/default/dark/1.0", emoteID),
				Positions: positions,
			})
		}
	}

	return emotes
}

// extractTwitchGifs parses the Twitch "gifs" tag into renderable media attachments and
// returns the message text with each GIF's alt caption removed (ADR-0037). The tag format
// is a comma-separated list of "start-end|gif_id|url", where start-end are inclusive,
// zero-based byte offsets into the original text marking the "[alt caption]" the GIF stands
// in for. Twitch hides that caption and shows the GIF; we mirror that by lifting the caption
// into each attachment's Filename (used as the render alt / hidden-state label) and stripping
// it from the visible text. removed lists the stripped spans so first-party emote offsets can
// be re-anchored (see remapEmotePositions). A GIF whose URL is present but whose span is
// malformed still yields an attachment (so the image renders); it just isn't stripped.
func extractTwitchGifs(text, gifsTag string) (attachments []models.Attachment, strippedText string, removed [][]int) {
	if gifsTag == "" {
		return nil, text, nil
	}

	type span struct{ start, end int }
	attachments = make([]models.Attachment, 0)
	valid := make([]span, 0) // strippable spans, in tag order

	for _, e := range strings.Split(gifsTag, ",") {
		// format: start-end|gif_id|url — SplitN(…, 3) keeps any "|" inside the URL intact.
		// The URL is the last, unescaped field: this mirrors Twitch's own IRC gifs-tag format,
		// which likewise comma-separates entries and puts the URL last, so Twitch guarantees the
		// URL carries no literal comma (query separators are "&"; commas would be percent-encoded).
		fields := strings.SplitN(e, "|", 3)
		if len(fields) != 3 {
			continue
		}
		url := fields[2]
		// The URL comes from Twitch's GIF picker over an authenticated channel (HMAC-verified
		// EventSub webhook or TLS IRC), but require https defensively so a malformed/garbage entry
		// never becomes a live <img> on the broadcast overlay.
		if !strings.HasPrefix(url, "https://") {
			continue
		}

		// Locate the alt caption the GIF stands in for. A malformed/out-of-range span still yields
		// an attachment (the image renders; the frontend falls back to a generic alt) — it just
		// can't be stripped from the text.
		alt := ""
		if startEnd := strings.SplitN(fields[0], "-", 2); len(startEnd) == 2 {
			start, err1 := strconv.Atoi(startEnd[0])
			end, err2 := strconv.Atoi(startEnd[1])
			if err1 == nil && err2 == nil && start >= 0 && end >= start && end < len(text) {
				valid = append(valid, span{start, end})
				alt = trimOneBracketPair(text[start : end+1])
			}
		}

		attachments = append(attachments, models.Attachment{
			Type:        "image",
			URL:         url,
			ContentType: "image/gif",
			Filename:    alt,
		})
		if len(attachments) >= maxTwitchGifs {
			break
		}
	}

	if len(attachments) == 0 {
		return nil, text, nil
	}

	// Reduce the strippable spans to a sorted, disjoint set so stripSpans and remapEmotePositions
	// agree on exactly which bytes were removed. (Twitch's fragment spans are already disjoint; this
	// guards against a degenerate overlapping tag desyncing the emote-offset shift.)
	sort.Slice(valid, func(i, j int) bool { return valid[i].start < valid[j].start })
	lastEnd := -1
	for _, s := range valid {
		if s.start <= lastEnd {
			continue // overlaps a span already slated for removal — skip to stay disjoint
		}
		removed = append(removed, []int{s.start, s.end})
		lastEnd = s.end
	}

	return attachments, stripSpans(text, removed), removed
}

// trimOneBracketPair removes exactly one leading "[" and one trailing "]" — the single pair
// Twitch wraps a GIF's alt caption in — without disturbing brackets that are part of the caption
// itself (e.g. "[[Meta] GIF]" → "[Meta] GIF").
func trimOneBracketPair(s string) string {
	s = strings.TrimPrefix(s, "[")
	s = strings.TrimSuffix(s, "]")
	return s
}

// stripSpans removes the given inclusive byte ranges (sorted by start, disjoint) from text.
// Offsets in the returned string are exact — no trimming — so callers can re-anchor other
// positions against it via remapEmotePositions.
func stripSpans(text string, spans [][]int) string {
	if len(spans) == 0 {
		return text
	}
	var b strings.Builder
	cursor := 0
	for _, s := range spans {
		start, end := s[0], s[1]
		if start < cursor || start > len(text) {
			continue // overlap or out of range — skip defensively
		}
		b.WriteString(text[cursor:start])
		cursor = end + 1
	}
	if cursor < len(text) {
		b.WriteString(text[cursor:])
	}
	return b.String()
}

// remapEmotePositions re-anchors inclusive emote byte positions from the original text to
// the text with removed spans (inclusive, sorted, disjoint) deleted. An emote occurrence that
// overlaps a removed span is dropped; an emote left with no occurrences is omitted entirely.
func remapEmotePositions(emotes []models.Emote, removed [][]int) []models.Emote {
	if len(removed) == 0 {
		return emotes
	}
	out := make([]models.Emote, 0, len(emotes))
	for _, em := range emotes {
		newPos := make([][]int, 0, len(em.Positions))
		for _, p := range em.Positions {
			if len(p) != 2 {
				continue
			}
			shift, overlaps := removedShift(p[0], p[1], removed)
			if overlaps {
				continue
			}
			newPos = append(newPos, []int{p[0] - shift, p[1] - shift})
		}
		if len(newPos) > 0 {
			em.Positions = newPos
			out = append(out, em)
		}
	}
	return out
}

// removedShift returns how many bytes are removed entirely before start, and whether
// [start,end] overlaps any removed span. Spans are disjoint fragment ranges, so an emote
// never straddles one.
func removedShift(start, end int, removed [][]int) (shift int, overlaps bool) {
	for _, r := range removed {
		rs, re := r[0], r[1]
		if start <= re && end >= rs {
			return 0, true
		}
		if re < start {
			shift += re - rs + 1
		}
	}
	return shift, false
}

// extractMetadata extracts additional metadata from tags
func (n *TwitchNormalizer) extractMetadata(raw *models.RawChatMessage) map[string]interface{} {
	metadata := make(map[string]interface{})

	tags := raw.Tags

	// Boolean flags
	metadata["is_subscriber"] = tags["subscriber"] == "1"
	metadata["is_moderator"] = tags["mod"] == "1"
	metadata["is_turbo"] = tags["turbo"] == "1"

	// Shared Chat detection and metadata
	sourceRoomID := tags["source-room-id"]
	if sourceRoomID != "" {
		metadata["is_shared_chat"] = true
		metadata["source_room_id"] = sourceRoomID
		// Note: source channel name would need to be resolved via Twitch API
		// For now, we just track the room ID
	} else {
		metadata["is_shared_chat"] = false
	}

	// Message ID
	if msgID, ok := tags["id"]; ok {
		metadata["twitch_message_id"] = msgID
	}

	// Room ID
	if roomID, ok := tags["room-id"]; ok {
		metadata["twitch_room_id"] = roomID
	}

	// Timestamp
	if tmiSentTs, ok := tags["tmi-sent-ts"]; ok {
		metadata["twitch_sent_ts"] = tmiSentTs
	}

	// Extract bits from tags (for cheermote messages)
	bits := 0
	if bitsStr, ok := tags["bits"]; ok && bitsStr != "" {
		if val, err := strconv.Atoi(bitsStr); err == nil {
			bits = val
		}
	}
	metadata["bits"] = bits

	// Super chat (YouTube only, always 0 for Twitch)
	metadata["super_chat_amount"] = 0

	return metadata
}

// NormalizeEvent converts a RawChatMessage with event data to UnifiedChatMessage with EventInfo
func (n *TwitchNormalizer) NormalizeEvent(raw *models.RawChatMessage, overlayID string) (*models.UnifiedChatMessage, error) {
	if raw.Platform != "twitch" {
		return nil, fmt.Errorf("unsupported platform: %s", raw.Platform)
	}

	if raw.EventType == "" {
		return nil, fmt.Errorf("missing event type")
	}

	// Extract user info
	userInfo := n.extractUserInfo(raw)

	// Extract Twitch native emotes from tags (for events with user text like resubs)
	emotes := n.extractTwitchEmotes(raw)

	// Build EventValue from EventData
	var eventValue *models.EventValue

	switch raw.EventType {
	case "subscription", "resubscription":
		// Extract tier and months
		tier := "1000" // Default to Tier 1
		if t, ok := raw.EventData["tier"].(string); ok {
			tier = t
		}
		months := 0
		if m, ok := raw.EventData["months"].(int); ok {
			months = m
		} else if m, ok := raw.EventData["months"].(float64); ok {
			months = int(m)
		}

		// channel.subscribe (EventSub) carries no cumulative_months — emitting
		// "Tier 1 - 0 months" was the visible part of the bug report in #254.
		// Render the duration only when we actually have it.
		tierName := getTierName(tier)
		displayText := tierName
		if months > 0 {
			displayText = fmt.Sprintf("%s - %d months", tierName, months)
		}
		eventValue = &models.EventValue{
			Amount:      float64(months),
			Currency:    "months",
			DisplayText: displayText,
		}

	case "gift_subscription":
		tier := "1000"
		if t, ok := raw.EventData["tier"].(string); ok {
			tier = t
		}
		recipient := "someone"
		if r, ok := raw.EventData["recipient_name"].(string); ok {
			recipient = r
		}

		tierName := getTierName(tier)
		eventValue = &models.EventValue{
			Amount:      1,
			Currency:    "gift",
			DisplayText: fmt.Sprintf("Gifted %s sub to %s", tierName, recipient),
		}

	case "mystery_gift":
		giftCount := 0
		if g, ok := raw.EventData["gift_count"].(int); ok {
			giftCount = g
		} else if g, ok := raw.EventData["gift_count"].(float64); ok {
			giftCount = int(g)
		}

		eventValue = &models.EventValue{
			Amount:      float64(giftCount),
			Currency:    "gifts",
			DisplayText: fmt.Sprintf("%d gift subs", giftCount),
		}

	case "raid":
		viewerCount := 0
		if v, ok := raw.EventData["viewer_count"].(int); ok {
			viewerCount = v
		} else if v, ok := raw.EventData["viewer_count"].(float64); ok {
			viewerCount = int(v)
		}

		eventValue = &models.EventValue{
			Amount:      float64(viewerCount),
			Currency:    "viewers",
			DisplayText: fmt.Sprintf("%d viewers", viewerCount),
		}

	case "bits":
		badgeTier := 0
		if b, ok := raw.EventData["badge_tier"].(int); ok {
			badgeTier = b
		} else if b, ok := raw.EventData["badge_tier"].(float64); ok {
			badgeTier = int(b)
		}

		eventValue = &models.EventValue{
			Amount:      float64(badgeTier),
			Currency:    "bits",
			DisplayText: fmt.Sprintf("%d bits", badgeTier),
		}

	case "channel_points":
		cost := 0
		if c, ok := raw.EventData["reward_cost"].(int); ok {
			cost = c
		} else if c, ok := raw.EventData["reward_cost"].(float64); ok {
			cost = int(c)
		}
		title := "Reward"
		if t, ok := raw.EventData["reward_title"].(string); ok {
			title = t
		}

		eventValue = &models.EventValue{
			Amount:      float64(cost),
			Currency:    "points",
			DisplayText: fmt.Sprintf("%s (%d points)", title, cost),
		}
	}

	// Classify event tier and duration
	tier, duration := classifier.ClassifyEvent("twitch", raw.EventType, eventValue)

	// Create EventInfo
	eventInfo := &models.EventInfo{
		Type:     raw.EventType,
		Tier:     tier,
		Value:    eventValue,
		Duration: duration,
		IsUpdate: false, // Twitch events don't have updates
		Metadata: raw.EventData,
	}

	// Create unified message
	unified := &models.UnifiedChatMessage{
		ID:          raw.MessageID,
		OverlayID:   overlayID,
		Platform:    "twitch",
		ChannelID:   raw.ChannelID,
		ChannelName: raw.ChannelID,
		User:        userInfo,
		Message: models.MessageInfo{
			Text:   raw.Text, // User message (resubs, channel points) or system message
			Emotes: emotes,   // Twitch native emotes from tags (will be enriched with 3rd party later)
		},
		Timestamp: raw.Timestamp,
		Metadata:  n.extractMetadata(raw),
		Event:     eventInfo, // Add event info
	}

	return unified, nil
}

// getTierName converts Twitch subscription tier to human-readable name
func getTierName(tier string) string {
	switch tier {
	case "1000":
		return "Tier 1"
	case "2000":
		return "Tier 2"
	case "3000":
		return "Tier 3"
	case "Prime":
		return "Prime"
	default:
		return tier
	}
}
