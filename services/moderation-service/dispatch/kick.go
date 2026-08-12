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

package dispatch

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/caesar/all-chat/services/moderation-service/clients"
	"github.com/caesar/all-chat/services/moderation-service/models"
	"github.com/caesar/all-chat/services/moderation-service/tokens"
	"go.uber.org/zap"
)

// kickTokenSource resolves and refreshes Kick broadcaster credentials, and answers the
// owner-reach anchor. *tokens.KickSource satisfies it; an interface keeps dispatch unit-testable.
type kickTokenSource interface {
	Resolve(ctx context.Context, userID, channelID string) (*tokens.KickCredential, error)
	Refresh(ctx context.Context, cred *tokens.KickCredential) error
	// OwnerKickAnchor proves the overlay OWNER controls a channel and yields the numeric
	// broadcaster_user_id. It applies no scope predicate and reads no token: an owner who
	// delegates precisely because they do not moderate themselves must still be able to delegate.
	OwnerKickAnchor(ctx context.Context, ownerUserID, channelID string) (string, error)
}

// kickAPI is the subset of clients.KickClient the dispatcher calls. DeleteMessage takes no
// broadcaster id: Kick addresses the message directly, and the channel is implied by it.
type kickAPI interface {
	DeleteMessage(ctx context.Context, token, messageID string) error
	TimeoutUser(ctx context.Context, token, broadcasterID, targetUserID string, durationSeconds int, reason string) error
	BanUser(ctx context.Context, token, broadcasterID, targetUserID, reason string) error
	UnbanUser(ctx context.Context, token, broadcasterID, targetUserID string) error
}

// Kick dispatches moderation commands to the Kick public API. It mirrors the Twitch
// dispatcher: scope pre-check, proactive + reactive refresh, and a single retry after a
// 401. Authorization (role, grant, source membership) has already happened in the handler.
type Kick struct {
	tokens kickTokenSource
	mod    modTokenSource // nil ⇒ delegation unsupported for this deployment
	api    kickAPI
	logger *zap.Logger
}

// NewKick wires a Kick dispatcher for owner actions only.
func NewKick(src kickTokenSource, api kickAPI, logger *zap.Logger) *Kick {
	return &Kick{tokens: src, api: api, logger: logger}
}

// SetModSource enables delegated moderation on Kick (ADR-0048). Until it is called, a delegated
// action is refused rather than falling back to the owner's credential — the refusal is the
// invariant, not an oversight.
func (d *Kick) SetModSource(src modTokenSource) { d.mod = src }

// kickCall is one resolved Kick write: whose token, and against which channel. Building it is
// the whole difference between an owner action and a delegated one; everything after it — scope
// pre-check, refresh, retry, error mapping — is identical.
type kickCall struct {
	accessToken   string
	broadcasterID string
	scopes        []string
	expiresAt     time.Time
	// credentialUserID is whose token this is, reported back as the proof that a delegated action
	// used the moderator's own credential and not the owner's.
	credentialUserID string
	// platformActorID is the Kick account that acted. Kick's moderation endpoints carry no
	// moderator field — the acting identity is implied by the bearer token — so unlike Twitch this
	// is not a value sent in the request. It is recorded anyway: reconciling an All-Chat audit row
	// against Kick's own moderation log needs to know which account Kick saw.
	platformActorID string
	// refresh renews this credential in place, whichever store it came from.
	refresh func(ctx context.Context) error
}

