package domain

import "time"

type User struct {
	ID                   string
	TwitchID             string
	Username             string
	DisplayName          string
	AvatarURL            string
	AccessTokenEncrypted string
	RefreshTokenEncrypted string
	TokenExpiresAt       time.Time
	CreatedAt            time.Time
	UpdatedAt            time.Time
	LastLoginAt          time.Time
}

type TwitchUser struct {
	ID              string `json:"id"`
	Login           string `json:"login"`
	DisplayName     string `json:"display_name"`
	ProfileImageURL string `json:"profile_image_url"`
	Email           string `json:"email"`
}
