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

func (n *KickNormalizer) extractBadges(raw *models.RawChatMessage, kickMsg *kickChatMessage) []models.Badge {
	badges := make([]models.Badge, 0)
	seen := map[string]struct{}{}

	addBadge := func(name, version string) {
		key := name + "/" + version
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		// Kick doesn't expose public badge image URLs; icon_url stays empty
		// and the frontend renders a text chip fallback.
		badges = append(badges, models.Badge{
			Name:    name,
			Version: version,
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
