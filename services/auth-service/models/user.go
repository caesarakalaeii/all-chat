package models

import "time"

// User represents a user in the system
type User struct {
	ID                string    `json:"id"`
	TwitchID          string    `json:"twitch_id"`
	Username          string    `json:"username"`
	DisplayName       string    `json:"display_name"`
	Email             string    `json:"email,omitempty"`
	ProfileImageURL   string    `json:"profile_image_url"`
	AccessToken       string    `json:"-"` // Never expose in JSON
	RefreshToken      string    `json:"-"` // Never expose in JSON
	TokenExpiresAt    time.Time `json:"-"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// TokenResponse represents JWT token response
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
	TokenType    string `json:"token_type"`
}

// TwitchUserInfo represents Twitch user data from OAuth
type TwitchUserInfo struct {
	ID              string `json:"id"`
	Login           string `json:"login"`
	DisplayName     string `json:"display_name"`
	Email           string `json:"email"`
	ProfileImageURL string `json:"profile_image_url"`
}
