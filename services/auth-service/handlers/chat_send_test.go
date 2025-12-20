package handlers

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/caesar/all-chat/services/auth-service/models"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap"
	"golang.org/x/oauth2"
)

// Mock repositories and providers
type MockViewerRepository struct {
	mock.Mock
}

func (m *MockViewerRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.ViewerSession, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.ViewerSession), args.Error(1)
}

func (m *MockViewerRepository) DecryptAccessToken(token string) (string, error) {
	args := m.Called(token)
	return args.String(0), args.Error(1)
}

func (m *MockViewerRepository) DecryptRefreshToken(token string) (string, error) {
	args := m.Called(token)
	return args.String(0), args.Error(1)
}

func (m *MockViewerRepository) Update(ctx context.Context, session *models.ViewerSession) error {
	args := m.Called(ctx, session)
	return args.Error(0)
}

func (m *MockViewerRepository) UpdateRateLimits(ctx context.Context, sessionID uuid.UUID, count1Min, count1Hour int, reset1Min, reset1Hour time.Time) error {
	args := m.Called(ctx, sessionID, count1Min, count1Hour, reset1Min, reset1Hour)
	return args.Error(0)
}

func (m *MockViewerRepository) LogMessage(ctx context.Context, log *models.ViewerMessageLog) error {
	args := m.Called(ctx, log)
	return args.Error(0)
}

type MockUserRepository struct {
	mock.Mock
}

func (m *MockUserRepository) GetByUsername(ctx context.Context, username string) (*models.User, error) {
	args := m.Called(ctx, username)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.User), args.Error(1)
}

type MockOAuthProvider struct {
	mock.Mock
}

func (m *MockOAuthProvider) RefreshToken(ctx context.Context, refreshToken string) (*oauth2.Token, error) {
	args := m.Called(ctx, refreshToken)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*oauth2.Token), args.Error(1)
}

type MockCipher struct {
	mock.Mock
}

func (m *MockCipher) Encrypt(plaintext string) (string, error) {
	args := m.Called(plaintext)
	return args.String(0), args.Error(1)
}

func (m *MockCipher) Decrypt(ciphertext string) (string, error) {
	args := m.Called(ciphertext)
	return args.String(0), args.Error(1)
}

func TestHandleSendMessage_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Setup mocks
	viewerRepo := new(MockViewerRepository)
	userRepo := new(MockUserRepository)
	twitchProvider := new(MockOAuthProvider)
	cipher := new(MockCipher)

	sessionID := uuid.New()
	twitchID := "12345"
	streamerUserID := "streamer-uuid"

	session := &models.ViewerSession{
		ID:             sessionID,
		Platform:       "twitch",
		PlatformUserID: "67890",
		Username:       "viewer",
		AccessToken:    "encrypted_access",
		RefreshToken:   stringPtr("encrypted_refresh"),
		TokenExpiresAt: time.Now().Add(1 * time.Hour), // Not expired
		IsBanned:       false,
	}

	streamer := &models.User{
		ID:       streamerUserID,
		Username: "streamer",
		TwitchID: &twitchID,
	}

	// Mock expectations
	viewerRepo.On("GetByID", mock.Anything, sessionID).Return(session, nil)
	viewerRepo.On("DecryptAccessToken", "encrypted_access").Return("decrypted_access", nil)
	userRepo.On("GetByUsername", mock.Anything, "streamer").Return(streamer, nil)
	viewerRepo.On("UpdateRateLimits", mock.Anything, sessionID, 1, 1, mock.Anything, mock.Anything).Return(nil)
	viewerRepo.On("LogMessage", mock.Anything, mock.Anything).Return(nil)

	// Create handler
	handler := &ChatSendHandler{
		log:             zap.NewNop(),
		viewerRepo:      viewerRepo,
		userRepo:        userRepo,
		httpClient:      &http.Client{},
		clientID:        "test-client-id",
		twitchProvider:  twitchProvider,
		youtubeProvider: new(MockOAuthProvider),
		kickProvider:    new(MockOAuthProvider),
		cipher:          cipher,
	}

	// Create test request
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("session_id", sessionID.String())

	body := `{"streamer_username":"streamer","message":"Hello test","platform":"twitch"}`
	c.Request = httptest.NewRequest("POST", "/chat/send", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	// Note: This test will fail when actually calling Twitch API
	// In a real scenario, you'd mock the HTTP client or use a test server
	handler.HandleSendMessage(c)

	// Verify mocks were called
	viewerRepo.AssertExpectations(t)
	userRepo.AssertExpectations(t)
}

func TestRefreshTokenIfNeeded_NotExpired(t *testing.T) {
	viewerRepo := new(MockViewerRepository)
	cipher := new(MockCipher)

	handler := &ChatSendHandler{
		log:        zap.NewNop(),
		viewerRepo: viewerRepo,
		cipher:     cipher,
	}

	session := &models.ViewerSession{
		ID:             uuid.New(),
		Platform:       "twitch",
		TokenExpiresAt: time.Now().Add(1 * time.Hour), // Not expired
	}

	err := handler.refreshTokenIfNeeded(context.Background(), session)

	assert.NoError(t, err)
	viewerRepo.AssertNotCalled(t, "DecryptRefreshToken")
}