// Dispatch resolves the acting human's own Kick credential, verifies it carries the scope the
// action needs, refreshes it if expired, and calls the Kick API — retrying once after a reactive
// refresh on 401.
//
// For a delegated moderator the credential is theirs and the channel comes from the owner-reach
// anchor. Kick makes that split load-bearing in a way Twitch does not: `broadcaster_user_id` is
// the only id in the request, so resolving it from the acting user's own credential would point
// the call at the moderator's channel instead of the streamer's.
func (d *Kick) Dispatch(ctx context.Context, actor models.Actor, action models.Action, req models.DispatchRequest) (models.DispatchResult, error) {
	if req.Platform != "kick" {
		return models.DispatchResult{Outcome: models.DispatchDryRun}, nil
	}

	call, result, err := d.resolveCall(ctx, actor, req)
	if err != nil || call == nil {
		return result, err
	}

	// Scope pre-check: fail fast (no API call) when the token cannot perform the action,
	// surfacing exactly the scope the re-consent must request. Kick grants delete and
	// ban/timeout/unban separately, so this is per action, not per platform.
	need := models.RequiredKickScope(action)
	if need != "" && !hasScope(call.scopes, need) {
		return models.DispatchResult{Outcome: models.DispatchReauthRequired, MissingScopes: []string{need}}, nil
	}

	// Proactive refresh when expiry is imminent so we don't spend the first attempt on a 401.
	if !call.expiresAt.IsZero() && time.Until(call.expiresAt) < refreshLeadTime {
		if err := call.refresh(ctx); err != nil {
			d.logger.Warn("proactive kick token refresh failed; attempting with current token",
				zap.String("channel_id", req.ChannelID), zap.Error(err))
		}
	}

	proof := models.DispatchResult{
		CredentialUserID: call.credentialUserID,
		PlatformActorID:  call.platformActorID,
	}

	err = d.call(ctx, action, call, req)
	refreshed := false
	if errors.Is(err, clients.ErrKickUnauthorized) {
		// Reactive refresh + single retry: the token expired between refresh cycles.
		if rerr := call.refresh(ctx); rerr != nil {
			d.logger.Warn("reactive kick token refresh failed", zap.String("channel_id", req.ChannelID), zap.Error(rerr))
			proof.Outcome = models.DispatchReauthRequired
			proof.PlatformStatus = "token refresh failed"
			return proof, nil
		}
		refreshed = true
		err = d.call(ctx, action, call, req)
	}

	switch {
	case err == nil:
		proof.Outcome = models.DispatchPerformed
		return proof, nil
	case errors.Is(err, clients.ErrKickForbidden):
		// For a delegated moderator a 403 after a passing scope pre-check is Kick answering the
		// question the design defers to it: this person does not moderate this channel. Reporting
		// it as "re-consent" would loop a volunteer through a screen that cannot fix anything.
		if actor.IsModerator() && need != "" {
			proof.Outcome = models.DispatchNotPlatformModerator
			proof.PlatformStatus = "forbidden"
			return proof, nil
		}
		missing := []string{}
		if need != "" {
			missing = []string{need}
		}
		proof.Outcome = models.DispatchReauthRequired
		proof.MissingScopes = missing
		proof.PlatformStatus = "forbidden"
		return proof, nil
	case errors.Is(err, clients.ErrKickUnauthorized):
		// A 401 that survives a SUCCESSFUL refresh cannot honestly be about token validity: the
		// token is seconds old and the pre-check confirmed its scope. On the delegated path that
		// leaves one explanation — Kick will not let this account act on that channel — which is
		// the same conclusion a 403 carries.
		//
		// This is deliberate insurance rather than a guess about Kick's semantics. Kick's
		// 401-vs-403 mapping is undocumented (ADR-0048 lists it as an empirical unknown), and the
		// failure mode it guards against is the expensive one: a moderator sent round a consent
		// screen forever for a channel they were never modded on. An owner keeps the old answer,
		// because a broadcaster is always a moderator of their own channel, so a 401 there really
		// does point at the credential.
		if actor.IsModerator() && refreshed {
			proof.Outcome = models.DispatchNotPlatformModerator
			proof.PlatformStatus = "unauthorized after refresh"
			return proof, nil
		}
		proof.Outcome = models.DispatchReauthRequired
		proof.PlatformStatus = "unauthorized after refresh"
		return proof, nil
	default:
		// An unexpected failure still carries the attribution: "which credential did we try?" is
		// exactly the question a failed delegated action raises.
		return proof, err
	}
}

