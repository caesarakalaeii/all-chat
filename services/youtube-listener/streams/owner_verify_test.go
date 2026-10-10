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
	"net/url"
	"testing"
	"time"

	"go.uber.org/zap"
	"golang.org/x/oauth2"
	"google.golang.org/api/googleapi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type fakeTokenChannel struct {
	channelID string
	err       error
	calls     int
}

func (f *fakeTokenChannel) lookup(context.Context, string, string) (string, error) {
	f.calls++
	return f.channelID, f.err
}

func newTestOwnerVerifier(tokens *fakeTokenChannel) *tokenOwnerVerifier {
	return &tokenOwnerVerifier{
		tokenChannel: tokens.lookup,
		logger:       zap.NewNop(),
		verdicts:     make(map[string]ownerVerdict),
	}
}

func TestOwnerVerifier_MatchingTokenIsVerified(t *testing.T) {
	v := newTestOwnerVerifier(&fakeTokenChannel{channelID: "UCowned"})

	verified, err := v.Verified(context.Background(), "user-1", "UCowned")
	if err != nil || !verified {
		t.Fatalf("Verified = %v, %v; want true, nil", verified, err)
	}
}

func TestOwnerVerifier_TokenOfOtherChannelIsNotVerified(t *testing.T) {
	v := newTestOwnerVerifier(&fakeTokenChannel{channelID: "UCforger"})

	verified, err := v.Verified(context.Background(), "forger", "UCvictim")
	if err != nil || verified {
		t.Fatalf("Verified = %v, %v; want false, nil", verified, err)
	}
}

func TestOwnerVerifier_ErrorIsReturnedAndNotCached(t *testing.T) {
	tokens := &fakeTokenChannel{err: errors.New("token refresh failed")}
	v := newTestOwnerVerifier(tokens)

	verified, err := v.Verified(context.Background(), "user-1", "UCowned")
	if err == nil || verified {
		t.Fatalf("Verified = %v, %v; want false and the error", verified, err)
	}

	tokens.err = nil
	tokens.channelID = "UCowned"
	verified, err = v.Verified(context.Background(), "user-1", "UCowned")
	if err != nil || !verified {
		t.Fatalf("retry Verified = %v, %v; want true, nil", verified, err)
	}
	if tokens.calls != 2 {
		t.Fatalf("token lookups = %d, want 2: an error must be retried, not cached", tokens.calls)
	}
}

func TestOwnerVerifier_CachesVerdictWithinTTL(t *testing.T) {
	for _, tc := range []struct {
		name, tokenChannel string
		want               bool
	}{
		{"verified", "UCowned", true},
		{"rejected", "UCforger", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tokens := &fakeTokenChannel{channelID: tc.tokenChannel}
			v := newTestOwnerVerifier(tokens)

			for i := range 3 {
				if verified, err := v.Verified(context.Background(), "user-1", "UCowned"); err != nil || verified != tc.want {
					t.Fatalf("call %d: Verified = %v, %v; want %v, nil", i, verified, err, tc.want)
				}
			}
			if tokens.calls != 1 {
				t.Fatalf("token lookups = %d, want 1 within the TTL", tokens.calls)
			}
		})
	}
}

func TestOwnerVerifier_RechecksExpiredVerdict(t *testing.T) {
	tokens := &fakeTokenChannel{channelID: "UCowned"}
	v := newTestOwnerVerifier(tokens)

	if _, err := v.Verified(context.Background(), "user-1", "UCowned"); err != nil {
		t.Fatal(err)
	}
	for key, verdict := range v.verdicts {
		verdict.expiresAt = time.Now().Add(-time.Second)
		v.verdicts[key] = verdict
	}

	tokens.channelID = "UCelsewhere"
	verified, err := v.Verified(context.Background(), "user-1", "UCowned")
	if err != nil || verified {
		t.Fatalf("Verified after expiry = %v, %v; want the fresh verdict false, nil", verified, err)
	}
	if tokens.calls != 2 {
		t.Fatalf("token lookups = %d, want 2 once the verdict expired", tokens.calls)
	}
}

func TestOwnerVerifier_ForgetClearsVerdict(t *testing.T) {
	tokens := &fakeTokenChannel{channelID: "UCowned"}
	v := newTestOwnerVerifier(tokens)

	if _, err := v.Verified(context.Background(), "user-1", "UCowned"); err != nil {
		t.Fatal(err)
	}
	v.Forget("user-1", "UCowned")

	tokens.err = errors.New("token revoked")
	if verified, err := v.Verified(context.Background(), "user-1", "UCowned"); err == nil || verified {
		t.Fatalf("Verified after Forget = %v, %v; want false and the token error", verified, err)
	}
	if tokens.calls != 2 {
		t.Fatalf("token lookups = %d, want 2: Forget must drop the cached verdict", tokens.calls)
	}
}