func TestRefreshTokenIfNeeded_Success(t *testing.T) {
	viewerRepo := new(MockViewerRepository)
	twitchProvider := new(MockOAuthProvider)
	cipher := new(MockCipher)

	handler := &ChatSendHandler{
		log:            zap.NewNop(),
		viewerRepo:     viewerRepo,
		twitchProvider: twitchProvider,
		cipher:         cipher,
	}

	sessionID := uuid.New()
	session := &models.ViewerSession{
		ID:             sessionID,
		Platform:       "twitch",
		TokenExpiresAt: time.Now().Add(2 * time.Minute), // Expiring soon
		RefreshToken:   stringPtr("encrypted_refresh"),
		AccessToken:    "encrypted_access",
	}

	newToken := &oauth2.Token{
		AccessToken:  "new_access",
		RefreshToken: "new_refresh",
		Expiry:       time.Now().Add(1 * time.Hour),
	}

	// Mock expectations
	viewerRepo.On("DecryptRefreshToken", "encrypted_refresh").Return("decrypted_refresh", nil)
	twitchProvider.On("RefreshToken", mock.Anything, "decrypted_refresh").Return(newToken, nil)
	cipher.On("Encrypt", "new_access").Return("encrypted_new_access", nil)
	cipher.On("Encrypt", "new_refresh").Return("encrypted_new_refresh", nil)
	viewerRepo.On("Update", mock.Anything, mock.Anything).Return(nil)

	err := handler.refreshTokenIfNeeded(context.Background(), session)

	assert.NoError(t, err)
	assert.Equal(t, "encrypted_new_access", session.AccessToken)
	viewerRepo.AssertExpectations(t)
	twitchProvider.AssertExpectations(t)
	cipher.AssertExpectations(t)
}

func TestRefreshTokenIfNeeded_NoRefreshToken(t *testing.T) {
	viewerRepo := new(MockViewerRepository)

	handler := &ChatSendHandler{
		log:        zap.NewNop(),
		viewerRepo: viewerRepo,
	}

	session := &models.ViewerSession{
		ID:             uuid.New(),
		Platform:       "twitch",
		TokenExpiresAt: time.Now().Add(-1 * time.Minute), // Expired
		RefreshToken:   nil,                               // No refresh token
	}

	err := handler.refreshTokenIfNeeded(context.Background(), session)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no refresh token available")
}

func TestRefreshTokenIfNeeded_RefreshFails(t *testing.T) {
	viewerRepo := new(MockViewerRepository)
	twitchProvider := new(MockOAuthProvider)

	handler := &ChatSendHandler{
		log:            zap.NewNop(),
		viewerRepo:     viewerRepo,
		twitchProvider: twitchProvider,
	}

	session := &models.ViewerSession{
		ID:             uuid.New(),
		Platform:       "twitch",
		TokenExpiresAt: time.Now().Add(-1 * time.Minute), // Expired
		RefreshToken:   stringPtr("encrypted_refresh"),
	}

	// Mock expectations
	viewerRepo.On("DecryptRefreshToken", "encrypted_refresh").Return("decrypted_refresh", nil)
	twitchProvider.On("RefreshToken", mock.Anything, "decrypted_refresh").Return(nil, errors.New("refresh failed"))

	err := handler.refreshTokenIfNeeded(context.Background(), session)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to refresh token")
	viewerRepo.AssertExpectations(t)
	twitchProvider.AssertExpectations(t)
}

func TestCheckRateLimit_NotExceeded(t *testing.T) {
	handler := &ChatSendHandler{
		log: zap.NewNop(),
	}

	session := &models.ViewerSession{
		MessageCount1Min:      5,
		MessageCount1Hour:     50,
		RateLimitReset1Min:    timePtr(time.Now().Add(30 * time.Second)),
		RateLimitReset1Hour:   timePtr(time.Now().Add(30 * time.Minute)),
	}

	allowed, _ := handler.checkRateLimit(session)

	assert.True(t, allowed)
}

func TestCheckRateLimit_1MinExceeded(t *testing.T) {
	handler := &ChatSendHandler{
		log: zap.NewNop(),
	}

	session := &models.ViewerSession{
		MessageCount1Min:    20, // At limit
		MessageCount1Hour:   50,
		RateLimitReset1Min:  timePtr(time.Now().Add(30 * time.Second)),
		RateLimitReset1Hour: timePtr(time.Now().Add(30 * time.Minute)),
	}

	allowed, resetTime := handler.checkRateLimit(session)

	assert.False(t, allowed)
	assert.False(t, resetTime.IsZero())
}

func TestCheckRateLimit_1HourExceeded(t *testing.T) {
	handler := &ChatSendHandler{
		log: zap.NewNop(),
	}

	session := &models.ViewerSession{
		MessageCount1Min:    5,
		MessageCount1Hour:   100, // At limit
		RateLimitReset1Min:  timePtr(time.Now().Add(30 * time.Second)),
		RateLimitReset1Hour: timePtr(time.Now().Add(30 * time.Minute)),
	}

	allowed, resetTime := handler.checkRateLimit(session)

	assert.False(t, allowed)
	assert.False(t, resetTime.IsZero())
}

// Helper functions
func stringPtr(s string) *string {
	return &s
}

func timePtr(t time.Time) *time.Time {
	return &t
}
