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

// Management endpoints for personal access tokens — create, list, revoke — for the
// authenticated user's own tokens only.
//
// The contract that matters: the PLAINTEXT TOKEN IS RETURNED EXACTLY ONCE, in the
// create response, and is unrecoverable afterwards. Only a SHA-256 digest is stored
// (api_tokens.token_hash, migration 086), so "show it again" is not a feature that was
// left out — it is impossible by construction, exactly like the delegated-moderator
// invite secret in migration 080. The list response carries metadata only.
//
// Nothing here logs a token. Log fields name the token by its api_tokens.id.

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/caesar/all-chat/services/auth-service/repository"
	"github.com/caesar/all-chat/shared/middleware"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// apiTokenStore is the persistence this handler needs. An interface, not the concrete
// repository, so the guard and validation paths are unit-testable without a database
// (mirroring how premiumQuerier is injected in shared/middleware).
type apiTokenStore interface {
	CreateAPIToken(ctx context.Context, userID, name string, tokenHash []byte, scopes []string, expiresAt *time.Time) (*repository.APIToken, error)
	ListAPITokensByUser(ctx context.Context, userID string) ([]repository.APIToken, error)
	RevokeAPIToken(ctx context.Context, userID, tokenID string) (*repository.APIToken, error)
}

// APITokenHandler serves /me/api-tokens.
type APITokenHandler struct {
	tokens apiTokenStore
	logger *zap.Logger
}

// NewAPITokenHandler wires the handler over the api_tokens repository.
func NewAPITokenHandler(repo *repository.APITokenRepository, logger *zap.Logger) *APITokenHandler {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &APITokenHandler{tokens: repo, logger: logger}
}

// newAPITokenHandlerWithStore builds a handler over an arbitrary store. Tests only.
func newAPITokenHandlerWithStore(store apiTokenStore, logger *zap.Logger) *APITokenHandler {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &APITokenHandler{tokens: store, logger: logger}
}

// allowedAPITokenScopes is the CLOSED set of grantable scopes. A closed set is the
// point: an unrecognised scope string is rejected at create time, so a typo can never
// mint a token that silently fails every scope check later, and a future scope cannot
// be requested before the server enforces it.
var allowedAPITokenScopes = map[string]bool{
	middleware.ScopeChatWrite:       true,
	middleware.ScopeEngagementWrite: true,
}

// maxAPITokenNameLength matches api_tokens.name VARCHAR(120) (migration 086).
const maxAPITokenNameLength = 120

// maxAPITokenLifetime caps an explicitly requested expiry. Absent expiry means "until
// revoked" — desktop plugins are configured once, and a forced expiry would break a
// live stream — so this only bounds what a client may ask for.
const maxAPITokenLifetime = 5 * 365 * 24 * time.Hour

// createAPITokenRequest is the POST /me/api-tokens body.
type createAPITokenRequest struct {
	// Name is a user-facing label, e.g. "Stream Deck (studio PC)".
	Name string `json:"name" binding:"required"`
	// Scopes is the requested capability set. Required and non-empty: a token with no
	// scopes could authenticate but do nothing, which is a support ticket, not a feature.
	Scopes []string `json:"scopes" binding:"required"`
	// ExpiresAt is optional. Omitted or null == valid until revoked.
	ExpiresAt *time.Time `json:"expires_at"`
}

// createAPITokenResponse is the ONLY place a plaintext token ever appears in an API
// response. The field is named `token` and documented as unrecoverable so no client
// author assumes it can be fetched again.
type createAPITokenResponse struct {
	repository.APIToken
	// Token is the plaintext. Shown once; never stored, never returned again.
	Token string `json:"token"`
}

