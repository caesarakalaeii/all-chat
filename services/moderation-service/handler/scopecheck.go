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

package handler

import (
	"context"

	"github.com/caesar/all-chat/services/moderation-service/models"
)

// MultiScopeChecker routes a capability scope check to the checker registered for the
// platform, so capabilities fan out the same way dispatch does. A platform with no
// registered checker yields no actions, which the handler reports as missing_scope —
// the correct default for a platform whose opt-in flow has not been wired in this
// deployment.
type MultiScopeChecker map[string]ScopeChecker

// GrantedActions delegates to the per-platform checker.
func (m MultiScopeChecker) GrantedActions(ctx context.Context, userID, platform, channelID string) ([]models.Action, error) {
	if c, ok := m[platform]; ok && c != nil {
		return c.GrantedActions(ctx, userID, platform, channelID)
	}
	return nil, nil
}

// StaticScopeChecker reports a fixed action set regardless of the user. It is used for
// platforms whose moderation authority is a shared service credential (a bot token),
// not a per-user OAuth grant — Discord. The actual platform permission (e.g. Discord's
// MANAGE_MESSAGES) is enforced by the platform at call time, surfacing as a dispatch
// error rather than a capability gate.
type StaticScopeChecker struct {
	Actions []models.Action
}

// GrantedActions returns the fixed action set.
func (s StaticScopeChecker) GrantedActions(context.Context, string, string, string) ([]models.Action, error) {
	return s.Actions, nil
}
