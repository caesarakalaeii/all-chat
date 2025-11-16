package oauth

import (
	"context"

	"golang.org/x/oauth2"
)

// Platform represents an OAuth platform (Twitch, YouTube, Kick, TikTok)
type Platform string

const (
	PlatformTwitch  Platform = "twitch"
	PlatformYouTube Platform = "youtube"
	PlatformKick    Platform = "kick"
	PlatformTikTok  Platform = "tiktok"
)

// PlatformUserInfo is a generic user info interface
type PlatformUserInfo interface {
	GetID() string
	GetUsername() string
	GetDisplayName() string
	GetEmail() string
	GetProfileImageURL() string
	GetPlatform() Platform
}

// OAuthProvider is a generic OAuth provider interface
type OAuthProvider interface {
	// GetAuthURL generates the OAuth authorization URL
	GetAuthURL(state string) string

	// ExchangeCode exchanges authorization code for tokens
	ExchangeCode(ctx context.Context, code string) (*oauth2.Token, error)

	// GetUserInfo fetches user information
	GetUserInfo(ctx context.Context, accessToken string) (PlatformUserInfo, error)

	// RefreshToken refreshes an OAuth token
	RefreshToken(ctx context.Context, refreshToken string) (*oauth2.Token, error)

	// GetPlatform returns the platform identifier
	GetPlatform() Platform
}

// TwitchUserInfoWrapper wraps TwitchUserInfo to implement PlatformUserInfo
type TwitchUserInfoWrapper struct {
	ID              string
	Login           string
	DisplayName     string
	Email           string
	ProfileImageURL string
}

func (t *TwitchUserInfoWrapper) GetID() string              { return t.ID }
func (t *TwitchUserInfoWrapper) GetUsername() string        { return t.Login }
func (t *TwitchUserInfoWrapper) GetDisplayName() string     { return t.DisplayName }
func (t *TwitchUserInfoWrapper) GetEmail() string           { return t.Email }
func (t *TwitchUserInfoWrapper) GetProfileImageURL() string { return t.ProfileImageURL }
func (t *TwitchUserInfoWrapper) GetPlatform() Platform      { return PlatformTwitch }

// YouTubeUserInfoWrapper wraps YouTubeUserInfo to implement PlatformUserInfo
type YouTubeUserInfoWrapper struct {
	ID      string
	Name    string
	Email   string
	Picture string
}

func (y *YouTubeUserInfoWrapper) GetID() string              { return y.ID }
func (y *YouTubeUserInfoWrapper) GetUsername() string        { return y.Email }
func (y *YouTubeUserInfoWrapper) GetDisplayName() string     { return y.Name }
func (y *YouTubeUserInfoWrapper) GetEmail() string           { return y.Email }
func (y *YouTubeUserInfoWrapper) GetProfileImageURL() string { return y.Picture }
func (y *YouTubeUserInfoWrapper) GetPlatform() Platform      { return PlatformYouTube }

// TikTokUserInfoWrapper wraps TikTokUserInfo to implement PlatformUserInfo
type TikTokUserInfoWrapper struct {
	OpenID      string
	UnionID     string
	DisplayName string
	Username    string
	AvatarURL   string
}

func (t *TikTokUserInfoWrapper) GetID() string { return t.OpenID }
func (t *TikTokUserInfoWrapper) GetUsername() string {
	if t.Username != "" {
		return t.Username
	}
	return t.DisplayName
}
func (t *TikTokUserInfoWrapper) GetDisplayName() string     { return t.DisplayName }
func (t *TikTokUserInfoWrapper) GetEmail() string           { return "" } // TikTok doesn't provide email
func (t *TikTokUserInfoWrapper) GetProfileImageURL() string { return t.AvatarURL }
func (t *TikTokUserInfoWrapper) GetPlatform() Platform      { return PlatformTikTok }

// KickUserInfoWrapper wraps KickUserInfo to implement PlatformUserInfo
type KickUserInfoWrapper struct {
	ID          string
	Username    string
	DisplayName string
	ProfilePic  string
	Email       string
}

func (k *KickUserInfoWrapper) GetID() string              { return k.ID }
func (k *KickUserInfoWrapper) GetUsername() string        { return k.Username }
func (k *KickUserInfoWrapper) GetDisplayName() string     { return k.DisplayName }
func (k *KickUserInfoWrapper) GetEmail() string           { return k.Email }
func (k *KickUserInfoWrapper) GetProfileImageURL() string { return k.ProfilePic }
func (k *KickUserInfoWrapper) GetPlatform() Platform      { return PlatformKick }