// HandleCreateAPIToken mints a personal access token for the authenticated user.
//
// Route: POST /me/api-tokens (JWT required)
//
// Impersonation is refused: an admin acting as a user must not be able to walk away
// with a long-lived credential for that user's account, which would outlive the
// impersonation session and be indistinguishable from the user's own token
// afterwards (ADR-0017 keeps impersonation attributable; a PAT would not be).
func (h *APITokenHandler) HandleCreateAPIToken(c *gin.Context) {
	userID, ok := requireSelf(c)
	if !ok {
		return
	}

	var req createAPITokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": `Body must be {"name": string, "scopes": [string], "expires_at": RFC3339|null}`,
		})
		return
	}

	name := strings.TrimSpace(req.Name)
	if name == "" || len([]rune(name)) > maxAPITokenNameLength {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "name must be 1-120 characters",
		})
		return
	}

	scopes, err := normalizeAPITokenScopes(req.Scopes)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":          err.Error(),
			"allowed_scopes": allowedAPITokenScopeList(),
		})
		return
	}

	if req.ExpiresAt != nil {
		expires := req.ExpiresAt.UTC()
		if !expires.After(time.Now().UTC()) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "expires_at must be in the future"})
			return
		}
		if expires.After(time.Now().UTC().Add(maxAPITokenLifetime)) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "expires_at is too far in the future"})
			return
		}
		req.ExpiresAt = &expires
	}

	// crypto/rand only: see middleware.GenerateAPIToken. A failure here is fatal to the
	// request — there is no weaker fallback.
	plaintext, tokenHash, err := middleware.GenerateAPIToken()
	if err != nil {
		h.logger.Error("Failed to generate personal access token", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create token"})
		return
	}

	token, err := h.tokens.CreateAPIToken(c.Request.Context(), userID, name, tokenHash, scopes, req.ExpiresAt)
	if err != nil {
		switch {
		case errors.Is(err, repository.ErrAPITokenLimitReached):
			c.JSON(http.StatusConflict, gin.H{
				"error":   "Token limit reached",
				"message": "Revoke an existing personal access token before creating another.",
			})
		case errors.Is(err, repository.ErrUserNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		default:
			// zap.Error only — the token is deliberately not among the fields.
			h.logger.Error("Failed to persist personal access token",
				zap.String("user_id", userID), zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create token"})
		}
		return
	}

	h.logger.Info("Personal access token created",
		zap.String("user_id", userID),
		zap.String("token_id", token.ID),
		zap.Strings("scopes", token.Scopes))

	c.JSON(http.StatusCreated, createAPITokenResponse{APIToken: *token, Token: plaintext})
}

// HandleListAPITokens returns the authenticated user's token METADATA.
//
// Route: GET /me/api-tokens (JWT required)
//
// The response type is repository.APIToken, whose struct has no field for the token or
// its digest and whose SQL projection never selects token_hash — so there is no
// serialisation path by which a secret could leak from here.
func (h *APITokenHandler) HandleListAPITokens(c *gin.Context) {
	userID, ok := requireSelf(c)
	if !ok {
		return
	}

	tokens, err := h.tokens.ListAPITokensByUser(c.Request.Context(), userID)
	if err != nil {
		h.logger.Error("Failed to list personal access tokens",
			zap.String("user_id", userID), zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list tokens"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"tokens": tokens})
}

// HandleRevokeAPIToken revokes one of the authenticated user's tokens.
//
// Route: DELETE /me/api-tokens/:id (JWT required)
//
// Revocation is read live by the resolver on every request, so it takes effect within
// one request — there is no cache to invalidate. Another user's token id yields 404,
// identical to a nonexistent one, so this cannot enumerate tokens.
func (h *APITokenHandler) HandleRevokeAPIToken(c *gin.Context) {
	userID, ok := requireSelf(c)
	if !ok {
		return
	}

	tokenID := c.Param("id")
	if _, err := uuid.Parse(tokenID); err != nil {
		// Rejected before the query so a malformed id is a clean 400 rather than a
		// PostgreSQL cast error surfacing as a 500.
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid token id"})
		return
	}

	token, err := h.tokens.RevokeAPIToken(c.Request.Context(), userID, tokenID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Token not found"})
			return
		}
		h.logger.Error("Failed to revoke personal access token",
			zap.String("user_id", userID), zap.String("token_id", tokenID), zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to revoke token"})
		return
	}

	h.logger.Info("Personal access token revoked",
		zap.String("user_id", userID), zap.String("token_id", token.ID))

	c.JSON(http.StatusOK, gin.H{"token": token})
}

// requireSelf resolves the authenticated user id and refuses both anonymous requests
// and impersonation sessions.
//
// Why impersonation is refused for all three verbs: a PAT is a bearer credential that
// outlives the session which minted it. An admin impersonating a user must not be able
// to mint one (a permanent backdoor), and should not be able to revoke the user's
// tokens either (a denial of service attributable to the wrong person).
//
// A PAT cannot be used to manage PATs either: token management is a session-only
// surface, so a leaked token cannot mint fresh tokens or revoke the victim's ability
// to lock it out. That check is the auth_method one below.
func requireSelf(c *gin.Context) (string, bool) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return "", false
	}
	if c.GetString("impersonated_by") != "" {
		c.JSON(http.StatusForbidden, gin.H{
			"error": "Personal access tokens cannot be managed while impersonating",
		})
		return "", false
	}
	if c.GetString(middleware.CtxAuthMethod) == middleware.AuthMethodAPIToken {
		c.JSON(http.StatusForbidden, gin.H{
			"error":   "Session required",
			"message": "Personal access tokens can only be managed from a signed-in session, not with a token.",
		})
		return "", false
	}
	return userID, true
}

// normalizeAPITokenScopes trims, de-duplicates and validates a requested scope set
// against the closed allowlist.
func normalizeAPITokenScopes(requested []string) ([]string, error) {
	seen := make(map[string]bool, len(requested))
	scopes := make([]string, 0, len(requested))
	for _, raw := range requested {
		scope := strings.TrimSpace(raw)
		if scope == "" {
			continue
		}
		if !allowedAPITokenScopes[scope] {
			return nil, errors.New("unknown scope: " + scope)
		}
		if seen[scope] {
			continue
		}
		seen[scope] = true
		scopes = append(scopes, scope)
	}
	if len(scopes) == 0 {
		return nil, errors.New("at least one scope is required")
	}
	return scopes, nil
}

// allowedAPITokenScopeList renders the allowlist for the 400 body, so a client can
// discover the valid scopes without reading the source.
func allowedAPITokenScopeList() []string {
	return []string{middleware.ScopeChatWrite, middleware.ScopeEngagementWrite}
}
