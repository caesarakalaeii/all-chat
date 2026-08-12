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
	"github.com/caesar/all-chat/shared/quota"
	"go.uber.org/zap"
)

// youtubeTokenSource resolves and refreshes YouTube broadcaster credentials, and answers the
// owner-reach anchor. *tokens.YouTubeSource satisfies it.
type youtubeTokenSource interface {
	Resolve(ctx context.Context, userID, channelID string) (*tokens.YouTubeCredential, error)
	Refresh(ctx context.Context, cred *tokens.YouTubeCredential) error
	// OwnerYouTubeAnchor proves a user controls a channel. It yields no id — a YouTube write is
	// addressed by the broadcast's liveChatId — so it is a pure gate.
	OwnerYouTubeAnchor(ctx context.Context, ownerUserID, channelID string) error
}

// youtubeAPI is the subset of clients.YouTubeClient the dispatcher calls. Both actions are the same
// liveChatBans.insert endpoint — YouTube models a timeout as a temporary ban.
type youtubeAPI interface {
	BanUser(ctx context.Context, token, liveChatID, bannedChannelID string) error
	TimeoutUser(ctx context.Context, token, liveChatID, bannedChannelID string, durationSeconds int) error
}

// liveChatResolver resolves a channel's active liveChatId (Redis stream-state cache).
type liveChatResolver interface {
	Resolve(ctx context.Context, channelID string) (string, error)
}

// quotaReserver implements ADR-0006 reserve-confirm-rollback for the YouTube ban cost.
type quotaReserver interface {
	Reserve(ctx context.Context, units int) (bool, error)
	Confirm(ctx context.Context, units int) error
	Rollback(ctx context.Context, units int) error
}

// YouTube dispatches timeout/ban moderation to the YouTube Data API with the acting human's own
// credential. It resolves the live broadcast's liveChatId from the listener's cache (no
// search.list), reserves quota before the call, and confirms or rolls it back by the outcome.
type YouTube struct {
	tokens   youtubeTokenSource
	mod      modTokenSource // nil ⇒ delegation unsupported for this deployment
	api      youtubeAPI
	liveChat liveChatResolver
	quota    quotaReserver
	logger   *zap.Logger
}

// NewYouTube wires a YouTube dispatcher for owner actions only.
func NewYouTube(src youtubeTokenSource, api youtubeAPI, liveChat liveChatResolver, q quotaReserver, logger *zap.Logger) *YouTube {
	return &YouTube{tokens: src, api: api, liveChat: liveChat, quota: q, logger: logger}
}

// SetModSource enables delegated moderation on YouTube (ADR-0048). Until it is called, a delegated
// action is refused rather than falling back to the owner's credential.
func (d *YouTube) SetModSource(src modTokenSource) { d.mod = src }

// youtubeCall is one resolved YouTube write: whose token performs it. There is no channel id to
// carry — the liveChatId does that job — so unlike Twitch and Kick the owner-reach anchor
// contributes a decision here rather than a value.
type youtubeCall struct {
	accessToken      string
	scopes           []string
	expiresAt        time.Time
	credentialUserID string
	platformActorID  string
	refresh          func(ctx context.Context) error
}

