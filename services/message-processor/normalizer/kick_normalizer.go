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
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/caesar/all-chat/services/message-processor/models"
)

// kickEmoteTokenRe matches [emote:ID:name] tokens in Kick message text
var kickEmoteTokenRe = regexp.MustCompile(`\[emote:(\d+):([^\]]+)\]`)

// KickNormalizer normalizes Kick chat messages to unified format
type KickNormalizer struct{}

// NewKickNormalizer creates a new Kick message normalizer
func NewKickNormalizer() *KickNormalizer {
	return &KickNormalizer{}
}

// Normalize converts a RawChatMessage to UnifiedChatMessage
func (n *KickNormalizer) Normalize(raw *models.RawChatMessage, overlayID string) (*models.UnifiedChatMessage, error) {
	if raw.Platform != "kick" {
		return nil, fmt.Errorf("unsupported platform: %s", raw.Platform)
	}

	var kickMsg *kickChatMessage
	if len(raw.RawMessage) > 0 {
		if event, err := parseKickMessage(raw.RawMessage); err == nil {
			kickMsg = event
		}
	}

	if err := validateChannelID(raw.ChannelID); err != nil {
		return nil, fmt.Errorf("invalid channel ID: %w", err)
	}

	timestamp := n.resolveTimestamp(raw.Timestamp, kickMsg)
	text := raw.Text
	if text == "" && kickMsg != nil {
		text = kickMsg.Content
	}

	messageID := raw.MessageID
	if messageID == "" && kickMsg != nil {
		messageID = kickMsg.ID
	}
	if messageID == "" {
		messageID = fmt.Sprintf("kick-%d", timestamp.UnixNano())
	}

	userID := raw.UserID
	if userID == "" && kickMsg != nil && kickMsg.Sender.ID != 0 {
		userID = strconv.Itoa(kickMsg.Sender.ID)
	}

	username := raw.Username
	if username == "" && kickMsg != nil {
		username = firstNonEmpty(kickMsg.Sender.Username, kickMsg.Sender.Slug)
	}

	color := raw.Tags["color"]
	if color == "" && kickMsg != nil {
		color = kickMsg.Sender.Identity.Color
	}

	badges := n.extractBadges(raw, kickMsg)
	metadata := n.extractMetadata(raw, kickMsg)

	// Parse [emote:ID:name] tokens from text, replacing them with just the name
	// and extracting positioned emotes with Kick CDN URLs.
	cleanText, emotes := parseKickEmotesFromText(text)
	// Fall back to MessageParts-based extraction if token parsing found nothing
	if len(emotes) == 0 {
		emotes = extractKickEmotes(kickMsg)
	}

	unified := &models.UnifiedChatMessage{
		ID:          messageID,
		OverlayID:   overlayID,
		Platform:    "kick",
		ChannelID:   raw.ChannelID,
		ChannelName: firstNonEmpty(raw.ChannelName, raw.ChannelID),
		User: models.UserInfo{
			ID:          userID,
			Username:    username,
			DisplayName: username,
			AvatarURL:   "",
			Badges:      badges,
			Color:       color,
		},
		Message: models.MessageInfo{
			Text:   cleanText,
			Emotes: emotes,
		},
		Timestamp: timestamp,
		Metadata:  metadata,
	}

	return unified, nil
}

func (n *KickNormalizer) resolveTimestamp(ts time.Time, kickMsg *kickChatMessage) time.Time {
	if !ts.IsZero() {
		return ts
	}

	if kickMsg != nil && kickMsg.CreatedAt != "" {
		if parsed, err := time.Parse(time.RFC3339Nano, kickMsg.CreatedAt); err == nil {
			return parsed
		}
	}

	return time.Now()
}

