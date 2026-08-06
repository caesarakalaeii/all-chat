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

// Package handler serves the chat-moderation HTTP endpoints. Every command is
// authorized (overlay ownership + source membership, owner-only per ADR-0017) and
// audited (including the real admin when acting under impersonation). On success it
// publishes a reflect-back message_deletion so the message vanishes from overlays
// and the dashboard.
//
// Phase 0 performs no platform API calls: actions are "dry-run" (reflect-back only).
// Phase 1 introduces per-platform clients (starting with Twitch) that replace the
// dry-run path with real Helix/etc. calls.
package handler

import (
	"context"
	"errors"
	"net/http"

	mpmodels "github.com/caesar/all-chat/services/message-processor/models"
	"github.com/caesar/all-chat/services/moderation-service/audit"
	"github.com/caesar/all-chat/services/moderation-service/models"
	"github.com/caesar/all-chat/services/moderation-service/publisher"
	"github.com/caesar/all-chat/services/moderation-service/repository"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"go.uber.org/zap"
)

// Authorizer is the subset of the repository the handler needs. Declared here (per
// the codebase's interface-for-DI convention) so handlers are unit-testable with fakes.
type Authorizer interface {
	VerifyOverlayOwnership(ctx context.Context, overlayID, userID string) (bool, error)
	// ResolveOverlayAccess answers the caller's role AND the owner's identity/entitlement in
	// one round trip. The action path uses this rather than VerifyOverlayOwnership, because
	// authorization keys on the caller while the premium gate keys on the owner (ADR-0048).
	ResolveOverlayAccess(ctx context.Context, overlayID, callerID string) (repository.OverlayAccess, error)
	IsModeratableSource(ctx context.Context, overlayID, platform, channelID string) (bool, error)
	ListModeratableSources(ctx context.Context, overlayID string) ([]repository.Source, error)
	// TouchGrantActivity records that a delegated grant was just used, which is what the
	// dormancy rule reads. A no-op for owner actions, which carry no grant.
	TouchGrantActivity(ctx context.Context, grantID string) error
}

// DeletionEmitter publishes the reflect-back deletion event onto chat:raw.
type DeletionEmitter interface {
	Publish(ctx context.Context, msg *mpmodels.RawChatMessage) error
}

// Recorder writes audit rows.
type Recorder interface {
	Record(ctx context.Context, e audit.Entry) error
}

// ScopeChecker reports which moderation actions the overlay owner can currently
// perform on a (platform, channel), based on the OAuth scopes granted to the owner's
// own broadcaster credential. Phase 0 wired NoScopeChecker; Phase 1 wires the real
// per-owner lookup (tokens.TwitchScopeChecker).
type ScopeChecker interface {
	GrantedActions(ctx context.Context, userID, platform, channelID string) ([]models.Action, error)
}

// NoScopeChecker reports no granted moderation actions. Used until the opt-in
// re-consent flow lets streamers grant moderation scopes.
type NoScopeChecker struct{}

// GrantedActions always returns none.
func (NoScopeChecker) GrantedActions(context.Context, string, string, string) ([]models.Action, error) {
	return nil, nil
}

// Dispatcher performs the real platform moderation call. It owns token resolution,
// scope pre-checks, and token refresh; the handler stays platform-agnostic. A
// platform with no client wired yet reports DispatchDryRun (the handler then only
// emits the reflect-back event). The error return is reserved for unexpected /
// transient failures (mapped to 502).
type Dispatcher interface {
	Dispatch(ctx context.Context, userID string, action models.Action, req models.DispatchRequest) (models.DispatchResult, error)
}

// DryRunDispatcher performs no platform calls; every dispatch is a dry run. Used in
// environments where no platform client is configured.
type DryRunDispatcher struct{}

// Dispatch always reports a dry run.
func (DryRunDispatcher) Dispatch(context.Context, string, models.Action, models.DispatchRequest) (models.DispatchResult, error) {
	return models.DispatchResult{Outcome: models.DispatchDryRun}, nil
}

// FeatureGate reports whether a moderation capability is enabled for a user under the ADR-0008
// cohort rollout. Both questions are asked about the OVERLAY OWNER, never the caller: a delegated
// moderator moderates on a premium streamer's overlay for free (ADR-0048).
type FeatureGate interface {
	// ModerationEnabled gates the moderation write-path itself.
	ModerationEnabled(ctx context.Context, userID string) (bool, error)
	// DelegationEnabled gates handing that write-path to someone else. A separate key so
	// delegation can be rolled back without disabling owner moderation.
	DelegationEnabled(ctx context.Context, userID string) (bool, error)
}

