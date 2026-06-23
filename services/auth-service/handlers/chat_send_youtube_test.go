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
	"errors"
	"testing"
	"time"

	"github.com/caesar/all-chat/shared/youtubetoken"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// fakeYouTubeTokenSource is a stand-in YouTubeTokenSource for exercising
// resolveYouTubeAccessToken without a database. It records whether Refresh was called.
type fakeYouTubeTokenSource struct {
	cred       *youtubetoken.YouTubeCredential
	resolveErr error
	refreshErr error
	refreshed  bool
}

func (f *fakeYouTubeTokenSource) Resolve(_ context.Context, _, _ string) (*youtubetoken.YouTubeCredential, error) {
	if f.resolveErr != nil {
		return nil, f.resolveErr
	}
	return f.cred, nil
}

func (f *fakeYouTubeTokenSource) Refresh(_ context.Context, _ *youtubetoken.YouTubeCredential) error {
	f.refreshed = true
	return f.refreshErr
}

func newYouTubeSendHandler(ts YouTubeTokenSource) *ChatSendHandler {
	return &ChatSendHandler{log: zap.NewNop(), ytTokenSource: ts}
}

// The fix: the streamer YouTube send must use the youtube_oauth_tokens credential
// (resolved by the shared source), NOT user.AccessToken — which for a Twitch-login
// streamer is a Twitch token the YouTube API rejects with 401.
func TestResolveYouTubeAccessToken_ReturnsCredentialToken(t *testing.T) {
	h := newYouTubeSendHandler(&fakeYouTubeTokenSource{
		cred: &youtubetoken.YouTubeCredential{AccessToken: "ya29.real", ExpiresAt: time.Now().Add(time.Hour)},
	})
	tok, err := h.resolveYouTubeAccessToken(context.Background(), "u1", "UC1")
	assert.NoError(t, err)
	assert.Equal(t, "ya29.real", tok, "must use the youtube_oauth_tokens credential token, not the users-row token")
}

// A missing YouTube credential (the streamer never linked YouTube, or the token was
// revoked) must surface as reauth_required so the monitor shows the Reconnect button
// instead of a generic 502.
func TestResolveYouTubeAccessToken_NoCredentialIsReauth(t *testing.T) {
	h := newYouTubeSendHandler(&fakeYouTubeTokenSource{resolveErr: youtubetoken.ErrNoCredential})
	_, err := h.resolveYouTubeAccessToken(context.Background(), "u1", "UC1")
	var se *streamerSendError
	require.ErrorAs(t, err, &se)
	assert.Equal(t, sendErrReauth, se.kind, "missing youtube credential must map to reauth_required")
}

// An unconfigured token source must fail safe to reauth_required rather than fall back to
// the (wrong) users-row token.
func TestResolveYouTubeAccessToken_NilSourceIsReauth(t *testing.T) {
	h := &ChatSendHandler{log: zap.NewNop()}
	_, err := h.resolveYouTubeAccessToken(context.Background(), "u1", "UC1")
	var se *streamerSendError
	require.ErrorAs(t, err, &se)
	assert.Equal(t, sendErrReauth, se.kind)
}

// A token expiring within the 5-minute lead time must be proactively refreshed so a
// foreseeable expiry doesn't cost the send attempt.
func TestResolveYouTubeAccessToken_RefreshesNearExpiry(t *testing.T) {
	src := &fakeYouTubeTokenSource{
		cred: &youtubetoken.YouTubeCredential{AccessToken: "ya29", ExpiresAt: time.Now().Add(time.Minute)},
	}
	h := newYouTubeSendHandler(src)
	tok, err := h.resolveYouTubeAccessToken(context.Background(), "u1", "UC1")
	assert.NoError(t, err)
	assert.Equal(t, "ya29", tok)
	assert.True(t, src.refreshed, "a token expiring within 5 minutes must be proactively refreshed")
}

// A token well within its lifetime must NOT trigger a refresh.
func TestResolveYouTubeAccessToken_SkipsRefreshWhenFarFromExpiry(t *testing.T) {
	src := &fakeYouTubeTokenSource{
		cred: &youtubetoken.YouTubeCredential{AccessToken: "ya29", ExpiresAt: time.Now().Add(time.Hour)},
	}
	h := newYouTubeSendHandler(src)
	_, err := h.resolveYouTubeAccessToken(context.Background(), "u1", "UC1")
	assert.NoError(t, err)
	assert.False(t, src.refreshed, "a token far from expiry must not be refreshed")
}

// A refresh failure must not abort the send — the still-current token is attempted.
func TestResolveYouTubeAccessToken_RefreshFailureIsNonFatal(t *testing.T) {
	src := &fakeYouTubeTokenSource{
		cred:       &youtubetoken.YouTubeCredential{AccessToken: "ya29", ExpiresAt: time.Now().Add(time.Minute)},
		refreshErr: errors.New("refresh failed"),
	}
	h := newYouTubeSendHandler(src)
	tok, err := h.resolveYouTubeAccessToken(context.Background(), "u1", "UC1")
	assert.NoError(t, err, "a refresh failure must not block the send")
	assert.Equal(t, "ya29", tok)
	assert.True(t, src.refreshed)
}