// kickBadgeIcons maps Kick badge type names to SVG data URIs sourced from Kick's actual badge assets.
var kickBadgeIcons = map[string]string{
	// Broadcaster/Host - pixel-art microphone, pink-to-purple gradient
	"broadcaster": "data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 16 16'%3E%3Cdefs%3E%3ClinearGradient id='g' gradientUnits='userSpaceOnUse' x1='8' y1='0' x2='8' y2='16'%3E%3Cstop offset='0' stop-color='%23FF1CD2'/%3E%3Cstop offset='.99' stop-color='%23B20DFF'/%3E%3C/linearGradient%3E%3C/defs%3E%3Crect x='3.2' y='9.6' width='1.6' height='1.6' fill='url(%23g)'/%3E%3Cpolygon points='6.4,9.6 9.6,9.6 9.6,8 11.2,8 11.2,1.6 9.6,1.6 9.6,0 6.4,0 6.4,1.6 4.8,1.6 4.8,8 6.4,8' fill='url(%23g)'/%3E%3Crect x='1.6' y='6.4' width='1.6' height='3.2' fill='url(%23g)'/%3E%3Crect x='11.2' y='9.6' width='1.6' height='1.6' fill='url(%23g)'/%3E%3Cpolygon points='4.8,12.8 6.4,12.8 6.4,14.4 4.8,14.4 4.8,16 11.2,16 11.2,14.4 9.6,14.4 9.6,12.8 11.2,12.8 11.2,11.2 4.8,11.2' fill='url(%23g)'/%3E%3Crect x='12.8' y='6.4' width='1.6' height='3.2' fill='url(%23g)'/%3E%3C/svg%3E",
	// Moderator - pixel-art sword, flat cyan
	"moderator": "data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 16 16'%3E%3Cpath d='M11.7,1.3v1.5h-1.5v1.5H8.7v1.5H7.3v1.5H5.8V5.8h-3v3h1.5v1.5H2.8v1.5H1.3v3h3v-1.5h1.5v-1.5h1.5v1.5h3v-3H8.7V8.7h1.5V7.3h1.5V5.8h1.5V4.3h1.5v-3h-3z' fill='%2300C7FF'/%3E%3C/svg%3E",
	// VIP - pixel-art crown, gold-to-orange gradient
	"vip": "data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 16 16'%3E%3Cdefs%3E%3ClinearGradient id='g' gradientUnits='userSpaceOnUse' x1='8' y1='0' x2='8' y2='16'%3E%3Cstop offset='0' stop-color='%23FFC900'/%3E%3Cstop offset='.99' stop-color='%23FF9500'/%3E%3C/linearGradient%3E%3C/defs%3E%3Cpath d='M13.9,2.4v1.1h-1.2v2.3h-1.1v1.1h-1.1V4.6H9.3V1.3H6.7v3.3H5.6v2.3H4.4V5.8H3.3V3.5H2.1V2.4H0v12.3h16V2.4H13.9z' fill='url(%23g)'/%3E%3C/svg%3E",
	// Subscriber - pixel-art 4-pointed star, lime-to-green gradient
	"subscriber": "data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 16 16'%3E%3Cdefs%3E%3ClinearGradient id='g' x1='0' y1='0' x2='1' y2='1'%3E%3Cstop offset='0' stop-color='%23E1FF00'/%3E%3Cstop offset='.99' stop-color='%232AA300'/%3E%3C/linearGradient%3E%3C/defs%3E%3Cpath d='M14.8,7.3V6.1h-2.4V4.9H11V3.7H9.9V1.2H8.7V0H7.3v1.2H6.1v2.5H5v1.2H3.7v1.3H1.2v1.2H0v1.4h1.2V10h2.4v1.3H5v1.2h1.2V15h1.2v1h1.3v-1.2h1.2v-2.5H11v-1.2h1.3V9.9h2.4V8.7H16V7.3H14.8z' fill='url(%23g)'/%3E%3C/svg%3E",
	// Founder - pixel-art shield with "1", gold-to-orange gradient
	"founder": "data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 16 16'%3E%3Cdefs%3E%3ClinearGradient id='g' gradientUnits='userSpaceOnUse' x1='8' y1='0' x2='8' y2='16'%3E%3Cstop offset='0' stop-color='%23FFC900'/%3E%3Cstop offset='.99' stop-color='%23FF9500'/%3E%3C/linearGradient%3E%3C/defs%3E%3Cpath d='M14.6,4V2.7h-1.3V1.4H12V0H4v1.4H2.7v1.3H1.3V4H0v8h1.3v1.3h1.4v1.3H4V16h8v-1.4h1.3v-1.3h1.3V12H16V4H14.6z M9.9,12.9H6.7V6.4H4.5V5.2h1V4.1h1v-1h3.4V12.9z' fill-rule='evenodd' clip-rule='evenodd' fill='url(%23g)'/%3E%3C/svg%3E",
	// Verified - starburst seal with checkmark, green gradient
	"verified": "data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 16 16'%3E%3Cdefs%3E%3ClinearGradient id='g' x1='.34' y1='.97' x2='.66' y2='.05'%3E%3Cstop stop-color='%231EFF00'/%3E%3Cstop offset='.99' stop-color='%2300FF8C'/%3E%3C/linearGradient%3E%3C/defs%3E%3Cpath d='M16 6.835l-2.265-1.9-.515-2.91h-2.955L8 .12 5.735 2.02H2.78l-.515 2.91L0 6.835l1.48 2.56-.515 2.91 2.78 1.01 1.48 2.56L8 14.865l2.78 1.01 1.48-2.56 2.78-1.01-.515-2.91L16 6.835zM6.495 12.405l-3.705-3.71 1.415-1.415 2.29 2.295 4.795-4.795 1.415 1.415-6.205 6.205z' fill='url(%23g)'/%3E%3C/svg%3E",
}

