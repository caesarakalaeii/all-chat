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

package handler

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/caesar/all-chat/services/moderation-service/invites"
	"github.com/caesar/all-chat/services/moderation-service/models"
	"github.com/caesar/all-chat/services/moderation-service/repository"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"go.uber.org/zap"
)

// GrantStore is the grant-lifecycle surface of the repository (ADR-0048). Declared here per the
// codebase's interface-for-DI convention so the handlers are unit-testable with fakes.
type GrantStore interface {
	CreateInvite(ctx context.Context, p repository.InviteParams) (repository.Grant, error)
	ListGrants(ctx context.Context, overlayID string) ([]repository.Grant, error)
	UpdateGrant(ctx context.Context, overlayID, grantID string, actions []string, legs map[string]bool) (repository.Grant, error)
	RevokeGrant(ctx context.Context, overlayID, grantID, revokedBy string) (bool, error)
	RevokeAllGrants(ctx context.Context, overlayID, revokedBy string) (int, error)
	PreviewInvite(ctx context.Context, tokenHash []byte) (repository.InviteDetails, error)
	AcceptInvite(ctx context.Context, tokenHash []byte, userID string) (repository.InviteDetails, error)
}

// AccessResolver answers who the caller is on an overlay. The grant endpoints need only the role,
// but they resolve it exactly the way the action path does, so "the overlay does not exist" and
// "you have no role" stay indistinguishable everywhere.
type AccessResolver interface {
	ResolveOverlayAccess(ctx context.Context, overlayID, callerID string) (repository.OverlayAccess, error)
}

// grantEvents counts lifecycle transitions, which is what a streamer's support ticket ("someone
// removed my mods") and a security review both need. The grant rows carry the durable trail;
// this makes the rate visible.
var grantEvents = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "allchat_moderation_grant_events_total",
	Help: "Delegated-moderation grant lifecycle events.",
}, []string{"event"})

// Machine-readable codes for the delegation endpoints. The frontend keys its copy on these rather
// than on message text.
const (
	codeCapReached        = "moderator_cap_reached"
	codeGrantNotFound     = "grant_not_found"
	codeInviteNotFound    = "invite_not_found"
	codeInviteExpired     = "invite_expired"
	codeAlreadyModerator  = "already_moderator"
	codeOwnerCannotAccept = "owner_cannot_accept"
	codeInviteBound       = "invite_bound_to_other_account"
	codeUnavailable       = "delegation_unavailable"
	codeInvalidRequest    = "invalid_request"
)

// GrantHandler serves the delegated-moderator grant lifecycle: invite, accept, revoke.
//
// Every owner-side route is owner-only. Managing who may moderate is an ownership power, not a
// moderation power: a delegated moderator who could invite others, widen their own grant or revoke
// a colleague would have escalated past what the streamer handed them.
type GrantHandler struct {
	access AccessResolver
	grants GrantStore
	gate   FeatureGate
	logger *zap.Logger
}

// NewGrantHandler creates the grant-lifecycle handler. The gate defaults to OpenGate (always
// enabled) for local/dry-run deployments; production overrides it with SetFeatureGate.
func NewGrantHandler(access AccessResolver, grants GrantStore, logger *zap.Logger) *GrantHandler {
	return &GrantHandler{access: access, grants: grants, gate: OpenGate{}, logger: logger}
}

// SetFeatureGate overrides the feature gate (ADR-0008). Call once at startup before serving.
func (h *GrantHandler) SetFeatureGate(g FeatureGate) { h.gate = g }

// HandleListModerators reports the overlay's delegation roster.
//
// Never gated. A streamer must always be able to see — and therefore remove — whoever can moderate
// for them, including after a rollback of the delegation feature.
func (h *GrantHandler) HandleListModerators(c *gin.Context) {
	overlayID, _, ok := h.requireOwner(c)
	if !ok {
		return
	}

	grants, err := h.grants.ListGrants(c.Request.Context(), overlayID)
	if err != nil {
		h.internalError(c, "list grants failed", err)
		return
	}

	list := models.ModeratorList{
		Moderators: make([]models.ModeratorGrant, 0, len(grants)),
		Cap:        models.ModeratorsPerOverlayCap,
		Used:       len(grants),
	}
	for _, g := range grants {
		list.Moderators = append(list.Moderators, toModeratorGrant(g))
	}
	c.JSON(http.StatusOK, list)
}

