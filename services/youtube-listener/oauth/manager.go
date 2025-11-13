package oauth

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	"google.golang.org/api/youtube/v3"
)

const (
	// YouTubeReadOnlyScope allows reading YouTube data
	YouTubeReadOnlyScope = "https://www.googleapis.com/auth/youtube.readonly"
)

// Manager handles YouTube OAuth 2.0 authentication
type Manager struct {
	config *oauth2.Config
	store  TokenStore
	logger *zap.Logger
}

// NewManager creates a new OAuth manager
func NewManager(clientID, clientSecret, redirectURL string, store TokenStore, logger *zap.Logger) *Manager {
	config := &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  redirectURL,
		Scopes:       []string{YouTubeReadOnlyScope},
		Endpoint:     google.Endpoint,
	}

	return &Manager{
		config: config,
		store:  store,
		logger: logger,
	}
}

// GetAuthURL generates an OAuth authorization URL for a user
func (m *Manager) GetAuthURL(state string) string {
	return m.config.AuthCodeURL(state, oauth2.AccessTypeOffline, oauth2.ApprovalForce)
}

// ExchangeCode exchanges an authorization code for tokens
func (m *Manager) ExchangeCode(ctx context.Context, code string) (*oauth2.Token, error) {
	token, err := m.config.Exchange(ctx, code)
	if err != nil {
		m.logger.Error("Failed to exchange authorization code",
			zap.Error(err),
		)
		return nil, fmt.Errorf("failed to exchange code: %w", err)
	}

	m.logger.Info("Successfully exchanged authorization code for tokens",
		zap.Time("expiry", token.Expiry),
	)

	return token, nil
}

// GetToken retrieves a valid token for a channel, refreshing if necessary
func (m *Manager) GetToken(ctx context.Context, userID, channelID string) (*oauth2.Token, error) {
	// Get token from store
	token, err := m.store.GetToken(ctx, userID, channelID)
	if err != nil {
		return nil, fmt.Errorf("failed to get token from store: %w", err)
	}

	// Check if token needs refresh
	if token.Expiry.Before(time.Now().Add(5 * time.Minute)) {
		m.logger.Info("Token expiring soon, refreshing",
			zap.String("channel_id", channelID),
			zap.Time("expiry", token.Expiry),
		)

		// Refresh token
		tokenSource := m.config.TokenSource(ctx, token)
		newToken, err := tokenSource.Token()
		if err != nil {
			m.logger.Error("Failed to refresh token",
				zap.String("channel_id", channelID),
				zap.Error(err),
			)
			return nil, fmt.Errorf("failed to refresh token: %w", err)
		}

		// Save refreshed token
		if err := m.store.SaveToken(ctx, userID, channelID, newToken); err != nil {
			m.logger.Error("Failed to save refreshed token",
				zap.String("channel_id", channelID),
				zap.Error(err),
			)
			// Continue with token even if save fails
		}

		m.logger.Info("Successfully refreshed token",
			zap.String("channel_id", channelID),
			zap.Time("new_expiry", newToken.Expiry),
		)

		return newToken, nil
	}

	return token, nil
}

// CreateYouTubeService creates an authenticated YouTube API service
func (m *Manager) CreateYouTubeService(ctx context.Context, userID, channelID string) (*youtube.Service, error) {
	token, err := m.GetToken(ctx, userID, channelID)
	if err != nil {
		return nil, fmt.Errorf("failed to get token: %w", err)
	}

	client := m.config.Client(ctx, token)

	service, err := youtube.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		m.logger.Error("Failed to create YouTube service",
			zap.String("channel_id", channelID),
			zap.Error(err),
		)
		return nil, fmt.Errorf("failed to create YouTube service: %w", err)
	}

	return service, nil
}

// SaveToken saves an OAuth token to the store
func (m *Manager) SaveToken(ctx context.Context, userID, channelID string, token *oauth2.Token) error {
	return m.store.SaveToken(ctx, userID, channelID, token)
}