// kickBadgeIconURL returns the SVG data URI for a known Kick badge type, or empty string.
func kickBadgeIconURL(badgeType string) string {
	normalized := strings.ToLower(badgeType)
	if url, ok := kickBadgeIcons[normalized]; ok {
		return url
	}
	// Handle common aliases
	switch normalized {
	case "host", "streamer":
		return kickBadgeIcons["broadcaster"]
	case "sub":
		return kickBadgeIcons["subscriber"]
	}
	return ""
}

func (n *KickNormalizer) extractBadges(raw *models.RawChatMessage, kickMsg *kickChatMessage) []models.Badge {
	badges := make([]models.Badge, 0)
	seen := map[string]struct{}{}

	addBadge := func(name, version string) {
		key := name + "/" + version
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		badges = append(badges, models.Badge{
			Name:    name,
			Version: version,
			IconURL: kickBadgeIconURL(name),
		})
	}

	// Prefer structured badges from the raw Kick message (most reliable)
	if kickMsg != nil {
		for _, badge := range kickMsg.Sender.Identity.Badges {
			if badge.Type == "" {
				continue
			}
			addBadge(badge.Type, badge.Text)
		}
	}

	// Fall back to tag-based badges
	if len(badges) == 0 && raw.Tags != nil {
		if badgeList := raw.Tags["badges"]; badgeList != "" {
			for _, name := range strings.Split(badgeList, ",") {
				name = strings.TrimSpace(name)
				if name != "" {
					addBadge(name, "1")
				}
			}
		}
	}

	return badges
}

func (n *KickNormalizer) extractMetadata(raw *models.RawChatMessage, kickMsg *kickChatMessage) map[string]interface{} {
	metadata := make(map[string]interface{})

	if raw.Tags != nil {
		if msgType, ok := raw.Tags["message_type"]; ok && msgType != "" {
			metadata["message_type"] = msgType
		}

		if chatroomID, ok := raw.Tags["chatroom_id"]; ok && chatroomID != "" {
			if numeric, err := strconv.Atoi(chatroomID); err == nil {
				metadata["chatroom_id"] = numeric
			} else {
				metadata["chatroom_id"] = chatroomID
			}
		}

		if slug, ok := raw.Tags["sender_slug"]; ok && slug != "" {
			metadata["sender_slug"] = slug
		}
	}

	if kickMsg != nil {
		if metadata["message_type"] == nil && kickMsg.Type != "" {
			metadata["message_type"] = kickMsg.Type
		}

		if metadata["chatroom_id"] == nil && kickMsg.ChatroomID != 0 {
			metadata["chatroom_id"] = kickMsg.ChatroomID
		}

		if metadata["sender_slug"] == nil && kickMsg.Sender.Slug != "" {
			metadata["sender_slug"] = kickMsg.Sender.Slug
		}
	}

	// Derived roles based on tags/badges
	badgeSet := make(map[string]struct{})
	if raw.Tags != nil {
		for _, badge := range strings.Split(raw.Tags["badges"], ",") {
			if badge = strings.TrimSpace(badge); badge != "" {
				badgeSet[strings.ToLower(badge)] = struct{}{}
			}
		}
	}
	if kickMsg != nil {
		for _, badge := range kickMsg.Sender.Identity.Badges {
			if badge.Type != "" {
				badgeSet[strings.ToLower(badge.Type)] = struct{}{}
			}
		}
	}

	_, isSub := badgeSet["subscriber"]
	_, isMod := badgeSet["moderator"]
	_, isVIP := badgeSet["vip"]
	_, isFounder := badgeSet["founder"]

	metadata["is_subscriber"] = isSub
	metadata["is_moderator"] = isMod
	metadata["is_vip"] = isVIP
	metadata["is_founder"] = isFounder

	return metadata
}

func parseKickMessage(raw json.RawMessage) (*kickChatMessage, error) {
	if len(raw) == 0 {
		return nil, fmt.Errorf("empty kick payload")
	}

	var msg kickChatMessage
	if err := json.Unmarshal(raw, &msg); err != nil {
		return nil, err
	}

	return &msg, nil
}

