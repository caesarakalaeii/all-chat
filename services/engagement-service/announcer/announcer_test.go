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

package announcer

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/caesar/all-chat/services/engagement-service/models"
	"github.com/caesar/all-chat/services/engagement-service/repository"
	sharedAuth "github.com/caesar/all-chat/shared/auth"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

type stubStore struct{}

func (stubStore) GetEarnConfig(context.Context, uuid.UUID) (models.EarnConfig, error) {
	return models.EarnConfig{}, nil
}
func (stubStore) OverlayOwner(context.Context, uuid.UUID) (string, error) { return "owner", nil }
func (stubStore) SourceChannelsForOverlay(context.Context, uuid.UUID) ([]repository.ChannelRef, error) {
	return nil, nil
}

// TestPostAnnounceSurfacesOutcome covers U2: auth-service returns HTTP 200 even when
// nothing was posted, so postAnnounce must decode the body and surface a non-delivery
// (with per-platform error_kind) rather than treating 200 as success.
func TestPostAnnounceSurfacesOutcome(t *testing.T) {
	t.Setenv("SERVICE_JWT_SECRET_V1", "test-service-secret-0123456789-abcdefghijklmnop")
	svcKeys, err := sharedAuth.NewKeyChainFromEnv("SERVICE_JWT_SECRET")
	require.NoError(t, err)

	cases := []struct {
		name        string
		status      int
		body        string
		wantErr     bool
		errContains string
		wantWarn    bool
	}{
		{"all platforms failed", 200, `{"success":false,"results":[{"platform":"twitch","success":false,"error_kind":"stream_offline"}]}`, true, "stream_offline", false},
		{"no sendable platform", 200, `{"success":false,"results":[]}`, true, "no sendable platform", false},
		{"delivered", 200, `{"success":true,"results":[{"platform":"twitch","success":true}]}`, false, "", false},
		{"partial delivery", 200, `{"success":true,"results":[{"platform":"twitch","success":true},{"platform":"youtube","success":false,"error_kind":"reauth_required"}]}`, false, "", true},
		{"server error", 500, `boom`, true, "500", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.status)
				_, _ = io.WriteString(w, tc.body)
			}))
			defer srv.Close()

			core, logs := observer.New(zap.WarnLevel)
			a := New(stubStore{}, srv.URL, "https://allch.at", svcKeys, zap.New(core))
			err := a.postAnnounce(context.Background(), "user-1", "hello", []string{"twitch"})

			if tc.wantErr {
				require.Error(t, err)
				if tc.errContains != "" {
					assert.Contains(t, err.Error(), tc.errContains)
				}
			} else {
				require.NoError(t, err)
			}
			if tc.wantWarn {
				found := false
				for _, e := range logs.All() {
					if strings.Contains(e.Message, "partially delivered") {
						found = true
					}
				}
				assert.True(t, found, "partial delivery should log a warning naming the failed platform")
			}
		})
	}
}
