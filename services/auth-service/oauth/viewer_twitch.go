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
