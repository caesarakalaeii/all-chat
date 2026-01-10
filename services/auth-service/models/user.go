package models

import "time"

// User represents a user in the system
type User struct {
	ID              string    `json:"id"`
	TwitchID        *string   `json:"twitch_id,omitempty"`      // Nullable for other platform users
	GoogleID        *string   `json:"google_id,omitempty"`      // Nullable for non-YouTube users
	KickID          *string   `json:"kick_id,omitempty"`        // Nullable for non-Kick users
	AuthProvider    string    `json:"auth_provider"`            // "twitch", "youtube", or "kick"
	Username        string    `json:"username"`
	DisplayName     string    `json:"display_name"`
	ProfileImageURL string     `json:"profile_image_url"`
	IsAdmin         bool       `json:"is_admin"`                  // Admin role for access control
	IsBanned        bool       `json:"is_banned"`                 // Ban status
	BannedAt        *time.Time `json:"banned_at,omitempty"`       // When user was banned
	BannedReason    *string    `json:"banned_reason,omitempty"`   // Reason for ban
	BannedBy        *string    `json:"banned_by,omitempty"`       // Admin who banned (user ID)
	AccessToken     string     `json:"-"`                         // Never expose in JSON
	RefreshToken    string     `json:"-"`                         // Never expose in JSON
	TokenExpiresAt  time.Time  `json:"-"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
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
	ProfileImageURL string `json:"profile_image_url"`
}

// YouTubeUserInfo represents Google/YouTube user data from OAuth
type YouTubeUserInfo struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	GivenName  string `json:"given_name"`
	FamilyName string `json:"family_name"`
	Picture    string `json:"picture"`
	Locale     string `json:"locale"`
}

// KickUserInfo represents Kick user data from OAuth
type KickUserInfo struct {
	UserID         int    `json:"user_id"`         // User's unique numeric ID
	Name           string `json:"name"`            // User's login username
	ProfilePicture string `json:"profile_picture"` // User's profile picture URL
}
