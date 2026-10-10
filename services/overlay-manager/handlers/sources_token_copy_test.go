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
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/caesar/all-chat/shared/middleware"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

type tokenCopyCall struct {
	platform, userID, channelID string
}

// postTokenCopySource adds a source as user-1 with the given roles and auth method and
// returns the admin-token copies the handler attempted. impersonatedBy is the real admin
// behind an impersonation token, empty otherwise.
func postTokenCopySource(t *testing.T, platform, channelID string, roles []string, authMethod, impersonatedBy string) []tokenCopyCall {
	t.Helper()
	gin.SetMode(gin.TestMode)

	var calls []tokenCopyCall
	h := gateTestHandler(nil)
	h.adminTokenCopy = func(_ context.Context, platform, userID, channelID string) error {
		calls = append(calls, tokenCopyCall{platform, userID, channelID})
		return nil
	}

	router := gin.New()
	router.POST("/overlays/:id/sources", func(c *gin.Context) {
		c.Set("user_id", "user-1")
		c.Set("roles", roles)
		c.Set(middleware.CtxAuthMethod, authMethod)
		c.Set("impersonated_by", impersonatedBy)
		h.HandleAddSource(c)
	})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/overlays/ov-1/sources",
		strings.NewReader(`{"platform":"`+platform+`","channel_id":"`+channelID+`"}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	return calls
}

// A copied row labels a credential Google issued for the caller's own account as another
// channel's, and youtube_oauth_tokens rows are read as proof of channel ownership
// (shared/youtubetoken OwnerYouTubeAnchor). Only an admin session may file one.
func TestAddSource_TokenCopy_NonAdminFilesNoCopy(t *testing.T) {
	const victim = "UCabcdefghijklmnopqrstuv"
	for _, platform := range []string{"youtube", "kick"} {
		t.Run(platform, func(t *testing.T) {
			calls := postTokenCopySource(t, platform, map[string]string{"youtube": victim, "kick": "victim"}[platform],
				[]string{"user"}, middleware.AuthMethodJWT, "")
			assert.Empty(t, calls, "a non-admin must not get a token row filed under a channel they added by link")
		})
	}
}

func TestAddSource_TokenCopy_AdminSessionCopies(t *testing.T) {
	for platform, channelID := range map[string]string{"youtube": "UCabcdefghijklmnopqrstuv", "kick": "somechannel"} {
		t.Run(platform, func(t *testing.T) {
			calls := postTokenCopySource(t, platform, channelID, []string{"user", "admin"}, middleware.AuthMethodJWT, "")
			assert.Equal(t, []tokenCopyCall{{platform, "user-1", channelID}}, calls)
		})
	}
}

func TestAddSource_TokenCopy_AdminAPITokenFilesNoCopy(t *testing.T) {
	// Admin surfaces are session-only (ADR-0051); a personal access token is not one.
	calls := postTokenCopySource(t, "youtube", "UCabcdefghijklmnopqrstuv", []string{"admin"}, middleware.AuthMethodAPIToken, "")
	assert.Empty(t, calls)
}

func TestAddSource_TokenCopy_ImpersonationFilesNoCopy(t *testing.T) {
	// An impersonation token carries the admin role with the TARGET user's id, so a copy
	// would file the impersonated user's own token under a channel they never controlled.
	calls := postTokenCopySource(t, "youtube", "UCabcdefghijklmnopqrstuv", []string{"user", "admin"}, middleware.AuthMethodJWT, "admin-7")
	assert.Empty(t, calls)
}
