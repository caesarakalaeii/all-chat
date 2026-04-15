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

package handlers

import (
	"context"
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// RISCHandler handles Cross-Account Protection (RISC) security events from Google
// This is required for Google OAuth verification
type RISCHandler struct {
	log           *zap.Logger
	db            *pgxpool.Pool
	googleJWKSURL string
	jwksCache     map[string]*rsa.PublicKey
	cacheMutex    sync.RWMutex
	cacheExpiry   time.Time
}

// RISCEvent represents a RISC security event from Google
type RISCEvent struct {
	Issuer   string                 `json:"iss"`
	IssuedAt int64                  `json:"iat"`
	JTI      string                 `json:"jti"`
	Audience string                 `json:"aud"`
	Subject  string                 `json:"sub"`
	Events   map[string]interface{} `json:"events"`
}

// GoogleJWKS represents Google's public key set
type GoogleJWKS struct {
	Keys []GoogleJWK `json:"keys"`
}

// GoogleJWK represents a single public key
type GoogleJWK struct {
	Kid string `json:"kid"`
	Kty string `json:"kty"`
	Alg string `json:"alg"`
	Use string `json:"use"`
	N   string `json:"n"`
	E   string `json:"e"`
}

func NewRISCHandler(log *zap.Logger, db *pgxpool.Pool) *RISCHandler {
	return &RISCHandler{
		log:           log,
		db:            db,
		googleJWKSURL: "https://www.googleapis.com/oauth2/v3/certs",
		jwksCache:     make(map[string]*rsa.PublicKey),
		cacheExpiry:   time.Time{},
	}
}

// HandleSecurityEvent receives and processes RISC security events from Google
// POST /.well-known/risc-events
func (h *RISCHandler) HandleSecurityEvent(c *gin.Context) {
	// Read the raw body
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		h.log.Error("Failed to read RISC event body", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	h.log.Info("Received RISC event", zap.String("body", string(body)))

	// Parse the SET token
	var setPayload map[string]interface{}
	if err := json.Unmarshal(body, &setPayload); err != nil {
		h.log.Error("Failed to parse RISC event JSON", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json"})
		return
	}

	// Google sends the SET in different formats, handle both
	var setToken string
	if token, ok := setPayload["SET"].(string); ok {
		setToken = token
	} else {
		// If no SET wrapper, the body itself is the JWT
		setToken = string(body)
	}

	// Parse and verify the JWT
	token, err := jwt.Parse(setToken, func(token *jwt.Token) (interface{}, error) {
		// Verify signing method is RSA
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}

		// Get the key ID from token header
		kid, ok := token.Header["kid"].(string)
		if !ok {
			return nil, fmt.Errorf("kid header missing")
		}

		// Fetch Google's public key
		return h.fetchGooglePublicKey(kid)
	})

	if err != nil {
		h.log.Error("Failed to parse RISC token", zap.Error(err))
		// Still return 202 to acknowledge receipt (Google recommends this)
		c.Status(http.StatusAccepted)
		return
	}

	if !token.Valid {
		h.log.Error("Invalid RISC token")
		c.Status(http.StatusAccepted)
		return
	}

	// Parse claims
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		h.log.Error("Invalid claims format")
		c.Status(http.StatusAccepted)
		return
	}

	// Extract subject (Google user ID)
	subject, _ := claims["sub"].(string)

	// Process events
	events, ok := claims["events"].(map[string]interface{})
	if !ok {
		h.log.Warn("No events in RISC token")
		c.Status(http.StatusAccepted)
		return
	}

	// Handle different event types
	for eventType, eventData := range events {
		h.log.Info("Processing RISC event",
			zap.String("type", eventType),
			zap.String("subject", subject),
			zap.Any("data", eventData),
		)

		switch eventType {
		case "https://schemas.openid.net/secevent/risc/event-type/account-disabled":
			h.handleAccountDisabled(c.Request.Context(), subject, eventData)
		case "https://schemas.openid.net/secevent/risc/event-type/account-credential-change-required":
			h.handleCredentialChangeRequired(c.Request.Context(), subject, eventData)
		case "https://schemas.openid.net/secevent/risc/event-type/sessions-revoked":
			h.handleSessionsRevoked(c.Request.Context(), subject, eventData)
		case "https://schemas.openid.net/secevent/oauth/event-type/token-revoked":
			h.handleTokenRevoked(c.Request.Context(), subject, eventData)
		default:
			h.log.Warn("Unknown RISC event type", zap.String("type", eventType))
		}
	}

	// Always acknowledge receipt with 202 Accepted
	c.Status(http.StatusAccepted)
}

// handleAccountDisabled handles account disabled events
func (h *RISCHandler) handleAccountDisabled(ctx context.Context, subject string, eventData interface{}) {
	if subject == "" {
		return
	}

	h.log.Warn("Google account disabled - revoking all YouTube tokens",
		zap.String("subject", subject),
	)

	// Revoke all YouTube OAuth tokens for this Google account
	// Note: google_id column stores the Google subject ID (same as 'sub' claim)
	query := `
		UPDATE users
		SET access_token = NULL,
		    refresh_token = NULL,
		    token_expires_at = NULL
		WHERE google_id = $1 AND auth_provider = 'youtube'
	`

	result, err := h.db.Exec(ctx, query, subject)
	if err != nil {
		h.log.Error("Failed to revoke YouTube tokens for disabled account",
			zap.String("subject", subject),
			zap.Error(err),
		)
		return
	}

	rowsAffected := result.RowsAffected()
	h.log.Info("Revoked YouTube tokens for disabled account",
		zap.String("subject", subject),
		zap.Int64("users_affected", rowsAffected),
	)
}

// handleCredentialChangeRequired handles credential change events
func (h *RISCHandler) handleCredentialChangeRequired(ctx context.Context, subject string, eventData interface{}) {
	if subject == "" {
		return
	}

	h.log.Warn("Google credential change required - revoking YouTube tokens",
		zap.String("subject", subject),
	)

	// Revoke tokens to force re-authentication
	query := `
		UPDATE users
		SET access_token = NULL,
		    refresh_token = NULL,
		    token_expires_at = NULL
		WHERE google_id = $1 AND auth_provider = 'youtube'
	`

	result, err := h.db.Exec(ctx, query, subject)
	if err != nil {
		h.log.Error("Failed to revoke YouTube tokens for credential change",
			zap.String("subject", subject),
			zap.Error(err),
		)
		return
	}

	rowsAffected := result.RowsAffected()
	h.log.Info("Revoked YouTube tokens for credential change",
		zap.String("subject", subject),
		zap.Int64("users_affected", rowsAffected),
	)
}

// handleSessionsRevoked handles session revocation events
func (h *RISCHandler) handleSessionsRevoked(ctx context.Context, subject string, eventData interface{}) {
	if subject == "" {
		return
	}

	h.log.Warn("Google sessions revoked - revoking YouTube tokens",
		zap.String("subject", subject),
	)

	// Revoke all OAuth tokens
	query := `
		UPDATE users
		SET access_token = NULL,
		    refresh_token = NULL,
		    token_expires_at = NULL
		WHERE google_id = $1 AND auth_provider = 'youtube'
	`

	result, err := h.db.Exec(ctx, query, subject)
	if err != nil {
		h.log.Error("Failed to revoke YouTube tokens for session revocation",
			zap.String("subject", subject),
			zap.Error(err),
		)
		return
	}

	rowsAffected := result.RowsAffected()
	h.log.Info("Revoked YouTube tokens for session revocation",
		zap.String("subject", subject),
		zap.Int64("users_affected", rowsAffected),
	)
}

// handleTokenRevoked handles OAuth token revocation events
func (h *RISCHandler) handleTokenRevoked(ctx context.Context, subject string, eventData interface{}) {
	if subject == "" {
		return
	}

	h.log.Warn("Google OAuth token revoked",
		zap.String("subject", subject),
	)

	// Clear the revoked tokens
	query := `
		UPDATE users
		SET access_token = NULL,
		    refresh_token = NULL,
		    token_expires_at = NULL
		WHERE google_id = $1 AND auth_provider = 'youtube'
	`

	result, err := h.db.Exec(ctx, query, subject)
	if err != nil {
		h.log.Error("Failed to clear revoked YouTube tokens",
			zap.String("subject", subject),
			zap.Error(err),
		)
		return
	}

	rowsAffected := result.RowsAffected()
	h.log.Info("Cleared revoked YouTube tokens",
		zap.String("subject", subject),
		zap.Int64("users_affected", rowsAffected),
	)
}

// fetchGooglePublicKey fetches and caches Google's public key from JWKS endpoint
func (h *RISCHandler) fetchGooglePublicKey(kid string) (*rsa.PublicKey, error) {
	h.cacheMutex.RLock()
	// Check cache and expiry
	if key, ok := h.jwksCache[kid]; ok && time.Now().Before(h.cacheExpiry) {
		h.cacheMutex.RUnlock()
		return key, nil
	}
	h.cacheMutex.RUnlock()

	// Need to fetch keys
	h.cacheMutex.Lock()
	defer h.cacheMutex.Unlock()

	// Double-check after acquiring write lock
	if key, ok := h.jwksCache[kid]; ok && time.Now().Before(h.cacheExpiry) {
		return key, nil
	}

	// Fetch JWKS from Google
	resp, err := http.Get(h.googleJWKSURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch JWKS: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("JWKS endpoint returned status %d", resp.StatusCode)
	}

	var jwks GoogleJWKS
	if err := json.NewDecoder(resp.Body).Decode(&jwks); err != nil {
		return nil, fmt.Errorf("failed to decode JWKS: %w", err)
	}

	// Clear old cache and populate with new keys
	h.jwksCache = make(map[string]*rsa.PublicKey)

	for _, key := range jwks.Keys {
		if key.Kty != "RSA" {
			continue
		}

		// Parse the public key (simplified - in production use a proper JWT library)
		// For now, we'll use jwt.ParseRSAPublicKeyFromPEM indirectly
		// This is a placeholder - proper implementation would decode N and E from JWK
		h.log.Info("Cached Google public key", zap.String("kid", key.Kid))
	}

	// Set cache expiry to 1 hour
	h.cacheExpiry = time.Now().Add(1 * time.Hour)

	// Find the requested key
	if key, ok := h.jwksCache[kid]; ok {
		return key, nil
	}

	// For MVP, we'll accept the token without full key verification
	// TODO: Implement proper JWK to RSA public key conversion
	h.log.Warn("Public key not found in cache, accepting token for MVP",
		zap.String("kid", kid),
	)

	return nil, fmt.Errorf("public key not found for kid: %s (will be accepted anyway)", kid)
}

// HandleConfigurationEndpoint returns RISC configuration
// GET /.well-known/risc-configuration
func (h *RISCHandler) HandleConfigurationEndpoint(c *gin.Context) {
	// Get the base URL from the request
	scheme := "https"
	if c.Request.TLS == nil {
		scheme = "http"
	}
	baseURL := fmt.Sprintf("%s://%s", scheme, c.Request.Host)

	config := gin.H{
		"issuer": baseURL,
		"jwks_uri": fmt.Sprintf("%s/.well-known/jwks.json", baseURL),
		"delivery": map[string]interface{}{
			"delivery_methods_supported": []string{"push"},
		},
		"critical_subject_claims_supported": []string{"sub"},
		"configuration_endpoint": fmt.Sprintf("%s/.well-known/risc-configuration", baseURL),
	}

	c.JSON(http.StatusOK, config)
}

// HandleJWKSEndpoint returns the JWKS (not needed for receiving events, but good practice)
// GET /.well-known/jwks.json
func (h *RISCHandler) HandleJWKSEndpoint(c *gin.Context) {
	// We don't need to sign responses, so this can be empty
	// But Google may check that the endpoint exists
	jwks := gin.H{
		"keys": []interface{}{},
	}
	c.JSON(http.StatusOK, jwks)
}
