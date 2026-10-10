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

	"github.com/caesar/all-chat/services/overlay-manager/models"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

// A pin is validated at create time too: otherwise a source could be created already
// pointing at someone else's stream, skipping the PATCH check entirely.
func TestAddYouTubeSource_ForeignPin422(t *testing.T) {
	created := false
	h := gateTestHandler(nil)
	h.sourceRepo = &mockSourceRepository{
		createFunc: func(_ context.Context, _ *models.ChatSource) error {
			created = true
			return nil
		},
	}
	videos := &fakeYouTubeVideoChannels{channels: map[string]string{pinVideo: pinForeignChannel}}
	h.SetYouTubeVideoResolver(videos)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/overlays/:id/sources", func(c *gin.Context) {
		c.Set("user_id", "user-1")
		h.HandleAddSource(c)
	})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/overlays/ov-1/sources", strings.NewReader(
		`{"platform":"youtube","channel_id":"`+pinSourceChannel+`","config":{"stream_id":"https://www.youtube.com/live/`+pinVideo+`"}}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code, w.Body.String())
	assert.False(t, created)
	assert.Equal(t, 1, videos.calls)
}
