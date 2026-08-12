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

// Package dispatch turns an authorized, platform-agnostic moderation command into a
// real platform API call. It owns credential resolution, the granted-scope
// pre-check, proactive + reactive token refresh, and the mapping of platform errors
// to the handler's DispatchResult. The handler stays platform-agnostic; adding a
// platform (Kick/Discord/YouTube) means extending the dispatch switch, not the HTTP
// layer.
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

// refreshLeadTime: refresh proactively when the access token is this close to expiry,
// so a foreseeable expiry does not cost the user a failed first attempt.
const refreshLeadTime = 5 * time.Minute

// twitchTokenSource resolves and refreshes broadcaster credentials, and answers the owner-reach
// anchor. *tokens.TwitchSource satisfies it; an interface keeps dispatch unit-testable.
type twitchTokenSource interface {
	Resolve(ctx context.Context, userID, channelID string) (*tokens.TwitchCredential, error)
	Refresh(ctx context.Context, cred *tokens.TwitchCredential) error
	// OwnerTwitchAnchor proves the overlay OWNER controls a channel and yields the numeric
	// broadcaster id. It applies no scope predicate and reads no token: an owner who delegates
	// precisely because they do not moderate themselves must still be able to delegate.
	OwnerTwitchAnchor(ctx context.Context, ownerUserID, channelID string) (string, error)
}

// modTokenSource resolves and refreshes a delegated moderator's OWN credential for one platform.
// *tokens.ModTwitchSource and *tokens.ModKickSource satisfy it — the shape is platform-agnostic
// because the credential is keyed on the moderator alone, never on a channel.
type modTokenSource interface {
	Resolve(ctx context.Context, userID string) (*tokens.ModCredential, error)
	Refresh(ctx context.Context, userID string, cred *tokens.ModCredential) error
}

// twitchAPI is the subset of clients.TwitchClient the dispatcher calls.
type twitchAPI interface {
	DeleteMessage(ctx context.Context, token, broadcasterID, moderatorID, nativeMessageID string) error
	TimeoutUser(ctx context.Context, token, broadcasterID, moderatorID, targetUserID string, durationSeconds int, reason string) error
	BanUser(ctx context.Context, token, broadcasterID, moderatorID, targetUserID, reason string) error
	UnbanUser(ctx context.Context, token, broadcasterID, moderatorID, targetUserID string) error
}

// Twitch dispatches moderation commands to the Twitch Helix API. Platforms other
// than Twitch report DispatchDryRun (their clients ship in later phases).
type Twitch struct {
	tokens twitchTokenSource
	mod    modTokenSource // nil ⇒ delegation unsupported for this deployment
	api    twitchAPI
	logger *zap.Logger
}

// NewTwitch wires a Twitch dispatcher for owner actions only.
func NewTwitch(src twitchTokenSource, api twitchAPI, logger *zap.Logger) *Twitch {
	return &Twitch{tokens: src, api: api, logger: logger}
}

// SetModSource enables delegated moderation (ADR-0048). Until it is called, a delegated action is
// refused rather than falling back to the owner's credential — the refusal is the invariant, not
// an oversight.
func (d *Twitch) SetModSource(src modTokenSource) { d.mod = src }

// twitchCall is one resolved Twitch write: whose token, against which channel, as which
// moderator. Building it is the whole difference between an owner action and a delegated one;
// everything after it — scope pre-check, refresh, retry, error mapping — is identical.
type twitchCall struct {
	accessToken   string
	broadcasterID string
	moderatorID   string
	scopes        []string
	expiresAt     time.Time
	// credentialUserID is whose token this is, reported back as the proof that a delegated action
	// used the moderator's own credential and not the owner's.
	credentialUserID string
	// refresh renews this credential in place, whichever store it came from.
	refresh func(ctx context.Context) error
}

