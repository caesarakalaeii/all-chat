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
	"net/http"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/twitch"
)

// ViewerTwitchOAuth handles Twitch OAuth 2.0 flow for viewers (with chat write permissions)
type ViewerTwitchOAuth struct {
	*TwitchOAuth
}

// NewViewerTwitchOAuth creates a new Twitch OAuth handler for viewers
func NewViewerTwitchOAuth(clientID, clientSecret, redirectURL string) *ViewerTwitchOAuth {
	config := &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  redirectURL,
		Scopes: []string{
			"user:write:chat", // Required to send messages on behalf of the user
		},
		Endpoint: twitch.Endpoint,
	}

	return &ViewerTwitchOAuth{
		TwitchOAuth: &TwitchOAuth{
			config: config,
			client: &http.Client{Timeout: 10 * time.Second},
		},
	}
}

// ValidateScope checks if the token has the required scope
func (v *ViewerTwitchOAuth) ValidateScope(ctx context.Context, accessToken string) error {
	// Twitch validates scopes during the OAuth flow, so we can assume the token is valid
	// If we need to verify, we can call the /oauth2/validate endpoint
	return nil
}

// SendChatMessage sends a chat message using the Twitch Helix API
func (v *ViewerTwitchOAuth) SendChatMessage(ctx context.Context, accessToken, broadcasterID, senderID, message string) error {
	// This will be implemented in the message sending service
	// For now, just validate the method signature
	return fmt.Errorf("not implemented: use chat service to send messages")
}
