package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var (
	ErrInvalidToken = errors.New("invalid token")
	ErrExpiredToken = errors.New("token expired")
)

// Claims represents the JWT claims for All-Chat
type Claims struct {
	UserID            string   `json:"sub"`
	TwitchID          string   `json:"twitch_id"`
	Username          string   `json:"username"`
	Roles             []string `json:"roles"`
	ImpersonatedBy    string   `json:"impersonated_by,omitempty"`    // Admin UserID who is impersonating
	ImpersonatedUser  string   `json:"impersonated_user,omitempty"`  // Target user being impersonated
	jwt.RegisteredClaims
}

// IsImpersonating returns true if this token represents an admin impersonating another user
func (c *Claims) IsImpersonating() bool {
	return c.ImpersonatedBy != "" && c.ImpersonatedUser != ""
}

// GetEffectiveUserID returns the user ID to use for authorization
// If impersonating, returns the impersonated user ID, otherwise returns the actual user ID
func (c *Claims) GetEffectiveUserID() string {
	if c.IsImpersonating() {
		return c.ImpersonatedUser
	}
	return c.UserID
}

// GetActualUserID returns the real user ID (admin if impersonating)
func (c *Claims) GetActualUserID() string {
	return c.UserID
}

// ViewerClaims represents JWT claims for viewer authentication
type ViewerClaims struct {
	SessionID      string `json:"session_id"`
	Platform       string `json:"platform"`
	PlatformUserID string `json:"platform_user_id"`
	Username       string `json:"username"`
	IsViewer       bool   `json:"is_viewer"`
	jwt.RegisteredClaims
}

// ServiceClaims represents JWT claims used for service-to-service auth
type ServiceClaims struct {
	ServiceName string   `json:"service_name"`
	Permissions []string `json:"permissions,omitempty"`
	jwt.RegisteredClaims
}

// GenerateJWT generates a new JWT token for the given user
func GenerateJWT(userID, twitchID, username, secret string, isAdmin bool) (string, error) {
	roles := []string{"user"}
	if isAdmin {
		roles = append(roles, "admin")
	}

	claims := Claims{
		UserID:   userID,
		TwitchID: twitchID,
		Username: username,
		Roles:    roles,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			Issuer:    "all-chat",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

// GenerateToken generates a JWT token with custom expiry duration
// This is a simpler version for services that don't need all user details
func GenerateToken(userID, username, secret string, expiry time.Duration, isAdmin bool) (string, error) {
	roles := []string{"user"}
	if isAdmin {
		roles = append(roles, "admin")
	}

	claims := Claims{
		UserID:   userID,
		Username: username,
		Roles:    roles,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(expiry)),
			Issuer:    "all-chat",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

// GenerateImpersonationJWT generates a JWT for an admin to impersonate another user
// The admin's identity is preserved in ImpersonatedBy, while UserID becomes the target user
func GenerateImpersonationJWT(adminUserID, adminUsername, targetUserID, targetUsername, targetTwitchID, secret string) (string, error) {
	// Impersonation tokens always have admin role (from the real admin)
	roles := []string{"user", "admin"}

	claims := Claims{
		UserID:           targetUserID,  // Use target user's ID as the primary ID
		TwitchID:         targetTwitchID,
		Username:         targetUsername,
		Roles:            roles,
		ImpersonatedBy:   adminUserID,
		ImpersonatedUser: targetUserID,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(2 * time.Hour)), // Shorter expiry for security
			Issuer:    "all-chat-admin",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

// ValidateJWT validates a JWT token and returns the claims
func ValidateJWT(tokenString, secret string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		// Verify signing method
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(secret), nil
	})

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrExpiredToken
		}
		return nil, ErrInvalidToken
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}

	return nil, ErrInvalidToken
}

// ValidateViewerJWT validates a viewer JWT token and returns the claims
func ValidateViewerJWT(tokenString, secret string) (*ViewerClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &ViewerClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(secret), nil
	})

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrExpiredToken
		}
		return nil, ErrInvalidToken
	}

	if claims, ok := token.Claims.(*ViewerClaims); ok && token.Valid {
		return claims, nil
	}

	return nil, ErrInvalidToken
}

// GenerateServiceJWT creates a JWT representing a specific internal service
func GenerateServiceJWT(serviceName, secret string, expiry time.Duration) (string, error) {
	claims := ServiceClaims{
		ServiceName: serviceName,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   serviceName,
			Issuer:    "all-chat-services",
			Audience:  []string{"internal"},
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(expiry)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

// ValidateServiceJWT validates service JWTs and returns the parsed claims
func ValidateServiceJWT(tokenString, secret string) (*ServiceClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &ServiceClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(secret), nil
	})

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrExpiredToken
		}
		return nil, ErrInvalidToken
	}

	if claims, ok := token.Claims.(*ServiceClaims); ok && token.Valid {
		return claims, nil
	}

	return nil, ErrInvalidToken
}
