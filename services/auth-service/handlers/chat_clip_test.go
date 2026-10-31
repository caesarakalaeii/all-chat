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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateTwitchClip(t *testing.T) {
	cases := []struct {
		name     string
		status   int
		body     string
		wantURL  string
		wantKind sendErrKind
	}{
		{"created", http.StatusAccepted, `{"data":[{"id":"FancyClip","edit_url":"https://clips.twitch.tv/FancyClip/edit"}]}`, "https://clips.twitch.tv/FancyClip", ""},
		{"offline", http.StatusNotFound, `{"message":"Clipping is not possible for an offline channel."}`, "", sendErrOffline},
		{"no scope", http.StatusForbidden, `{"message":"missing scope"}`, "", sendErrMissingScope},
		{"expired token", http.StatusUnauthorized, `{}`, "", sendErrReauth},
		{"empty data", http.StatusAccepted, `{"data":[]}`, "", sendErrUpstream},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, http.MethodPost, r.Method)
				assert.Equal(t, "12345", r.URL.Query().Get("broadcaster_id"))
				assert.Equal(t, "Bearer tok", r.Header.Get("Authorization"))
				assert.Equal(t, "cid", r.Header.Get("Client-Id"))
				w.WriteHeader(c.status)
				_, _ = w.Write([]byte(c.body))
			}))
			defer srv.Close()

			url, err := createTwitchClip(context.Background(), srv.Client(), srv.URL, "cid", "tok", "12345")
			if c.wantKind == "" {
				require.NoError(t, err)
				assert.Equal(t, c.wantURL, url)
				return
			}
			require.Error(t, err)
			assert.Equal(t, c.wantKind, sendResultErrorKind(err))
		})
	}
}