func TestIsOwnerAuthError(t *testing.T) {
	refreshRejected := &oauth2.RetrieveError{Response: &http.Response{StatusCode: http.StatusBadRequest}, ErrorCode: "invalid_grant"}
	tokenEndpointDown := &oauth2.RetrieveError{Response: &http.Response{StatusCode: http.StatusServiceUnavailable}}

	for _, tc := range []struct {
		name string
		err  error
		want bool
	}{
		{"http_401", fmt.Errorf("failed to list active broadcasts: %w", &googleapi.Error{Code: http.StatusUnauthorized}), true},
		{"http_403_forbidden", &googleapi.Error{Code: http.StatusForbidden, Errors: []googleapi.ErrorItem{{Reason: "forbidden"}}}, true},
		{"http_403_quota", &googleapi.Error{Code: http.StatusForbidden, Errors: []googleapi.ErrorItem{{Reason: "quotaExceeded"}}}, false},
		{"http_403_rate_limit", &googleapi.Error{Code: http.StatusForbidden, Errors: []googleapi.ErrorItem{{Reason: "rateLimitExceeded"}}}, false},
		{"http_403_user_rate_limit", &googleapi.Error{Code: http.StatusForbidden, Errors: []googleapi.ErrorItem{{Reason: "userRateLimitExceeded"}}}, false},
		{"http_403_daily_limit", &googleapi.Error{Code: http.StatusForbidden, Errors: []googleapi.ErrorItem{{Reason: "dailyLimitExceeded"}}}, false},
		{"http_500", &googleapi.Error{Code: http.StatusInternalServerError}, false},
		{"http_503", &googleapi.Error{Code: http.StatusServiceUnavailable}, false},
		{"refresh_rejected", &url.Error{Op: "Get", URL: "https://youtube.googleapis.com", Err: refreshRejected}, true},
		{"token_endpoint_down", &url.Error{Op: "Get", URL: "https://youtube.googleapis.com", Err: tokenEndpointDown}, false},
		{"token_store_db", fmt.Errorf("failed to get token: failed to get token from store: %w", errors.New("failed to get token: conn closed")), false},
		{"manager_refresh_400", fmt.Errorf("failed to get token: failed to refresh token: %w", refreshRejected), true},
		{"manager_refresh_503", fmt.Errorf("failed to get token: failed to refresh token: %w", tokenEndpointDown), false},
		{"grpc_unauthenticated", fmt.Errorf("stream: %w", status.Error(codes.Unauthenticated, "bad token")), true},
		{"grpc_permission_denied", status.Error(codes.PermissionDenied, "not owner"), true},
		{"grpc_unavailable", status.Error(codes.Unavailable, "try again"), false},
		{"network", &url.Error{Op: "Get", URL: "https://youtube.googleapis.com", Err: errors.New("connection reset by peer")}, false},
		{"deadline", context.DeadlineExceeded, false},
		{"nil", nil, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := isOwnerAuthError(tc.err); got != tc.want {
				t.Fatalf("isOwnerAuthError(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}

// The classification has to hold for the errors api.Client actually returns, not only for
// hand-built googleapi values.
func TestOwnerAuthError_FromAPIClient(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		body   string
		want   bool
	}{
		{"401", http.StatusUnauthorized, unauthorizedBody, true},
		{"403_forbidden", http.StatusForbidden, `{"error":{"code":403,"message":"Forbidden","errors":[{"reason":"forbidden","message":"Forbidden"}]}}`, true},
		{"403_quota", http.StatusForbidden, `{"error":{"code":403,"message":"quota","errors":[{"reason":"quotaExceeded","message":"quota"}]}}`, false},
		{"503", http.StatusServiceUnavailable, `{"error":{"code":503,"message":"backend error","errors":[{"reason":"backendError","message":"backend error"}]}}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client := newHTTPAPIClient(t, tc.status, tc.body)

			_, err := client.GetActiveBroadcasts(context.Background(), "UCowned", nil)
			if got := isOwnerAuthError(err); got != tc.want {
				t.Fatalf("GetActiveBroadcasts: isOwnerAuthError(%v) = %v, want %v", err, got, tc.want)
			}
			_, err = client.TokenChannelID(context.Background())
			if got := isOwnerAuthError(err); got != tc.want {
				t.Fatalf("TokenChannelID: isOwnerAuthError(%v) = %v, want %v", err, got, tc.want)
			}
		})
	}
}