// Dispatch resolves the acting human's own Twitch credential, verifies it carries the scope the
// action needs, refreshes it if expired, and calls Helix — retrying once after a reactive refresh
// on 401. Authorization (role, grant, source membership) has already happened in the handler.
//
// For a delegated moderator the credential is theirs, and the channel comes from the owner-reach
// anchor. There is deliberately no fallback between the two: if the moderator has not consented,
// the action fails rather than quietly running as the streamer.
func (d *Twitch) Dispatch(ctx context.Context, actor models.Actor, action models.Action, req models.DispatchRequest) (models.DispatchResult, error) {
	if req.Platform != "twitch" {
		// No client for this platform yet — keep the reflect-back-only dry run.
		return models.DispatchResult{Outcome: models.DispatchDryRun}, nil
	}

	call, result, err := d.resolveCall(ctx, actor, req)
	if err != nil || call == nil {
		return result, err
	}

	// Scope pre-check: fail fast (no API call) when the token cannot perform the
	// action, surfacing exactly the scope the opt-in re-consent must request.
	need := models.RequiredTwitchScope(action)
	if need != "" && !hasScope(call.scopes, need) {
		return models.DispatchResult{Outcome: models.DispatchReauthRequired, MissingScopes: []string{need}}, nil
	}

	// Proactive refresh: token-refresh-service keeps tokens fresh, but refresh here
	// too if expiry is imminent so we don't spend the first attempt on a 401.
	if !call.expiresAt.IsZero() && time.Until(call.expiresAt) < refreshLeadTime {
		if err := call.refresh(ctx); err != nil {
			d.logger.Warn("proactive token refresh failed; attempting with current token",
				zap.String("channel_id", req.ChannelID), zap.Error(err))
		}
	}

	proof := models.DispatchResult{
		CredentialUserID: call.credentialUserID,
		PlatformActorID:  call.moderatorID,
	}

	err = d.call(ctx, action, call, req)
	if errors.Is(err, clients.ErrUnauthorized) {
		// Reactive refresh + single retry: the token expired between refresh cycles.
		if rerr := call.refresh(ctx); rerr != nil {
			d.logger.Warn("reactive token refresh failed", zap.String("channel_id", req.ChannelID), zap.Error(rerr))
			proof.Outcome = models.DispatchReauthRequired
			proof.PlatformStatus = "token refresh failed"
			return proof, nil
		}
		err = d.call(ctx, action, call, req)
	}

	switch {
	case err == nil:
		proof.Outcome = models.DispatchPerformed
		return proof, nil
	case errors.Is(err, clients.ErrForbidden):
		// Helix accepted the token and refused the action. For a delegated moderator that is
		// almost never about scope — the pre-check above already confirmed the scope is present —
		// so it is Twitch answering the question the whole design defers to it: this person is not
		// a moderator of this channel. Reporting it as "re-authorize" would loop a volunteer
		// through a consent screen that cannot fix anything; the remediation is the streamer
		// modding them on Twitch.
		if actor.IsModerator() && need != "" {
			proof.Outcome = models.DispatchNotPlatformModerator
			proof.PlatformStatus = "forbidden"
			return proof, nil
		}
		// Owner path unchanged: a broadcaster is always a moderator of their own channel, so a
		// 403 there really is about the grant.
		if need != "" {
			proof.MissingScopes = []string{need}
		}
		proof.Outcome = models.DispatchReauthRequired
		proof.PlatformStatus = "forbidden"
		return proof, nil
	case errors.Is(err, clients.ErrUnauthorized):
		proof.Outcome = models.DispatchReauthRequired
		proof.PlatformStatus = "unauthorized after refresh"
		return proof, nil
	default:
		// An unexpected failure still carries the attribution: "which credential did we try?" is
		// exactly the question a failed delegated action raises, and dropping it here would leave
		// the audit row unable to answer it.
		return proof, err
	}
}