// HandleCreateInvite mints a single-use invite.
//
// The secret is returned in this response and never again: it is stored only as a digest, so there
// is no "show it again" — a lost invite is re-minted, which is the correct trade for a token that
// can hand someone the moderation write-path on a live channel.
func (h *GrantHandler) HandleCreateInvite(c *gin.Context) {
	overlayID, access, ok := h.requireOwner(c)
	if !ok {
		return
	}

	var req models.CreateInviteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.badRequest(c, err.Error())
		return
	}

	actions, err := models.NormalizeDelegatedActions(req.Actions)
	if err != nil {
		h.badRequest(c, err.Error())
		return
	}
	platforms, err := models.NormalizeDelegatedPlatforms(req.Platforms)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error(), "code": codeInvalidRequest})
		return
	}
	if err := validatePreBinding(req.ExpectedPlatform, req.ExpectedPlatformUserID); err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, errBindingUnsupported) {
			status = http.StatusUnprocessableEntity
		}
		c.JSON(status, gin.H{"error": err.Error(), "code": codeInvalidRequest})
		return
	}

	// Entitlement keys on the overlay owner, who is also the caller here — but the copy stays
	// owner-facing, because only the streamer can act on it.
	enabled, err := h.gate.DelegationEnabled(c.Request.Context(), access.OwnerUserID)
	if err != nil {
		h.internalError(c, "delegation gate check failed", err)
		return
	}
	if !enabled {
		c.JSON(http.StatusForbidden, gin.H{
			"error":       "delegating moderation requires All-Chat premium",
			"code":        codeUnavailable,
			"upgrade_url": "/upgrade",
		})
		return
	}

	secret, err := invites.NewSecret()
	if err != nil {
		h.internalError(c, "mint invite secret failed", err)
		return
	}
	expiresAt := time.Now().Add(invites.TTL)

	grant, err := h.grants.CreateInvite(c.Request.Context(), repository.InviteParams{
		OverlayID:              overlayID,
		GrantedBy:              c.GetString("user_id"),
		Actions:                actions,
		Platforms:              platforms,
		InviteeLabel:           models.TrimInviteeLabel(req.InviteeLabel),
		ExpectedPlatform:       req.ExpectedPlatform,
		ExpectedPlatformUserID: req.ExpectedPlatformUserID,
		TokenHash:              invites.Hash(secret),
		ExpiresAt:              expiresAt,
		// An admin raising the cap for a large channel is the documented escape hatch.
		BypassCap: isAdmin(c),
	})
	switch {
	case errors.Is(err, repository.ErrModeratorCapReached):
		grantEvents.WithLabelValues("cap_reached").Inc()
		c.JSON(http.StatusConflict, gin.H{
			"error": "this overlay already has the maximum number of moderators",
			"code":  codeCapReached,
			"cap":   models.ModeratorsPerOverlayCap,
		})
		return
	case errors.Is(err, repository.ErrOverlayNotFound):
		// Racing a deletion between the access check and the insert.
		h.denyNoRole(c, overlayID, "unknown_overlay")
		return
	case err != nil:
		h.internalError(c, "create invite failed", err)
		return
	}

	grantEvents.WithLabelValues("invited").Inc()
	h.logger.Info("delegated-moderation invite created",
		zap.String("overlay_id", overlayID),
		zap.String("grant_id", grant.ID),
		zap.String("granted_by", c.GetString("user_id")),
		zap.Strings("actions", actions),
		zap.Strings("platforms", platforms))

	c.JSON(http.StatusCreated, models.InviteCreated{
		GrantID:      grant.ID,
		InviteToken:  secret,
		ExpiresAt:    expiresAt,
		Actions:      actions,
		Platforms:    platforms,
		InviteeLabel: grant.InviteeLabel,
	})
}

// HandleUpdateGrant narrows or widens a live grant: which actions it carries, and which platform
// legs are enabled.
//
// Not gated. Narrowing a grant is a de-escalation, and a closed gate must never stand between a
// streamer and taking a permission away.
func (h *GrantHandler) HandleUpdateGrant(c *gin.Context) {
	overlayID, _, ok := h.requireOwner(c)
	if !ok {
		return
	}

	var req models.UpdateGrantRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.badRequest(c, err.Error())
		return
	}

	var actions []string
	if req.Actions != nil {
		normalized, err := models.NormalizeDelegatedActions(req.Actions)
		if err != nil {
			h.badRequest(c, err.Error())
			return
		}
		actions = normalized
	}
	for platform := range req.Platforms {
		if _, err := models.NormalizeDelegatedPlatforms([]string{platform}); err != nil {
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error(), "code": codeInvalidRequest})
			return
		}
	}

	grantID := c.Param("grant_id")
	updated, err := h.grants.UpdateGrant(c.Request.Context(), overlayID, grantID, actions, req.Platforms)
	switch {
	case errors.Is(err, repository.ErrGrantNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "no such moderator on this overlay", "code": codeGrantNotFound})
		return
	case err != nil:
		h.internalError(c, "update grant failed", err)
		return
	}

	grantEvents.WithLabelValues("updated").Inc()
	h.logger.Info("delegated-moderation grant updated",
		zap.String("overlay_id", overlayID),
		zap.String("grant_id", grantID),
		zap.Strings("actions", updated.Actions))

	c.JSON(http.StatusOK, toModeratorGrant(updated))
}