// resolveCall builds the credential + channel pair for this actor. A non-nil DispatchResult with
// a nil call means the dispatch is already decided (no credential, owner unverified, delegation
// unsupported).
func (d *Kick) resolveCall(ctx context.Context, actor models.Actor, req models.DispatchRequest) (*kickCall, models.DispatchResult, error) {
	if !actor.IsModerator() {
		cred, err := d.tokens.Resolve(ctx, actor.UserID, req.ChannelID)
		if errors.Is(err, tokens.ErrNoCredential) {
			return nil, models.DispatchResult{Outcome: models.DispatchNoCredential}, nil
		}
		if err != nil {
			return nil, models.DispatchResult{}, fmt.Errorf("resolve kick credential: %w", err)
		}
		call := &kickCall{
			accessToken:      cred.AccessToken,
			broadcasterID:    cred.BroadcasterID,
			scopes:           cred.GrantedScopes,
			expiresAt:        cred.ExpiresAt,
			credentialUserID: actor.UserID,
			platformActorID:  cred.BroadcasterID, // a streamer acting on their own channel
		}
		// The refresh writes the new token back onto the call, not just onto the credential: the
		// retry after a 401 reads from the call, so a snapshot taken at construction would
		// silently retry with the same expired token.
		call.refresh = func(ctx context.Context) error {
			if err := d.tokens.Refresh(ctx, cred); err != nil {
				return err
			}
			call.accessToken, call.expiresAt = cred.AccessToken, cred.ExpiresAt
			return nil
		}
		return call, models.DispatchResult{}, nil
	}

	if d.mod == nil {
		return nil, models.DispatchResult{Outcome: models.DispatchDelegationUnsupported}, nil
	}

	// The owner-reach anchor first: delegation never exceeds what the owner could do themselves.
	// It gates every delegated action, including delete — where its value is unused, because Kick
	// addresses the message directly, but the question it answers ("does the owner control this
	// channel at all?") is exactly as necessary there.
	broadcasterID, err := d.tokens.OwnerKickAnchor(ctx, actor.OwnerUserID, req.ChannelID)
	if errors.Is(err, tokens.ErrOwnerChannelUnverified) {
		return nil, models.DispatchResult{Outcome: models.DispatchOwnerUnverified}, nil
	}
	if err != nil {
		return nil, models.DispatchResult{}, fmt.Errorf("resolve owner kick anchor: %w", err)
	}

	cred, err := d.mod.Resolve(ctx, actor.UserID)
	if errors.Is(err, tokens.ErrNoCredential) {
		// They have not consented for Kick yet. Consent is deferred to first use, so this is the
		// normal state of a fresh grant, not a fault.
		return nil, models.DispatchResult{Outcome: models.DispatchNoCredential}, nil
	}
	if err != nil {
		return nil, models.DispatchResult{}, fmt.Errorf("resolve moderator kick credential: %w", err)
	}

	call := &kickCall{
		accessToken:   cred.AccessToken,
		broadcasterID: broadcasterID,
		scopes:        cred.GrantedScopes,
		expiresAt:     cred.ExpiresAt,
		// The moderator's own id, never the owner's. This is the field an auditor reads to confirm
		// the no-fallback invariant held.
		credentialUserID: actor.UserID,
		platformActorID:  cred.PlatformUserID,
	}
	call.refresh = func(ctx context.Context) error {
		if err := d.mod.Refresh(ctx, actor.UserID, cred); err != nil {
			return err
		}
		call.accessToken, call.expiresAt = cred.AccessToken, cred.ExpiresAt
		return nil
	}
	return call, models.DispatchResult{}, nil
}

// call routes an action to the matching Kick endpoint.
func (d *Kick) call(ctx context.Context, action models.Action, c *kickCall, req models.DispatchRequest) error {
	switch action {
	case models.ActionDelete:
		return d.api.DeleteMessage(ctx, c.accessToken, req.NativeMessageID)
	case models.ActionTimeout:
		return d.api.TimeoutUser(ctx, c.accessToken, c.broadcasterID, req.TargetUserID, req.DurationSeconds, req.Reason)
	case models.ActionBan:
		return d.api.BanUser(ctx, c.accessToken, c.broadcasterID, req.TargetUserID, req.Reason)
	case models.ActionUnban:
		return d.api.UnbanUser(ctx, c.accessToken, c.broadcasterID, req.TargetUserID)
	default:
		return fmt.Errorf("dispatch: unsupported kick action %q", action)
	}
}