// OpenGate enables moderation for everyone. It is the default when no feature-gate
// cache is wired (local / dry-run deployments), so moderation stays fully usable
// there; production wires a real gate via Handler.SetFeatureGate.
type OpenGate struct{}

// ModerationEnabled always reports enabled.
func (OpenGate) ModerationEnabled(context.Context, string) (bool, error) { return true, nil }

// DelegationEnabled always reports enabled.
func (OpenGate) DelegationEnabled(context.Context, string) (bool, error) { return true, nil }

// Handler serves the moderation endpoints.
type Handler struct {
	repo       Authorizer
	pub        DeletionEmitter
	audit      Recorder
	scopes     ScopeChecker
	send       SendChecker
	dispatch   Dispatcher
	gate       FeatureGate
	rediscover RediscoverPublisher // optional; nil = YouTube rediscovery unavailable
	logger     *zap.Logger
}

// New creates a moderation Handler. The feature gate defaults to OpenGate (always
// enabled); production overrides it with SetFeatureGate once the gate cache is up.
func New(repo Authorizer, pub DeletionEmitter, rec Recorder, scopes ScopeChecker, dispatch Dispatcher, logger *zap.Logger) *Handler {
	return &Handler{repo: repo, pub: pub, audit: rec, scopes: scopes, send: NoSendChecker{}, dispatch: dispatch, gate: OpenGate{}, logger: logger}
}

// SetFeatureGate overrides the moderation feature gate (ADR-0008). Call once at
// startup before serving; the default is OpenGate.
func (h *Handler) SetFeatureGate(g FeatureGate) { h.gate = g }

// SetSendChecker overrides the chat-send capability checker (defaults to NoSendChecker,
// which reports nothing sendable). Call once at startup before serving.
func (h *Handler) SetSendChecker(s SendChecker) { h.send = s }

// notAuthorizedMsg is used for BOTH "you have no role on this overlay" and "this overlay does
// not exist", so the two are indistinguishable to a caller probing overlay ids.
const notAuthorizedMsg = "not authorized for this overlay"

// unauthorizedDenials counts refusals of callers who hold no role on the overlay.
//
// These are deliberately NOT written to moderation_actions (ADR-0048): the overlay id is
// caller-supplied and that table has no foreign key on it, so probing would pad the audit log
// with rows for overlays that never existed. Denials of a legitimate owner or moderator — the
// forensically interesting ones — are still audited as ADR-0017 requires. A counter plus a Warn
// log carrying the caller's id keeps probing visible and alertable without the junk rows.
var unauthorizedDenials = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "allchat_moderation_unauthorized_denials_total",
	Help: "Moderation requests refused because the caller holds no role on the overlay.",
}, []string{"reason"})

// caller is the authenticated identity behind a request.
type caller struct {
	userID         string
	impersonatedBy string // the real admin when acting under impersonation, else ""
	overlayID      string
	// access is filled in by authorize() and drives audit attribution.
	access repository.OverlayAccess
}

func newCaller(c *gin.Context) caller {
	return caller{
		userID:         c.GetString("user_id"),
		impersonatedBy: c.GetString("impersonated_by"),
		overlayID:      c.Param("id"),
	}
}

// HandleDelete removes a single message.
func (h *Handler) HandleDelete(c *gin.Context) {
	cl := newCaller(c)
	var req models.DeleteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if !h.authorize(c, &cl, req.Platform, req.ChannelID, models.ActionDelete) {
		return
	}

	dreq := models.DispatchRequest{Platform: req.Platform, ChannelID: req.ChannelID, NativeMessageID: req.NativeMessageID}
	event := publisher.BuildSingleDeletion(req.Platform, req.ChannelID, req.NativeMessageID, req.TargetUUID)
	h.execute(c, cl, models.ActionDelete, dreq, audit.Entry{
		Platform: req.Platform, ChannelID: req.ChannelID, Action: string(models.ActionDelete),
		TargetMessageID: req.NativeMessageID,
	}, event)
}