// HandleRevokeGrant removes one delegation. Revocation takes effect on the very next request: the
// action path reads the grant live and never caches it.
//
// Never gated — see HandleListModerators.
func (h *GrantHandler) HandleRevokeGrant(c *gin.Context) {
	overlayID, _, ok := h.requireOwner(c)
	if !ok {
		return
	}

	grantID := c.Param("grant_id")
	revoked, err := h.grants.RevokeGrant(c.Request.Context(), overlayID, grantID, c.GetString("user_id"))
	if err != nil {
		h.internalError(c, "revoke grant failed", err)
		return
	}
	if !revoked {
		c.JSON(http.StatusNotFound, gin.H{"error": "no such moderator on this overlay", "code": codeGrantNotFound})
		return
	}

	grantEvents.WithLabelValues("revoked").Inc()
	h.logger.Info("delegated-moderation grant revoked",
		zap.String("overlay_id", overlayID),
		zap.String("grant_id", grantID),
		zap.String("revoked_by", c.GetString("user_id")))

	c.JSON(http.StatusOK, gin.H{"revoked": true})
}

// HandleRevokeAll is the kill switch: every delegation on the overlay goes at once, unredeemed
// invites included.
//
// Never gated, and deliberately blunt — the streamer reaching for this is not in a mood to
// deselect ten checkboxes.
func (h *GrantHandler) HandleRevokeAll(c *gin.Context) {
	overlayID, _, ok := h.requireOwner(c)
	if !ok {
		return
	}

	count, err := h.grants.RevokeAllGrants(c.Request.Context(), overlayID, c.GetString("user_id"))
	if err != nil {
		h.internalError(c, "revoke all grants failed", err)
		return
	}

	grantEvents.WithLabelValues("revoked_all").Add(float64(count))
	h.logger.Warn("delegated moderation revoked for a whole overlay",
		zap.String("overlay_id", overlayID),
		zap.String("revoked_by", c.GetString("user_id")),
		zap.Int("revoked", count))

	c.JSON(http.StatusOK, gin.H{"revoked": count})
}

// HandlePreviewInvite shows what an invite is for, without redeeming it: which overlay, whose, and
// what the moderator would be agreeing to do.
//
// Authentication is still required (the route sits behind the service's JWT middleware) but no
// role is: the invite secret is the authority, and the holder has no relationship to the overlay
// yet by definition.
func (h *GrantHandler) HandlePreviewInvite(c *gin.Context) {
	hash, ok := h.inviteHash(c)
	if !ok {
		return
	}

	details, err := h.grants.PreviewInvite(c.Request.Context(), hash)
	if !h.writeInviteError(c, err) {
		return
	}

	c.JSON(http.StatusOK, models.InvitePreview{
		OverlayName:      details.OverlayName,
		OwnerDisplayName: details.OwnerDisplayName,
		Actions:          details.Actions,
		Platforms:        toLegs(details.Platforms),
		ExpiresAt:        derefTime(details.InviteExpiresAt),
		InviteeLabel:     details.InviteeLabel,
		ExpectedPlatform: details.ExpectedPlatform,
		ExpectedAccount:  expectedAccount(details.Grant),
	})
}

