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
	"strings"
	"testing"
	"time"

	"github.com/caesar/all-chat/services/message-processor/models"
)

func makeDiscordRaw() *models.RawChatMessage {
	return &models.RawChatMessage{
		MessageID:   "msg-123",
		Platform:    "discord",
		OverlayID:   "",
		ChannelID:   "ch-456",
		ChannelName: "general",
		UserID:      "user-789",
		Username:    "testuser",
		Text:        "hello world",
		Timestamp:   time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
		Tags:        map[string]string{},
	}
}

func TestDiscordNormalizer_HappyPath(t *testing.T) {
	n := NewDiscordNormalizer()
	raw := makeDiscordRaw()
	raw.Tags["member_nick"] = "TestNick"
	raw.Tags["role_color"] = "#ff6600"
	raw.Tags["badges"] = "moderator,vip"
	raw.Tags["author_id"] = "user-789"

	unified, err := n.Normalize(raw, "overlay-1")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if unified.ID != "msg-123" {
		t.Errorf("ID: want msg-123, got %s", unified.ID)
	}
	if unified.OverlayID != "overlay-1" {
		t.Errorf("OverlayID: want overlay-1, got %s", unified.OverlayID)
	}
	if unified.Platform != "discord" {
		t.Errorf("Platform: want discord, got %s", unified.Platform)
	}
	if unified.ChannelID != "ch-456" {
		t.Errorf("ChannelID: want ch-456, got %s", unified.ChannelID)
	}
	if unified.ChannelName != "general" {
		t.Errorf("ChannelName: want general, got %s", unified.ChannelName)
	}
	if unified.User.ID != "user-789" {
		t.Errorf("User.ID: want user-789, got %s", unified.User.ID)
	}
	if unified.User.Username != "testuser" {
		t.Errorf("User.Username: want testuser, got %s", unified.User.Username)
	}
	if unified.User.DisplayName != "TestNick" {
		t.Errorf("User.DisplayName: want TestNick, got %s", unified.User.DisplayName)
	}
	if unified.User.Color != "#ff6600" {
		t.Errorf("User.Color: want #ff6600, got %s", unified.User.Color)
	}
	if len(unified.User.Badges) != 2 {
		t.Fatalf("expected 2 badges, got %d", len(unified.User.Badges))
	}
	if unified.User.Badges[0].Name != "moderator" || unified.User.Badges[0].Version != "1" {
		t.Errorf("Badge[0]: want {moderator 1}, got {%s %s}", unified.User.Badges[0].Name, unified.User.Badges[0].Version)
	}
	if unified.User.Badges[1].Name != "vip" || unified.User.Badges[1].Version != "1" {
		t.Errorf("Badge[1]: want {vip 1}, got {%s %s}", unified.User.Badges[1].Name, unified.User.Badges[1].Version)
	}
	if unified.Message.Text != "hello world" {
		t.Errorf("Message.Text: want 'hello world', got %s", unified.Message.Text)
	}
	if unified.Message.Emotes == nil {
		t.Error("Message.Emotes should not be nil (should be empty slice)")
	}
	if len(unified.Message.Emotes) != 0 {
		t.Errorf("Message.Emotes: expected 0 emotes, got %d", len(unified.Message.Emotes))
	}
}

func TestDiscordNormalizer_NickFallback(t *testing.T) {
	n := NewDiscordNormalizer()
	raw := makeDiscordRaw()
	raw.Tags["member_nick"] = ""
	raw.Tags["role_color"] = "#aabbcc"
	raw.Tags["badges"] = ""

	unified, err := n.Normalize(raw, "overlay-2")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if unified.User.DisplayName != raw.Username {
		t.Errorf("DisplayName: want %s (fallback to Username), got %s", raw.Username, unified.User.DisplayName)
	}
}

func TestDiscordNormalizer_BlackColor(t *testing.T) {
	n := NewDiscordNormalizer()
	raw := makeDiscordRaw()
	raw.Tags["member_nick"] = "Nick"
	raw.Tags["role_color"] = "#000000"
	raw.Tags["badges"] = ""

	unified, err := n.Normalize(raw, "overlay-3")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if unified.User.Color != "" {
		t.Errorf("Color: want empty string for #000000, got %s", unified.User.Color)
	}
}

func TestDiscordNormalizer_EmptyColor(t *testing.T) {
	n := NewDiscordNormalizer()
	raw := makeDiscordRaw()
	raw.Tags["member_nick"] = "Nick"
	raw.Tags["role_color"] = ""
	raw.Tags["badges"] = ""

	unified, err := n.Normalize(raw, "overlay-4")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if unified.User.Color != "" {
		t.Errorf("Color: want empty string for empty role_color, got %s", unified.User.Color)
	}
}

func TestDiscordNormalizer_WrongPlatform(t *testing.T) {
	n := NewDiscordNormalizer()
	raw := makeDiscordRaw()
	raw.Platform = "twitch"

	_, err := n.Normalize(raw, "overlay-5")
	if err == nil {
		t.Fatal("expected error for wrong platform, got nil")
	}

	if !containsString(err.Error(), "unsupported platform") {
		t.Errorf("error should contain 'unsupported platform', got: %s", err.Error())
	}
}

