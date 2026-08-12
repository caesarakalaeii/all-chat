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
	// ModeratorGrantedScopes reads the scopes on a delegated moderator's OWN credential for a
	// platform, and whether one exists at all. Never the token itself: the capability answer
	// needs no decryption.
	ModeratorGrantedScopes(ctx context.Context, userID, platform string) ([]string, bool, error)
	// DiscordIdentity reports whether a user has linked a Discord account, and which one. It is
	// Discord's analogue of the credential lookup above: the shared bot performs every write, so
	// what a Discord source needs from a person is not a token but an identity to check their
	// server permissions against (ADR-0048).
	DiscordIdentity(ctx context.Context, userID string) (string, bool, error)
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
	Dispatch(ctx context.Context, actor models.Actor, action models.Action, req models.DispatchRequest) (models.DispatchResult, error)
}

// DryRunDispatcher performs no platform calls; every dispatch is a dry run. Used in
// environments where no platform client is configured.
type DryRunDispatcher struct{}

// Dispatch always reports a dry run.
func (DryRunDispatcher) Dispatch(context.Context, models.Actor, models.Action, models.DispatchRequest) (models.DispatchResult, error) {
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

// Machine-readable codes on the action endpoints' failure bodies. The frontend switches on these
// rather than on the prose, which differs by role and is free to change.
const (
	// codeConnectRequired: the actor holds no credential for this platform. For a moderator that
	// is the deferred-consent state, and the fix is theirs to perform.
	codeConnectRequired = "connect_required"
	// codeOwnerUnverified: the overlay owner cannot be shown to control this channel, so there is
	// nothing to delegate on it. Only the owner can clear it.
	codeOwnerUnverified = "owner_channel_unverified"
	// codeDelegationUnsupported: this platform's delegated path is not built yet.
	codeDelegationUnsupported = "delegation_unsupported"
	// codeNotPlatformModerator: the platform says the caller does not moderate that channel. The
	// fix is on the platform, not in All-Chat.
	codeNotPlatformModerator = "not_moderator_on_platform"
	// The Discord refusals (ADR-0048). Discord has no per-user moderation API, so no platform
	// message comes back to explain a refusal — these codes are the whole explanation, and they
	// are separate because the remedies are separate PEOPLE's jobs. Only the first is the
	// moderator's own to clear.
	//
	// codeDiscordLinkRequired: the moderator has not linked a Discord account, so there is no
	// snowflake to read their guild permissions against. Not folded into connect_required: that
	// means a missing OAuth credential, whereas the Discord link deliberately stores no token, so
	// it is a different flow the UI has to send them to.
	codeDiscordLinkRequired = "discord_link_required"
	// codeModNotInGuild: the moderator is not a member of the Discord server.
	codeModNotInGuild = "mod_not_in_guild"
	// codeModLacksPermission: they are in the server but their roles do not carry this permission,
	// and All-Chat will not let them do through the bot what Discord would refuse them directly.
	codeModLacksPermission = "mod_lacks_permission"
	// codeModBelowTarget: Discord's role hierarchy refuses the member operation.
	codeModBelowTarget = "mod_below_target"
	// codeBotMissingPermission: the bot was never invited with the permission, so nobody can
	// borrow it. Cleared by re-inviting the bot — never by an OAuth re-consent.
	codeBotMissingPermission = "bot_missing_permission"
	// codeTargetNotActionable: the platform refused this action against this TARGET, whoever asked
	// (YouTube protects the chat owner and other moderators). Nobody can clear it, which is why it
	// is not a re-consent and not a 502: the action was understood and declined.
	codeTargetNotActionable = "target_not_actionable"
)

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

// noRoleCapabilities is the answer for a caller who may not moderate this overlay.
//
// The SAME body is returned for "you hold no role here" and "this overlay does not exist", so the
// endpoint cannot be used to test overlay ids — the action path already takes that care and the
// capability path must not undo it.
func noRoleCapabilities() models.Capabilities {
	return models.Capabilities{
		Role:    repository.RoleNone,
		Sources: []models.SourceCapability{},
	}
}

// HandleCapabilities reports, per source, which moderation actions the caller can use.
//
// Role-aware since ADR-0048: an overlay owner and a delegated moderator both get real source
// detail, computed from different authorities. The owner's comes from the scopes on their
// broadcaster credential; the moderator's from what the streamer delegated intersected with the
// scopes on the moderator's OWN credential.
func (h *Handler) HandleCapabilities(c *gin.Context) {
	cl := newCaller(c)
	if cl.userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing user context"})
		return
	}
	ctx := c.Request.Context()

	access, err := h.repo.ResolveOverlayAccess(ctx, cl.overlayID, cl.userID)
	switch {
	case errors.Is(err, repository.ErrOverlayNotFound):
		c.JSON(http.StatusOK, noRoleCapabilities())
		return
	case err != nil:
		h.logger.Error("capabilities: access resolution failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	if !access.Authorized() {
		c.JSON(http.StatusOK, noRoleCapabilities())
		return
	}

	// Feature-gate cohort check (ADR-0008), keyed on the OVERLAY OWNER exactly as authorize()
	// keys it — a premium streamer's moderators moderate for free, and a moderator must never be
	// told to upgrade a plan that is not theirs. A gate-lookup error fails closed so a transient
	// DB hiccup never opens the feature to a non-cohort user.
	enabled, err := h.gateOpenFor(ctx, access)
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

	caps := models.Capabilities{
		Role:        access.Role,
		IsOwner:     access.IsOwner(),
		Enabled:     enabled,
		CanModerate: enabled,
		Sources:     make([]models.SourceCapability, 0, len(sources)),
	}
	if !access.IsOwner() {
		// Only a grant has an action set; an owner's authority is not enumerable this way.
		caps.DelegatedActions = models.IntersectActions(models.DelegatableActions, access.Actions)
	}
	for _, s := range sources {
		if access.IsOwner() {
			caps.Sources = append(caps.Sources, h.capabilityFor(ctx, cl.userID, s))
			continue
		}
		caps.Sources = append(caps.Sources, h.delegatedCapabilityFor(ctx, cl.userID, access, s))
	}
	c.JSON(http.StatusOK, caps)
}

// gateOpenFor answers whether moderation is open for this overlay, keyed on the owner.
//
// A delegated moderator needs BOTH keys: closing `delegated_moderation` must stop delegated
// actions without touching the streamer's own write-path, which is the entire point of it being a
// second key (ADR-0048).
func (h *Handler) gateOpenFor(ctx context.Context, access repository.OverlayAccess) (bool, error) {
	if access.IsOwner() {
		return h.gate.ModerationEnabled(ctx, access.OwnerUserID)
	}
	return h.gate.DelegationEnabled(ctx, access.OwnerUserID)
}

// delegatedCapabilityFor computes one source's capability for a DELEGATED MODERATOR.
//
// Three fail-closed filters, in the order that produces the most actionable answer: what the
// streamer delegated (only they can widen it), then whether the moderator has connected their own
// account for that platform (only the moderator can clear that). Chat-send is never reported —
// moderators get no send in v1, and it is a distinct, higher-trust capability.
func (h *Handler) delegatedCapabilityFor(
	ctx context.Context, userID string, access repository.OverlayAccess, s repository.Source,
) models.SourceCapability {
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
	// The grant's platform leg and its action set are separate grants of authority, and a source
	// the streamer did not hand over is indistinguishable from one where they delegated only
	// actions this platform cannot perform: both mean "ask the streamer", not "connect your
	// account".
	delegated := models.IntersectActions(models.PlatformActions[s.Platform], access.Actions)
	if !access.MayUsePlatform(s.Platform) || len(delegated) == 0 {
		sc.Reason = models.ReasonNotDelegated
		return sc
	}
	if s.Platform == "discord" {
		return h.discordDelegatedCapabilityFor(ctx, userID, access, s, sc)
	}

	scopes, ok, err := h.repo.ModeratorGrantedScopes(ctx, userID, s.Platform)
	if err != nil {
		h.logger.Warn("capabilities: moderator scope lookup failed; treating as not connected",
			zap.String("platform", s.Platform), zap.Error(err))
		ok = false
	}
	if !ok {
		sc.Reason = models.ReasonNeedsConsent
		return sc
	}
	// The scope→action mapping is already per-platform, so the delegated action set is the only
	// narrowing left. An empty result means they consented for a narrower set than this streamer
	// delegated — they may have connected for someone who delegated less — and re-running consent
	// is exactly the fix, so it reads as "connect" rather than as a refusal.
	usable := models.IntersectActions(models.ActionsForModeratorScopes(s.Platform, scopes), access.Actions)
	if len(usable) == 0 {
		sc.Reason = models.ReasonNeedsConsent
		return sc
	}
	sc.Moderatable = true
	sc.Actions = usable
	return sc
}

// discordDelegatedCapabilityFor computes one DISCORD source's capability for a delegated moderator.
//
// Discord needs its own branch because what a person must supply here is not an OAuth credential
// but an identity: the shared bot performs every write, so All-Chat checks the acting human's own
// server permissions instead of handing over their token. Two people therefore have to be linked,
// and the whole value of saying so is that only one of them can fix each case — a volunteer told to
// link their Discord account when it is the streamer who has not is a dead end.
//
// What this deliberately does NOT do is read the moderator's live guild permissions. Capabilities
// is advisory (ADR-0048: a cached moderator state is telemetry, never authorization) and the action
// path is the authority — exactly as on Twitch, where capabilities checks the scope and Helix
// decides whether they moderate the channel. Reading them here would also put Discord API traffic
// on every dashboard load, driven by a caller-supplied overlay id, to produce an answer that can go
// stale before the button is pressed.
func (h *Handler) discordDelegatedCapabilityFor(
	ctx context.Context, userID string, access repository.OverlayAccess,
	s repository.Source, sc models.SourceCapability,
) models.SourceCapability {
	// The moderator's own link first: it is the one blocker on this list they can clear.
	if _, linked, err := h.repo.DiscordIdentity(ctx, userID); err != nil || !linked {
		if err != nil {
			h.logger.Warn("capabilities: discord identity lookup failed; treating as not linked",
				zap.String("platform", s.Platform), zap.Error(err))
		}
		sc.Reason = models.ReasonNeedsDiscordLink
		return sc
	}
	// The owner's link is what proves they still control the guild on a delegated action, so
	// without it nothing on this source is delegable. Only the streamer can clear it, which is why
	// it does not read as "link your Discord account" — the moderator would be linking the wrong
	// account and nothing would change.
	if _, linked, err := h.repo.DiscordIdentity(ctx, access.OwnerUserID); err != nil || !linked {
		if err != nil {
			h.logger.Warn("capabilities: owner discord identity lookup failed; treating as unverified",
				zap.String("platform", s.Platform), zap.Error(err))
		}
		sc.Reason = models.ReasonOwnerChannelUnverified
		return sc
	}

	// The bot's permissions in this guild are the ceiling for everyone, so they bound what can be
	// offered. The moderator's own permissions narrow it further at action time.
	botActions, err := h.scopes.GrantedActions(ctx, userID, s.Platform, s.ChannelID)
	if err != nil {
		h.logger.Warn("capabilities: discord bot permission lookup failed; reporting no actions",
			zap.String("channel_id", s.ChannelID), zap.Error(err))
		botActions = nil
	}
	usable := models.IntersectActions(botActions, access.Actions)
	if len(usable) == 0 {
		// The bot was invited without the permissions this grant covers. Re-inviting it is the
		// streamer's job, and it is the same wall the action path reports as bot_missing_permission.
		sc.Reason = models.ReasonBotMissingPermission
		return sc
	}
	sc.Moderatable = true
	sc.Actions = usable
	return sc
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
	// theirs to buy. A moderator additionally needs the `delegated_moderation` key, which is what
	// makes that key a working rollback lever rather than a label on the invite button.
	enabled, err := h.gateOpenFor(ctx, access)
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

	// ...and to the platforms whose leg the owner enabled. A separate grant of authority from the
	// action set: without this check, delegating Twitch would silently delegate every other source
	// on the overlay, since the action names are shared across platforms.
	if !access.MayUsePlatform(platform) {
		h.deny(c, *cl, platform, channelID, action, http.StatusForbidden, "this platform was not delegated to you")
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
	// Five identities must stay distinguishable forever (ADR-0048): the human who acted, their
	// role, the owner they acted for, whose credential actually acted, and the platform id we
	// sent as the moderator. The last two come back from the dispatcher, because only it knows
	// which credential it reached for — asserting them here would document an intention rather
	// than record a fact.
	e.ActorRole = cl.access.Role
	e.OnBehalfOfUserID = cl.access.OwnerUserID
	e.GrantID = cl.access.GrantID

	actor := models.Actor{
		UserID:      cl.userID,
		Role:        cl.access.Role,
		OwnerUserID: cl.access.OwnerUserID,
		GrantID:     cl.access.GrantID,
	}
	res, err := h.dispatch.Dispatch(ctx, actor, action, dreq)
	e.CredentialUserID = res.CredentialUserID
	e.PlatformActorID = res.PlatformActorID
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
		// The copy differs by role because the remedy does: a streamer is not the broadcaster of
		// this channel, whereas a moderator simply has not connected their own account yet —
		// which is the expected state of a fresh grant, since consent is deferred to first use.
		msg := "you do not hold moderator credentials for this channel"
		if actor.IsModerator() {
			msg = "connect your own " + dreq.Platform + " account to moderate here"
		}
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": msg, "code": codeConnectRequired})
		return
	case models.DispatchOwnerUnverified:
		// Delegation never exceeds what the owner could do themselves. Only the owner can fix
		// this, so a moderator is given nothing to attempt.
		//
		// The owner can reach this too — the anchor gates their own path on YouTube, where
		// credential resolution is not channel-scoped — and there the delegation wording would be
		// nonsense to someone moderating alone. Same code, same audit row, copy addressed to
		// whoever is actually reading it.
		e.Outcome = audit.OutcomeOwnerUnverified
		h.record(ctx, e)
		msg := "your " + dreq.Platform + " account isn't connected for this channel, so it can't be moderated from here"
		if actor.IsModerator() {
			msg = "this streamer's " + dreq.Platform + " account is not connected, so nothing can be delegated on this channel"
		}
		c.JSON(http.StatusForbidden, gin.H{"error": msg, "code": codeOwnerUnverified})
		return
	case models.DispatchNotPlatformModerator:
		// The platform refused, not All-Chat. Pointing this at a re-consent screen would loop a
		// volunteer through a flow that cannot fix it — the streamer has to mod them there.
		e.Outcome = audit.OutcomeNotPlatformModerator
		e.PlatformStatus = res.PlatformStatus
		h.record(ctx, e)
		c.JSON(http.StatusForbidden, gin.H{
			"error": dreq.Platform + " says you're not a moderator of this channel — ask the streamer to add you in " + dreq.Platform + "'s own tools",
			"code":  codeNotPlatformModerator,
		})
		return
	case models.DispatchDelegationUnsupported:
		e.Outcome = audit.OutcomeDelegationUnsupported
		h.record(ctx, e)
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"error": "moderators cannot act on " + dreq.Platform + " yet — ask the streamer to handle this one",
			"code":  codeDelegationUnsupported,
		})
		return
	case models.DispatchModNotLinked:
		// The one Discord refusal the moderator can clear themselves, so it is the one that gets a
		// 422 and copy addressed to them.
		e.Outcome = audit.OutcomeDiscordLinkRequired
		h.record(ctx, e)
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"error": "link your Discord account to moderate here — All-Chat checks your own server permissions before acting",
			"code":  codeDiscordLinkRequired,
		})
		return
	case models.DispatchModNotInGuild:
		e.Outcome = audit.OutcomeModNotInGuild
		h.record(ctx, e)
		c.JSON(http.StatusForbidden, gin.H{
			"error": "you're not a member of this Discord server — ask the streamer to invite you",
			"code":  codeModNotInGuild,
		})
		return
	case models.DispatchModLacksPermission:
		e.Outcome = audit.OutcomeModLacksPermission
		h.record(ctx, e)
		c.JSON(http.StatusForbidden, gin.H{
			"error": "your Discord roles don't allow this — ask the streamer to give you a role that can",
			"code":  codeModLacksPermission,
		})
		return
	case models.DispatchModBelowTarget:
		// Naming the rule matters: this refusal looks arbitrary otherwise, and it is the one case
		// where nothing about All-Chat is wrong — Discord would refuse it in its own client too.
		e.Outcome = audit.OutcomeModBelowTarget
		h.record(ctx, e)
		c.JSON(http.StatusForbidden, gin.H{
			"error": "Discord's role hierarchy blocks this — your highest role has to sit above theirs",
			"code":  codeModBelowTarget,
		})
		return
	case models.DispatchBotMissingPermission:
		e.Outcome = audit.OutcomeBotMissingPermission
		h.record(ctx, e)
		c.JSON(http.StatusForbidden, gin.H{
			"error": "the All-Chat bot wasn't given this permission — ask the streamer to re-invite it with moderation permissions",
			"code":  codeBotMissingPermission,
		})
		return
	case models.DispatchTargetNotActionable:
		// The platform declined this target for everyone. Naming that is the whole remedy: there is
		// nothing to reconnect and nobody to ask.
		e.Outcome = audit.OutcomeTargetNotActionable
		e.PlatformStatus = res.PlatformStatus
		h.record(ctx, e)
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"error": dreq.Platform + " won't let anyone moderate this person — they're the channel owner or another moderator",
			"code":  codeTargetNotActionable,
		})
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
//
// The role and the overlay owner are recorded here too, not only on successes: "a delegated
// moderator was repeatedly refused" is one of the signals that a grant has gone wrong, and it is
// invisible if a denial cannot be told apart from an owner's.
func (h *Handler) deny(c *gin.Context, cl caller, platform, channelID string, action models.Action, status int, msg string) {
	h.record(c.Request.Context(), audit.Entry{
		OverlayID:        cl.overlayID,
		ActorUserID:      cl.userID,
		ActorRole:        cl.access.Role,
		OnBehalfOfUserID: cl.access.OwnerUserID,
		GrantID:          cl.access.GrantID,
		ImpersonatedBy:   cl.impersonatedBy,
		Platform:         platform,
		ChannelID:        channelID,
		Action:           string(action),
		Outcome:          audit.OutcomeDenied,
		PlatformStatus:   msg,
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
