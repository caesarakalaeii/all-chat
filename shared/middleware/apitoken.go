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

package middleware

// Personal access tokens (PATs) for non-browser clients — the Stream Deck and
// StreamController desktop plugins. Those clients cannot participate in the
// cookie/JWT session flow (no browser, no redirect, no refresh), so they present a
// long-lived token as `Authorization: Bearer allchat_pat_<secret>`.
//
// Two invariants hold this file together:
//
//  1. AUTHENTICATION ONLY. A resolved PAT populates exactly the same request context
//     identity a valid session JWT would (user_id, username, roles, …), so every
//     downstream handler, ownership check and premium gate behaves identically. A PAT
//     must never be an authorization bypass: scopes NARROW what a token may do and are
//     enforced IN ADDITION to gates such as RequirePremium, never instead of them.
//
//  2. THE PLAINTEXT IS NEVER PERSISTED OR LOGGED. Only a SHA-256 digest reaches the
//     database (api_tokens.token_hash BYTEA, migration 086 — the same convention as
//     overlay_moderators.invite_token_hash from migration 080). Nothing in this package
//     puts a token, or any prefix of one, into a log field.

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// APITokenPrefix is the fixed, public prefix of every personal access token. It is
// part of the client contract (the desktop plugins and the frontend token page all
// depend on it) and is also what makes a leaked token recognisable to secret
// scanners. A Bearer value carrying this prefix is resolved against api_tokens; any
// other value falls through to the unchanged JWT path.
const APITokenPrefix = "allchat_pat_"

// Scope constants. Scopes are least-privilege capability strings stored per token in
// api_tokens.scopes and enforced by RequireAPITokenScope.
const (
	// ScopeChatWrite permits sending chat messages on the user's behalf.
	ScopeChatWrite = "chat:write"
	// ScopeEngagementWrite permits opening/managing polls and predictions.
	ScopeEngagementWrite = "engagement:write"
)

// Context keys set by the PAT path, in addition to the identity keys a JWT sets.
// They exist so a route can require a scope (RequireAPITokenScope) and so audit
// logs can distinguish a desktop plugin from a browser session.
const (
	// CtxAuthMethod is "api_token" for PAT-authenticated requests and "jwt" for the
	// session path. A device token (ADR-0049) also sets "api_token": it travels the
	// same resolver seam, so every existing session-only guard (AdminOnly,
	// RefuseAPIToken) covers it without change. Use CtxTokenKind to tell the two apart.
	CtxAuthMethod = "auth_method"
	// CtxAPITokenID is the api_tokens.id (or device_tokens.id) of the presented token,
	// never the token.
	CtxAPITokenID = "api_token_id"
	// CtxAPITokenScopes is []string of the token's granted scopes.
	CtxAPITokenScopes = "api_token_scopes"
	// CtxTokenKind is TokenKindPAT or TokenKindDevice for token-authenticated requests,
	// and unset for a browser session. It exists so a route can apply a rule that only
	// makes sense for one row shape (RequireDeviceTokenOverlay) without duplicating the
	// authentication path.
	CtxTokenKind = "token_kind"
	// CtxDeviceOverlayID is the overlay a device token is bound to (device_tokens.
	// overlay_id). Empty for a PAT, which is user-scoped, and for a session.
	CtxDeviceOverlayID = "device_overlay_id"

	// AuthMethodAPIToken / AuthMethodJWT are the two values of CtxAuthMethod.
	AuthMethodAPIToken = "api_token"
	AuthMethodJWT      = "jwt"

	// TokenKindPAT is a personal access token (api_tokens, ADR-0051): user-scoped,
	// pasted by a human, optional expiry.
	TokenKindPAT = "pat"
	// TokenKindDevice is a paired device token (device_tokens, ADR-0049): bound to one
	// overlay, never typed by a human, mandatory sliding expiry.
	TokenKindDevice = "device"
)

// ErrAPITokenNotFound is returned by an APITokenResolver when the digest matches no
// usable row — unknown, revoked or expired. The three cases are deliberately
// indistinguishable to the caller so the 401 body cannot be used as an oracle.
var ErrAPITokenNotFound = errors.New("api token not found")