func TestDiscordNormalizer_Badges(t *testing.T) {
	n := NewDiscordNormalizer()
	raw := makeDiscordRaw()
	raw.Tags["member_nick"] = "Nick"
	raw.Tags["role_color"] = "#aabbcc"
	raw.Tags["badges"] = "moderator,vip"

	unified, err := n.Normalize(raw, "overlay-6")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if len(unified.User.Badges) != 2 {
		t.Fatalf("expected 2 badges, got %d", len(unified.User.Badges))
	}

	wantBadges := []struct {
		name    string
		version string
	}{
		{"moderator", "1"},
		{"vip", "1"},
	}

	for i, want := range wantBadges {
		if unified.User.Badges[i].Name != want.name {
			t.Errorf("Badge[%d].Name: want %s, got %s", i, want.name, unified.User.Badges[i].Name)
		}
		if unified.User.Badges[i].Version != want.version {
			t.Errorf("Badge[%d].Version: want %s, got %s", i, want.version, unified.User.Badges[i].Version)
		}
	}
}

func TestDiscordNormalizer_NoBadges(t *testing.T) {
	n := NewDiscordNormalizer()
	raw := makeDiscordRaw()
	raw.Tags["member_nick"] = "Nick"
	raw.Tags["role_color"] = "#aabbcc"
	raw.Tags["badges"] = ""

	unified, err := n.Normalize(raw, "overlay-7")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if unified.User.Badges == nil {
		t.Error("Badges should not be nil (should be empty slice)")
	}
	if len(unified.User.Badges) != 0 {
		t.Errorf("Badges: expected 0 badges, got %d", len(unified.User.Badges))
	}
}

func TestParseDiscordEmotes_StaticAndAnimated(t *testing.T) {
	emotes := parseDiscordEmotes("hi <:pepe:123> and <a:blob:456> !")
	if len(emotes) != 2 {
		t.Fatalf("expected 2 emotes, got %d", len(emotes))
	}

	if emotes[0].Code != "pepe" || emotes[0].Provider != "discord" {
		t.Errorf("emote[0]: want {pepe discord}, got {%s %s}", emotes[0].Code, emotes[0].Provider)
	}
	wantStatic := "https://cdn.discordapp.com/emojis/123.png?size=48&quality=lossless"
	if emotes[0].URL != wantStatic {
		t.Errorf("emote[0].URL: want %s, got %s", wantStatic, emotes[0].URL)
	}

	if emotes[1].Code != "blob" {
		t.Errorf("emote[1].Code: want blob, got %s", emotes[1].Code)
	}
	wantAnimated := "https://cdn.discordapp.com/emojis/456.gif?size=48&quality=lossless"
	if emotes[1].URL != wantAnimated {
		t.Errorf("emote[1].URL (animated must be .gif): want %s, got %s", wantAnimated, emotes[1].URL)
	}
}

func TestParseDiscordEmotes_Positions(t *testing.T) {
	text := "hi <:pepe:123> yo"
	emotes := parseDiscordEmotes(text)
	if len(emotes) != 1 {
		t.Fatalf("expected 1 emote, got %d", len(emotes))
	}
	pos := emotes[0].Positions
	if len(pos) != 1 || len(pos[0]) != 2 {
		t.Fatalf("unexpected positions shape: %v", pos)
	}
	start, end := pos[0][0], pos[0][1]
	// Byte offsets with inclusive end, matching the frontend renderer's
	// text.slice(start, end+1) contract.
	if start != 3 || end != 13 {
		t.Errorf("positions: want [3 13], got [%d %d]", start, end)
	}
	if got := text[start : end+1]; got != "<:pepe:123>" {
		t.Errorf("sliced token: want <:pepe:123>, got %q", got)
	}
}

func TestParseDiscordEmotes_None(t *testing.T) {
	emotes := parseDiscordEmotes("just plain text, no emoji")
	if emotes == nil {
		t.Fatal("expected non-nil empty slice, got nil")
	}
	if len(emotes) != 0 {
		t.Errorf("expected 0 emotes, got %d", len(emotes))
	}
}

func TestParseDiscordEmotes_IgnoresMalformed(t *testing.T) {
	// A colon-wrapped word is a plain-text emoji shortcode, not a custom emoji token.
	emotes := parseDiscordEmotes("nice :thumbsup: <notanemoji>")
	if len(emotes) != 0 {
		t.Errorf("expected 0 emotes for non-token text, got %d", len(emotes))
	}
}

func TestParseDiscordEmotes_Cap(t *testing.T) {
	var sb strings.Builder
	for i := 0; i < maxDiscordEmotes+5; i++ {
		sb.WriteString("<:ee:1> ")
	}
	emotes := parseDiscordEmotes(sb.String())
	if len(emotes) != maxDiscordEmotes {
		t.Errorf("expected cap of %d, got %d", maxDiscordEmotes, len(emotes))
	}
}