// resolveCall builds the credential + channel pair for this actor. A non-nil DispatchResult with
// a nil call means the dispatch is already decided (no credential, owner unverified, delegation
// unsupported).
func (d *Twitch) resolveCall(ctx context.Context, actor models.Actor, req models.DispatchRequest) (*twitchCall, models.DispatchResult, error) {
	if !actor.IsModerator() {
		cred, err := d.tokens.Resolve(ctx, actor.UserID, req.ChannelID)
		if errors.Is(err, tokens.ErrNoCredential) {
			return nil, models.DispatchResult{Outcome: models.DispatchNoCredential}, nil
		}
		if err != nil {
			return nil, models.DispatchResult{}, fmt.Errorf("resolve twitch credential: %w", err)
		}
		// A streamer moderating their own channel IS the moderator, so Helix gets the same id
		// twice. This is the one case where they legitimately coincide.
		call := &twitchCall{
			accessToken:      cred.AccessToken,
			broadcasterID:    cred.BroadcasterID,
			moderatorID:      cred.BroadcasterID,
			scopes:           cred.GrantedScopes,
			expiresAt:        cred.ExpiresAt,
			credentialUserID: actor.UserID,
		}
		// The refresh writes the new token back onto the call, not just onto the credential:
		// the retry after a 401 reads from the call, so a snapshot taken at construction would
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

	// The owner-reach anchor first: delegation never exceeds what the owner could do themselves,
	// and this is also the only honest source of the broadcaster id — the moderator's credential
	// knows nothing about the streamer's channel.
	broadcasterID, err := d.tokens.OwnerTwitchAnchor(ctx, actor.OwnerUserID, req.ChannelID)
	if errors.Is(err, tokens.ErrOwnerChannelUnverified) {
		return nil, models.DispatchResult{Outcome: models.DispatchOwnerUnverified}, nil
	}
	if err != nil {
		return nil, models.DispatchResult{}, fmt.Errorf("resolve owner anchor: %w", err)
	}

	cred, err := d.mod.Resolve(ctx, actor.UserID)
	if errors.Is(err, tokens.ErrNoCredential) {
		// They have not consented for Twitch yet. Consent is deferred to first use, so this is
		// the normal state of a fresh grant, not a fault.
		return nil, models.DispatchResult{Outcome: models.DispatchNoCredential}, nil
	}
	if err != nil {
		return nil, models.DispatchResult{}, fmt.Errorf("resolve moderator twitch credential: %w", err)
	}

	call := &twitchCall{
		accessToken:   cred.AccessToken,
		broadcasterID: broadcasterID,
		moderatorID:   cred.PlatformUserID,
		scopes:        cred.GrantedScopes,
		expiresAt:     cred.ExpiresAt,
		// The moderator's own id, never the owner's. This is the field an auditor reads to
		// confirm the no-fallback invariant held.
		credentialUserID: actor.UserID,
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

// call routes an action to the matching Helix endpoint.
func (d *Twitch) call(ctx context.Context, action models.Action, c *twitchCall, req models.DispatchRequest) error {
	switch action {
	case models.ActionDelete:
		return d.api.DeleteMessage(ctx, c.accessToken, c.broadcasterID, c.moderatorID, req.NativeMessageID)
	case models.ActionTimeout:
		return d.api.TimeoutUser(ctx, c.accessToken, c.broadcasterID, c.moderatorID, req.TargetUserID, req.DurationSeconds, req.Reason)
	case models.ActionBan:
		return d.api.BanUser(ctx, c.accessToken, c.broadcasterID, c.moderatorID, req.TargetUserID, req.Reason)
	case models.ActionUnban:
		return d.api.UnbanUser(ctx, c.accessToken, c.broadcasterID, c.moderatorID, req.TargetUserID)
	default:
		return fmt.Errorf("dispatch: unsupported twitch action %q", action)
	}
}

func hasScope(scopes []string, want string) bool {
	for _, s := range scopes {
		if s == want {
			return true
		}
	}
	return false
}