// HandleAcceptInvite redeems an invite for the signed-in account.
//
// Acceptance is the record that this person agreed to act on someone else's behalf, and it is what
// binds a pre-bound invite to the right account. It grants nothing by itself: consent for each
// platform is deferred to the first time the moderator actually uses it.
func (h *GrantHandler) HandleAcceptInvite(c *gin.Context) {
	callerID := c.GetString("user_id")
	if callerID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing user context"})
		return
	}
	hash, ok := h.inviteHash(c)
	if !ok {
		return
	}

	details, err := h.grants.AcceptInvite(c.Request.Context(), hash, callerID)
	// Handled here rather than in writeInviteError because only acceptance can produce it, and the
	// response needs the expectation the repository returned alongside the error: naming the
	// account the invite is for turns a dead end into "sign in as that account".
	if errors.Is(err, repository.ErrInviteBoundToOtherAccount) {
		c.JSON(http.StatusConflict, gin.H{
			"error":             "this invite was created for a different account",
			"code":              codeInviteBound,
			"expected_account":  details.InviteeLabel,
			"expected_platform": details.ExpectedPlatform,
		})
		return
	}
	if !h.writeInviteError(c, err) {
		return
	}

	grantEvents.WithLabelValues("accepted").Inc()
	h.logger.Info("delegated-moderation invite accepted",
		zap.String("overlay_id", details.OverlayID),
		zap.String("grant_id", details.ID),
		zap.String("moderator_user_id", callerID))

	c.JSON(http.StatusOK, models.InviteAccepted{
		GrantID:          details.ID,
		OverlayID:        details.OverlayID,
		OverlayName:      details.OverlayName,
		OwnerDisplayName: details.OwnerDisplayName,
		Actions:          details.Actions,
		Platforms:        toLegs(details.Platforms),
	})
}

// ---------------------------------------------------------------------------
// Shared plumbing
// ---------------------------------------------------------------------------

// requireOwner resolves the caller's role and admits only the overlay owner.
//
// Every refusal — no role, a delegated moderator reaching for an owner power, or an overlay that
// does not exist — produces the identical 403 body, so these endpoints cannot be used to discover
// which overlay ids are real.
func (h *GrantHandler) requireOwner(c *gin.Context) (string, repository.OverlayAccess, bool) {
	overlayID := c.Param("id")
	callerID := c.GetString("user_id")
	if callerID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing user context"})
		return "", repository.OverlayAccess{}, false
	}

	access, err := h.access.ResolveOverlayAccess(c.Request.Context(), overlayID, callerID)
	switch {
	case errors.Is(err, repository.ErrOverlayNotFound):
		h.denyNoRole(c, overlayID, "unknown_overlay")
		return "", repository.OverlayAccess{}, false
	case err != nil:
		h.internalError(c, "overlay access resolution failed", err)
		return "", repository.OverlayAccess{}, false
	}
	if !access.IsOwner() {
		reason := "not_owner"
		if access.Role == repository.RoleModerator {
			// Worth its own label: a moderator reaching for an owner power is a different signal
			// from a stranger probing ids.
			reason = "moderator_attempted_owner_action"
		}
		h.denyNoRole(c, overlayID, reason)
		return "", repository.OverlayAccess{}, false
	}
	return overlayID, access, true
}

// inviteHash reads and hashes the submitted secret. The plaintext never leaves this function, and
// is never logged.
func (h *GrantHandler) inviteHash(c *gin.Context) ([]byte, bool) {
	var req models.InviteTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.badRequest(c, "an invite token is required")
		return nil, false
	}
	token := strings.TrimSpace(req.Token)
	if token == "" {
		h.badRequest(c, "an invite token is required")
		return nil, false
	}
	return invites.Hash(token), true
}

// writeInviteError maps the invite sentinels onto responses and reports whether the caller may
// proceed.
//
// "Unknown", "already redeemed" and "revoked" collapse into one 404: all three are equally dead
// and the difference tells the holder nothing they can act on. Expiry is distinct because whoever
// holds the secret already knows the invite was real, and "expired, ask again" beats "not found".
func (h *GrantHandler) writeInviteError(c *gin.Context, err error) bool {
	switch {
	case err == nil:
		return true
	case errors.Is(err, repository.ErrInviteNotFound):
		c.JSON(http.StatusNotFound, gin.H{
			"error": "this invite is no longer valid", "code": codeInviteNotFound})
	case errors.Is(err, repository.ErrInviteExpired):
		c.JSON(http.StatusGone, gin.H{
			"error": "this invite has expired — ask the streamer for a new one", "code": codeInviteExpired})
	case errors.Is(err, repository.ErrAlreadyModerator):
		c.JSON(http.StatusConflict, gin.H{
			"error": "you already moderate this overlay", "code": codeAlreadyModerator})
	case errors.Is(err, repository.ErrOwnerCannotAccept):
		c.JSON(http.StatusConflict, gin.H{
			"error": "this is your own overlay — you already have full moderation access",
			"code":  codeOwnerCannotAccept})
	default:
		h.internalError(c, "invite operation failed", err)
	}
	return false
}

