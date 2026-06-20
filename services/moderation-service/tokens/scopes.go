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

package tokens

import (
	"context"
	"errors"

	"github.com/caesar/all-chat/services/moderation-service/models"
)

// credentialResolver is the subset of TwitchSource the scope checker needs.
// Declared as an interface so the checker is unit-testable with a fake.
type credentialResolver interface {
	Resolve(ctx context.Context, userID, channelID string) (*TwitchCredential, error)
}

// TwitchScopeChecker reports which moderation actions the owner's granted Twitch
// scopes currently allow. It satisfies handler.ScopeChecker. For non-Twitch
// platforms it returns no actions (their re-consent flows are later phases), so the
// capability handler reports missing_scope until those ship.
type TwitchScopeChecker struct {
	src credentialResolver
}

// NewTwitchScopeChecker wires a scope checker over a TwitchSource.
func NewTwitchScopeChecker(src *TwitchSource) *TwitchScopeChecker {
	return &TwitchScopeChecker{src: src}
}

// GrantedActions resolves the owner's Twitch credential and maps its granted scopes
// to moderation actions. A missing credential (the owner is not the broadcaster, or
// never opted in) yields no actions rather than an error, so the source is simply
// reported as not-yet-moderatable.
func (c *TwitchScopeChecker) GrantedActions(ctx context.Context, userID, platform, channelID string) ([]models.Action, error) {
	if platform != "twitch" {
		return nil, nil
	}
	cred, err := c.src.Resolve(ctx, userID, channelID)
	if errors.Is(err, ErrNoCredential) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return models.ActionsForTwitchScopes(cred.GrantedScopes), nil
}

// kickCredentialResolver is the subset of KickSource the scope checker needs.
type kickCredentialResolver interface {
	Resolve(ctx context.Context, userID, channelID string) (*KickCredential, error)
}

// KickScopeChecker reports which moderation actions the owner's granted Kick scopes
// currently allow. It satisfies handler.ScopeChecker. For non-Kick platforms it
// returns no actions.
type KickScopeChecker struct {
	src kickCredentialResolver
}

// NewKickScopeChecker wires a scope checker over a KickSource.
func NewKickScopeChecker(src *KickSource) *KickScopeChecker {
	return &KickScopeChecker{src: src}
}

// GrantedActions resolves the owner's Kick credential and maps its granted scopes to
// moderation actions. A missing credential (not the broadcaster, or never opted in)
// yields no actions, so the source is reported as not-yet-moderatable.
func (c *KickScopeChecker) GrantedActions(ctx context.Context, userID, platform, channelID string) ([]models.Action, error) {
	if platform != "kick" {
		return nil, nil
	}
	cred, err := c.src.Resolve(ctx, userID, channelID)
	if errors.Is(err, ErrNoCredential) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return models.ActionsForKickScopes(cred.GrantedScopes), nil
}

// youtubeCredentialResolver is the subset of YouTubeSource the scope checker needs.
type youtubeCredentialResolver interface {
	Resolve(ctx context.Context, userID, channelID string) (*YouTubeCredential, error)
}

// YouTubeScopeChecker reports which moderation actions the owner's granted YouTube
// scopes allow (force-ssl ⇒ ban). It satisfies handler.ScopeChecker.
type YouTubeScopeChecker struct {
	src youtubeCredentialResolver
}

// NewYouTubeScopeChecker wires a scope checker over a YouTubeSource.
func NewYouTubeScopeChecker(src *YouTubeSource) *YouTubeScopeChecker {
	return &YouTubeScopeChecker{src: src}
}

// GrantedActions resolves the owner's YouTube credential and maps its granted scopes to
// moderation actions. A missing credential yields no actions (reported missing_scope).
func (c *YouTubeScopeChecker) GrantedActions(ctx context.Context, userID, platform, channelID string) ([]models.Action, error) {
	if platform != "youtube" {
		return nil, nil
	}
	cred, err := c.src.Resolve(ctx, userID, channelID)
	if errors.Is(err, ErrNoCredential) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return models.ActionsForYouTubeScopes(cred.GrantedScopes), nil
}
