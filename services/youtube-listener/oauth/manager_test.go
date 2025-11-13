package oauth

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap"
	"golang.org/x/oauth2"
)

// MockTokenStore is a mock implementation of TokenStore
type MockTokenStore struct {
	mock.Mock
}

func (m *MockTokenStore) SaveToken(ctx context.Context, userID, channelID string, token *oauth2.Token) error {
	args := m.Called(ctx, userID, channelID, token)
	return args.Error(0)
}

func (m *MockTokenStore) GetToken(ctx context.Context, userID, channelID string) (*oauth2.Token, error) {
	args := m.Called(ctx, userID, channelID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*oauth2.Token), args.Error(1)
}

func (m *MockTokenStore) DeleteToken(ctx context.Context, userID, channelID string) error {
	args := m.Called(ctx, userID, channelID)
	return args.Error(0)
}

func TestNewManager(t *testing.T) {
	logger := zap.NewNop()
	store := &MockTokenStore{}

	manager := NewManager("client-id", "client-secret", "http://localhost/callback", store, logger)

	assert.NotNil(t, manager)
	assert.NotNil(t, manager.config)
	assert.Equal(t, "client-id", manager.config.ClientID)
	assert.Equal(t, "client-secret", manager.config.ClientSecret)
	assert.Equal(t, "http://localhost/callback", manager.config.RedirectURL)
	assert.Contains(t, manager.config.Scopes, YouTubeReadOnlyScope)
}

func TestGetAuthURL(t *testing.T) {
	logger := zap.NewNop()
	store := &MockTokenStore{}

	manager := NewManager("client-id", "client-secret", "http://localhost/callback", store, logger)

	authURL := manager.GetAuthURL("test-state")

	assert.NotEmpty(t, authURL)
	assert.Contains(t, authURL, "client-id")
	assert.Contains(t, authURL, "test-state")
	assert.Contains(t, authURL, "youtube.readonly")
}

func TestGetToken_ValidToken(t *testing.T) {
	logger := zap.NewNop()
	store := &MockTokenStore{}

	validToken := &oauth2.Token{
		AccessToken:  "valid-access-token",
		RefreshToken: "refresh-token",
		TokenType:    "Bearer",
		Expiry:       time.Now().Add(1 * time.Hour),
	}

	store.On("GetToken", mock.Anything, "user-123", "channel-456").Return(validToken, nil)

	manager := NewManager("client-id", "client-secret", "http://localhost/callback", store, logger)

	token, err := manager.GetToken(context.Background(), "user-123", "channel-456")

	assert.NoError(t, err)
	assert.NotNil(t, token)
	assert.Equal(t, "valid-access-token", token.AccessToken)
	store.AssertExpectations(t)
}

func TestGetToken_ExpiredToken_ShouldRefresh(t *testing.T) {
	logger := zap.NewNop()
	store := &MockTokenStore{}

	// Token that expires in 2 minutes (should trigger refresh since threshold is 5 minutes)
	expiredToken := &oauth2.Token{
		AccessToken:  "old-access-token",
		RefreshToken: "refresh-token",
		TokenType:    "Bearer",
		Expiry:       time.Now().Add(2 * time.Minute),
	}

	store.On("GetToken", mock.Anything, "user-123", "channel-456").Return(expiredToken, nil)

	// Note: In real test, we'd need to mock the OAuth2 token source
	// For now, this test documents the expected behavior

	manager := NewManager("client-id", "client-secret", "http://localhost/callback", store, logger)

	// This will fail in test because we can't mock the actual OAuth refresh
	// In integration tests, this would work with a test OAuth server
	_, err := manager.GetToken(context.Background(), "user-123", "channel-456")

	// We expect an error here since we don't have a real OAuth server
	assert.Error(t, err)
	store.AssertExpectations(t)
}

func TestSaveToken(t *testing.T) {
	logger := zap.NewNop()
	store := &MockTokenStore{}

	token := &oauth2.Token{
		AccessToken:  "access-token",
		RefreshToken: "refresh-token",
		TokenType:    "Bearer",
		Expiry:       time.Now().Add(1 * time.Hour),
	}

	store.On("SaveToken", mock.Anything, "user-123", "channel-456", token).Return(nil)

	manager := NewManager("client-id", "client-secret", "http://localhost/callback", store, logger)

	err := manager.SaveToken(context.Background(), "user-123", "channel-456", token)

	assert.NoError(t, err)
	store.AssertExpectations(t)
}
