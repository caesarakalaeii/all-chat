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

// twitchTokenSource resolves and refreshes broadcaster credentials.
// *tokens.TwitchSource satisfies it; an interface keeps dispatch unit-testable.
type twitchTokenSource interface {
	Resolve(ctx context.Context, userID, channelID string) (*tokens.TwitchCredential, error)
	Refresh(ctx context.Context, cred *tokens.TwitchCredential) error
}

// twitchAPI is the subset of clients.TwitchClient the dispatcher calls.
type twitchAPI interface {
	DeleteMessage(ctx context.Context, token, broadcasterID, nativeMessageID string) error
	TimeoutUser(ctx context.Context, token, broadcasterID, targetUserID string, durationSeconds int, reason string) error
	BanUser(ctx context.Context, token, broadcasterID, targetUserID, reason string) error
	UnbanUser(ctx context.Context, token, broadcasterID, targetUserID string) error
}

// Twitch dispatches moderation commands to the Twitch Helix API. Platforms other
// than Twitch report DispatchDryRun (their clients ship in later phases).
type Twitch struct {
	tokens twitchTokenSource
	api    twitchAPI
	logger *zap.Logger
}

// NewTwitch wires a Twitch dispatcher.
func NewTwitch(src twitchTokenSource, api twitchAPI, logger *zap.Logger) *Twitch {
	return &Twitch{tokens: src, api: api, logger: logger}
}

// Dispatch resolves the owner's Twitch credential, verifies it carries the scope the
// action needs, refreshes it if expired, and calls Helix — retrying once after a
// reactive refresh on 401. Authorization (ownership, source membership) has already
// happened in the handler.
func (d *Twitch) Dispatch(ctx context.Context, userID string, action models.Action, req models.DispatchRequest) (models.DispatchResult, error) {
	if req.Platform != "twitch" {
		// No client for this platform yet — keep the reflect-back-only dry run.
		return models.DispatchResult{Outcome: models.DispatchDryRun}, nil
	}

	cred, err := d.tokens.Resolve(ctx, userID, req.ChannelID)
	if errors.Is(err, tokens.ErrNoCredential) {
		return models.DispatchResult{Outcome: models.DispatchNoCredential}, nil
	}
	if err != nil {
		return models.DispatchResult{}, fmt.Errorf("resolve twitch credential: %w", err)
	}

	// Scope pre-check: fail fast (no API call) when the token cannot perform the
	// action, surfacing exactly the scope the opt-in re-consent must request.
	need := models.RequiredTwitchScope(action)
	if need != "" && !hasScope(cred.GrantedScopes, need) {
		return models.DispatchResult{Outcome: models.DispatchReauthRequired, MissingScopes: []string{need}}, nil
	}

	// Proactive refresh: token-refresh-service keeps tokens fresh, but refresh here
	// too if expiry is imminent so we don't spend the first attempt on a 401.
	if !cred.ExpiresAt.IsZero() && time.Until(cred.ExpiresAt) < refreshLeadTime {
		if err := d.tokens.Refresh(ctx, cred); err != nil {
			d.logger.Warn("proactive token refresh failed; attempting with current token",
				zap.String("channel_id", req.ChannelID), zap.Error(err))
		}
	}

	err = d.call(ctx, action, cred, req)
	if errors.Is(err, clients.ErrUnauthorized) {
		// Reactive refresh + single retry: the token expired between refresh cycles.
		if rerr := d.tokens.Refresh(ctx, cred); rerr != nil {
			d.logger.Warn("reactive token refresh failed", zap.String("channel_id", req.ChannelID), zap.Error(rerr))
			return models.DispatchResult{Outcome: models.DispatchReauthRequired, PlatformStatus: "token refresh failed"}, nil
		}
		err = d.call(ctx, action, cred, req)
	}

	switch {
	case err == nil:
		return models.DispatchResult{Outcome: models.DispatchPerformed}, nil
	case errors.Is(err, clients.ErrForbidden):
		// Token is valid but lacks the scope (or moderator privilege) — re-consent.
		missing := []string{}
		if need != "" {
			missing = []string{need}
		}
		return models.DispatchResult{Outcome: models.DispatchReauthRequired, MissingScopes: missing, PlatformStatus: "forbidden"}, nil
	case errors.Is(err, clients.ErrUnauthorized):
		return models.DispatchResult{Outcome: models.DispatchReauthRequired, PlatformStatus: "unauthorized after refresh"}, nil
	default:
		return models.DispatchResult{}, err
	}
}

// call routes an action to the matching Helix endpoint.
func (d *Twitch) call(ctx context.Context, action models.Action, cred *tokens.TwitchCredential, req models.DispatchRequest) error {
	switch action {
	case models.ActionDelete:
		return d.api.DeleteMessage(ctx, cred.AccessToken, cred.BroadcasterID, req.NativeMessageID)
	case models.ActionTimeout:
		return d.api.TimeoutUser(ctx, cred.AccessToken, cred.BroadcasterID, req.TargetUserID, req.DurationSeconds, req.Reason)
	case models.ActionBan:
		return d.api.BanUser(ctx, cred.AccessToken, cred.BroadcasterID, req.TargetUserID, req.Reason)
	case models.ActionUnban:
		return d.api.UnbanUser(ctx, cred.AccessToken, cred.BroadcasterID, req.TargetUserID)
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
