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

// kickTokenSource resolves and refreshes Kick broadcaster credentials.
// *tokens.KickSource satisfies it; an interface keeps dispatch unit-testable.
type kickTokenSource interface {
	Resolve(ctx context.Context, userID, channelID string) (*tokens.KickCredential, error)
	Refresh(ctx context.Context, cred *tokens.KickCredential) error
}

// kickAPI is the subset of clients.KickClient the dispatcher calls. Kick has no
// single-message delete endpoint, so there is no DeleteMessage.
type kickAPI interface {
	TimeoutUser(ctx context.Context, token, broadcasterID, targetUserID string, durationSeconds int, reason string) error
	BanUser(ctx context.Context, token, broadcasterID, targetUserID, reason string) error
	UnbanUser(ctx context.Context, token, broadcasterID, targetUserID string) error
}

// Kick dispatches moderation commands to the Kick public API as the broadcaster. It
// mirrors the Twitch dispatcher: scope pre-check, proactive + reactive refresh, and a
// single retry after a 401. Authorization (ownership, source membership) has already
// happened in the handler.
type Kick struct {
	tokens kickTokenSource
	api    kickAPI
	logger *zap.Logger
}

// NewKick wires a Kick dispatcher.
func NewKick(src kickTokenSource, api kickAPI, logger *zap.Logger) *Kick {
	return &Kick{tokens: src, api: api, logger: logger}
}

// Dispatch resolves the owner's Kick credential, verifies it carries the moderation
// scope, refreshes it if expired, and calls the Kick API — retrying once after a
// reactive refresh on 401.
func (d *Kick) Dispatch(ctx context.Context, userID string, action models.Action, req models.DispatchRequest) (models.DispatchResult, error) {
	if req.Platform != "kick" {
		return models.DispatchResult{Outcome: models.DispatchDryRun}, nil
	}

	cred, err := d.tokens.Resolve(ctx, userID, req.ChannelID)
	if errors.Is(err, tokens.ErrNoCredential) {
		return models.DispatchResult{Outcome: models.DispatchNoCredential}, nil
	}
	if err != nil {
		return models.DispatchResult{}, fmt.Errorf("resolve kick credential: %w", err)
	}

	// Scope pre-check: fail fast (no API call) when the token cannot perform the action.
	need := models.RequiredKickScope(action)
	if need != "" && !hasScope(cred.GrantedScopes, need) {
		return models.DispatchResult{Outcome: models.DispatchReauthRequired, MissingScopes: []string{need}}, nil
	}

	// Proactive refresh when expiry is imminent so we don't spend the first attempt on a 401.
	if !cred.ExpiresAt.IsZero() && time.Until(cred.ExpiresAt) < refreshLeadTime {
		if err := d.tokens.Refresh(ctx, cred); err != nil {
			d.logger.Warn("proactive kick token refresh failed; attempting with current token",
				zap.String("channel_id", req.ChannelID), zap.Error(err))
		}
	}

	err = d.call(ctx, action, cred, req)
	if errors.Is(err, clients.ErrKickUnauthorized) {
		// Reactive refresh + single retry: the token expired between refresh cycles.
		if rerr := d.tokens.Refresh(ctx, cred); rerr != nil {
			d.logger.Warn("reactive kick token refresh failed", zap.String("channel_id", req.ChannelID), zap.Error(rerr))
			return models.DispatchResult{Outcome: models.DispatchReauthRequired, PlatformStatus: "token refresh failed"}, nil
		}
		err = d.call(ctx, action, cred, req)
	}

	switch {
	case err == nil:
		return models.DispatchResult{Outcome: models.DispatchPerformed}, nil
	case errors.Is(err, clients.ErrKickForbidden):
		missing := []string{}
		if need != "" {
			missing = []string{need}
		}
		return models.DispatchResult{Outcome: models.DispatchReauthRequired, MissingScopes: missing, PlatformStatus: "forbidden"}, nil
	case errors.Is(err, clients.ErrKickUnauthorized):
		return models.DispatchResult{Outcome: models.DispatchReauthRequired, PlatformStatus: "unauthorized after refresh"}, nil
	default:
		return models.DispatchResult{}, err
	}
}

// call routes an action to the matching Kick endpoint.
func (d *Kick) call(ctx context.Context, action models.Action, cred *tokens.KickCredential, req models.DispatchRequest) error {
	switch action {
	case models.ActionTimeout:
		return d.api.TimeoutUser(ctx, cred.AccessToken, cred.BroadcasterID, req.TargetUserID, req.DurationSeconds, req.Reason)
	case models.ActionBan:
		return d.api.BanUser(ctx, cred.AccessToken, cred.BroadcasterID, req.TargetUserID, req.Reason)
	case models.ActionUnban:
		return d.api.UnbanUser(ctx, cred.AccessToken, cred.BroadcasterID, req.TargetUserID)
	default:
		return fmt.Errorf("dispatch: unsupported kick action %q", action)
	}
}
