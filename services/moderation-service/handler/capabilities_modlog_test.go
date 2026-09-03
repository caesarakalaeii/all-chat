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
	"errors"
	"net/http"
	"testing"

	"github.com/caesar/all-chat/services/moderation-service/models"
	"github.com/caesar/all-chat/services/moderation-service/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// mod_log_granted on the capabilities payload (issue #815) is what lets the monitor stop
// re-offering the mod-log opt-in to a streamer who already consented. It is one flag with
// one safe direction: false means "show the banner", so every case that cannot prove the
// grant exists — a moderator, no role, a lookup failure — must report false.

// fakeModLogScopes stands in for the owner's broadcaster Twitch credential.
type fakeModLogScopes struct {
	scopes []string
	err    error
}

func (f fakeModLogScopes) ModLogGranted(_ context.Context, _, _ string) (bool, error) {
	if f.err != nil {
		return false, f.err
	}
	return models.ModLogGranted(f.scopes), nil
}

// modLogCaps runs the capabilities endpoint as callerID with the given owner credential wired.
func modLogCaps(t *testing.T, auth *fakeAuthorizer, callerID string, cred fakeModLogScopes) models.Capabilities {
	t.Helper()
	h := New(auth, &fakeEmitter{}, &fakeRecorder{}, NoScopeChecker{}, DryRunDispatcher{}, zap.NewNop())
	h.SetModLogChecker(cred)

	resp := do(newTestRouter(h, callerID, ""), http.MethodGet, capsPath, "")
	require.Equal(t, http.StatusOK, resp.Code, resp.Body.String())

	var caps models.Capabilities
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &caps))
	return caps
}

func TestCapabilities_ModLogGrantedForOwnerHoldingAllNineScopes(t *testing.T) {
	auth := &fakeAuthorizer{owns: true, sources: twitchSource()}

	caps := modLogCaps(t, auth, ownerID, fakeModLogScopes{scopes: models.ModLogTwitchScopes})

	assert.True(t, caps.ModLogGranted, "the consent is complete, so the opt-in must not be re-offered")
}

func TestCapabilities_ModLogNotGrantedForEightOfNineScopes(t *testing.T) {
	auth := &fakeAuthorizer{owns: true, sources: twitchSource()}
	eight := models.ModLogTwitchScopes[:len(models.ModLogTwitchScopes)-1]

	caps := modLogCaps(t, auth, ownerID, fakeModLogScopes{scopes: eight})

	assert.False(t, caps.ModLogGranted,
		"a credential one scope short cannot subscribe, so the streamer does still need to re-consent")
}

// The mod-log scopes live on the broadcaster credential and are explicitly not delegatable
// (auth-service drops a delegated "modlog" request rather than widening a moderator's consent).
// A moderator therefore has nothing to grant and must never be shown the streamer's opt-in.
func TestCapabilities_ModLogNotGrantedForModeratorRegardlessOfScopes(t *testing.T) {
	auth := &fakeAuthorizer{
		access:    moderatorAccess([]string{"delete", "timeout", "ban", "unban"}, []string{"twitch"}),
		sources:   twitchSource(),
		modScopes: map[string][]string{"twitch": twitchModScopes},
	}

	caps := modLogCaps(t, auth, modUserID, fakeModLogScopes{scopes: models.ModLogTwitchScopes})

	assert.False(t, caps.ModLogGranted, "modlog is the broadcaster's grant, never a moderator's")
}

func TestCapabilities_ModLogNotGrantedForCallerWithNoRole(t *testing.T) {
	auth := &fakeAuthorizer{owns: false, sources: twitchSource()}

	caps := modLogCaps(t, auth, "22222222-2222-2222-2222-222222222222",
		fakeModLogScopes{scopes: models.ModLogTwitchScopes})

	assert.False(t, caps.ModLogGranted)
}

// A credential lookup that fails answers "cannot tell", and cannot-tell has to keep the CTA
// visible: hiding it on a transient database error would leave the streamer with no way to
// enable the mod log and no explanation.
func TestCapabilities_ModLogNotGrantedWhenTheScopeLookupFails(t *testing.T) {
	auth := &fakeAuthorizer{owns: true, sources: twitchSource()}

	caps := modLogCaps(t, auth, ownerID, fakeModLogScopes{err: errors.New("database down")})

	assert.False(t, caps.ModLogGranted)
}

// No Twitch source means no mod log to read, and the monitor hides the banner on the source
// list alone. Reporting true here would be a claim about a grant nobody checked.
func TestCapabilities_ModLogNotGrantedWithoutATwitchSource(t *testing.T) {
	auth := &fakeAuthorizer{
		owns:    true,
		sources: []repository.Source{{Platform: "youtube", ChannelID: "chan-1", ChannelName: "Chan"}},
	}

	caps := modLogCaps(t, auth, ownerID, fakeModLogScopes{scopes: models.ModLogTwitchScopes})

	assert.False(t, caps.ModLogGranted)
}

// The default when no checker is wired (a deployment with no token cipher, so no way to read a
// credential) is the same cannot-tell answer.
func TestCapabilities_ModLogNotGrantedWithNoCheckerWired(t *testing.T) {
	auth := &fakeAuthorizer{owns: true, sources: twitchSource()}
	h := New(auth, &fakeEmitter{}, &fakeRecorder{}, NoScopeChecker{}, DryRunDispatcher{}, zap.NewNop())

	resp := do(newTestRouter(h, ownerID, ""), http.MethodGet, capsPath, "")
	require.Equal(t, http.StatusOK, resp.Code, resp.Body.String())

	var caps models.Capabilities
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &caps))
	assert.False(t, caps.ModLogGranted)
}
