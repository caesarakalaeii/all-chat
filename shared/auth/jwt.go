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

package auth

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var (
	ErrInvalidToken          = errors.New("invalid token")
	ErrExpiredToken          = errors.New("token expired")
	ErrNoVersionedJWTSecrets = errors.New("no versioned JWT secrets configured: set <PREFIX>_V1")
	ErrUnknownKidNoLegacy    = errors.New("unknown kid and no legacy fallback secret")
	ErrSecretTooShort        = errors.New("JWT secret must be at least 32 bytes")
	ErrInvalidIssuer         = errors.New("invalid token issuer")
)

// Allowed user-token issuers. Regular user tokens use "all-chat"; impersonation
// tokens use "all-chat-admin" (both validated via ValidateJWTWithKeyChain, audit L3).
var allowedUserIssuers = map[string]bool{
	"all-chat":       true,
	"all-chat-admin": true,
}

// minSecretBytes is the minimum acceptable length for a JWT signing secret (audit M3).
const minSecretBytes = 32

// Claims represents the JWT claims for All-Chat
type Claims struct {
	UserID           string   `json:"sub"`
	TwitchID         string   `json:"twitch_id"`
	Username         string   `json:"username"`
	Roles            []string `json:"roles"`
	ImpersonatedBy   string   `json:"impersonated_by,omitempty"`   // Admin UserID who is impersonating
	ImpersonatedUser string   `json:"impersonated_user,omitempty"` // Target user being impersonated
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
	ViewerID       string `json:"viewer_id"` // durable viewer UUID (empty for old tokens)
	SessionID      string `json:"session_id"`
	Platform       string `json:"platform"`
	PlatformUserID string `json:"platform_user_id"`
	Username       string `json:"username"`
	DisplayName    string `json:"display_name"` // for extension popup display
	AvatarURL      string `json:"avatar_url"`   // for extension popup display
	IsViewer       bool   `json:"is_viewer"`
	IsPremium      bool   `json:"is_premium"` // true if viewer OR linked streamer account has premium
	IsAdmin        bool   `json:"is_admin"`   // true if linked streamer account has admin role
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
		UserID:           targetUserID, // Use target user's ID as the primary ID
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

// ─────────────────────────────────────────────────────────────────────────────
// KeyChain — multi-key JWT validation (D-07, D-08, D-10, D-12)
// ─────────────────────────────────────────────────────────────────────────────

// KeyChain holds multiple HS256 secrets indexed by string kid ("v1", "v2", ...).
// The legacy field holds the kid-less <PREFIX> env var value for backwards-compat
// with tokens issued before kid headers were introduced (D-08).
//
// Two independent chains are used in practice:
//   - NewKeyChainFromEnv("JWT_SECRET")         — User + Viewer JWTs
//   - NewKeyChainFromEnv("SERVICE_JWT_SECRET") — Service JWTs (D-10)
type KeyChain struct {
	byKid     map[string][]byte
	legacy    []byte
	latestKid string // highest "v<n>" registered; used by callers for issuance
}

// NewKeyChainFromEnv reads <prefix>_V1, <prefix>_V2, ... in sequence until an env
// var is missing or empty. <prefix> (no version suffix) is loaded as the legacy
// fallback for tokens that have no kid header.
//
// prefix examples: "JWT_SECRET" for user/viewer tokens, "SERVICE_JWT_SECRET" for
// service-to-service tokens. The prefix determines which env var namespace is
// scanned, giving the two chains full isolation (D-10).
//
// Returns ErrNoVersionedJWTSecrets if no <prefix>_V1 is set; the legacy var alone
// is not sufficient because new code MUST issue with kid headers per D-07.
func NewKeyChainFromEnv(prefix string) (*KeyChain, error) {
	byKid := make(map[string][]byte)
	var latestKid string
	for n := 1; ; n++ {
		envName := prefix + "_V" + strconv.Itoa(n)
		v := os.Getenv(envName)
		if v == "" {
			break
		}
		if len(v) < minSecretBytes {
			return nil, fmt.Errorf("%w: %s is %d bytes, need at least %d",
				ErrSecretTooShort, envName, len(v), minSecretBytes)
		}
		kid := "v" + strconv.Itoa(n)
		byKid[kid] = []byte(v)
		latestKid = kid
	}
	if len(byKid) == 0 {
		return nil, fmt.Errorf("%w (looked for %s_V1)", ErrNoVersionedJWTSecrets, prefix)
	}
	var legacy []byte
	if v := os.Getenv(prefix); v != "" {
		if len(v) < minSecretBytes {
			return nil, fmt.Errorf("%w: %s is %d bytes, need at least %d",
				ErrSecretTooShort, prefix, len(v), minSecretBytes)
		}
		legacy = []byte(v)
	}
	return &KeyChain{byKid: byKid, legacy: legacy, latestKid: latestKid}, nil
}

// NewKeyChain constructs a KeyChain from explicit arguments. Intended for tests
// and for callers that already hold secret bytes (e.g. sweeper, bootstrapper).
func NewKeyChain(byKid map[string][]byte, legacy []byte, latestKid string) *KeyChain {
	return &KeyChain{byKid: byKid, legacy: legacy, latestKid: latestKid}
}

// LatestKid returns the highest "v<n>" registered in this chain.
// Issuers should sign new tokens with this kid.
func (kc *KeyChain) LatestKid() string { return kc.latestKid }

// LatestSecret returns the secret bytes for LatestKid.
// Convenience for issuer call sites that need the raw bytes for SignedString.
func (kc *KeyChain) LatestSecret() []byte { return kc.byKid[kc.latestKid] }

// KeyFunc is a jwt.Keyfunc that selects the correct HS256 secret based on the
// token's kid header.
//
// Dispatch logic:
//  1. Reject any non-HMAC signing method (alg-confusion defence, D-12).
//  2. If kid header is absent or empty → use legacy secret (backwards compat).
//  3. If kid is present and known → use that version's secret.
//  4. If kid is present but unknown → fall back to legacy (future-version tolerance).
//  5. If both lookup paths miss AND legacy is nil → return ErrUnknownKidNoLegacy.
func (kc *KeyChain) KeyFunc(token *jwt.Token) (interface{}, error) {
	if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
		return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
	}
	kid, hasKid := token.Header["kid"].(string)
	if !hasKid || kid == "" {
		if kc.legacy == nil {
			return nil, errors.New("token has no kid and no legacy fallback configured")
		}
		return kc.legacy, nil
	}
	if key, ok := kc.byKid[kid]; ok {
		return key, nil
	}
	if kc.legacy == nil {
		return nil, fmt.Errorf("%w: kid=%q", ErrUnknownKidNoLegacy, kid)
	}
	return kc.legacy, nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Issuance helpers — *WithKid variants (D-07)
// ─────────────────────────────────────────────────────────────────────────────

// GenerateJWTWithKid is identical to GenerateJWT but sets token.Header["kid"]
// before signing, enabling multi-key validation on the receiving end.
func GenerateJWTWithKid(kid, userID, twitchID, username, secret string, isAdmin bool) (string, error) {
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
	token.Header["kid"] = kid
	return token.SignedString([]byte(secret))
}

// GenerateTokenWithKid is identical to GenerateToken but sets token.Header["kid"].
func GenerateTokenWithKid(kid, userID, username, secret string, expiry time.Duration, isAdmin bool) (string, error) {
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
	token.Header["kid"] = kid
	return token.SignedString([]byte(secret))
}

// GenerateImpersonationJWTWithKid is identical to GenerateImpersonationJWT but
// sets token.Header["kid"] before signing.
func GenerateImpersonationJWTWithKid(kid, adminUserID, adminUsername, targetUserID, targetUsername, targetTwitchID, secret string) (string, error) {
	roles := []string{"user", "admin"}
	claims := Claims{
		UserID:           targetUserID,
		TwitchID:         targetTwitchID,
		Username:         targetUsername,
		Roles:            roles,
		ImpersonatedBy:   adminUserID,
		ImpersonatedUser: targetUserID,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(2 * time.Hour)),
			Issuer:    "all-chat-admin",
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	token.Header["kid"] = kid
	return token.SignedString([]byte(secret))
}

// GenerateServiceJWTWithKid is identical to GenerateServiceJWT but sets
// token.Header["kid"] before signing. Use this for new service tokens; pass
// the kid from ServiceKeyChain.LatestKid() for automatic rotation support.
func GenerateServiceJWTWithKid(kid, serviceName, secret string, expiry time.Duration) (string, error) {
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
	token.Header["kid"] = kid
	return token.SignedString([]byte(secret))
}

// GenerateViewerJWTWithKid issues a viewer JWT with a kid header. The caller
// provides a fully constructed ViewerClaims value (matching the inline pattern
// in services/auth-service/handlers/viewer_auth.go) so that all viewer-specific
// fields are set before signing.
func GenerateViewerJWTWithKid(kid string, claims ViewerClaims, secret string) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	token.Header["kid"] = kid
	return token.SignedString([]byte(secret))
}

