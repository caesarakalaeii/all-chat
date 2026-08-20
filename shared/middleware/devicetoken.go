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

// Paired-device tokens for desktop control surfaces (ADR-0049, step 2). A device
// token is what a Stream Deck / StreamController plugin holds AFTER the streamer has
// linked it, presented as `Authorization: Bearer allchat_dev_<secret>`.
//
// This is not a second authentication path. ADR-0051 left the seam on purpose:
//
//	Device-token recognition (step 2's "token type in the auth middleware") is
//	already built as middleware.SetAPITokenResolver + APITokenResolver [...] A device
//	token becomes another row shape behind that interface rather than a second auth
//	path.
//
// So everything here plugs into the existing resolver. A service wires ONE resolver
// that handles both prefixes:
//
//	middleware.SetAPITokenResolver(middleware.NewTokenResolverDispatch(
//	    middleware.NewPgxAPITokenResolver(db),
//	    middleware.NewPgxDeviceTokenResolver(db),
//	))
//
// The two invariants from apitoken.go hold verbatim, and one is added:
//
//  1. AUTHENTICATION ONLY. A resolved device token populates exactly the same request
//     identity a session JWT would, so every ownership check and premium gate behaves
//     identically. Scopes and the overlay binding NARROW what the token may do; they
//     never authorize anything the owning session could not do.
//
//  2. THE PLAINTEXT IS NEVER PERSISTED OR LOGGED. Only a SHA-256 digest reaches
//     device_tokens.token_hash (migration 088).
//
//  3. THE PLAINTEXT IS NEVER SHOWN TO A HUMAN EITHER. This is the difference that
//     justifies a second credential type at all: the secret goes from the exchange
//     endpoint straight to the plugin over the loopback redirect, so it cannot be read
//     aloud, screenshotted or leaked on camera — the failure mode ADR-0049 rejected
//     option 3 for. Nothing in the dashboard renders it.

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// DeviceTokenPrefix is the fixed, public prefix of every paired-device token. It is
// part of the frozen contract (both plugins, the dashboard and three services depend
// on it) and it is deliberately a DIFFERENT prefix from APITokenPrefix rather than a
// flag inside the same namespace: a switch on the prefix stays unambiguous, a secret
// scanner can tell the two apart, and support can tell from a redacted log line which
// credential a streamer is using.
const DeviceTokenPrefix = "allchat_dev_"

// DeviceTokenLifetime is the default validity of a freshly minted device token, and
// also the window the expiry is slid forward to on use.
//
// 90 days with a sliding window, rather than a PAT's optional expiry, is the third
// property ADR-0049 wants from a device token: a control surface in daily service
// never expires, while a pairing on a machine that was sold, reinstalled or forgotten
// lapses on its own instead of living until somebody notices.
const DeviceTokenLifetime = 90 * 24 * time.Hour

// IsDeviceToken reports whether a Bearer value is a paired-device token. IsAPIToken
// covers both credential types (that is what routes a bearer away from the JWT path);
// this is the narrower test, for code that must distinguish them.
func IsDeviceToken(bearer string) bool {
	return strings.HasPrefix(bearer, DeviceTokenPrefix)
}

// HashDeviceToken returns the SHA-256 digest stored in device_tokens.token_hash. The
// whole presented token including its prefix is hashed, so the digest is bound to the
// exact string the client sends. Identical construction to HashAPIToken, kept as its
// own name so a caller cannot hash a device token with the PAT helper by accident and
// silently look it up in the wrong table.
func HashDeviceToken(token string) []byte {
	return HashAPIToken(token)
}

// GenerateDeviceToken mints a new device token: 256 bits from crypto/rand, base64url
// without padding, prefixed. Exactly the shape GenerateAPIToken produces, because the
// entropy requirement is the same and a second format would be a second thing to get
// right.
//
// It returns the plaintext — which the caller returns to the PLUGIN exactly once, from
// the exchange endpoint, and never renders in a browser or writes to a log — and the
// digest to persist.
func GenerateDeviceToken() (plaintext string, tokenHash []byte, err error) {
	buf := make([]byte, apiTokenSecretBytes)
	if _, err := rand.Read(buf); err != nil {
		// crypto/rand failure is unrecoverable: never fall back to a weaker source.
		return "", nil, fmt.Errorf("generate device token: %w", err)
	}
	plaintext = DeviceTokenPrefix + base64.RawURLEncoding.EncodeToString(buf)
	return plaintext, HashDeviceToken(plaintext), nil
}

