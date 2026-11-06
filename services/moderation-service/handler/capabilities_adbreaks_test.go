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
	"encoding/json"
	"net/http"
	"testing"

	"github.com/caesar/all-chat/services/moderation-service/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// fakeOwnerTwitchScopes answers both opt-in checks from one broadcaster credential, the way
// tokens.TwitchScopeChecker does in production.
type fakeOwnerTwitchScopes struct{ scopes []string }

func (f fakeOwnerTwitchScopes) ModLogGranted(context.Context, string, string) (bool, error) {
	return models.ModLogGranted(f.scopes), nil
}

func (f fakeOwnerTwitchScopes) AdBreaksGranted(context.Context, string, string) (bool, error) {
	return models.AdBreaksGranted(f.scopes), nil
}

func ownerOptInCaps(t *testing.T, auth *fakeAuthorizer, callerID string, cred fakeOwnerTwitchScopes) models.Capabilities {
	t.Helper()
	h := New(auth, &fakeEmitter{}, &fakeRecorder{}, NoScopeChecker{}, DryRunDispatcher{}, zap.NewNop())
	h.SetModLogChecker(cred)
	h.SetAdBreaksChecker(cred)

	resp := do(newTestRouter(h, callerID, ""), http.MethodGet, capsPath, "")
	require.Equal(t, http.StatusOK, resp.Code, resp.Body.String())

	var caps models.Capabilities
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &caps))
	return caps
}

// The two opt-ins are separate consents, so each banner must hide on its own grant only:
// a streamer with the ad break scope still needs to be offered the mod log, and vice versa.
func TestCapabilities_AdBreaksAndModLogGrantsAreIndependent(t *testing.T) {
	auth := &fakeAuthorizer{owns: true, sources: twitchSource()}

	adsOnly := ownerOptInCaps(t, auth, ownerID, fakeOwnerTwitchScopes{scopes: []string{models.AdBreaksTwitchScope}})
	assert.True(t, adsOnly.AdBreaksGranted)
	assert.False(t, adsOnly.ModLogGranted)

	modLogOnly := ownerOptInCaps(t, auth, ownerID, fakeOwnerTwitchScopes{scopes: models.ModLogTwitchScopes})
	assert.False(t, modLogOnly.AdBreaksGranted)
	assert.True(t, modLogOnly.ModLogGranted)
}

// channel:read:ads sits on the broadcaster credential and is not delegatable, so a moderator
// has nothing to grant and must never be told the streamer's opt-in is done.
func TestCapabilities_AdBreaksNotGrantedForModerator(t *testing.T) {
	auth := &fakeAuthorizer{
		access:    moderatorAccess([]string{"delete", "timeout", "ban", "unban"}, []string{"twitch"}),
		sources:   twitchSource(),
		modScopes: map[string][]string{"twitch": twitchModScopes},
	}

	caps := ownerOptInCaps(t, auth, modUserID, fakeOwnerTwitchScopes{scopes: []string{models.AdBreaksTwitchScope}})

	assert.False(t, caps.AdBreaksGranted)
}