// HandleTimeout removes a user's messages for a duration.
func (h *Handler) HandleTimeout(c *gin.Context) {
	cl := newCaller(c)
	var req models.TimeoutRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if !h.authorize(c, &cl, req.Platform, req.ChannelID, models.ActionTimeout) {
		return
	}

	dreq := models.DispatchRequest{
		Platform: req.Platform, ChannelID: req.ChannelID, TargetUserID: req.TargetUserID,
		DurationSeconds: req.DurationSeconds, Reason: req.Reason,
	}
	event := publisher.BuildBatchDeletion(req.Platform, req.ChannelID, req.TargetUserID, req.TargetUsername, req.DurationSeconds)
	h.execute(c, cl, models.ActionTimeout, dreq, audit.Entry{
		Platform: req.Platform, ChannelID: req.ChannelID, Action: string(models.ActionTimeout),
		TargetUserID: req.TargetUserID, Reason: req.Reason,
	}, event)
}

// HandleBan permanently removes a user.
func (h *Handler) HandleBan(c *gin.Context) {
	cl := newCaller(c)
	var req models.BanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if !h.authorize(c, &cl, req.Platform, req.ChannelID, models.ActionBan) {
		return
	}

	dreq := models.DispatchRequest{
		Platform: req.Platform, ChannelID: req.ChannelID, TargetUserID: req.TargetUserID, Reason: req.Reason,
	}
	// duration 0 => permanent ban (no ban_duration on the reflect-back event).
	event := publisher.BuildBatchDeletion(req.Platform, req.ChannelID, req.TargetUserID, req.TargetUsername, 0)
	h.execute(c, cl, models.ActionBan, dreq, audit.Entry{
		Platform: req.Platform, ChannelID: req.ChannelID, Action: string(models.ActionBan),
		TargetUserID: req.TargetUserID, Reason: req.Reason,
	}, event)
}

// HandleUnban lifts a ban/timeout. There is no reflect-back event (nothing is
// deleted), so only the audit row is written.
func (h *Handler) HandleUnban(c *gin.Context) {
	cl := newCaller(c)
	var req models.UnbanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if !h.authorize(c, &cl, req.Platform, req.ChannelID, models.ActionUnban) {
		return
	}

	dreq := models.DispatchRequest{Platform: req.Platform, ChannelID: req.ChannelID, TargetUserID: req.TargetUserID}
	// Unban deletes nothing, so there is no reflect-back event — only the platform
	// call (real) and the audit row.
	h.execute(c, cl, models.ActionUnban, dreq, audit.Entry{
		Platform: req.Platform, ChannelID: req.ChannelID, Action: string(models.ActionUnban),
		TargetUserID: req.TargetUserID,
	}, nil)
}

