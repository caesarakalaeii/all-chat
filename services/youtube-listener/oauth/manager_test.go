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

	manager := NewManager("client-id", "client-secret", "http://localhost/callback", false, store, logger)

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

	manager := NewManager("client-id", "client-secret", "http://localhost/callback", false, store, logger)

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

	manager := NewManager("client-id", "client-secret", "http://localhost/callback", false, store, logger)

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
	store.On("SaveToken", mock.Anything, "user-123", "channel-456", mock.AnythingOfType("*oauth2.Token")).Return(nil)

	// Note: In real test, we'd need to mock the OAuth2 token source
	// For now, this test documents the expected behavior

	manager := NewManager("client-id", "client-secret", "http://localhost/callback", false, store, logger)

	// Call should succeed because oauth2.TokenSource returns the cached token for tests
	refreshedToken, err := manager.GetToken(context.Background(), "user-123", "channel-456")
	assert.NoError(t, err)
	assert.NotNil(t, refreshedToken)
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

	manager := NewManager("client-id", "client-secret", "http://localhost/callback", false, store, logger)

	err := manager.SaveToken(context.Background(), "user-123", "channel-456", token)

	assert.NoError(t, err)
	store.AssertExpectations(t)
}
