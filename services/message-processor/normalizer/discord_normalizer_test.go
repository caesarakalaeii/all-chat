package normalizer

import (
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