// HandleCapabilities reports, per source, which moderation actions the caller can use.
func (h *Handler) HandleCapabilities(c *gin.Context) {
	cl := newCaller(c)
	if cl.userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing user context"})
		return
	}
	ctx := c.Request.Context()
	owns, err := h.repo.VerifyOverlayOwnership(ctx, cl.overlayID, cl.userID)
	if err != nil {
		h.logger.Error("capabilities: ownership check failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	// Non-owners get a read-only view: no source detail, no controls.
	if !owns {
		c.JSON(http.StatusOK, models.Capabilities{IsOwner: false, Sources: []models.SourceCapability{}})
		return
	}

	// Feature-gate cohort check (ADR-0008). A gate-lookup error fails closed so a
	// transient DB hiccup never opens the feature to a non-cohort user.
	enabled, err := h.gate.ModerationEnabled(ctx, cl.userID)
	if err != nil {
		h.logger.Warn("capabilities: feature-gate check failed; treating moderation as disabled", zap.Error(err))
		enabled = false
	}

	sources, err := h.repo.ListModeratableSources(ctx, cl.overlayID)
	if err != nil {
		h.logger.Error("capabilities: list sources failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	caps := models.Capabilities{IsOwner: true, Enabled: enabled, Sources: make([]models.SourceCapability, 0, len(sources))}
	for _, s := range sources {
		caps.Sources = append(caps.Sources, h.capabilityFor(ctx, cl.userID, s))
	}
	c.JSON(http.StatusOK, caps)
}

// capabilityFor computes one source's capability: unsupported platforms (TikTok) are
// reported as such; supported platforms are moderatable only for the actions whose
// OAuth scopes the owner has granted (else missing_scope, prompting opt-in re-consent).
func (h *Handler) capabilityFor(ctx context.Context, userID string, s repository.Source) models.SourceCapability {
	sc := models.SourceCapability{
		Platform:    s.Platform,
		ChannelID:   s.ChannelID,
		ChannelName: s.ChannelName,
		Actions:     []models.Action{},
	}
	if !models.PlatformSupported(s.Platform) {
		sc.Reason = models.ReasonUnsupportedPlatform
		return sc
	}
	// can_send is a SEPARATE capability from moderation (a different OAuth scope), so
	// compute it up front: a source can be sendable without any moderation action
	// granted, and vice versa. A send-check failure degrades to not-sendable.
	if canSend, sErr := h.send.CanSend(ctx, userID, s.Platform, s.ChannelID); sErr != nil {
		h.logger.Warn("capabilities: send-scope check failed; treating as not sendable",
			zap.String("platform", s.Platform), zap.Error(sErr))
	} else {
		sc.CanSend = canSend
	}
	actions, err := h.scopes.GrantedActions(ctx, userID, s.Platform, s.ChannelID)
	if err != nil {
		h.logger.Warn("capabilities: scope check failed; treating as no scope",
			zap.String("platform", s.Platform), zap.Error(err))
		actions = nil
	}
	if len(actions) == 0 {
		sc.Reason = models.ReasonMissingScope
		return sc
	}
	sc.Moderatable = true
	sc.Actions = actions
	return sc
}

// authorize runs the role + entitlement + source-membership + platform-support checks
// shared by every action. On failure it audits the denial (when the caller is known) and
// writes the HTTP response. Returns true only when the action may proceed.
//
// The resolved access is stashed on the caller so execute() can attribute the audit row
// without a second lookup.
func (h *Handler) authorize(c *gin.Context, cl *caller, platform, channelID string, action models.Action) bool {
	ctx := c.Request.Context()
	if cl.userID == "" {
		// No known actor, so nothing to attribute an audit row to.
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing user context"})
		return false
	}

	access, err := h.repo.ResolveOverlayAccess(ctx, cl.overlayID, cl.userID)
	switch {
	case errors.Is(err, repository.ErrOverlayNotFound):
		h.denyUnauthorized(c, *cl, "unknown_overlay")
		return false
	case err != nil:
		h.logger.Error("overlay access resolution failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return false
	}
	cl.access = access

	if !access.Authorized() {
		h.denyUnauthorized(c, *cl, "no_role")
		return false
	}

	// Entitlement is keyed on the OVERLAY OWNER, never the caller: a premium streamer's
	// moderators moderate for free. Enforced here rather than in middleware so the denial is
	// audited like every other denial in this service, and so the copy can differ by role —
	// a delegated moderator must never be pointed at an upgrade page for a plan that is not
	// theirs to buy.
	enabled, err := h.gate.ModerationEnabled(ctx, access.OwnerUserID)
	if err != nil {
		h.logger.Error("moderation gate check failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return false
	}
	if !enabled {
		msg := "moderation requires All-Chat premium"
		if !access.IsOwner() {
			msg = "this streamer's plan does not include moderation right now"
		}
		h.deny(c, *cl, platform, channelID, action, http.StatusForbidden, msg)
		return false
	}

	// A delegated moderator is limited to the actions the owner granted. Owners may perform
	// anything the platform supports.
	if !access.MayPerform(string(action)) {
		h.deny(c, *cl, platform, channelID, action, http.StatusForbidden, "this action was not delegated to you")
		return false
	}

	if !models.SupportsAction(platform, action) {
		h.deny(c, *cl, platform, channelID, action, http.StatusUnprocessableEntity, "platform does not support this moderation action")
		return false
	}

	// Unchanged, and now security-critical rather than incidental: this is what keeps a
	// shared_overlay source non-moderatable. Owner-only authorization made "a recipient must
	// not moderate the original streamer's channel" true by construction; role-based
	// authorization does not, so the predicate carries the invariant on its own.
	moderatable, err := h.repo.IsModeratableSource(ctx, cl.overlayID, platform, channelID)
	if err != nil {
		h.logger.Error("source check failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return false
	}
	if !moderatable {
		h.deny(c, *cl, platform, channelID, action, http.StatusUnprocessableEntity, "channel is not a moderatable source on this overlay")
		return false
	}
	return true
}

// execute performs the platform call (via the Dispatcher), then on success/dry-run
// emits the reflect-back event (if any), audits the outcome, and responds. event may
// be nil (e.g. unban). A reauth/no-credential dispatch result short-circuits with the
// appropriate status and NO reflect-back (nothing happened on the platform).
func (h *Handler) execute(c *gin.Context, cl caller, action models.Action, dreq models.DispatchRequest, e audit.Entry, event *mpmodels.RawChatMessage) {
	ctx := c.Request.Context()
	e.OverlayID = cl.overlayID
	e.ActorUserID = cl.userID
	e.ImpersonatedBy = cl.impersonatedBy

	res, err := h.dispatch.Dispatch(ctx, cl.userID, action, dreq)
	if err != nil {
		h.logger.Error("platform dispatch failed", zap.Error(err), zap.String("action", e.Action))
		e.Outcome = audit.OutcomePlatformError
		e.PlatformStatus = err.Error()
		h.record(ctx, e)
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to apply moderation"})
		return
	}

	switch res.Outcome {
	case models.DispatchNoCredential:
		e.Outcome = audit.OutcomeNoCredential
		h.record(ctx, e)
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "you do not hold moderator credentials for this channel"})
		return
	case models.DispatchReauthRequired:
		e.Outcome = audit.OutcomeReauthRequired
		e.PlatformStatus = res.PlatformStatus
		h.record(ctx, e)
		c.JSON(http.StatusForbidden, gin.H{
			"error":           "moderation re-consent required",
			"requires_reauth": true,
			"missing_scopes":  res.MissingScopes,
		})
		return
	}

	// Performed or DryRun: apply the reflect-back so the message vanishes live.
	if event != nil {
		if err := h.pub.Publish(ctx, event); err != nil {
			h.logger.Error("failed to publish reflect-back deletion", zap.Error(err), zap.String("action", e.Action))
			e.Outcome = audit.OutcomePlatformError
			e.PlatformStatus = err.Error()
			h.record(ctx, e)
			c.JSON(http.StatusBadGateway, gin.H{"error": "failed to apply moderation"})
			return
		}
	}

	dryRun := res.Outcome == models.DispatchDryRun
	if dryRun {
		e.Outcome = audit.OutcomeDryRun
	} else {
		e.Outcome = audit.OutcomeSuccess
	}
	h.record(ctx, e)
	h.touchGrant(ctx, cl)
	c.JSON(http.StatusOK, gin.H{"status": "ok", "dry_run": dryRun})
}

// touchGrant stamps a delegated grant's last successful action.
//
// This is what the 90-day dormancy suspension reads, so it is written from the very first delegated
// action: a dormancy job introduced later must not find a working mod team looking idle since the
// day their grant was created. Owners have no grant to stamp. A failure is logged, never surfaced —
// the moderation action already succeeded and the stamp is bookkeeping.
func (h *Handler) touchGrant(ctx context.Context, cl caller) {
	if cl.access.GrantID == "" {
		return
	}
	if err := h.repo.TouchGrantActivity(ctx, cl.access.GrantID); err != nil {
		h.logger.Warn("failed to stamp grant activity",
			zap.String("grant_id", cl.access.GrantID), zap.Error(err))
	}
}

// denyUnauthorized refuses a caller who holds no role on the overlay.
//
// The response is byte-identical whether the overlay is unauthorized or does not exist at all —
// including the absence of an audit row — because any observable difference, side effects
// included, would make these endpoints an overlay-existence oracle for any valid token holder.
// The reason is recorded in the metric and log, which the caller cannot see.
func (h *Handler) denyUnauthorized(c *gin.Context, cl caller, reason string) {
	unauthorizedDenials.WithLabelValues(reason).Inc()
	h.logger.Warn("moderation request from a caller with no role on the overlay",
		zap.String("user_id", cl.userID),
		zap.String("overlay_id", cl.overlayID),
		zap.String("reason", reason))
	c.JSON(http.StatusForbidden, gin.H{"error": notAuthorizedMsg})
}

// deny audits an authorization failure (when the caller is known) and responds.
func (h *Handler) deny(c *gin.Context, cl caller, platform, channelID string, action models.Action, status int, msg string) {
	h.record(c.Request.Context(), audit.Entry{
		OverlayID:      cl.overlayID,
		ActorUserID:    cl.userID,
		ImpersonatedBy: cl.impersonatedBy,
		Platform:       platform,
		ChannelID:      channelID,
		Action:         string(action),
		Outcome:        audit.OutcomeDenied,
		PlatformStatus: msg,
	})
	c.JSON(status, gin.H{"error": msg})
}

// record writes an audit row, logging (not failing the request) on error.
func (h *Handler) record(ctx context.Context, e audit.Entry) {
	if err := h.audit.Record(ctx, e); err != nil {
		h.logger.Error("failed to record moderation audit",
			zap.String("action", e.Action), zap.String("outcome", e.Outcome), zap.Error(err))
	}
}