// APITokenIdentity is the identity a resolved PAT stands for. It mirrors the subset
// of auth.Claims that JWTAuthWithRevocation puts into the request context, so the
// PAT path and the JWT path are indistinguishable downstream.
type APITokenIdentity struct {
	// TokenID is api_tokens.id — safe to log, unlike the token itself.
	TokenID string
	UserID  string
	// Username and TwitchID mirror the JWT claims of the owning user.
	Username string
	TwitchID string
	// Roles is the same shape JWTAuth sets: {"user"} plus "admin" when applicable.
	Roles []string
	// Scopes is api_tokens.scopes.
	Scopes []string
	// Kind is TokenKindPAT or TokenKindDevice — which row shape resolved. Empty is
	// treated as TokenKindPAT so a resolver written before ADR-0049 keeps behaving
	// exactly as it did.
	Kind string
	// OverlayID is the overlay a device token is bound to (device_tokens.overlay_id).
	// Always empty for a PAT: ADR-0051 tokens are user-scoped, which is the residual
	// risk ADR-0049's per-overlay binding exists to remove.
	OverlayID string
}

// APITokenResolver resolves a SHA-256 token digest to the identity it authenticates.
// Implementations MUST reject revoked and expired tokens by returning
// ErrAPITokenNotFound, and SHOULD record last_used_at as best-effort telemetry.
//
// It is an interface rather than a *pgxpool.Pool so this package stays unit-testable
// without a database, matching how premiumQuerier / betaTesterQuerier are injected.
type APITokenResolver interface {
	ResolveAPIToken(ctx context.Context, tokenHash []byte) (*APITokenIdentity, error)
}

// APITokenResolverFunc adapts a plain function to APITokenResolver (tests, and
// services that want to wrap a resolver with caching or metrics).
type APITokenResolverFunc func(ctx context.Context, tokenHash []byte) (*APITokenIdentity, error)

// ResolveAPIToken implements APITokenResolver.
func (f APITokenResolverFunc) ResolveAPIToken(ctx context.Context, tokenHash []byte) (*APITokenIdentity, error) {
	return f(ctx, tokenHash)
}

// apiTokenResolver is the process-wide resolver, wired by SetAPITokenResolver.
// Default nil == "PATs are not enabled in this service", in which case an
// `allchat_pat_` bearer is rejected rather than misinterpreted as a JWT.
var (
	apiTokenResolverMu sync.RWMutex
	apiTokenResolver   APITokenResolver
)

// SetAPITokenResolver wires the personal-access-token resolver used by the auth
// middleware. Services call it once at startup, immediately after their database
// pool exists, exactly like SetLogger above:
//
//	middleware.SetAPITokenResolver(middleware.NewPgxAPITokenResolver(db))
//
// This must be called in EVERY service that authenticates end users, not just the
// gateway: api-gateway's proxy forwards the client's Authorization header verbatim
// and each backend re-validates independently, so a service without a resolver
// would reject every PAT as a malformed JWT.
//
// Passing nil disables the PAT path (used by tests to restore the default).
func SetAPITokenResolver(r APITokenResolver) {
	apiTokenResolverMu.Lock()
	apiTokenResolver = r
	apiTokenResolverMu.Unlock()
}

// apiTokenResolverOrNil returns the wired resolver, or nil when PATs are disabled.
func apiTokenResolverOrNil() APITokenResolver {
	apiTokenResolverMu.RLock()
	defer apiTokenResolverMu.RUnlock()
	return apiTokenResolver
}

// IsAPIToken reports whether a Bearer value is one of our opaque token credentials —
// a personal access token (`allchat_pat_`) or a paired device token (`allchat_dev_`) —
// rather than a JWT.
//
// BOTH prefixes belong here, and this is load-bearing rather than tidy. Every caller
// of this function is asking "is this an opaque token, so keep it away from the JWT
// machinery?": shared/middleware/auth.go routes it to the resolver instead of the JWT
// parser and the logout blacklist, CookieToBearer refuses to promote it out of a
// cookie, and blacklistableSessionToken keeps its plaintext out of a Redis key. A
// device token that failed this test would fall through to the JWT parser and 401 in
// every service.
//
// It is a prefix test only: a value that looks like one of our tokens is never parsed
// as a JWT, and a value that does not is never looked up in a token table.
func IsAPIToken(bearer string) bool {
	return strings.HasPrefix(bearer, APITokenPrefix) ||
		strings.HasPrefix(bearer, DeviceTokenPrefix)
}

// HashAPIToken returns the SHA-256 digest stored in api_tokens.token_hash. The whole
// presented token including its prefix is hashed, so the digest is bound to the
// exact string the client sends.
func HashAPIToken(token string) []byte {
	sum := sha256.Sum256([]byte(token))
	return sum[:]
}

// apiTokenSecretBytes is the entropy behind a token: 32 bytes = 256 bits from
// crypto/rand, well beyond guessing range for a value that never expires by default.
const apiTokenSecretBytes = 32