// ─────────────────────────────────────────────────────────────────────────────
// Validation helpers — *WithKeyChain variants (D-08)
// ─────────────────────────────────────────────────────────────────────────────

// ValidateJWTWithKeyChain validates a user JWT using multi-key dispatch via kc.KeyFunc.
// Preserves identical error semantics to ValidateJWT: ErrExpiredToken or ErrInvalidToken.
func ValidateJWTWithKeyChain(tokenString string, kc *KeyChain) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, kc.KeyFunc)
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrExpiredToken
		}
		return nil, ErrInvalidToken
	}
	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, ErrInvalidToken
	}
	// Issuer validation (audit L3): accept "all-chat" (regular) and
	// "all-chat-admin" (impersonation); reject empty/unknown issuers.
	if !allowedUserIssuers[claims.Issuer] {
		return nil, fmt.Errorf("%w: %q", ErrInvalidIssuer, claims.Issuer)
	}
	return claims, nil
}

// ValidateViewerJWTWithKeyChain validates a viewer JWT using multi-key dispatch via kc.KeyFunc.
func ValidateViewerJWTWithKeyChain(tokenString string, kc *KeyChain) (*ViewerClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &ViewerClaims{}, kc.KeyFunc, jwt.WithIssuer("all-chat"))
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

// ValidateServiceJWTWithKeyChain validates a service JWT using multi-key dispatch via kc.KeyFunc.
// Pass the service-chain KeyChain (built from "SERVICE_JWT_SECRET" prefix) to enforce
// cross-chain isolation (D-10): a user-chain token will fail here because the HMAC
// was computed with a different secret.
func ValidateServiceJWTWithKeyChain(tokenString string, kc *KeyChain) (*ServiceClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &ServiceClaims{}, kc.KeyFunc, jwt.WithIssuer("all-chat-services"))
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