func TestNormalizeAttachments_FiltersInvalidAndCaps(t *testing.T) {
	in := []models.Attachment{
		{Type: "image", URL: "https://x/1.png"},
		{Type: "image", URL: ""},          // dropped: no url
		{Type: "audio", URL: "https://x/a.mp3"}, // dropped: unknown type
		{Type: "video", URL: "https://x/2.mp4"},
		{Type: "image", URL: "https://x/3.png"},
		{Type: "image", URL: "https://x/4.png"},
		{Type: "image", URL: "https://x/5.png"}, // beyond cap
	}
	out := normalizeAttachments(in)
	if len(out) != maxDiscordAttachments {
		t.Fatalf("expected cap of %d, got %d", maxDiscordAttachments, len(out))
	}
	if out[0].URL != "https://x/1.png" || out[1].URL != "https://x/2.mp4" {
		t.Errorf("unexpected order/filtering: %+v", out)
	}
}

func TestNormalizeAttachments_EmptyReturnsNil(t *testing.T) {
	if normalizeAttachments(nil) != nil {
		t.Error("nil input should return nil")
	}
	if normalizeAttachments([]models.Attachment{{Type: "audio", URL: "https://x/a.mp3"}}) != nil {
		t.Error("all-filtered input should return nil")
	}
}

func TestDiscordNormalizer_WithAttachmentsAndEmotes(t *testing.T) {
	n := NewDiscordNormalizer()
	raw := makeDiscordRaw()
	raw.Text = "look <:kek:99>"
	raw.Attachments = []models.Attachment{
		{Type: "image", URL: "https://media.discordapp.net/cat.gif", ContentType: "image/gif", Spoiler: true, Filename: "cat.gif"},
	}

	unified, err := n.Normalize(raw, "overlay-1")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if len(unified.Message.Emotes) != 1 {
		t.Fatalf("expected 1 inline emote, got %d", len(unified.Message.Emotes))
	}
	if unified.Message.Emotes[0].Code != "kek" {
		t.Errorf("emote code: want kek, got %s", unified.Message.Emotes[0].Code)
	}
	if len(unified.Message.Attachments) != 1 {
		t.Fatalf("expected 1 attachment, got %d", len(unified.Message.Attachments))
	}
	att := unified.Message.Attachments[0]
	if att.Type != "image" || att.URL != "https://media.discordapp.net/cat.gif" || !att.Spoiler {
		t.Errorf("unexpected attachment: %+v", att)
	}
}

// TestDiscordNormalizer_ParsesWireJSON feeds the exact JSON the discord-listener
// publishes to chat:raw through the real deserialization path (ParseRawMessage ->
// Normalize), verifying the cross-service attachment contract on the consumer side.
func TestDiscordNormalizer_ParsesWireJSON(t *testing.T) {
	wire := []byte(`{
		"message_id":"m1","platform":"discord","overlay_id":"","channel_id":"c1",
		"channel_name":"general","user_id":"u1","username":"bob","text":"gg <:pog:42>",
		"tags":{"member_nick":"Bob"},
		"attachments":[
			{"type":"image","url":"https://media.discordapp.net/cat.gif","content_type":"image/gif","width":200,"height":150,"spoiler":true,"filename":"cat.gif"},
			{"type":"video","url":"https://media.discordapp.net/ext/foo.mp4","thumb_url":"https://media.discordapp.net/ext/foo.png"}
		],
		"timestamp":"2024-01-01T00:00:00Z"
	}`)

	raw, err := models.ParseRawMessage(wire)
	if err != nil {
		t.Fatalf("ParseRawMessage failed: %v", err)
	}

	unified, err := NewDiscordNormalizer().Normalize(raw, "overlay-1")
	if err != nil {
		t.Fatalf("Normalize failed: %v", err)
	}

	if len(unified.Message.Attachments) != 2 {
		t.Fatalf("expected 2 attachments from wire JSON, got %d", len(unified.Message.Attachments))
	}
	if unified.Message.Attachments[0].Type != "image" ||
		unified.Message.Attachments[0].URL != "https://media.discordapp.net/cat.gif" ||
		!unified.Message.Attachments[0].Spoiler {
		t.Errorf("attachment[0] mismatch: %+v", unified.Message.Attachments[0])
	}
	if unified.Message.Attachments[1].Type != "video" ||
		unified.Message.Attachments[1].ThumbURL != "https://media.discordapp.net/ext/foo.png" {
		t.Errorf("attachment[1] mismatch: %+v", unified.Message.Attachments[1])
	}
	if len(unified.Message.Emotes) != 1 || unified.Message.Emotes[0].Code != "pog" {
		t.Errorf("expected inline discord emote 'pog', got %+v", unified.Message.Emotes)
	}
}

// containsString is a helper since we can't import strings in _test.go helpers here
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		func() bool {
			for i := 0; i <= len(s)-len(substr); i++ {
				if s[i:i+len(substr)] == substr {
					return true
				}
			}
			return false
		}())
}