// tokenResolverDispatch routes a digest to the resolver for its credential type.
//
// Why a dispatcher rather than two SetAPITokenResolver calls: the wired resolver is
// process-wide and receives a DIGEST, not the bearer, so it cannot recover the prefix
// itself. Dispatching on the bearer prefix at authentication time and handing each
// resolver only the digests it owns keeps one lookup per request — a device token does
// not pay for a miss against api_tokens first.
type tokenResolverDispatch struct {
	pat    APITokenResolver
	device APITokenResolver
}

// NewTokenResolverDispatch combines the PAT and device resolvers into the single
// resolver SetAPITokenResolver takes. Either may be nil, in which case that credential
// type is simply not enabled in this service and its bearers are rejected (never
// misinterpreted as the other type, and never as a JWT).
func NewTokenResolverDispatch(pat, device APITokenResolver) APITokenResolver {
	return &tokenResolverDispatch{pat: pat, device: device}
}

// ResolveAPIToken implements APITokenResolver by delegating on credential type.
//
// The digest alone does not say which table to look in, so the dispatcher tries the
// device resolver and then the PAT resolver, treating ErrAPITokenNotFound as "not this
// kind". That is at most one extra lookup on a genuine miss (an unknown token, which
// is already the error path) and never on the hot path for a valid credential, because
// authenticateAPIToken passes the digest of a bearer whose prefix already decided the
// order below.
func (d *tokenResolverDispatch) ResolveAPIToken(ctx context.Context, tokenHash []byte) (*APITokenIdentity, error) {
	for _, r := range []APITokenResolver{d.device, d.pat} {
		if r == nil {
			continue
		}
		identity, err := r.ResolveAPIToken(ctx, tokenHash)
		if err == nil && identity != nil {
			return identity, nil
		}
		if err != nil && !errors.Is(err, ErrAPITokenNotFound) {
			// A real failure (database down) must not be reported as "unknown token":
			// that would turn a transient outage into an authentication oracle saying
			// the credential does not exist.
			return nil, err
		}
	}
	return nil, ErrAPITokenNotFound
}

// pgxDeviceTokenResolver is the production device-token resolver, backed by the
// device_tokens table (migration 088).
type pgxDeviceTokenResolver struct {
	db *pgxpool.Pool
}

// NewPgxDeviceTokenResolver returns the production device resolver. Every service that
// authenticates end users wires one from its own pool, for the same reason PATs are
// wired everywhere: api-gateway forwards the client's Authorization header verbatim and
// each backend re-validates independently, so a service without a resolver would 401
// every device token.
func NewPgxDeviceTokenResolver(db *pgxpool.Pool) APITokenResolver {
	return &pgxDeviceTokenResolver{db: db}
}

// resolveDeviceTokenSQL looks up a live device token by digest and joins the owning
// user, so the resolved identity carries the same fields a JWT would.
//
// Validity is decided entirely in SQL, in one round trip on the request path, exactly
// as resolveAPITokenSQL does:
//   - revoked_at IS NULL      — revocation takes effect within one request
//   - expires_at > now        — NOT NULL here, unlike a PAT: the sliding window is the
//     backstop for an abandoned pairing
//   - the owner is not banned — a device token has no 24-hour JWT expiry to fall back
//     on either, so without this clause a banned account would keep
//     acting through a token issued before the ban, for months
//
// Unknown, revoked, expired and banned all yield zero rows, so neither the caller nor a
// client can tell them apart.
const resolveDeviceTokenSQL = `
	SELECT d.id::text,
	       d.user_id::text,
	       u.username,
	       COALESCE(u.twitch_id, ''),
	       u.is_admin,
	       d.scopes,
	       d.overlay_id::text
	  FROM device_tokens d
	  JOIN users u ON u.id = d.user_id
	 WHERE d.token_hash = $1
	   AND d.revoked_at IS NULL
	   AND d.expires_at > NOW()
	   AND u.is_banned = FALSE`

// touchDeviceTokenSQL records last_used_at AND slides the expiry forward. Deliberately
// one best-effort statement: it is telemetry plus a renewal, and a write failure
// (read-only replica, lock wait) must never turn a valid token into a 401 mid-stream.
//
// Throttled to one write per minute per token, exactly as touchAPITokenSQL, so a plugin
// polling the active poll every second does not write on every request. The throttle
// costs nothing in expiry terms: skipping a renewal for up to a minute out of a 90-day
// window is not observable.
const touchDeviceTokenSQL = `
	UPDATE device_tokens
	   SET last_used_at = NOW(),
	       expires_at   = NOW() + ($2 || ' seconds')::INTERVAL
	 WHERE id = $1
	   AND (last_used_at IS NULL OR last_used_at < NOW() - INTERVAL '1 minute')`