// Dispatch times out or bans a user in a channel's live chat.
//
// For a delegated moderator the credential is theirs, with no fallback to the owner's. YouTube then
// re-checks on every call that the token's account owns or moderates that live chat, which is the
// authority ADR-0048 defers to: All-Chat never caches a moderator status.
func (d *YouTube) Dispatch(ctx context.Context, actor models.Actor, action models.Action, req models.DispatchRequest) (models.DispatchResult, error) {
	if req.Platform != "youtube" {
		return models.DispatchResult{Outcome: models.DispatchDryRun}, nil
	}
	if action != models.ActionBan && action != models.ActionTimeout {
		// Delete and unban are unsupported for lack of a usable id, not a scope — see clients/youtube.go.
		return models.DispatchResult{}, fmt.Errorf("dispatch: unsupported youtube action %q", action)
	}

	call, decided, err := d.resolveCall(ctx, actor, req)
	if err != nil || call == nil {
		return decided, err
	}

	need := models.RequiredYouTubeScope(action)
	if need != "" && !hasScope(call.scopes, need) {
		return models.DispatchResult{Outcome: models.DispatchReauthRequired, MissingScopes: []string{need}}, nil
	}

	// liveChatId from the listener's cache — the action needs the active broadcast's chat.
	liveChatID, err := d.liveChat.Resolve(ctx, req.ChannelID)
	if err != nil {
		// Not live / not cached, or a Redis error: cannot act. Surfaced as a platform
		// failure (no reflect-back) rather than a re-consent.
		return models.DispatchResult{}, fmt.Errorf("resolve youtube live chat: %w", err)
	}

	// Proactive refresh near expiry so a foreseeable expiry doesn't cost the attempt.
	if !call.expiresAt.IsZero() && time.Until(call.expiresAt) < refreshLeadTime {
		if err := call.refresh(ctx); err != nil {
			d.logger.Warn("proactive youtube token refresh failed; attempting with current token",
				zap.String("channel_id", req.ChannelID), zap.Error(err))
		}
	}

	// Reserve quota (ADR-0006) before the API call; confirm on success, roll back on failure.
	ok, err := d.quota.Reserve(ctx, quota.QuotaCostBan)
	if err != nil {
		return models.DispatchResult{}, fmt.Errorf("reserve youtube quota: %w", err)
	}
	if !ok {
		return models.DispatchResult{}, errors.New("youtube daily quota exhausted; moderation not attempted")
	}

	proof := models.DispatchResult{
		CredentialUserID: call.credentialUserID,
		PlatformActorID:  call.platformActorID,
	}

	err = d.call(ctx, action, call.accessToken, liveChatID, req)
	if errors.Is(err, clients.ErrYouTubeUnauthorized) {
		if rerr := call.refresh(ctx); rerr != nil {
			d.logger.Warn("reactive youtube token refresh failed", zap.String("channel_id", req.ChannelID), zap.Error(rerr))
			d.rollback(ctx)
			proof.Outcome = models.DispatchReauthRequired
			proof.PlatformStatus = "token refresh failed"
			return proof, nil
		}
		err = d.call(ctx, action, call.accessToken, liveChatID, req)
	}

	switch {
	case err == nil:
		if cerr := d.quota.Confirm(ctx, quota.QuotaCostBan); cerr != nil {
			d.logger.Warn("failed to confirm youtube quota after successful moderation", zap.Error(cerr))
		}
		proof.Outcome = models.DispatchPerformed
		return proof, nil
	case errors.Is(err, clients.ErrYouTubeBanNotAllowed):
		// YouTube protects the chat owner and other moderators. Nobody can perform this, so it is
		// neither a re-consent nor a credential problem — naming the target is the only useful thing
		// to say about it.
		d.rollback(ctx)
		proof.Outcome = models.DispatchTargetNotActionable
		proof.PlatformStatus = "target cannot be banned"
		return proof, nil
	case errors.Is(err, clients.ErrYouTubeQuotaExceeded):
		// Google's own quota refusal, which arrives as a 403 like a permission failure. Surfaced as
		// a platform error: nothing the streamer holds is wrong, and telling them to re-consent
		// would send them somewhere that cannot help.
		d.rollback(ctx)
		return proof, fmt.Errorf("youtube moderation refused for quota reasons: %w", err)
	case errors.Is(err, clients.ErrYouTubeForbidden):
		d.rollback(ctx)
		// The scope pre-check already passed, so for a delegated moderator this is YouTube
		// answering the question the design defers to it: this account does not moderate that live
		// chat. A re-consent cannot fix that, and offering one would loop a volunteer.
		if actor.IsModerator() && need != "" {
			proof.Outcome = models.DispatchNotPlatformModerator
			proof.PlatformStatus = "forbidden"
			return proof, nil
		}
		if need != "" {
			proof.MissingScopes = []string{need}
		}
		proof.Outcome = models.DispatchReauthRequired
		proof.PlatformStatus = "forbidden"
		return proof, nil
	case errors.Is(err, clients.ErrYouTubeUnauthorized):
		d.rollback(ctx)
		proof.Outcome = models.DispatchReauthRequired
		proof.PlatformStatus = "unauthorized after refresh"
		return proof, nil
	default:
		d.rollback(ctx)
		return proof, err
	}
}

