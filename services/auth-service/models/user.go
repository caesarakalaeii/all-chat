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

package models

import "time"

// User represents a user in the system
type User struct {
	ID               string     `json:"id"`
	TwitchID         *string    `json:"twitch_id,omitempty"` // Nullable for other platform users
	GoogleID         *string    `json:"google_id,omitempty"` // Nullable for non-YouTube users
	KickID           *string    `json:"kick_id,omitempty"`   // Nullable for non-Kick users
	AuthProvider     string     `json:"auth_provider"`       // "twitch", "youtube", or "kick"
	Username         string     `json:"username"`
	DisplayName      string     `json:"display_name"`
	ProfileImageURL  string     `json:"profile_image_url"`
	IsAdmin          bool       `json:"is_admin"`                     // Admin role for access control
	IsPremium        bool       `json:"is_premium"`                   // Premium feature access (derived)
	IsBetaTester     bool       `json:"is_beta_tester"`               // Beta-tester role (ADR-0020): all premium + early-access
	IsAmbassador     bool       `json:"is_ambassador"`                // Ambassador role (ADR-0041): all premium + early-access + public homepage showcase
	PremiumExpiresAt *time.Time `json:"premium_expires_at,omitempty"` // ADR-0027: time-limited admin premium override deadline (NULL = permanent)
	// Ambassador showcase card metadata (ADR-0041), populated ONLY by the admin
	// listing (GetAllUsers LEFT JOINs ambassador_showcase). omitempty keeps them off
	// the /auth/me response, which does not populate them.
	AmbassadorTagline   *string    `json:"ambassador_tagline,omitempty"`
	AmbassadorSortOrder int        `json:"ambassador_sort_order,omitempty"`
	IsBanned            bool       `json:"is_banned"`               // Ban status
	BannedAt            *time.Time `json:"banned_at,omitempty"`     // When user was banned
	BannedReason        *string    `json:"banned_reason,omitempty"` // Reason for ban
	BannedBy            *string    `json:"banned_by,omitempty"`     // Admin who banned (user ID)
	AccessToken         string     `json:"-"`                       // Never expose in JSON
	RefreshToken        string     `json:"-"`                       // Never expose in JSON
	TokenExpiresAt      time.Time  `json:"-"`
	GrantedScopes       []string   `json:"-"` // OAuth scopes granted at consent (TEXT[] in DB); gates EventSub chat reading
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
	// NULL = first-run setup guide is shown; deliberately no omitempty so the
	// frontend receives an explicit null for users who have not onboarded.
	OnboardingCompletedAt *time.Time `json:"onboarding_completed_at"`
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