// denyNoRole refuses a caller who is not the overlay owner, indistinguishably from a caller
// naming an overlay that does not exist. The real reason goes to the metric and the log, which the
// caller cannot see.
func (h *GrantHandler) denyNoRole(c *gin.Context, overlayID, reason string) {
	unauthorizedDenials.WithLabelValues(reason).Inc()
	h.logger.Warn("delegation management request from a caller who does not own the overlay",
		zap.String("user_id", c.GetString("user_id")),
		zap.String("overlay_id", overlayID),
		zap.String("reason", reason))
	c.JSON(http.StatusForbidden, gin.H{"error": notAuthorizedMsg})
}

func (h *GrantHandler) badRequest(c *gin.Context, msg string) {
	c.JSON(http.StatusBadRequest, gin.H{"error": msg, "code": codeInvalidRequest})
}

func (h *GrantHandler) internalError(c *gin.Context, msg string, err error) {
	h.logger.Error(msg, zap.Error(err))
	c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
}

// errBindingUnsupported marks a pre-binding to a platform whose identity acceptance cannot verify.
// It is the one pre-binding failure that is a semantic refusal (422) rather than a malformed
// request (400).
var errBindingUnsupported = errors.New("an invite can only be pre-bound to a Twitch account")

// maxPlatformUserIDLen mirrors overlay_moderators.expected_platform_user_id.
const maxPlatformUserIDLen = 100

// validatePreBinding admits a pre-binding only where acceptance can actually verify it.
func validatePreBinding(platform, platformUserID string) error {
	if platform == "" && platformUserID == "" {
		return nil
	}
	if platform == "" || platformUserID == "" {
		return errors.New("expected_platform and expected_platform_user_id must be given together")
	}
	// The message names no platform on purpose: it would be echoing unvalidated input back to the
	// caller, and the only platform that works is documented.
	if !models.PreBindablePlatform(platform) {
		return errBindingUnsupported
	}
	// The column is VARCHAR(100). Bounding it here turns an over-long value into a 400 instead of
	// letting Postgres raise 22001 and surface as a 500.
	if len(platformUserID) > maxPlatformUserIDLen {
		return errors.New("expected_platform_user_id is too long")
	}
	return nil
}

// isAdmin reports whether the caller holds the admin role, as set by the shared JWT middleware.
func isAdmin(c *gin.Context) bool {
	roles, ok := c.Get("roles")
	if !ok {
		return false
	}
	list, ok := roles.([]string)
	if !ok {
		return false
	}
	for _, role := range list {
		if role == "admin" {
			return true
		}
	}
	return false
}

func toLegs(in []repository.GrantLeg) []models.GrantPlatformLeg {
	out := make([]models.GrantPlatformLeg, 0, len(in))
	for _, leg := range in {
		out = append(out, models.GrantPlatformLeg{
			Platform:     leg.Platform,
			Enabled:      leg.Enabled,
			Verification: leg.Verification,
			VerifiedAt:   leg.VerifiedAt,
		})
	}
	return out
}

func toModeratorGrant(g repository.Grant) models.ModeratorGrant {
	return models.ModeratorGrant{
		ID:               g.ID,
		Status:           g.Status,
		ModeratorUserID:  g.ModeratorUserID,
		DisplayName:      g.ModeratorDisplayName,
		InviteeLabel:     g.InviteeLabel,
		Actions:          g.Actions,
		Platforms:        toLegs(g.Platforms),
		ExpectedPlatform: g.ExpectedPlatform,
		ExpectedAccount:  expectedAccount(g),
		CreatedAt:        g.CreatedAt,
		AcceptedAt:       g.AcceptedAt,
		InviteExpiresAt:  g.InviteExpiresAt,
		SuspendedAt:      g.SuspendedAt,
		LastActionAt:     g.LastActionAt,
	}
}

// expectedAccount names the account a pre-bound invite is for.
//
// There is no column holding the login: the streamer picks a moderator from a platform list, so
// the label they were shown IS the account name, while expected_platform_user_id holds the id we
// actually compare. Without a binding the label is a free-text note and means nothing about which
// account may redeem, so it is not reported as an expectation.
func expectedAccount(g repository.Grant) string {
	if g.ExpectedPlatform == "" {
		return ""
	}
	return g.InviteeLabel
}

func derefTime(t *time.Time) time.Time {
	if t == nil {
		return time.Time{}
	}
	return *t
}
