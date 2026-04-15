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

package oauth

import (
	"context"
	"fmt"

	"golang.org/x/oauth2"
)

// Platform represents an OAuth platform (Twitch, YouTube, Kick)
type Platform string

const (
	PlatformTwitch  Platform = "twitch"
	PlatformYouTube Platform = "youtube"
	PlatformKick    Platform = "kick"
	PlatformDiscord Platform = "discord"
)

// PlatformUserInfo is a generic user info interface
type PlatformUserInfo interface {
	GetID() string
	GetUsername() string
	GetDisplayName() string
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
	ProfileImageURL string
}

func (t *TwitchUserInfoWrapper) GetID() string              { return t.ID }
func (t *TwitchUserInfoWrapper) GetUsername() string        { return t.Login }
func (t *TwitchUserInfoWrapper) GetDisplayName() string     { return t.DisplayName }
func (t *TwitchUserInfoWrapper) GetProfileImageURL() string { return t.ProfileImageURL }
func (t *TwitchUserInfoWrapper) GetPlatform() Platform      { return PlatformTwitch }

// YouTubeUserInfoWrapper wraps YouTubeUserInfo to implement PlatformUserInfo
type YouTubeUserInfoWrapper struct {
	ID      string
	Name    string
	Picture string
}

func (y *YouTubeUserInfoWrapper) GetID() string { return y.ID }
func (y *YouTubeUserInfoWrapper) GetUsername() string {
	if y.ID != "" {
		return fmt.Sprintf("youtube_%s", y.ID)
	}
	return "youtube_user"
}
func (y *YouTubeUserInfoWrapper) GetDisplayName() string     { return y.Name }
func (y *YouTubeUserInfoWrapper) GetProfileImageURL() string { return y.Picture }
func (y *YouTubeUserInfoWrapper) GetPlatform() Platform      { return PlatformYouTube }

// KickUserInfoWrapper wraps KickUserInfo to implement PlatformUserInfo
type KickUserInfoWrapper struct {
	ID          string
	Username    string
	DisplayName string
	ProfilePic  string
}

func (k *KickUserInfoWrapper) GetID() string              { return k.ID }
func (k *KickUserInfoWrapper) GetUsername() string        { return k.Username }
func (k *KickUserInfoWrapper) GetDisplayName() string     { return k.DisplayName }
func (k *KickUserInfoWrapper) GetProfileImageURL() string { return k.ProfilePic }
func (k *KickUserInfoWrapper) GetPlatform() Platform      { return PlatformKick }