// GenerateAPIToken mints a new personal access token. It returns the plaintext —
// which the caller must show to the user exactly once and never store or log — and
// the digest to persist in api_tokens.token_hash.
//
// The secret is base64url without padding, so the token is a single copy-pasteable
// word with no characters that need escaping in a shell, a config file or a header.
func GenerateAPIToken() (plaintext string, tokenHash []byte, err error) {
	buf := make([]byte, apiTokenSecretBytes)
	if _, err := rand.Read(buf); err != nil {
		// crypto/rand failure is unrecoverable: never fall back to a weaker source.
		return "", nil, fmt.Errorf("generate api token: %w", err)
	}
	plaintext = APITokenPrefix + base64.RawURLEncoding.EncodeToString(buf)
	return plaintext, HashAPIToken(plaintext), nil
}

// authenticateAPIToken resolves a PAT bearer and populates the request context with
// the same identity a valid JWT would. It returns false (having already written the
// 401 and aborted) when the token is unknown, revoked, expired, or when no resolver
// is wired.
//
// It is only ever reached for values that passed IsAPIToken, so the JWT path is
// bit-for-bit unchanged for everything else.
func authenticateAPIToken(c *gin.Context, bearer string) bool {
	resolver := apiTokenResolverOrNil()
	if resolver == nil {
		// PATs are not enabled here. Say "invalid token" and nothing more: whether a
		// deployment supports PATs is not something a 401 body should teach an attacker.
		revocationLog().Warn("Personal access token presented but no resolver is wired; " +
			"call middleware.SetAPITokenResolver at startup")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired token"})
		c.Abort()
		return false
	}

	identity, err := resolver.ResolveAPIToken(c.Request.Context(), HashAPIToken(bearer))
	if err != nil || identity == nil {
		if err != nil && !errors.Is(err, ErrAPITokenNotFound) {
			// A database failure is not a rejection reason we can distinguish for the
			// client, but it must be visible to operators. Note the absence of any token
			// material in the log fields — that is deliberate.
			revocationLog().Error("Personal access token lookup failed", zap.Error(err))
		}
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired token"})
		c.Abort()
		return false
	}

	// Same context identity as the JWT path (see JWTAuthWithRevocation), so no
	// downstream handler needs to know how the request authenticated.
	c.Set("user_id", identity.UserID)
	c.Set("username", identity.Username)
	c.Set("twitch_id", identity.TwitchID)
	roles := identity.Roles
	if roles == nil {
		roles = []string{}
	}
	c.Set("roles", roles)
	// Impersonation provenance is always empty for a PAT: a token belongs to one user
	// and cannot be minted for an admin acting as someone else (ADR-0017).
	c.Set("impersonated_by", "")
	c.Set("impersonated_user", "")

	// Token-specific extras, used by RequireAPITokenScope, RequireDeviceTokenOverlay and
	// audit logging.
	scopes := identity.Scopes
	if scopes == nil {
		scopes = []string{}
	}
	kind := identity.Kind
	if kind == "" {
		// A resolver that predates ADR-0049 (or a test one) reports no kind. Default to
		// PAT, which is the shape those resolvers return, so nothing changes for them.
		kind = TokenKindPAT
	}
	c.Set(CtxAuthMethod, AuthMethodAPIToken)
	c.Set(CtxAPITokenID, identity.TokenID)
	c.Set(CtxAPITokenScopes, scopes)
	c.Set(CtxTokenKind, kind)
	c.Set(CtxDeviceOverlayID, identity.OverlayID)
	return true
}

// APITokenScopes returns the scopes of the PAT that authenticated this request, and
// whether the request was PAT-authenticated at all. A browser session returns
// (nil, false) — it has no scopes because it is not scope-limited.
func APITokenScopes(c *gin.Context) ([]string, bool) {
	if c.GetString(CtxAuthMethod) != AuthMethodAPIToken {
		return nil, false
	}
	scopes, _ := c.Get(CtxAPITokenScopes)
	list, _ := scopes.([]string)
	return list, true
}

