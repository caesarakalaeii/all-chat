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
	"testing"

	"github.com/caesar/all-chat/services/moderation-service/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeScopeChecker records the platform it was asked about and returns a fixed result.
type fakeScopeChecker struct {
	actions      []models.Action
	seenPlatform string
}

func (f *fakeScopeChecker) GrantedActions(_ context.Context, _, platform, _ string) ([]models.Action, error) {
	f.seenPlatform = platform
	return f.actions, nil
}

func TestMultiScopeChecker_RoutesByPlatform(t *testing.T) {
	twitch := &fakeScopeChecker{actions: []models.Action{models.ActionDelete, models.ActionBan}}
	discord := StaticScopeChecker{Actions: []models.Action{models.ActionDelete}}
	m := MultiScopeChecker{"twitch": twitch, "discord": discord}

	got, err := m.GrantedActions(context.Background(), "u1", "twitch", "chan")
	require.NoError(t, err)
	assert.Equal(t, []models.Action{models.ActionDelete, models.ActionBan}, got)
	assert.Equal(t, "twitch", twitch.seenPlatform)

	got, err = m.GrantedActions(context.Background(), "u1", "discord", "chan")
	require.NoError(t, err)
	assert.Equal(t, []models.Action{models.ActionDelete}, got)
}

func TestMultiScopeChecker_UnregisteredPlatformReturnsNil(t *testing.T) {
	m := MultiScopeChecker{"twitch": &fakeScopeChecker{actions: []models.Action{models.ActionDelete}}}
	got, err := m.GrantedActions(context.Background(), "u1", "kick", "chan")
	require.NoError(t, err)
	assert.Empty(t, got, "an unregistered platform yields no actions (reported as missing_scope)")
}

func TestStaticScopeChecker_ReturnsFixedActions(t *testing.T) {
	s := StaticScopeChecker{Actions: []models.Action{models.ActionDelete}}
	got, err := s.GrantedActions(context.Background(), "anyone", "discord", "anychan")
	require.NoError(t, err)
	assert.Equal(t, []models.Action{models.ActionDelete}, got)
}
