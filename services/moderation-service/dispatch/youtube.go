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

// youtubeTokenSource resolves and refreshes YouTube broadcaster credentials.
type youtubeTokenSource interface {
	Resolve(ctx context.Context, userID, channelID string) (*tokens.YouTubeCredential, error)
	Refresh(ctx context.Context, cred *tokens.YouTubeCredential) error
}

// youtubeAPI is the subset of clients.YouTubeClient the dispatcher calls.
type youtubeAPI interface {
	BanUser(ctx context.Context, token, liveChatID, bannedChannelID string) error
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

// YouTube dispatches ban moderation to the YouTube Data API as the broadcaster. It
// resolves the live broadcast's liveChatId from the listener's cache (no search.list),
// reserves quota before the call, and confirms or rolls it back by the outcome.
type YouTube struct {
	tokens   youtubeTokenSource
	api      youtubeAPI
	liveChat liveChatResolver
	quota    quotaReserver
	logger   *zap.Logger
}

// NewYouTube wires a YouTube dispatcher.
func NewYouTube(src youtubeTokenSource, api youtubeAPI, liveChat liveChatResolver, q quotaReserver, logger *zap.Logger) *YouTube {
	return &YouTube{tokens: src, api: api, liveChat: liveChat, quota: q, logger: logger}
}

// Dispatch bans a user from a channel's live chat.
func (d *YouTube) Dispatch(ctx context.Context, actor models.Actor, action models.Action, req models.DispatchRequest) (models.DispatchResult, error) {
	if req.Platform != "youtube" {
		return models.DispatchResult{Outcome: models.DispatchDryRun}, nil
	}
	// The delegated path for this platform is not built yet (ADR-0048 gates each leg
	// independently). Refuse explicitly: resolving by the caller's id happens to find nothing for
	// a moderator today, but relying on that coincidence would turn any future change to
	// credential selection into a silent privilege escalation.
	if actor.IsModerator() {
		return models.DispatchResult{Outcome: models.DispatchDelegationUnsupported}, nil
	}
	if action != models.ActionBan {
		return models.DispatchResult{}, fmt.Errorf("dispatch: unsupported youtube action %q", action)
	}

	cred, err := d.tokens.Resolve(ctx, actor.UserID, req.ChannelID)
	if errors.Is(err, tokens.ErrNoCredential) {
		return models.DispatchResult{Outcome: models.DispatchNoCredential}, nil
	}
	if err != nil {
		return models.DispatchResult{}, fmt.Errorf("resolve youtube credential: %w", err)
	}

	need := models.RequiredYouTubeScope(action)
	if need != "" && !hasScope(cred.GrantedScopes, need) {
		return models.DispatchResult{Outcome: models.DispatchReauthRequired, MissingScopes: []string{need}}, nil
	}

	// liveChatId from the listener's cache — a ban needs the active broadcast's chat.
	liveChatID, err := d.liveChat.Resolve(ctx, req.ChannelID)
	if err != nil {
		// Not live / not cached, or a Redis error: cannot ban. Surfaced as a platform
		// failure (no reflect-back) rather than a re-consent.
		return models.DispatchResult{}, fmt.Errorf("resolve youtube live chat: %w", err)
	}

	// Proactive refresh near expiry so a foreseeable expiry doesn't cost the attempt.
	if !cred.ExpiresAt.IsZero() && time.Until(cred.ExpiresAt) < refreshLeadTime {
		if err := d.tokens.Refresh(ctx, cred); err != nil {
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
		return models.DispatchResult{}, errors.New("youtube daily quota exhausted; ban not attempted")
	}

	err = d.api.BanUser(ctx, cred.AccessToken, liveChatID, req.TargetUserID)
	if errors.Is(err, clients.ErrYouTubeUnauthorized) {
		if rerr := d.tokens.Refresh(ctx, cred); rerr != nil {
			d.logger.Warn("reactive youtube token refresh failed", zap.String("channel_id", req.ChannelID), zap.Error(rerr))
			d.rollback(ctx)
			return models.DispatchResult{Outcome: models.DispatchReauthRequired, PlatformStatus: "token refresh failed"}, nil
		}
		err = d.api.BanUser(ctx, cred.AccessToken, liveChatID, req.TargetUserID)
	}

	switch {
	case err == nil:
		if cerr := d.quota.Confirm(ctx, quota.QuotaCostBan); cerr != nil {
			d.logger.Warn("failed to confirm youtube quota after successful ban", zap.Error(cerr))
		}
		return models.DispatchResult{Outcome: models.DispatchPerformed}, nil
	case errors.Is(err, clients.ErrYouTubeForbidden):
		d.rollback(ctx)
		missing := []string{}
		if need != "" {
			missing = []string{need}
		}
		return models.DispatchResult{Outcome: models.DispatchReauthRequired, MissingScopes: missing, PlatformStatus: "forbidden"}, nil
	case errors.Is(err, clients.ErrYouTubeUnauthorized):
		d.rollback(ctx)
		return models.DispatchResult{Outcome: models.DispatchReauthRequired, PlatformStatus: "unauthorized after refresh"}, nil
	default:
		d.rollback(ctx)
		return models.DispatchResult{}, err
	}
}

// rollback releases a held quota reservation, logging (not failing) on error.
func (d *YouTube) rollback(ctx context.Context) {
	if err := d.quota.Rollback(ctx, quota.QuotaCostBan); err != nil {
		d.logger.Warn("failed to roll back youtube quota after failed ban", zap.Error(err))
	}
}
