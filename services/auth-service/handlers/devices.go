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

// The dashboard's half of device linking (ADR-0049 step 3): approve a pending request,
// list paired devices, revoke one.
//
// Every route here is SESSION-ONLY, in exactly the sense the PAT management surface
// already means it (api_tokens.go's requireSelf, and middleware.AdminOnly): the handler
// refuses a token-authenticated request outright. A device token must not be able to
// mint or revoke devices — otherwise a single compromised Stream Deck becomes a
// self-renewing foothold that can also lock the streamer out by revoking their other
// devices. Impersonation is refused for the same reason it is on PATs: an admin acting
// as a user must not walk away with a credential that outlives the session.
//
// THE DIFFERENCE FROM api_tokens.go, and it is the important one: there is no
// "shown once" plaintext anywhere in this file. A device token's secret goes from the
// exchange endpoint to the PLUGIN over the loopback redirect. It never enters a browser,
// so the dashboard has nothing to render and no client-side secret to be careful with.
// If a future change adds a plaintext field to any response here, that is the bug.

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/caesar/all-chat/services/auth-service/repository"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// deviceStore is the persistence the dashboard handlers need. An interface so the guard
// and validation paths are unit-testable without a database.
type deviceStore interface {
	GetPendingLinkRequest(ctx context.Context, id string) (*repository.LinkRequest, error)
	FindPendingByUserCode(ctx context.Context, userCodeHash []byte) (*repository.LinkRequest, error)
	RegisterFailedAttempt(ctx context.Context, id string) (bool, error)
	ApproveLinkRequest(ctx context.Context, id, userID, overlayID string, grantedScopes []string,
		deviceName string, authCodeHash []byte, authCodeTTL time.Duration) (*repository.LinkRequest, error)
	DenyLinkRequest(ctx context.Context, id string) error
	ListDeviceTokensByUser(ctx context.Context, userID string) ([]repository.DeviceToken, error)
	RevokeDeviceToken(ctx context.Context, userID, deviceID string) (*repository.DeviceToken, error)
}

// overlayOwnershipChecker answers "does this overlay belong to this user?". Narrow on
// purpose: the approve handler needs exactly this one fact, and a narrow interface keeps
// the test double trivial.
type overlayOwnershipChecker interface {
	UserOwnsOverlay(ctx context.Context, userID, overlayID string) (bool, error)
}

// DeviceHandler serves /me/devices*.
type DeviceHandler struct {
	devices  deviceStore
	overlays overlayOwnershipChecker
	logger   *zap.Logger
}

// NewDeviceHandler wires the handler over the device-token repository.
func NewDeviceHandler(repo *repository.DeviceTokenRepository, overlays overlayOwnershipChecker, logger *zap.Logger) *DeviceHandler {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &DeviceHandler{devices: repo, overlays: overlays, logger: logger}
}

// newDeviceHandlerWithStore builds a handler over arbitrary stores. Tests only.
func newDeviceHandlerWithStore(store deviceStore, overlays overlayOwnershipChecker, logger *zap.Logger) *DeviceHandler {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &DeviceHandler{devices: store, overlays: overlays, logger: logger}
}

// approveDeviceRequest is the POST /me/devices/approve body.
type approveDeviceRequest struct {
	// RequestID identifies the pending link. Supplied by the loopback flow, where the
	// browser was opened with ?request_id=…
	RequestID string `json:"request_id"`
	// UserCode is the typed pairing code, for the fallback flow where the streamer is on
	// a different machine and there is no request id in the URL.
	UserCode string `json:"user_code"`
	// OverlayID is the overlay this device will be bound to — the whole reason the
	// approve screen exists. Required on the approve path (there is no "any overlay"
	// device token) but NOT declared `binding:"required"`, because Deny needs neither an
	// overlay nor a scope set and should not have to invent them; both are validated
	// below, after the deny branch.
	OverlayID string `json:"overlay_id"`
	// Scopes is what the streamer actually grants, which may be narrower than the
	// plugin requested.
	Scopes []string `json:"scopes"`
	// DeviceName lets the streamer rename the device at approval time. The plugin's
	// self-reported name is the default; a streamer who does not trust it can replace it.
	DeviceName string `json:"device_name"`
	// Deny terminates the request instead of approving it. Same endpoint because both
	// outcomes end the row, and a Deny that silently did nothing would leave the plugin
	// polling forever.
	Deny bool `json:"deny"`
}