// resolveCall builds the credential for this actor, gated by the owner-reach anchor. A non-nil
// DispatchResult with a nil call means the dispatch is already decided.
//
// The anchor applies to BOTH paths here, unlike Twitch and Kick where credential resolution is
// channel-scoped and implies it. YouTube's credential lookup falls back to a channel-agnostic
// `users` row, so without the anchor an owner could act on any channel merely added as a read-only
// source — YouTube would refuse it, but All-Chat would have asked. Measured before enabling: 406 of
// 406 YouTube sources in production are anchored, so no existing streamer is affected.
func (d *YouTube) resolveCall(ctx context.Context, actor models.Actor, req models.DispatchRequest) (*youtubeCall, models.DispatchResult, error) {
	if err := d.tokens.OwnerYouTubeAnchor(ctx, actor.OwnerUserID, req.ChannelID); err != nil {
		if errors.Is(err, tokens.ErrOwnerChannelUnverified) {
			return nil, models.DispatchResult{Outcome: models.DispatchOwnerUnverified}, nil
		}
		return nil, models.DispatchResult{}, fmt.Errorf("resolve owner youtube anchor: %w", err)
	}

	if !actor.IsModerator() {
		cred, err := d.tokens.Resolve(ctx, actor.UserID, req.ChannelID)
		if errors.Is(err, tokens.ErrNoCredential) {
			return nil, models.DispatchResult{Outcome: models.DispatchNoCredential}, nil
		}
		if err != nil {
			return nil, models.DispatchResult{}, fmt.Errorf("resolve youtube credential: %w", err)
		}
		call := &youtubeCall{
			accessToken:      cred.AccessToken,
			scopes:           cred.GrantedScopes,
			expiresAt:        cred.ExpiresAt,
			credentialUserID: actor.UserID,
		}
		// The refresh writes back onto the call, not just the credential: the retry after a 401
		// reads the token from the call, so a snapshot would retry with the same expired token.
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

	cred, err := d.mod.Resolve(ctx, actor.UserID)
	if errors.Is(err, tokens.ErrNoCredential) {
		// They have not consented for YouTube yet — the normal state of a fresh grant.
		return nil, models.DispatchResult{Outcome: models.DispatchNoCredential}, nil
	}
	if err != nil {
		return nil, models.DispatchResult{}, fmt.Errorf("resolve moderator youtube credential: %w", err)
	}

	call := &youtubeCall{
		accessToken: cred.AccessToken,
		scopes:      cred.GrantedScopes,
		expiresAt:   cred.ExpiresAt,
		// The moderator's own, never the owner's — the field an auditor reads to confirm the
		// no-fallback invariant held. The platform actor id is their Google account id; YouTube's
		// request carries no actor field, so it is attribution only.
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

// call routes an action to the matching liveChatBans insert. Both cost the same quota, because both
// are the same endpoint.
func (d *YouTube) call(ctx context.Context, action models.Action, token, liveChatID string, req models.DispatchRequest) error {
	switch action {
	case models.ActionTimeout:
		return d.api.TimeoutUser(ctx, token, liveChatID, req.TargetUserID, req.DurationSeconds)
	case models.ActionBan:
		return d.api.BanUser(ctx, token, liveChatID, req.TargetUserID)
	default:
		return fmt.Errorf("dispatch: unsupported youtube action %q", action)
	}
}

// rollback releases a held quota reservation, logging (not failing) on error.
func (d *YouTube) rollback(ctx context.Context) {
	if err := d.quota.Rollback(ctx, quota.QuotaCostBan); err != nil {
		d.logger.Warn("failed to roll back youtube quota after failed ban", zap.Error(err))
	}
}
