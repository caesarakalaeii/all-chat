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

// Device linking for desktop control surfaces (ADR-0049 steps 2-3): the endpoints a
// Stream Deck / StreamController plugin drives to obtain a device token WITHOUT the
// streamer typing or pasting a secret.
//
// Two delivery paths, one credential, one state machine (device_link_requests,
// migration 088):
//
//	loopback (primary, RFC 8252)   plugin binds 127.0.0.1:<ephemeral>, opens the system
//	                               browser at /link?request_id=…, streamer clicks
//	                               Approve, we redirect to the loopback with a one-time
//	                               code, the plugin exchanges it with its PKCE verifier.
//	                               Nothing is typed. Nothing is pasted.
//
//	code (fallback, RFC 8628)      plugin shows XXXX-XXXX, streamer types it into /link
//	                               on whatever machine has the browser, then the plugin's
//	                               poll of /device/link/exchange starts returning a
//	                               token. For a second PC, a headless host, or a plugin
//	                               that cannot bind a port.
//
// Endpoint map, and why the auth on each is what it is:
//
//	POST /device/link/start     NO AUTH. The caller is a freshly installed plugin with no
//	                            credential of any kind — that is the whole problem being
//	                            solved. It creates nothing of value on its own: a request
//	                            row is inert until a signed-in streamer approves it.
//	                            Rate-limited at the GATEWAY (auth-service does not depend
//	                            on shared/ratelimit, which is its own Go module).
//	GET  /device/link/callback  SESSION. The loopback landing after Approve. Session-only
//	                            because it hands over the authorization code.
//	POST /device/link/exchange  NO AUTH, but possession of the one-time code IS the
//	                            authentication, plus the PKCE verifier that proves the
//	                            presenter is the same client that started the flow.
//	POST /me/devices/approve    SESSION ONLY, premium-gated. See devices.go.
//
// Nothing in this file logs a code, a verifier or a token. Log fields name a request by
// its uuid.

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"math/big"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/caesar/all-chat/services/auth-service/repository"
	"github.com/caesar/all-chat/shared/middleware"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// userCodeAlphabet is the alphabet for a typed pairing code: 32 symbols with every
// ambiguous glyph removed — no 0/O, no 1/I/L. A streamer reads this off a Stream Deck
// key or a plugin dialog and types it on another machine, sometimes on camera, so a
// character they can misread is a support ticket.
//
// 32 symbols, 8 characters = 40 bits. Combined with a 10-minute TTL and 5 attempts per
// request, guessing is not a viable attack; the per-request attempt counter, not the
// entropy alone, is the bound ADR-0049 asks to pin.
const userCodeAlphabet = "23456789ABCDEFGHJKMNPQRSTUVWXYZ"

// userCodeLength is the code length before grouping. Displayed as XXXX-XXXX.
const userCodeLength = 8

// maxDeviceNameLength matches device_tokens.name / device_link_requests.device_name
// VARCHAR(120) (migration 088).
const maxDeviceNameLength = 120

// pkceMethodS256 is the ONLY accepted code_challenge_method.
//
// `plain` is rejected outright rather than supported. A plain challenge is the verifier,
// so anything that can read the start request can complete the exchange, which removes
// the entire property PKCE exists to provide. RFC 7636 permits `plain` for clients that
// cannot compute SHA-256; both of our plugins can, and a published plugin is exactly the
// client that must not have the weaker option available.
const pkceMethodS256 = "S256"

// pollInterval is the `interval` hint returned to the plugin: how often it should poll
// the exchange endpoint while the streamer approves. Five seconds keeps a linking
// session under a handful of requests without feeling slow to a human clicking Approve.
const pollInterval = 5