// ResolveAPIToken implements APITokenResolver against PostgreSQL for device tokens.
func (r *pgxDeviceTokenResolver) ResolveAPIToken(ctx context.Context, tokenHash []byte) (*APITokenIdentity, error) {
	var (
		id       string
		identity APITokenIdentity
		isAdmin  bool
	)
	err := r.db.QueryRow(ctx, resolveDeviceTokenSQL, tokenHash).Scan(
		&id, &identity.UserID, &identity.Username, &identity.TwitchID, &isAdmin,
		&identity.Scopes, &identity.OverlayID,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrAPITokenNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("resolve device token: %w", err)
	}
	identity.TokenID = id
	identity.Kind = TokenKindDevice
	// Same role shape as auth.GenerateJWT, so AdminOnly() behaves identically — which
	// for a token means "refused", since AdminOnly is session-only.
	identity.Roles = []string{"user"}
	if isAdmin {
		identity.Roles = append(identity.Roles, "admin")
	}

	r.touch(ctx, id)
	return &identity, nil
}

// touch slides the expiry and records last_used_at without letting that write affect
// the request. It runs on a detached context with a short deadline so a slow UPDATE
// cannot hold up an authenticated request, and it never surfaces an error.
func (r *pgxDeviceTokenResolver) touch(ctx context.Context, tokenID string) {
	touchCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
	defer cancel()
	seconds := int64(DeviceTokenLifetime / time.Second)
	if _, err := r.db.Exec(touchCtx, touchDeviceTokenSQL, tokenID, seconds); err != nil {
		revocationLog().Debug("Failed to slide device token expiry (non-fatal)",
			zap.String("token_id", tokenID), zap.Error(err))
	}
}

// DeviceTokenOverlay returns the overlay a device token is bound to, and whether the
// request was device-authenticated at all. A browser session and a PAT both return
// ("", false) — neither has an overlay binding.
func DeviceTokenOverlay(c *gin.Context) (string, bool) {
	if c.GetString(CtxAuthMethod) != AuthMethodAPIToken ||
		c.GetString(CtxTokenKind) != TokenKindDevice {
		return "", false
	}
	return c.GetString(CtxDeviceOverlayID), true
}

// RequireDeviceTokenOverlay returns middleware that, for DEVICE-authenticated requests
// only, requires the overlay id in route parameter `param` to equal the overlay the
// token was bound to at pairing time. A session and a PAT pass through untouched.
//
// It is additive, exactly like RequireAPITokenScope, and must sit BESIDE
// RequireAPITokenScope and RequirePremium rather than in place of either:
//
//	auth.POST("/overlays/:id/polls",
//	    middleware.RequireAPITokenScope(middleware.ScopeEngagementWrite),
//	    middleware.RequireDeviceTokenOverlay("id"),
//	    requireEngagementPremium, h.CreatePoll)
//
// WHAT THIS BINDING CANNOT DO, stated honestly because implying more would be worse
// than saying nothing: it narrows OVERLAY-KEYED routes and nothing else.
// POST /api/v1/auth/chat/send has no overlay dimension at all — it fans a message out
// to the account's connected platforms — so there is no id for this middleware to
// compare and mounting it there would be theatre. On that route the SCOPE SET
// (chat:write) is what limits the device, and a compromised control surface can send
// chat as the account. The approve screen says so in as many words.
//
// A session passing through is not a gap: it was already authorized by the surrounding
// ownership checks, and it is not overlay-limited because a streamer may drive any
// overlay they own. A PAT passes through for the same reason — ADR-0051 tokens are
// user-scoped by construction, which is the residual risk the device binding removes
// for the credential that ships in a published plugin.
func RequireDeviceTokenOverlay(param string) gin.HandlerFunc {
	return func(c *gin.Context) {
		bound, isDevice := DeviceTokenOverlay(c)
		if !isDevice {
			c.Next()
			return
		}
		requested := c.Param(param)
		// A device token with no binding cannot exist (device_tokens.overlay_id is NOT
		// NULL), so an empty value here means the row shape changed under us. Refuse
		// rather than fall open: an unbound device token is precisely what this
		// middleware exists to prevent.
		if bound == "" || requested == "" || !strings.EqualFold(bound, requested) {
			revocationLog().Info("Device token refused for an overlay it is not bound to",
				zap.String("token_id", c.GetString(CtxAPITokenID)),
				zap.String("bound_overlay_id", bound),
				zap.String("requested_overlay_id", requested))
			c.JSON(http.StatusForbidden, gin.H{
				"error": "device not paired with this overlay",
				"message": "This control surface was paired with a different overlay. " +
					"Re-link it from the dashboard to control this one.",
			})
			c.Abort()
			return
		}
		c.Next()
	}
}