// approveDeviceResponse tells the dashboard where to send the browser next.
//
// For the loopback flow that is the callback URL carrying the one-time code, which the
// dashboard navigates to so the code reaches the plugin's listener. For the code flow
// there is nothing to navigate to: the plugin is polling and will pick the token up
// itself, which is exactly why `redirect_to` is empty there.
//
// There is deliberately NO token field. See the file comment.
type approveDeviceResponse struct {
	RequestID  string   `json:"request_id"`
	Flow       string   `json:"flow"`
	DeviceName string   `json:"device_name"`
	OverlayID  string   `json:"overlay_id"`
	Scopes     []string `json:"scopes"`
	RedirectTo string   `json:"redirect_to,omitempty"`
}

// pendingLinkResponse is what the approve screen renders BEFORE the streamer decides.
// It is metadata about a request, so it is safe for a session to read — but note that
// device_name is self-reported by the plugin, and the field name in the JSON says so.
type pendingLinkResponse struct {
	RequestID string `json:"request_id"`
	Flow      string `json:"flow"`
	// DeviceNameSelfReported is the plugin's own claim about what it is. The dashboard
	// labels it as self-reported; the field name makes that hard to forget.
	DeviceNameSelfReported string   `json:"device_name_self_reported"`
	RequestedScopes        []string `json:"requested_scopes"`
	ExpiresAt              string   `json:"expires_at"`
}

// HandleGetPendingLink returns the pending request the approve screen is about to show.
//
// Route: GET /me/devices/pending?request_id=…|user_code=… (session only)
//
// A wrong typed code costs the request one of its five attempts here as well as at
// exchange, because a guess is a guess whichever endpoint it arrives at. The
// request_id path does not consume an attempt: an id is not a secret a human types, and
// making a stale dashboard tab burn attempts would be a self-inflicted denial of
// service.
func (h *DeviceHandler) HandleGetPendingLink(c *gin.Context) {
	userID, ok := requireDeviceSelf(c)
	if !ok {
		return
	}

	linkReq, ok := h.lookupPending(c, strings.TrimSpace(c.Query("request_id")), c.Query("user_code"))
	if !ok {
		return
	}

	h.logger.Debug("Device link request rendered for approval",
		zap.String("request_id", linkReq.ID), zap.String("user_id", userID))

	c.JSON(http.StatusOK, pendingLinkResponse{
		RequestID:              linkReq.ID,
		Flow:                   linkReq.Flow,
		DeviceNameSelfReported: linkReq.DeviceName,
		RequestedScopes:        linkReq.RequestedScopes,
		ExpiresAt:              linkReq.ExpiresAt.UTC().Format(time.RFC3339),
	})
}