// deviceLinkStore is the persistence this handler needs. An interface, not the concrete
// repository, so the validation and state-machine paths are unit-testable without a
// database — the same shape apiTokenStore uses.
type deviceLinkStore interface {
	CreateLinkRequest(ctx context.Context, flow string, userCodeHash []byte,
		pkceChallenge, pkceMethod, redirectURI, deviceName string,
		requestedScopes []string, ttl time.Duration) (*repository.LinkRequest, error)
	GetPendingLinkRequest(ctx context.Context, id string) (*repository.LinkRequest, error)
	FindPendingByUserCode(ctx context.Context, userCodeHash []byte) (*repository.LinkRequest, error)
	RegisterFailedAttempt(ctx context.Context, id string) (bool, error)
	ConsumeAuthCode(ctx context.Context, id string, authCodeHash []byte) (*repository.LinkRequest, error)
	CreateDeviceToken(ctx context.Context, userID, overlayID, name string, tokenHash []byte,
		scopes []string, lifetime time.Duration, linkRequestID string) (*repository.DeviceToken, error)
	RevokeDeviceTokenByID(ctx context.Context, deviceID string) error
}

// DeviceLinkHandler serves /device/link/*.
type DeviceLinkHandler struct {
	links deviceLinkStore
	// frontendURL is where a plugin sends the streamer to approve: FRONTEND_URL + /link.
	// Read from config rather than derived from the request, because the request arrives
	// from a plugin that must not get to choose which site the streamer is sent to.
	frontendURL string
	logger      *zap.Logger
}

// NewDeviceLinkHandler wires the handler over the device-token repository.
func NewDeviceLinkHandler(repo *repository.DeviceTokenRepository, frontendURL string, logger *zap.Logger) *DeviceLinkHandler {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &DeviceLinkHandler{
		links:       repo,
		frontendURL: strings.TrimRight(frontendURL, "/"),
		logger:      logger,
	}
}

// newDeviceLinkHandlerWithStore builds a handler over an arbitrary store. Tests only.
func newDeviceLinkHandlerWithStore(store deviceLinkStore, frontendURL string, logger *zap.Logger) *DeviceLinkHandler {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &DeviceLinkHandler{
		links:       store,
		frontendURL: strings.TrimRight(frontendURL, "/"),
		logger:      logger,
	}
}

// startLinkRequest is the POST /device/link/start body.
type startLinkRequest struct {
	// Flow is "loopback" or "code". Explicit rather than inferred from the presence of
	// redirect_uri, so a plugin that meant to use loopback and failed to bind a port
	// gets a clear rejection instead of silently falling into the other path.
	Flow string `json:"flow" binding:"required"`
	// DeviceName is SELF-REPORTED by the plugin. Untrusted for anything but display, and
	// the approve screen labels it as such.
	DeviceName string `json:"device_name"`
	// Scopes is what the plugin asks for. The streamer grants a subset on the approve
	// screen; the request is a request, not a grant.
	Scopes []string `json:"scopes"`
	// CodeChallenge is the PKCE S256 challenge (RFC 7636). Required for both flows: the
	// fallback needs it just as much, because the exchange is unauthenticated there too.
	CodeChallenge string `json:"code_challenge" binding:"required"`
	// CodeChallengeMethod must be "S256". See pkceMethodS256.
	CodeChallengeMethod string `json:"code_challenge_method"`
	// RedirectURI is required for the loopback flow and must be absent for the code
	// flow. Validated by ValidateLoopbackRedirect before it is stored.
	RedirectURI string `json:"redirect_uri"`
}

