package domain

import "time"

type Overlay struct {
	ID          string
	UserID      string
	Name        string
	Description string
	IsActive    bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type OverlayConfig struct {
	ID              string
	OverlayID       string
	TwitchChannel   string
	Enable7TV       bool
	EnableBTTV      bool
	EnableFFZ       bool
	DisplaySettings DisplaySettings
	FilterSettings  FilterSettings
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type DisplaySettings struct {
	MaxMessages     int    `json:"max_messages"`
	MessageDuration int    `json:"message_duration"`
	FontSize        int    `json:"font_size"`
	Animation       string `json:"animation"`
	Theme           string `json:"theme"`
}

type FilterSettings struct {
	BlockedUsers    []string `json:"blocked_users"`
	BlockedWords    []string `json:"blocked_words"`
	SubscriberOnly  bool     `json:"subscriber_only"`
	ModeratorOnly   bool     `json:"moderator_only"`
	MinChatDelay    int      `json:"min_chat_delay"`
}