// HandleApproveDevice binds a pending link request to the signed-in streamer, one of
// their overlays and a granted scope set — or denies it.
//
// Route: POST /me/devices/approve (session only, premium-gated by
// featuregates.GateDesktopControlSurfaces)
//
// This is the ONLY place a link request acquires an owner. Everything before it is
// inert; everything after it is a credential. The premium gate sits on this route alone,
// per ADR-0049: gating pairing rather than each action keeps enforcement in one place
// and leaves the existing per-action gates untouched.
func (h *DeviceHandler) HandleApproveDevice(c *gin.Context) {
	userID, ok := requireDeviceSelf(c)
	if !ok {
		return
	}

	var req approveDeviceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": `Body must be {"request_id"|"user_code": string, "overlay_id": string, ` +
				`"scopes": [string], "device_name": string, "deny": bool}`,
		})
		return
	}

	linkReq, ok := h.lookupPending(c, strings.TrimSpace(req.RequestID), req.UserCode)
	if !ok {
		return
	}

	if req.Deny {
		// Deny terminates the row, so the plugin's next poll gets a terminal error
		// instead of polling until the TTL runs out.
		if err := h.devices.DenyLinkRequest(c.Request.Context(), linkReq.ID); err != nil {
			h.logger.Error("Failed to deny a device link request",
				zap.String("request_id", linkReq.ID), zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to deny device"})
			return
		}
		h.logger.Info("Device link request denied",
			zap.String("request_id", linkReq.ID), zap.String("user_id", userID))
		c.JSON(http.StatusOK, gin.H{"request_id": linkReq.ID, "denied": true})
		return
	}

	overlayID := strings.TrimSpace(req.OverlayID)
	if _, err := uuid.Parse(overlayID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid overlay_id"})
		return
	}
	owns, err := h.overlays.UserOwnsOverlay(c.Request.Context(), userID, overlayID)
	if err != nil {
		h.logger.Error("Failed to check overlay ownership",
			zap.String("user_id", userID), zap.String("overlay_id", overlayID), zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to approve device"})
		return
	}
	if !owns {
		// 404, not 403: whether an overlay id exists is not something an unrelated
		// session should learn from this endpoint.
		c.JSON(http.StatusNotFound, gin.H{"error": "Overlay not found"})
		return
	}

	// Granted scopes are validated against the same closed allowlist PATs use, and
	// cannot exceed what the plugin requested — a dashboard bug must not be able to hand
	// a device more than it asked for.
	granted, err := normalizeAPITokenScopes(req.Scopes)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":          err.Error(),
			"allowed_scopes": allowedAPITokenScopeList(),
		})
		return
	}
	if extra := scopesNotIn(granted, linkReq.RequestedScopes); len(extra) > 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":            "cannot grant a scope the device did not request",
			"unrequested":      extra,
			"requested_scopes": linkReq.RequestedScopes,
		})
		return
	}

	deviceName := strings.TrimSpace(req.DeviceName)
	if len([]rune(deviceName)) > maxDeviceNameLength {
		c.JSON(http.StatusBadRequest, gin.H{"error": "device_name must be 1-120 characters"})
		return
	}

	// The one-time authorization code is minted here, at approval, and its plaintext
	// exists only in this response path. Only the digest is stored.
	authCode, authCodeHash, err := generateAuthCode()
	if err != nil {
		h.logger.Error("Failed to generate a device authorization code", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to approve device"})
		return
	}

	approved, err := h.devices.ApproveLinkRequest(c.Request.Context(), linkReq.ID, userID,
		overlayID, granted, deviceName, authCodeHash, repository.AuthCodeTTL)
	if err != nil {
		if errors.Is(err, repository.ErrLinkRequestNotFound) {
			// Either it expired between the lookup and here, or another tab approved it
			// first. `AND user_id IS NULL` in the UPDATE makes approval single-shot.
			c.JSON(http.StatusConflict, gin.H{
				"error": "This device link request is no longer pending",
			})
			return
		}
		h.logger.Error("Failed to approve a device link request",
			zap.String("request_id", linkReq.ID), zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to approve device"})
		return
	}

	h.logger.Info("Device link request approved",
		zap.String("request_id", approved.ID),
		zap.String("user_id", userID),
		zap.String("overlay_id", overlayID),
		zap.Strings("granted_scopes", granted))

	resp := approveDeviceResponse{
		RequestID:  approved.ID,
		Flow:       approved.Flow,
		DeviceName: approved.DeviceName,
		OverlayID:  overlayID,
		Scopes:     granted,
	}
	if approved.Flow == repository.FlowLoopback {
		// The dashboard navigates here; the server-side callback validates the stored
		// redirect again and builds the Location header itself, so the browser never
		// chooses where the code goes.
		resp.RedirectTo = "/api/v1/auth/device/link/callback?request_id=" +
			url.QueryEscape(approved.ID) + "&code=" + url.QueryEscape(authCode)
	} else {
		// Code flow: the plugin is polling the exchange endpoint with the code it
		// displayed, so there is nothing for the browser to do. The authorization code
		// still exists (it is what the exchange claims), it is simply delivered by the
		// plugin presenting the pairing code rather than by a redirect.
		//
		// The pairing code the plugin holds is what claims the row, so the auth code
		// minted above is not reachable by the plugin in this flow — which is why
		// ConsumeAuthCode accepts either digest.
		resp.RedirectTo = ""
	}
	c.JSON(http.StatusOK, resp)
}

// HandleListDevices returns the authenticated user's paired devices.
//
// Route: GET /me/devices (session only)
//
// The response type is repository.DeviceToken, whose struct has no field for the token
// or its digest and whose SQL projection never selects token_hash — so there is no
// serialisation path by which a secret could leak from here. Unlike the PAT list, there
// is not even a create response that could carry one.
func (h *DeviceHandler) HandleListDevices(c *gin.Context) {
	userID, ok := requireDeviceSelf(c)
	if !ok {
		return
	}

	devices, err := h.devices.ListDeviceTokensByUser(c.Request.Context(), userID)
	if err != nil {
		h.logger.Error("Failed to list paired devices",
			zap.String("user_id", userID), zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list devices"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"devices": devices})
}

// HandleRevokeDevice revokes one of the authenticated user's paired devices.
//
// Route: DELETE /me/devices/:id (session only)
//
// Revocation is read live by the resolver on every request, so it takes effect within
// one request — there is no cache to invalidate. Another user's device id yields 404,
// identical to a nonexistent one, so this cannot enumerate devices.
func (h *DeviceHandler) HandleRevokeDevice(c *gin.Context) {
	userID, ok := requireDeviceSelf(c)
	if !ok {
		return
	}

	deviceID := c.Param("id")
	if _, err := uuid.Parse(deviceID); err != nil {
		// Rejected before the query so a malformed id is a clean 400 rather than a
		// PostgreSQL cast error surfacing as a 500.
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid device id"})
		return
	}

	device, err := h.devices.RevokeDeviceToken(c.Request.Context(), userID, deviceID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Device not found"})
			return
		}
		h.logger.Error("Failed to revoke a paired device",
			zap.String("user_id", userID), zap.String("device_id", deviceID), zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to revoke device"})
		return
	}

	h.logger.Info("Paired device revoked",
		zap.String("user_id", userID), zap.String("device_id", device.ID))
	c.JSON(http.StatusOK, gin.H{"device": device})
}

// lookupPending resolves a pending request from either identifier, writing the response
// and returning false when it cannot.
//
// A typed code that misses costs the request one of its five attempts. A request id that
// misses does not: an id is not something a human guesses at, and letting a stale
// dashboard tab burn attempts would be a self-inflicted denial of service on the
// streamer's own pairing.
func (h *DeviceHandler) lookupPending(c *gin.Context, requestID, rawUserCode string) (*repository.LinkRequest, bool) {
	ctx := c.Request.Context()
	userCode := normalizeUserCode(rawUserCode)

	switch {
	case requestID != "":
		if _, err := uuid.Parse(requestID); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request_id"})
			return nil, false
		}
		linkReq, err := h.devices.GetPendingLinkRequest(ctx, requestID)
		if err != nil {
			if errors.Is(err, repository.ErrLinkRequestNotFound) {
				c.JSON(http.StatusNotFound, gin.H{
					"error":   "This link request has expired or was already used",
					"message": "Start linking again from the plugin.",
				})
				return nil, false
			}
			h.logger.Error("Failed to read a device link request",
				zap.String("request_id", requestID), zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read link request"})
			return nil, false
		}
		return linkReq, true

	case userCode != "":
		linkReq, err := h.devices.FindPendingByUserCode(ctx, hashUserCode(userCode))
		if err != nil {
			if errors.Is(err, repository.ErrLinkRequestNotFound) {
				// A wrong code cannot be attributed to a row (that is the point of
				// hashing it), so there is nothing to increment here. The bound for a
				// blind guess is the gateway rate limit; the per-request counter binds
				// an attacker who already has the request id.
				c.JSON(http.StatusNotFound, gin.H{
					"error":   "That pairing code is not valid",
					"message": "Check the code shown in the plugin, or start linking again.",
				})
				return nil, false
			}
			h.logger.Error("Failed to look up a pairing code", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read link request"})
			return nil, false
		}
		return linkReq, true

	default:
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "either request_id or user_code is required",
		})
		return nil, false
	}
}

// requireDeviceSelf resolves the authenticated user id and refuses both impersonation
// and token-authenticated requests.
//
// Identical policy to api_tokens.go's requireSelf, and stated separately rather than
// shared so the message names the right surface. The reasoning is the same: a device
// token is a bearer credential that outlives the session which minted it, so an admin
// impersonating a user must not be able to mint one (a permanent backdoor) and a leaked
// device token must not be able to mint more or revoke the victim's ability to lock it
// out.
func requireDeviceSelf(c *gin.Context) (string, bool) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return "", false
	}
	if c.GetString("impersonated_by") != "" {
		c.JSON(http.StatusForbidden, gin.H{
			"error": "Paired devices cannot be managed while impersonating",
		})
		return "", false
	}
	if !RefuseAPIToken(c, "Paired devices can only be managed from a signed-in session, not with a token.") {
		return "", false
	}
	return userID, true
}

// scopesNotIn returns the members of granted that are absent from requested.
func scopesNotIn(granted, requested []string) []string {
	have := make(map[string]bool, len(requested))
	for _, s := range requested {
		have[s] = true
	}
	var extra []string
	for _, s := range granted {
		if !have[s] {
			extra = append(extra, s)
		}
	}
	return extra
}