// startLinkResponse is what the plugin gets back. `user_code` is present only for the
// fallback flow; the loopback flow shows the streamer nothing to read.
type startLinkResponse struct {
	RequestID string `json:"request_id"`
	// UserCode is the grouped XXXX-XXXX form, omitted entirely for loopback.
	UserCode string `json:"user_code,omitempty"`
	// VerificationURI is where the streamer approves. For loopback it carries the
	// request id, so the plugin can just open it; for the code flow it is the bare /link
	// page the streamer types the code into.
	VerificationURI string `json:"verification_uri"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
}

// HandleStartDeviceLink opens a pending link request.
//
// Route: POST /device/link/start (NO AUTH — see the file comment)
//
// Nothing of value is created here. A request row with no user_id is inert: it cannot
// authenticate anything, and it becomes a credential only when a signed-in streamer
// approves it on a page that shows them exactly what they are granting. That is what
// makes an unauthenticated start endpoint safe, and it is why the rate limit at the
// gateway is about noise rather than about privilege.
func (h *DeviceLinkHandler) HandleStartDeviceLink(c *gin.Context) {
	var req startLinkRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": `Body must be {"flow": "loopback"|"code", "code_challenge": string, ` +
				`"code_challenge_method": "S256", "device_name": string, "scopes": [string], ` +
				`"redirect_uri": string (loopback only)}`,
		})
		return
	}

	// PKCE method first: an unsupported method is a client bug worth reporting plainly,
	// and there is no point validating anything else about a request we will refuse.
	method := strings.TrimSpace(req.CodeChallengeMethod)
	if method == "" {
		method = pkceMethodS256
	}
	if method != pkceMethodS256 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "code_challenge_method must be S256",
			"message": "`plain` is not supported: a plain challenge IS the verifier, so it " +
				"provides none of the protection PKCE exists for.",
		})
		return
	}
	if !validPKCEChallenge(req.CodeChallenge) {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "code_challenge must be 43-128 characters of base64url (RFC 7636)",
		})
		return
	}

	deviceName := strings.TrimSpace(req.DeviceName)
	if deviceName == "" {
		deviceName = "Unnamed device"
	}
	if len([]rune(deviceName)) > maxDeviceNameLength {
		c.JSON(http.StatusBadRequest, gin.H{"error": "device_name must be 1-120 characters"})
		return
	}

	// Requested scopes are validated against the same closed allowlist PATs use, so a
	// typo cannot produce a request the approve screen then renders as a mystery.
	scopes, err := normalizeAPITokenScopes(req.Scopes)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":          err.Error(),
			"allowed_scopes": allowedAPITokenScopeList(),
		})
		return
	}

	var (
		userCodeHash []byte
		userCode     string
		redirectURI  string
	)
	switch req.Flow {
	case repository.FlowLoopback:
		if strings.TrimSpace(req.RedirectURI) == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "redirect_uri is required for the loopback flow",
			})
			return
		}
		// The dedicated, narrow rule — never the general user-facing redirect guard.
		// See loopback_redirect.go for why that distinction is deliberate.
		validated, err := ValidateLoopbackRedirect(req.RedirectURI)
		if err != nil {
			// The rule that fired is logged, not returned: an attacker probing the
			// validator learns nothing from the response, while an operator debugging a
			// plugin can see exactly which clause rejected it.
			h.logger.Info("Rejected a device-link redirect_uri", zap.Error(err))
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "redirect_uri must be http://127.0.0.1:<port>" + LoopbackPath +
					" or http://[::1]:<port>" + LoopbackPath,
				"message": "The literal loopback addresses only. `localhost` is a DNS name and " +
					"can be pointed elsewhere, so it is not accepted.",
			})
			return
		}
		redirectURI = validated
	case repository.FlowCode:
		if strings.TrimSpace(req.RedirectURI) != "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "redirect_uri must be omitted for the code flow",
			})
			return
		}
		code, err := generateUserCode()
		if err != nil {
			h.logger.Error("Failed to generate a pairing code", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start device link"})
			return
		}
		userCode = code
		userCodeHash = hashUserCode(code)
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": `flow must be "loopback" or "code"`})
		return
	}

	linkReq, err := h.links.CreateLinkRequest(c.Request.Context(), req.Flow, userCodeHash,
		req.CodeChallenge, pkceMethodS256, redirectURI, deviceName, scopes,
		repository.LinkRequestTTL)
	if err != nil {
		h.logger.Error("Failed to create a device link request", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start device link"})
		return
	}

	h.logger.Info("Device link request opened",
		zap.String("request_id", linkReq.ID),
		zap.String("flow", linkReq.Flow),
		zap.Strings("requested_scopes", linkReq.RequestedScopes))

	resp := startLinkResponse{
		RequestID:       linkReq.ID,
		VerificationURI: h.verificationURI(req.Flow, linkReq.ID),
		ExpiresIn:       int(time.Until(linkReq.ExpiresAt).Seconds()),
		Interval:        pollInterval,
	}
	if userCode != "" {
		resp.UserCode = groupUserCode(userCode)
	}
	c.JSON(http.StatusCreated, resp)
}

// verificationURI builds the page the streamer approves on.
//
// The loopback flow gets ?request_id=… so the plugin can open the browser straight onto
// the right approval. The code flow deliberately does NOT, because the point of that
// path is that the streamer is on a different machine and types the code themselves.
func (h *DeviceLinkHandler) verificationURI(flow, requestID string) string {
	base := h.frontendURL + "/link"
	if flow == repository.FlowLoopback {
		return base + "?request_id=" + url.QueryEscape(requestID)
	}
	return base
}

// HandleDeviceLinkCallback is the loopback landing: it turns an approved request into a
// redirect to the plugin's listener, carrying the one-time code.
//
// Route: GET /device/link/callback?request_id=…&state=… (SESSION required)
//
// Why a server-side redirect rather than letting the frontend navigate: the redirect
// target must be the string ValidateLoopbackRedirect produced and stored, not one the
// browser (or anything running in it) supplied. Building the Location header here is
// what guarantees that.
func (h *DeviceLinkHandler) HandleDeviceLinkCallback(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	// Session-only, exactly as the PAT management surface is: a token must not be able
	// to walk a linking flow to completion on its own.
	if !RefuseAPIToken(c, "Device linking can only be completed from a signed-in session, not with a token.") {
		return
	}

	requestID := strings.TrimSpace(c.Query("request_id"))
	if _, err := uuid.Parse(requestID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request_id"})
		return
	}

	linkReq, err := h.links.GetPendingLinkRequest(c.Request.Context(), requestID)
	if err != nil {
		if errors.Is(err, repository.ErrLinkRequestNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Device link request not found or expired"})
			return
		}
		h.logger.Error("Failed to read a device link request",
			zap.String("request_id", requestID), zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to complete device link"})
		return
	}
	if linkReq.Flow != repository.FlowLoopback || linkReq.RedirectURI == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "This device link request does not use the loopback flow",
		})
		return
	}
	if linkReq.ApprovedAt == nil {
		// The approve step has not happened. The dashboard calls /me/devices/approve
		// first and only then sends the browser here, so this is either a stale tab or
		// somebody poking at the endpoint.
		c.JSON(http.StatusConflict, gin.H{
			"error": "This device link request has not been approved yet",
		})
		return
	}
	if linkReq.UserID != userID {
		// Not the approver. 404 rather than 403: whether a request id exists is not
		// something an unrelated session should be able to learn.
		c.JSON(http.StatusNotFound, gin.H{"error": "Device link request not found or expired"})
		return
	}

	// The plaintext authorization code is minted at APPROVAL and handed to the browser
	// once, here, in a Location header. It is not stored anywhere in plaintext.
	code := c.Query("code")
	if code == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing authorization code"})
		return
	}
	location, err := BuildLoopbackRedirect(linkReq.RedirectURI, code, c.Query("state"))
	if err != nil {
		h.logger.Error("Stored loopback redirect failed re-validation",
			zap.String("request_id", requestID), zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid redirect target"})
		return
	}

	h.logger.Info("Device link callback redirecting to the plugin listener",
		zap.String("request_id", requestID), zap.String("user_id", userID))
	c.Redirect(http.StatusFound, location)
}

// exchangeRequest is the POST /device/link/exchange body.
type exchangeRequest struct {
	RequestID string `json:"request_id"`
	// Code is the one-time authorization code from the loopback redirect. Absent for the
	// code flow, where approval alone releases the token to the polling plugin.
	Code string `json:"code"`
	// UserCode is the typed pairing code, for the fallback flow. Either this or
	// request_id identifies the request.
	UserCode string `json:"user_code"`
	// CodeVerifier is the PKCE verifier. Required: it is what proves the presenter is
	// the same client that started the flow, and it is what stands in for the client
	// secret a published plugin cannot have.
	CodeVerifier string `json:"code_verifier" binding:"required"`
}

// exchangeResponse hands the plugin its credential. This is the ONE place a device
// token plaintext appears in any response, and unlike a PAT it is never shown to a
// human at all — it goes machine-to-machine over the loopback.
type exchangeResponse struct {
	Token     string    `json:"token"`
	TokenType string    `json:"token_type"`
	DeviceID  string    `json:"device_id"`
	OverlayID string    `json:"overlay_id"`
	Scopes    []string  `json:"scopes"`
	ExpiresAt time.Time `json:"expires_at"`
}

// HandleExchangeDeviceLink trades an approved link request for a device token.
//
// Route: POST /device/link/exchange (NO AUTH; the code + verifier are the credential)
//
// 428 Precondition Required means "still pending, keep polling" — the plugin's signal
// that the streamer has not clicked Approve yet. Every other failure is terminal and the
// plugin must stop.
func (h *DeviceLinkHandler) HandleExchangeDeviceLink(c *gin.Context) {
	var req exchangeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": `Body must be {"request_id": string, "code": string (loopback), ` +
				`"user_code": string (code flow), "code_verifier": string}`,
		})
		return
	}

	linkReq, ok := h.resolveExchangeTarget(c, &req)
	if !ok {
		return
	}

	// PKCE verification. Done BEFORE the code is claimed for the code flow (there is no
	// separate code to burn there) and re-checked after the claim for loopback, so a
	// wrong verifier still burns a loopback code — see the ConsumeAuthCode comment on
	// why one-time has to mean that.
	if !verifyPKCE(linkReq.PKCEChallenge, req.CodeVerifier) {
		h.logger.Warn("Device link exchange failed PKCE verification",
			zap.String("request_id", linkReq.ID))
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid code_verifier"})
		return
	}

	plaintext, tokenHash, err := middleware.GenerateDeviceToken()
	if err != nil {
		h.logger.Error("Failed to generate a device token", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to complete device link"})
		return
	}

	device, err := h.links.CreateDeviceToken(c.Request.Context(), linkReq.UserID,
		linkReq.OverlayID, linkReq.DeviceName, tokenHash, linkReq.GrantedScopes,
		middleware.DeviceTokenLifetime, linkReq.ID)
	if err != nil {
		switch {
		case errors.Is(err, repository.ErrDeviceTokenLimitReached):
			c.JSON(http.StatusConflict, gin.H{
				"error":   "Paired device limit reached",
				"message": "Revoke a paired device in the dashboard before linking another.",
			})
		case errors.Is(err, repository.ErrNotFound), errors.Is(err, repository.ErrUserNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "Overlay not found"})
		default:
			// zap.Error only — no token material among the fields.
			h.logger.Error("Failed to persist a device token",
				zap.String("request_id", linkReq.ID), zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to complete device link"})
		}
		return
	}

	h.logger.Info("Device token issued",
		zap.String("request_id", linkReq.ID),
		zap.String("device_id", device.ID),
		zap.String("user_id", linkReq.UserID),
		zap.String("overlay_id", device.OverlayID),
		zap.Strings("scopes", device.Scopes))

	c.JSON(http.StatusOK, exchangeResponse{
		Token:     plaintext,
		TokenType: "Bearer",
		DeviceID:  device.ID,
		OverlayID: device.OverlayID,
		Scopes:    device.Scopes,
		ExpiresAt: device.ExpiresAt,
	})
}

// resolveExchangeTarget identifies and claims the request being exchanged, writing the
// response and returning false when it cannot.
//
// The two flows claim differently, and the difference is the point:
//
//   - loopback: a one-time authorization code exists, so ConsumeAuthCode burns it
//     atomically. A second presentation is a 400 AND revokes the token the first
//     exchange minted, because a replayed code means the code leaked.
//   - code: there is no second secret to burn — the typed code IS the identifier and the
//     streamer's approval is the authorisation — so the claim is the same ConsumeAuthCode
//     call against the digest of the code, keeping one state machine for both paths.
func (h *DeviceLinkHandler) resolveExchangeTarget(c *gin.Context, req *exchangeRequest) (*repository.LinkRequest, bool) {
	ctx := c.Request.Context()

	requestID := strings.TrimSpace(req.RequestID)
	userCode := normalizeUserCode(req.UserCode)

	// The typed-code path may identify the request by code alone: the plugin knows the
	// code it displayed, and on a second machine it may have lost the request id.
	if requestID == "" && userCode != "" {
		found, err := h.links.FindPendingByUserCode(ctx, hashUserCode(userCode))
		if err != nil {
			if errors.Is(err, repository.ErrLinkRequestNotFound) {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Unknown or expired pairing code"})
				return nil, false
			}
			h.logger.Error("Failed to look up a pairing code", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to complete device link"})
			return nil, false
		}
		requestID = found.ID
	}

	if _, err := uuid.Parse(requestID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request_id"})
		return nil, false
	}

	// Which secret claims the row: the loopback code when one was supplied, otherwise
	// the typed pairing code.
	var claimHash []byte
	switch {
	case strings.TrimSpace(req.Code) != "":
		claimHash = hashAuthCode(strings.TrimSpace(req.Code))
	case userCode != "":
		claimHash = hashUserCode(userCode)
	default:
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "either code (loopback) or user_code (pairing code) is required",
		})
		return nil, false
	}

	linkReq, err := h.links.ConsumeAuthCode(ctx, requestID, claimHash)
	switch {
	case err == nil:
		return linkReq, true

	case errors.Is(err, repository.ErrLinkRequestPending):
		// The streamer has not approved yet. 428 is the plugin's "keep polling".
		c.JSON(http.StatusPreconditionRequired, gin.H{
			"error":    "pending",
			"message":  "Waiting for the streamer to approve this device in the dashboard.",
			"interval": pollInterval,
		})
		return nil, false

	case errors.Is(err, repository.ErrLinkRequestReplayed):
		// A replay means the code leaked, so the token the FIRST exchange minted is no
		// longer trustworthy either — revoke it. Losing a working pairing is the correct
		// outcome here: re-linking costs one click, and the alternative is leaving a
		// credential alive that somebody else has seen the code for.
		if linkReq != nil && linkReq.MintedTokenID != "" {
			if rerr := h.links.RevokeDeviceTokenByID(ctx, linkReq.MintedTokenID); rerr != nil {
				h.logger.Error("Failed to revoke a device token after a code replay",
					zap.String("request_id", requestID), zap.Error(rerr))
			} else {
				h.logger.Warn("Device token revoked after an authorization code replay",
					zap.String("request_id", requestID),
					zap.String("device_id", linkReq.MintedTokenID))
			}
		}
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "This device link code has already been used.",
			"message": "The device token it produced has been revoked, because a reused code " +
				"means the code was seen by someone else. Start linking again.",
		})
		return nil, false

	case errors.Is(err, repository.ErrLinkRequestNotFound):
		// Wrong or expired secret. For the typed-code flow this is a guess, so it costs
		// the request one of its five attempts — the actual brute-force bound.
		if userCode != "" {
			if dead, aerr := h.links.RegisterFailedAttempt(ctx, requestID); aerr != nil {
				h.logger.Error("Failed to record a pairing-code attempt",
					zap.String("request_id", requestID), zap.Error(aerr))
			} else if dead {
				h.logger.Warn("Device link request exhausted its pairing-code attempts",
					zap.String("request_id", requestID))
			}
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": "Unknown or expired device link code"})
		return nil, false

	default:
		h.logger.Error("Failed to claim a device link code",
			zap.String("request_id", requestID), zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to complete device link"})
		return nil, false
	}
}

// ---------------------------------------------------------------------------
// Code generation and hashing. Nothing here logs or returns a secret.
// ---------------------------------------------------------------------------

// generateUserCode mints a typed pairing code from userCodeAlphabet.
//
// crypto/rand with rejection-free modulo-safe selection (big.Int over the alphabet
// length) — never math/rand, and never a modulo of a byte, which would bias the
// distribution towards the start of the alphabet.
func generateUserCode() (string, error) {
	limit := big.NewInt(int64(len(userCodeAlphabet)))
	out := make([]byte, userCodeLength)
	for i := range out {
		n, err := rand.Int(rand.Reader, limit)
		if err != nil {
			// crypto/rand failure is unrecoverable: never fall back to a weaker source.
			return "", err
		}
		out[i] = userCodeAlphabet[n.Int64()]
	}
	return string(out), nil
}

// groupUserCode renders a code as XXXX-XXXX for display. The hyphen is presentation
// only — normalizeUserCode strips it, so a streamer may type it or not.
func groupUserCode(code string) string {
	if len(code) != userCodeLength {
		return code
	}
	return code[:4] + "-" + code[4:]
}

// normalizeUserCode canonicalises a typed code: upper-cased, hyphens and spaces
// removed. A streamer reading a code off a screen may or may not type the hyphen, and
// may type lower case; neither should be a failed attempt against the five they get.
func normalizeUserCode(raw string) string {
	var b strings.Builder
	for _, r := range strings.ToUpper(strings.TrimSpace(raw)) {
		if r == '-' || r == ' ' {
			continue
		}
		b.WriteRune(r)
	}
	code := b.String()
	if len(code) != userCodeLength {
		// Wrong length is not a code. Returning "" makes it a clean rejection rather
		// than a digest lookup that cannot match.
		return ""
	}
	return code
}

// hashUserCode is the digest stored in device_link_requests.user_code_hash.
func hashUserCode(code string) []byte {
	sum := sha256.Sum256([]byte("allchat-device-user-code:" + code))
	return sum[:]
}

// hashAuthCode is the digest stored in device_link_requests.auth_code_hash.
//
// Domain-separated from hashUserCode by its prefix, so a value that is somehow valid in
// one namespace cannot be replayed into the other.
func hashAuthCode(code string) []byte {
	sum := sha256.Sum256([]byte("allchat-device-auth-code:" + code))
	return sum[:]
}

// generateAuthCode mints the one-time authorization code delivered over the loopback
// redirect: 256 bits from crypto/rand, base64url unpadded, so it survives a URL query
// with no escaping.
func generateAuthCode() (plaintext string, hash []byte, err error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", nil, err
	}
	plaintext = base64.RawURLEncoding.EncodeToString(buf)
	return plaintext, hashAuthCode(plaintext), nil
}

// validPKCEChallenge checks the shape of a code challenge (RFC 7636 §4.2): 43-128
// characters from the unreserved set. It says nothing about the value, which only the
// verifier presented later can validate.
func validPKCEChallenge(challenge string) bool {
	if len(challenge) < 43 || len(challenge) > 128 {
		return false
	}
	for _, r := range challenge {
		switch {
		case r >= 'A' && r <= 'Z', r >= 'a' && r <= 'z', r >= '0' && r <= '9':
		case r == '-', r == '.', r == '_', r == '~':
		default:
			return false
		}
	}
	return true
}

// verifyPKCE checks a verifier against a stored S256 challenge.
//
// SHA-256 of the verifier, base64url unpadded, compared in constant time. Constant time
// because the comparison is against a value derived from a secret and there is no reason
// to leak a prefix-match length through timing.
func verifyPKCE(challenge, verifier string) bool {
	if !validPKCEChallenge(verifier) || challenge == "" {
		return false
	}
	sum := sha256.Sum256([]byte(verifier))
	computed := base64.RawURLEncoding.EncodeToString(sum[:])
	return subtle.ConstantTimeCompare([]byte(computed), []byte(challenge)) == 1
}
