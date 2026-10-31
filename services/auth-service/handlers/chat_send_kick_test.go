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

package handlers

import (
	"context"
	"testing"
	"time"

	"github.com/caesar/all-chat/services/auth-service/models"
	"github.com/caesar/all-chat/services/auth-service/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"golang.org/x/oauth2"
)

// fakeKickTokenSource is a stand-in KickTokenSource for exercising resolveKickSender
// without a database. It records the tokens written back after a refresh.
type fakeKickTokenSource struct {
	cred    *repository.LinkedKickCredential
	err     error
	updated *repository.LinkedKickCredential
}

func (f *fakeKickTokenSource) GetLinkedKickSendCredential(_ context.Context, _ string) (*repository.LinkedKickCredential, error) {
	return f.cred, f.err
}

func (f *fakeKickTokenSource) UpdateLinkedKickTokens(_ context.Context, id int64, accessToken, refreshToken string, expiry time.Time) error {
	f.updated = &repository.LinkedKickCredential{ID: id, AccessToken: accessToken, RefreshToken: refreshToken, Expiry: expiry}
	return nil
}

func strPtr(s string) *string { return &s }

// A Kick-login streamer's own credential is the users row, unchanged.
func TestResolveKickSender_KickLoginUsesUsersRow(t *testing.T) {
	h := &ChatSendHandler{log: zap.NewNop(), kickTokenSource: &fakeKickTokenSource{}}
	user := &models.User{AuthProvider: "kick", KickID: strPtr("42"), AccessToken: "kick-login-token"}

	tok, broadcasterID, err := h.resolveKickSender(context.Background(), user)
	require.NoError(t, err)
	assert.Equal(t, "kick-login-token", tok)
	assert.Equal(t, "42", broadcasterID)
}

// The reported bug: a Twitch-login streamer who linked Kick has a TWITCH token on the users
// row and no users.kick_id, so the Kick send failed even after the advanced-controls consent.
// Their Kick credential lives in kick_oauth_tokens and must be what the send uses.
func TestResolveKickSender_LinkedAccountUsesLinkedCredential(t *testing.T) {
	src := &fakeKickTokenSource{cred: &repository.LinkedKickCredential{
		ID: 7, KickUserID: "130554250", AccessToken: "kick-linked-token", RefreshToken: "r",
		Expiry: time.Now().Add(time.Hour),
	}}
	h := &ChatSendHandler{log: zap.NewNop(), kickTokenSource: src}
	user := &models.User{ID: "u1", AuthProvider: "twitch", TwitchID: strPtr("1"), AccessToken: "twitch-token"}

	tok, broadcasterID, err := h.resolveKickSender(context.Background(), user)
	require.NoError(t, err)
	assert.Equal(t, "kick-linked-token", tok, "must never send the users-row (Twitch) token to Kick")
	assert.Equal(t, "130554250", broadcasterID)
	assert.Nil(t, src.updated, "a credential far from expiry must not be refreshed")
}

// No linked credential carrying chat:write means sending was never enabled for Kick: the
// monitor must offer the enable flow, not a generic failure.
func TestResolveKickSender_NoLinkedCredentialIsMissingScope(t *testing.T) {
	h := &ChatSendHandler{log: zap.NewNop(), kickTokenSource: &fakeKickTokenSource{}}
	user := &models.User{ID: "u1", AuthProvider: "twitch", AccessToken: "twitch-token"}

	_, _, err := h.resolveKickSender(context.Background(), user)
	var se *streamerSendError
	require.ErrorAs(t, err, &se)
	assert.Equal(t, sendErrMissingScope, se.kind)
}

// An unconfigured source must fail safe rather than fall back to the users-row token.
func TestResolveKickSender_NilSourceIsReauth(t *testing.T) {
	h := &ChatSendHandler{log: zap.NewNop()}
	user := &models.User{ID: "u1", AuthProvider: "twitch", AccessToken: "twitch-token"}

	_, _, err := h.resolveKickSender(context.Background(), user)
	var se *streamerSendError
	require.ErrorAs(t, err, &se)
	assert.Equal(t, sendErrReauth, se.kind)
}

// A linked credential about to expire is refreshed and the rotated tokens are written back,
// so the next send (and the moderation service, which reads the same row) uses them.
func TestResolveKickSender_RefreshesLinkedCredentialNearExpiry(t *testing.T) {
	src := &fakeKickTokenSource{cred: &repository.LinkedKickCredential{
		ID: 7, KickUserID: "130554250", AccessToken: "stale", RefreshToken: "old-refresh",
		Expiry: time.Now().Add(time.Minute),
	}}
	kick := new(MockOAuthProvider)
	kick.On("RefreshToken", mock.Anything, "old-refresh").Return(&oauth2.Token{
		AccessToken: "fresh", RefreshToken: "new-refresh", Expiry: time.Now().Add(time.Hour),
	}, nil)
	h := &ChatSendHandler{log: zap.NewNop(), kickTokenSource: src, kickProvider: kick}
	user := &models.User{ID: "u1", AuthProvider: "twitch", AccessToken: "twitch-token"}

	tok, _, err := h.resolveKickSender(context.Background(), user)
	require.NoError(t, err)
	assert.Equal(t, "fresh", tok)
	require.NotNil(t, src.updated, "the refreshed tokens must be persisted")
	assert.Equal(t, int64(7), src.updated.ID)
	assert.Equal(t, "fresh", src.updated.AccessToken)
	assert.Equal(t, "new-refresh", src.updated.RefreshToken)
}
