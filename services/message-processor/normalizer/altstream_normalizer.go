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
	"strings"
	"time"

	"github.com/caesar/all-chat/services/message-processor/models"
)

// altstreamSiteURL prefixes the site-relative media paths Altstream's API returns.
const altstreamSiteURL = "https://altstream.online"

// altstreamRoles is the order the roles object becomes badges in.
var altstreamRoles = []string{"broadcaster", "moderator", "subscriber", "supporter"}

// altstreamEvent is the Altstream chat event the listener forwards verbatim in
// raw_message (ADR-0082), plus the listener-added emotes list. The SSE shape
// carries authorUsername/authorImage/authorPronouns/badges; the REST shape
// lacks them, so every one of those is optional.
type altstreamEvent struct {
	Type           string          `json:"type"`
	ID             string          `json:"id"`
	Author         string          `json:"author"` // display name
	AuthorUsername string          `json:"authorUsername"`
	AuthorImage    string          `json:"authorImage"`
	AuthorPronouns string          `json:"authorPronouns"`
	Text           string          `json:"text"`
	SentAt         int64           `json:"sentAt"` // epoch ms
	UserColor      string          `json:"userColor"`
	Color          string          `json:"color"`
	Roles          map[string]bool `json:"roles"`
	Badges         []string        `json:"badges"`
	StickerURL     string          `json:"stickerUrl"` // a sticker post, sent with empty text
	Data           struct {
		Text       string `json:"text"`
		RewardName string `json:"rewardName"`
	} `json:"data"`
	Emotes []struct {
		Name string `json:"name"`
		URL  string `json:"url"`
	} `json:"emotes"`
}

// AltstreamNormalizer normalizes Altstream chat messages (public SSE chat API,
// ADR-0082) to the unified format.
type AltstreamNormalizer struct{}

// NewAltstreamNormalizer creates a new Altstream message normalizer.
func NewAltstreamNormalizer() *AltstreamNormalizer {
	return &AltstreamNormalizer{}
}

// Normalize converts a RawChatMessage to UnifiedChatMessage.
func (n *AltstreamNormalizer) Normalize(raw *models.RawChatMessage, overlayID string) (*models.UnifiedChatMessage, error) {
	if raw.Platform != "altstream" {
		return nil, fmt.Errorf("unsupported platform: %s", raw.Platform)
	}

	if err := validateChannelID(raw.ChannelID); err != nil {
		return nil, fmt.Errorf("invalid channel ID: %w", err)
	}

	// A raw_message that does not parse leaves the event empty; the listener's
	// top-level fields still carry id, user, text and timestamp.
	var ev altstreamEvent
	_ = json.Unmarshal(raw.RawMessage, &ev)

	text := raw.Text
	if text == "" {
		// A highlight's own text is "[HIGHLIGHT]:{json}"; the chat line is data.text.
		if ev.Type == "highlight" {
			text = ev.Data.Text
		} else {
			text = ev.Text
		}
	}

	timestamp := raw.Timestamp
	if timestamp.IsZero() && ev.SentAt > 0 {
		timestamp = time.UnixMilli(ev.SentAt).UTC()
	}
	if timestamp.IsZero() {
		timestamp = time.Now().UTC()
	}

	username := firstNonEmpty(raw.Username, ev.AuthorUsername, ev.Author)
	color := firstNonEmpty(ev.UserColor, ev.Color)
	if !isHexColor(color) {
		color = ""
	}

	metadata := map[string]interface{}{}
	messageType := firstNonEmpty(raw.Tags["message_type"], ev.Type)
	if messageType != "" {
		metadata["message_type"] = messageType
	}
	if messageType == "highlight" {
		metadata["is_highlighted"] = true
		if ev.Data.RewardName != "" {
			metadata["reward_name"] = ev.Data.RewardName
		}
	}

	return &models.UnifiedChatMessage{
		ID:          firstNonEmpty(raw.MessageID, ev.ID, fmt.Sprintf("altstream-%d", timestamp.UnixNano())),
		OverlayID:   overlayID,
		Platform:    "altstream",
		ChannelID:   raw.ChannelID,
		ChannelName: firstNonEmpty(raw.ChannelName, raw.ChannelID),
		User: models.UserInfo{
			ID:          firstNonEmpty(raw.UserID, strings.ToLower(username)),
			Username:    username,
			DisplayName: firstNonEmpty(ev.Author, username),
			AvatarURL:   altstreamAbsURL(ev.AuthorImage),
			Badges:      altstreamBadges(&ev),
			Color:       color,
			Pronouns:    ev.AuthorPronouns,
		},
		Message: models.MessageInfo{
			Text:        text,
			Emotes:      altstreamEmotes(text, &ev),
			Attachments: altstreamSticker(ev.StickerURL),
		},
		Timestamp: timestamp,
		Metadata:  metadata,
	}, nil
}

// altstreamSticker renders a sticker post as an image attachment (ADR-0037 path).
func altstreamSticker(url string) []models.Attachment {
	if url = altstreamAbsURL(url); !strings.HasPrefix(url, "https://") && !strings.HasPrefix(url, "http://") {
		return nil
	}
	return []models.Attachment{{Type: "image", URL: url, Filename: "Sticker"}}
}

// altstreamBadges merges the roles object and the badges list into one
// de-duplicated list, roles first. Altstream publishes no badge artwork, so
// IconURL stays empty (the same render state as TikTok badges).
func altstreamBadges(ev *altstreamEvent) []models.Badge {
	badges := []models.Badge{}
	seen := map[string]bool{}
	add := func(name string) {
		name = strings.ToLower(strings.TrimSpace(name))
		if name == "" || seen[name] {
			return
		}
		seen[name] = true
		badges = append(badges, models.Badge{Name: name, Version: "1"})
	}
	for _, role := range altstreamRoles {
		if ev.Roles[role] {
			add(role)
		}
	}
	for _, b := range ev.Badges {
		add(b)
	}
	return badges
}

// altstreamEmotes turns the listener's resolved emotes into unified emotes
// positioned on every :name: token in text (byte offsets, inclusive end, the
// whole token including the colons). Tokens the listener could not resolve
// are not listed and stay plain text.
func altstreamEmotes(text string, ev *altstreamEvent) []models.Emote {
	emotes := []models.Emote{}
	seen := map[string]bool{}
	for _, e := range ev.Emotes {
		if e.Name == "" || e.URL == "" || seen[e.Name] {
			continue
		}
		seen[e.Name] = true
		positions := findAllPositions(text, ":"+e.Name+":")
		if len(positions) == 0 {
			continue
		}
		emotes = append(emotes, models.Emote{
			Code:      e.Name,
			Provider:  "altstream",
			URL:       altstreamAbsURL(e.URL),
			Positions: positions,
		})
	}
	return emotes
}

// altstreamAbsURL makes a site-relative Altstream media path absolute.
func altstreamAbsURL(u string) string {
	if strings.HasPrefix(u, "/") && !strings.HasPrefix(u, "//") {
		return altstreamSiteURL + u
	}
	return u
}