// parseKickEmotesFromText parses [emote:ID:name] tokens from Kick message text,
// returns the emotes with positions and the cleaned text with tokens replaced by emote names.
// Kick CDN URL pattern: https://files.kick.com/emotes/{ID}/fullsize
func parseKickEmotesFromText(text string) (cleanText string, emotes []models.Emote) {
	if text == "" {
		return text, nil
	}

	type token struct {
		id    string
		name  string
		start int
		end   int // inclusive
	}

	// Find all tokens and record their positions in the original text
	matches := kickEmoteTokenRe.FindAllStringSubmatchIndex(text, -1)
	if len(matches) == 0 {
		return text, nil
	}

	var tokens []token
	for _, m := range matches {
		// m[0]:m[1] = full match, m[2]:m[3] = ID, m[4]:m[5] = name
		tokens = append(tokens, token{
			id:    text[m[2]:m[3]],
			name:  text[m[4]:m[5]],
			start: m[0],
			end:   m[1] - 1,
		})
	}

	// Build cleaned text: replace each token with just the emote name
	var sb strings.Builder
	cursor := 0
	// offset tracks how the positions shift as we replace tokens
	offset := 0
	type adjustedToken struct {
		name  string
		id    string
		start int
		end   int
	}
	var adjusted []adjustedToken

	for _, tok := range tokens {
		sb.WriteString(text[cursor:tok.start])
		newStart := tok.start - offset
		sb.WriteString(tok.name)
		newEnd := newStart + len(tok.name) - 1
		adjusted = append(adjusted, adjustedToken{
			name:  tok.name,
			id:    tok.id,
			start: newStart,
			end:   newEnd,
		})
		// original token length vs replacement (name) length
		offset += (tok.end + 1 - tok.start) - len(tok.name)
		cursor = tok.end + 1
	}
	sb.WriteString(text[cursor:])
	cleanText = sb.String()

	// Build emotes list, merging duplicate codes into multiple positions
	type emoteEntry struct {
		id        string
		positions [][]int
	}
	seen := map[string]*emoteEntry{}
	order := []string{}

	for _, adj := range adjusted {
		if e, ok := seen[adj.name]; ok {
			e.positions = append(e.positions, []int{adj.start, adj.end})
		} else {
			seen[adj.name] = &emoteEntry{
				id:        adj.id,
				positions: [][]int{{adj.start, adj.end}},
			}
			order = append(order, adj.name)
		}
	}

	for _, name := range order {
		e := seen[name]
		emotes = append(emotes, models.Emote{
			Code:      name,
			Provider:  "kick",
			URL:       fmt.Sprintf("https://files.kick.com/emotes/%s/fullsize", e.id),
			Positions: e.positions,
		})
	}

	return cleanText, emotes
}

func extractKickEmotes(msg *kickChatMessage) []models.Emote {
	if msg == nil || len(msg.MessageParts) == 0 {
		return []models.Emote{}
	}

	emotes := make([]models.Emote, 0)
	for _, part := range msg.MessageParts {
		if part.Type == "" {
			continue
		}

		switch strings.ToLower(part.Type) {
		case "emote", "emoticon":
			code := firstNonEmpty(part.Text, part.Name, part.Value)
			if code == "" {
				continue
			}
			emotes = append(emotes, models.Emote{
				Code:     code,
				Provider: "kick",
			})
		}
	}

	return emotes
}

func firstNonEmpty(values ...string) string {
	for _, val := range values {
		if val != "" {
			return val
		}
	}
	return ""
}

type kickChatMessage struct {
	ID           string            `json:"id"`
	ChatroomID   int               `json:"chatroom_id"`
	Content      string            `json:"content"`
	Type         string            `json:"type"`
	CreatedAt    string            `json:"created_at"`
	Sender       kickSender        `json:"sender"`
	MessageParts []kickMessagePart `json:"message_parts"`
}

type kickSender struct {
	ID       int          `json:"id"`
	Username string       `json:"username"`
	Slug     string       `json:"slug"`
	Identity kickIdentity `json:"identity"`
}

type kickIdentity struct {
	Color  string      `json:"color"`
	Badges []kickBadge `json:"badges"`
}

type kickBadge struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type kickMessagePart struct {
	Type  string `json:"type"`
	Text  string `json:"text"`
	Name  string `json:"name"`
	Value string `json:"value"`
	URL   string `json:"url"`
}
