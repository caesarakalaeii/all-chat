package services

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/caesar/all-chat/internal/auth-service/core/domain"
	"github.com/caesar/all-chat/internal/auth-service/core/ports"
	pkgauth "github.com/caesar/all-chat/pkg/auth"
	"github.com/google/uuid"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/twitch"
)

var (
	ErrInvalidCode    = errors.New("invalid authorization code")
	ErrUserNotFound   = errors.New("user not found")
	ErrInvalidToken   = errors.New("invalid token")
)

type authService struct {
	repo        ports.UserRepository
	oauthConfig *oauth2.Config
	jwtSecret   string
	httpClient  *http.Client
}

func NewAuthService(repo ports.UserRepository, clientID, clientSecret, redirectURL, jwtSecret string) ports.AuthService {
	return &authService{
		repo: repo,
		oauthConfig: &oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			RedirectURL:  redirectURL,
			Scopes:       []string{"user:read:email"},
			Endpoint:     twitch.Endpoint,
		},
		jwtSecret:  jwtSecret,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

func (s *authService) GetAuthURL(state string) string {
	return s.oauthConfig.AuthCodeURL(state, oauth2.AccessTypeOffline)
}

func (s *authService) ExchangeCode(ctx context.Context, code string) (*domain.User, *pkgauth.TokenPair, error) {
	// Exchange code for token
	token, err := s.oauthConfig.Exchange(ctx, code)
	if err != nil {
		return nil, nil, fmt.Errorf("%w: %v", ErrInvalidCode, err)
	}

	// Get user info from Twitch
	twitchUser, err := s.getTwitchUserInfo(ctx, token.AccessToken)
	if err != nil {
		return nil, nil, err
	}

	// Check if user exists
	user, err := s.repo.GetByTwitchID(ctx, twitchUser.ID)
	if err != nil {
		// Create new user
		user = &domain.User{
			ID:                   uuid.New().String(),
			TwitchID:             twitchUser.ID,
			Username:             twitchUser.Login,
			DisplayName:          twitchUser.DisplayName,
			AvatarURL:            twitchUser.ProfileImageURL,
			AccessTokenEncrypted: s.encryptToken(token.AccessToken),
			RefreshTokenEncrypted: s.encryptToken(token.RefreshToken),
			TokenExpiresAt:       token.Expiry,
			CreatedAt:            time.Now(),
			UpdatedAt:            time.Now(),
			LastLoginAt:          time.Now(),
		}

		if err := s.repo.Create(ctx, user); err != nil {
			return nil, nil, fmt.Errorf("failed to create user: %w", err)
		}
	} else {
		// Update existing user
		user.DisplayName = twitchUser.DisplayName
		user.AvatarURL = twitchUser.ProfileImageURL
		user.LastLoginAt = time.Now()
		user.AccessTokenEncrypted = s.encryptToken(token.AccessToken)
		user.RefreshTokenEncrypted = s.encryptToken(token.RefreshToken)
		user.TokenExpiresAt = token.Expiry

		if err := s.repo.Update(ctx, user); err != nil {
			return nil, nil, fmt.Errorf("failed to update user: %w", err)
		}
	}

	// Generate JWT tokens
	tokenPair, err := pkgauth.GenerateTokenPair(user.ID, user.TwitchID, user.Username, s.jwtSecret)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate tokens: %w", err)
	}

	return user, tokenPair, nil
}

func (s *authService) RefreshToken(ctx context.Context, refreshToken string) (*pkgauth.TokenPair, error) {
	// Validate refresh token
	claims, err := pkgauth.ValidateToken(refreshToken, s.jwtSecret)
	if err != nil {
		return nil, ErrInvalidToken
	}

	if claims.TokenType != "refresh" {
		return nil, ErrInvalidToken
	}

	// Get user
	user, err := s.repo.GetByID(ctx, claims.UserID)
	if err != nil {
		return nil, ErrUserNotFound
	}

	// Generate new token pair
	tokenPair, err := pkgauth.GenerateTokenPair(user.ID, user.TwitchID, user.Username, s.jwtSecret)
	if err != nil {
		return nil, fmt.Errorf("failed to generate tokens: %w", err)
	}

	return tokenPair, nil
}

func (s *authService) GetUserInfo(ctx context.Context, userID string) (*domain.User, error) {
	return s.repo.GetByID(ctx, userID)
}

func (s *authService) Logout(ctx context.Context, userID string) error {
	// In a real implementation, you might want to invalidate the session in Redis
	// For now, we'll just return nil as the client will delete the token
	return nil
}

func (s *authService) getTwitchUserInfo(ctx context.Context, accessToken string) (*domain.TwitchUser, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", "https://api.twitch.tv/helix/users", nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Client-Id", s.oauthConfig.ClientID)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("twitch API error: %s", string(body))
	}

	var result struct {
		Data []domain.TwitchUser `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if len(result.Data) == 0 {
		return nil, errors.New("no user data returned from Twitch")
	}

	return &result.Data[0], nil
}

func (s *authService) encryptToken(token string) string {
	// TODO: Implement proper encryption using AES-GCM or similar
	// For now, we'll just base64 encode (NOT SECURE FOR PRODUCTION)
	return base64.StdEncoding.EncodeToString([]byte(token))
}

func (s *authService) decryptToken(encryptedToken string) (string, error) {
	// TODO: Implement proper decryption
	decoded, err := base64.StdEncoding.DecodeString(encryptedToken)
	if err != nil {
		return "", err
	}
	return string(decoded), nil
}

func GenerateStateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}
