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

package refresher

import (
	"errors"
	"testing"
)

// TestCategorizeRefreshError_MatchesNonRetryable guards against the metric
// category and the retry decision drifting apart. Every error that is treated as
// non-retryable because the user's grant is gone must also be reported under the
// "token_revoked" metric label — otherwise revocations hide in the "other" bucket
// and never trip a dashboard/alert keyed on token_revoked.
func TestCategorizeRefreshError_MatchesNonRetryable(t *testing.T) {
	cases := []struct {
		name     string
		err      error
		wantCat  string
		wantPerm bool // expected isNonRetryableErrorString result
	}{
		{
			// Twitch's revocation signal is this literal string, NOT invalid_grant.
			name:     "twitch invalid refresh token",
			err:      errors.New("non-retryable error: failed to refresh token: oauth2: cannot fetch token: 400 Bad Request\nResponse: {\"status\":400,\"message\":\"Invalid refresh token\"}"),
			wantCat:  "token_revoked",
			wantPerm: true,
		},
		{
			name:     "oauth invalid_grant",
			err:      errors.New("oauth2: \"invalid_grant\""),
			wantCat:  "token_revoked",
			wantPerm: true,
		},
		{
			name:     "access_denied",
			err:      errors.New("oauth2: \"access_denied\""),
			wantCat:  "token_revoked",
			wantPerm: true,
		},
		{
			name:     "invalid_client",
			err:      errors.New("oauth2: \"invalid_client\""),
			wantCat:  "invalid_client",
			wantPerm: true,
		},
		{
			name:     "network timeout is retryable",
			err:      errors.New("connection timeout: upstream unreachable"),
			wantCat:  "network_error",
			wantPerm: false,
		},
		{
			// Twitch transient error — retryable, lands in "other".
			name:     "internal_failure is retryable",
			err:      errors.New("failed to refresh token: oauth2: \"internal_failure\""),
			wantCat:  "other",
			wantPerm: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := categorizeRefreshError(tc.err); got != tc.wantCat {
				t.Errorf("categorizeRefreshError() = %q, want %q", got, tc.wantCat)
			}
			if got := isNonRetryableErrorString(tc.err.Error()); got != tc.wantPerm {
				t.Errorf("isNonRetryableErrorString() = %v, want %v", got, tc.wantPerm)
			}
		})
	}
}
