package models

import "time"

// User represents a user in the system
type User struct {
	ID              string    `json:"id"`
	TwitchID        *string   `json:"twitch_id,omitempty"`      // Nullable for other platform users
	GoogleID        *string   `json:"google_id,omitempty"`      // Nullable for non-YouTube users
	TikTokOpenID    *string   `json:"tiktok_open_id,omitempty"` // Nullable for non-TikTok users
	KickID          *string   `json:"kick_id,omitempty"`        // Nullable for non-Kick users
	AuthProvider    string    `json:"auth_provider"`            // "twitch", "youtube", "tiktok", or "kick"
	Username        string    `json:"username"`
	DisplayName     string    `json:"display_name"`
	Email           string    `json:"email,omitempty"`
	ProfileImageURL string    `json:"profile_image_url"`
	AccessToken     string    `json:"-"` // Never expose in JSON
	RefreshToken    string    `json:"-"` // Never expose in JSON
	TokenExpiresAt  time.Time `json:"-"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
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

// YouTubeUserInfo represents Google/YouTube user data from OAuth
type YouTubeUserInfo struct {
	ID            string `json:"id"`
	Email         string `json:"email"`
	VerifiedEmail bool   `json:"verified_email"`
	Name          string `json:"name"`
	GivenName     string `json:"given_name"`
	FamilyName    string `json:"family_name"`
	Picture       string `json:"picture"`
	Locale        string `json:"locale"`
}

// TikTokUserInfo represents TikTok user data from OAuth
type TikTokUserInfo struct {
	OpenID      string `json:"open_id"`       // User's unique ID for this app
	UnionID     string `json:"union_id"`      // User's unique ID across all apps
	DisplayName string `json:"display_name"`  // User's display name
	AvatarURL   string `json:"avatar_url"`    // User's profile picture URL
}

// KickUserInfo represents Kick user data from OAuth
type KickUserInfo struct {
	ID         int    `json:"id"`          // User's unique numeric ID
	Username   string `json:"username"`    // User's login username
	Slug       string `json:"slug"`        // User's URL slug
	Bio        string `json:"bio"`         // User's biography
	ProfilePic string `json:"profile_pic"` // User's profile picture URL
	Email      string `json:"email"`       // User's email address
}
