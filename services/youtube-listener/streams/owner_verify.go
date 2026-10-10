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

package streams

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/caesar/all-chat/services/youtube-listener/api"
	"github.com/caesar/all-chat/services/youtube-listener/oauth"
	"github.com/caesar/all-chat/services/youtube-listener/quota"
	"go.uber.org/zap"
	"golang.org/x/oauth2"
	"google.golang.org/api/googleapi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ownerVerifyTTL bounds how long a verdict is trusted. Long enough that steady-state claim
// rounds cost no quota; short enough that a token re-filed under a different channel is noticed
// the same day.
const ownerVerifyTTL = 6 * time.Hour

// ownerVerifier proves that userID's stored token for channelID actually belongs to channelID.
// A youtube_oauth_tokens row alone proves nothing: overlay-manager's add-by-link path files a
// copy of an admin's token under the channel id added, and filed one for every user before it
// was restricted, so without this check a premium user holding such a row could point
// official-API mode, and its quota, at someone else's channel.
type ownerVerifier interface {
	Verified(ctx context.Context, userID, channelID string) (bool, error)
	// Forget drops a cached verdict, so a token that stopped working is re-checked before the
	// channel can be claimed again.
	Forget(userID, channelID string)
}

type ownerVerdict struct {
	verified  bool
	expiresAt time.Time
}

type tokenOwnerVerifier struct {
	// tokenChannel resolves the channel userID's stored token for channelID belongs to.
	tokenChannel func(ctx context.Context, userID, channelID string) (string, error)
	logger       *zap.Logger

	mu       sync.Mutex
	verdicts map[string]ownerVerdict // userID + "|" + channelID
}

func newTokenOwnerVerifier(oauthManager *oauth.Manager, quotaTracker *quota.Tracker, logger *zap.Logger) *tokenOwnerVerifier {
	return &tokenOwnerVerifier{
		tokenChannel: func(ctx context.Context, userID, channelID string) (string, error) {
			service, httpClient, err := oauthManager.CreateYouTubeService(ctx, userID, channelID)
			if err != nil {
				return "", fmt.Errorf("load owner token: %w", err)
			}
			return api.NewClient(service, httpClient, quotaTracker, logger).TokenChannelID(ctx)
		},
		logger:   logger,
		verdicts: make(map[string]ownerVerdict),
	}
}

// Verified reports whether the token's own channel (channels.list mine=true) is channelID.
// Both verdicts are cached, so a forged row costs one channels.list call per TTL and is logged
// once per TTL. Errors are not cached: a token that cannot be loaded or refreshed is retried on
// the next round, and the caller treats it as unverified meanwhile.
func (v *tokenOwnerVerifier) Verified(ctx context.Context, userID, channelID string) (bool, error) {
	key := userID + "|" + channelID
	now := time.Now()

	v.mu.Lock()
	if verdict, ok := v.verdicts[key]; ok && now.Before(verdict.expiresAt) {
		v.mu.Unlock()
		return verdict.verified, nil
	}
	v.mu.Unlock()

	tokenChannelID, err := v.tokenChannel(ctx, userID, channelID)
	if err != nil {
		return false, err
	}

	verified := tokenChannelID == channelID
	if !verified {
		v.logger.Warn("Official-API owner token belongs to a different channel, skipping row",
			zap.String("user_id", userID),
			zap.String("channel_id", channelID),
			zap.String("token_channel_id", tokenChannelID),
		)
	}

	v.mu.Lock()
	v.verdicts[key] = ownerVerdict{verified: verified, expiresAt: now.Add(ownerVerifyTTL)}
	v.mu.Unlock()
	return verified, nil
}

func (v *tokenOwnerVerifier) Forget(userID, channelID string) {
	v.mu.Lock()
	delete(v.verdicts, userID+"|"+channelID)
	v.mu.Unlock()
}

// isOwnerAuthError reports whether err means the owner's token itself is no longer accepted, as
// opposed to a transient failure (5xx, network, quota) that the next sync can simply retry.
func isOwnerAuthError(err error) bool {
	if err == nil {
		return false
	}
	var apiErr *googleapi.Error
	if errors.As(err, &apiErr) {
		switch apiErr.Code {
		case http.StatusUnauthorized:
			return true
		case http.StatusForbidden:
			// YouTube reports quota and rate limits as 403 too; those pass without a new token.
			for _, item := range apiErr.Errors {
				switch item.Reason {
				case "quotaExceeded", "rateLimitExceeded", "userRateLimitExceeded", "dailyLimitExceeded":
					return false
				}
			}
			return true
		}
		return false
	}
	// A refresh the token endpoint rejected (invalid_grant: revoked or expired grant), surfaced
	// through the oauth2 transport. A 5xx from the token endpoint is an outage, not a revocation.
	var retrieveErr *oauth2.RetrieveError
	if errors.As(err, &retrieveErr) {
		return retrieveErr.Response != nil && retrieveErr.Response.StatusCode < http.StatusInternalServerError
	}
	switch status.Code(err) {
	case codes.Unauthenticated, codes.PermissionDenied:
		return true
	}
	return false
}

// ownerTokenSource feeds the owner token to the gRPC chat stream. grpc-go reports any per-RPC
// credentials error that is not a status as codes.Unauthenticated, which would make a network
// blip or a token-endpoint outage look like a revoked grant; a status error passes through as is.
type ownerTokenSource struct{ oauth2.TokenSource }

func (s ownerTokenSource) Token() (*oauth2.Token, error) {
	token, err := s.TokenSource.Token()
	if err == nil {
		return token, nil
	}
	if isOwnerAuthError(err) {
		return nil, status.Error(codes.Unauthenticated, err.Error())
	}
	return nil, status.Error(codes.Unavailable, err.Error())
}

// pollerOwnerRejected reports whether a poll error means the owner token was rejected. An ended or
// disabled chat is answered with 403 (PermissionDenied over gRPC) as well, and it says nothing
// about the token.
func pollerOwnerRejected(err error) bool {
	if !isOwnerAuthError(err) {
		return false
	}
	msg := err.Error()
	for _, marker := range []string{"liveChatEnded", "liveChatDisabled", "liveChatNotFound", "videoNotFound", "live chat is no longer live"} {
		if strings.Contains(msg, marker) {
			return false
		}
	}
	return true
}
