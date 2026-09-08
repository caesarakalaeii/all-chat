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

	"github.com/caesar/all-chat/services/moderation-service/clients"
	"github.com/caesar/all-chat/services/moderation-service/models"
	"github.com/caesar/all-chat/services/moderation-service/tokens"
	"go.uber.org/zap"
)

// facebookTokenSource resolves and refreshes Facebook page credentials and
// answers the owner-reach anchor. *tokens.FacebookSource satisfies it.
type facebookTokenSource interface {
	Resolve(ctx context.Context, userID, channelID string) (*tokens.FacebookCredential, error)
	OwnerFacebookAnchor(ctx context.Context, ownerUserID, channelID string) error
}

// facebookAPI is the subset of clients.FacebookClient the dispatcher calls.
type facebookAPI interface {
	DeleteComment(ctx context.Context, token, commentID string) error
	BanUser(ctx context.Context, token, pageID, userID string) error
	UnbanUser(ctx context.Context, token, pageID, userID string) error
}

// Facebook dispatches moderation commands to the Facebook Graph API with the
// acting human's own stored Page token. Delegation is refused for now (same
// default as the other platforms before their SetModSource call): a delegated
// moderator has no Facebook consent surface in this phase, so the dispatcher
// must never fall back to the owner's token.
type Facebook struct {
	tokens facebookTokenSource
	api    facebookAPI
	logger *zap.Logger
}

// NewFacebook wires a Facebook dispatcher for owner actions only.
func NewFacebook(src facebookTokenSource, api facebookAPI, logger *zap.Logger) *Facebook {
	return &Facebook{tokens: src, api: api, logger: logger}
}

// Dispatch resolves the acting human's page credential, verifies it carries
// the permission the action needs, and calls the Graph API. Facebook page
// tokens do not expire (ADR-0060), so there is no proactive/reactive refresh:
// a 401 is terminal and surfaces as re-consent.
func (d *Facebook) Dispatch(ctx context.Context, actor models.Actor, action models.Action, req models.DispatchRequest) (models.DispatchResult, error) {
	if req.Platform != "facebook" {
		return models.DispatchResult{Outcome: models.DispatchDryRun}, nil
	}

	if actor.IsModerator() {
		// No delegated Facebook consent surface exists (ADR-0060); refusing is
		// the invariant, never a fallback to the owner's credential.
		return models.DispatchResult{Outcome: models.DispatchDelegationUnsupported}, nil
	}

	cred, err := d.tokens.Resolve(ctx, actor.UserID, req.ChannelID)
	if errors.Is(err, tokens.ErrNoCredential) {
		return models.DispatchResult{Outcome: models.DispatchNoCredential}, nil
	}
	if err != nil {
		return models.DispatchResult{}, fmt.Errorf("resolve facebook credential: %w", err)
	}

	// Scope pre-check: fail fast (no API call) when the token cannot perform
	// the action. All Facebook actions share pages_manage_engagement, granted
	// at first consent.
	need := models.RequiredFacebookScope(action)
	if need != "" && !hasScope(cred.GrantedScopes, need) {
		return models.DispatchResult{Outcome: models.DispatchReauthRequired, MissingScopes: []string{need}}, nil
	}

	proof := models.DispatchResult{
		CredentialUserID: actor.UserID,
		PlatformActorID:  cred.PageID, // the Page acts; no per-moderator field exists
	}

	var callErr error
	switch action {
	case models.ActionDelete:
		callErr = d.api.DeleteComment(ctx, cred.AccessToken, req.NativeMessageID)
	case models.ActionBan:
		callErr = d.api.BanUser(ctx, cred.AccessToken, req.ChannelID, req.TargetUserID)
	case models.ActionUnban:
		callErr = d.api.UnbanUser(ctx, cred.AccessToken, req.ChannelID, req.TargetUserID)
	default:
		return models.DispatchResult{}, fmt.Errorf("dispatch: unsupported facebook action %q", action)
	}

	switch {
	case callErr == nil:
		proof.Outcome = models.DispatchPerformed
		return proof, nil
	case errors.Is(callErr, clients.ErrFacebookUnauthorized):
		// Page tokens do not expire, so a 401 means revoked/deauthorized —
		// only a re-consent fixes it.
		proof.Outcome = models.DispatchReauthRequired
		proof.PlatformStatus = "unauthorized"
		return proof, nil
	case errors.Is(callErr, clients.ErrFacebookForbidden):
		proof.Outcome = models.DispatchReauthRequired
		proof.MissingScopes = []string{models.ScopeFacebookModeration}
		proof.PlatformStatus = "forbidden"
		return proof, nil
	default:
		proof.PlatformStatus = callErr.Error()
		return proof, callErr
	}
}

// hasScope is shared with the other dispatchers.