// RequireAPITokenScope returns middleware that, for PAT-authenticated requests only,
// requires the token to carry at least one of the given scopes.
//
// It is additive by design and must be placed ALONGSIDE the existing authorization
// middleware, never in place of it:
//
//	auth.POST("/overlays/:id/polls",
//	    middleware.RequireAPITokenScope(middleware.ScopeEngagementWrite),
//	    requireEngagementPremium, h.CreatePoll)
//
// A browser session passes straight through (it is not scope-limited and was already
// authorized by the surrounding gates), so wiring this on a route cannot change
// behaviour for existing web clients. A PAT missing the scope gets 403 — it
// authenticated fine, it simply may not do this.
func RequireAPITokenScope(required ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		scopes, isAPIToken := APITokenScopes(c)
		if !isAPIToken {
			c.Next()
			return
		}
		for _, want := range required {
			for _, have := range scopes {
				if have == want {
					c.Next()
					return
				}
			}
		}
		revocationLog().Info("Personal access token lacks required scope",
			zap.String("token_id", c.GetString(CtxAPITokenID)),
			zap.Strings("required", required))
		c.JSON(http.StatusForbidden, gin.H{
			"error":           "insufficient token scope",
			"required_scopes": required,
			"message":         "This personal access token is not permitted to perform this action.",
		})
		c.Abort()
	}
}

// pgxAPITokenResolver is the production APITokenResolver, backed by the api_tokens
// table (migration 086).
type pgxAPITokenResolver struct {
	db *pgxpool.Pool
}

// NewPgxAPITokenResolver returns the production resolver for SetAPITokenResolver.
// Every service that authenticates end users wires one from its own pool.
func NewPgxAPITokenResolver(db *pgxpool.Pool) APITokenResolver {
	return &pgxAPITokenResolver{db: db}
}

// resolveAPITokenSQL looks up a live token by digest and joins the owning user so the
// resolved identity carries the same fields as a JWT.
//
// Validity is decided entirely in SQL, in one round trip on the request path:
//   - revoked_at IS NULL          — revocation takes effect within one request
//   - expires_at IS NULL OR > now — NULL means "until revoked" (migration 086)
//   - the owner is not banned     — see below
//
// The ban predicate makes a PAT strictly stricter than a session JWT, deliberately. A
// ban blocks LOGIN (no new JWT is issued, migration 015), and an already-issued JWT is
// backstopped by its 24-hour expiry. A PAT has no such backstop: without this clause a
// banned account would keep acting through a token issued before the ban, indefinitely.
//
// Unknown, revoked, expired and banned all yield zero rows, so the caller cannot tell
// them apart and neither can a client.
const resolveAPITokenSQL = `
	SELECT t.id::text,
	       t.user_id::text,
	       u.username,
	       COALESCE(u.twitch_id, ''),
	       u.is_admin,
	       t.scopes
	  FROM api_tokens t
	  JOIN users u ON u.id = t.user_id
	 WHERE t.token_hash = $1
	   AND t.revoked_at IS NULL
	   AND (t.expires_at IS NULL OR t.expires_at > NOW())
	   AND u.is_banned = FALSE`

// touchAPITokenSQL records last_used_at. Deliberately a separate, best-effort
// statement: it is telemetry, and a write failure (read-only replica, lock wait)
// must never turn a valid token into a 401. It is also throttled to one write per
// minute per token so a plugin polling every second does not write on every request.
const touchAPITokenSQL = `
	UPDATE api_tokens
	   SET last_used_at = NOW()
	 WHERE id = $1
	   AND (last_used_at IS NULL OR last_used_at < NOW() - INTERVAL '1 minute')`

// ResolveAPIToken implements APITokenResolver against PostgreSQL.
func (r *pgxAPITokenResolver) ResolveAPIToken(ctx context.Context, tokenHash []byte) (*APITokenIdentity, error) {
	var (
		id       string
		identity APITokenIdentity
		isAdmin  bool
	)
	err := r.db.QueryRow(ctx, resolveAPITokenSQL, tokenHash).Scan(
		&id, &identity.UserID, &identity.Username, &identity.TwitchID, &isAdmin, &identity.Scopes,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrAPITokenNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("resolve api token: %w", err)
	}
	identity.TokenID = id
	// Same role shape as auth.GenerateJWT, so AdminOnly() behaves identically.
	identity.Roles = []string{"user"}
	if isAdmin {
		identity.Roles = append(identity.Roles, "admin")
	}

	r.touch(ctx, id)
	return &identity, nil
}

// touch updates last_used_at without letting that write affect the request. It runs
// on a detached context with a short deadline so a slow UPDATE cannot hold up the
// authenticated request, and it never surfaces an error.
func (r *pgxAPITokenResolver) touch(ctx context.Context, tokenID string) {
	touchCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
	defer cancel()
	if _, err := r.db.Exec(touchCtx, touchAPITokenSQL, tokenID); err != nil {
		revocationLog().Debug("Failed to update api token last_used_at (non-fatal)",
			zap.String("token_id", tokenID), zap.Error(err))
	}
}
