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

package clients

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestKickClient_GetUserID(t *testing.T) {
	tests := []struct {
		name        string
		slug        string
		status      int
		body        string
		wantID      string
		wantErr     bool
		errContains string
	}{
		{
			name:   "successful lookup, integer user_id",
			slug:   "xqc",
			status: http.StatusOK,
			body:   `{"id": 12345, "user_id": 67890, "slug": "xqc"}`,
			wantID: "67890",
		},
		{
			name:   "successful lookup, string user_id",
			slug:   "trainwreckstv",
			status: http.StatusOK,
			body:   `{"user_id": "98765"}`,
			wantID: "98765",
		},
		{
			name:        "channel not found",
			slug:        "doesnotexist",
			status:      http.StatusNotFound,
			body:        ``,
			wantErr:     true,
			errContains: "not found",
		},
		{
			name:        "rate limited",
			slug:        "xqc",
			status:      http.StatusTooManyRequests,
			body:        ``,
			wantErr:     true,
			errContains: "status 429",
		},
		{
			name:        "empty user_id",
			slug:        "broken",
			status:      http.StatusOK,
			body:        `{"user_id": 0}`,
			wantErr:     true,
			errContains: "empty user_id",
		},
		{
			name:        "malformed JSON",
			slug:        "broken",
			status:      http.StatusOK,
			body:        `{not json`,
			wantErr:     true,
			errContains: "decode",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.True(t, strings.Contains(r.URL.Path, "/api/v2/channels/"+tt.slug),
					"unexpected path: %s", r.URL.Path)
				w.WriteHeader(tt.status)
				if tt.body != "" {
					w.Write([]byte(tt.body))
				}
			}))
			defer server.Close()

			client := NewKickClient(zaptest.NewLogger(t))
			client.urlTemplate = server.URL + "/api/v2/channels/%s"

			id, err := client.GetUserID(context.Background(), tt.slug)

			if tt.wantErr {
				require.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantID, id)
		})
	}
}

func TestKickClient_Cache(t *testing.T) {
	var requestCount atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"user_id": 42}`))
	}))
	defer server.Close()

	client := NewKickClient(zaptest.NewLogger(t))
	client.urlTemplate = server.URL + "/api/v2/channels/%s"

	id1, err := client.GetUserID(context.Background(), "cached")
	require.NoError(t, err)
	id2, err := client.GetUserID(context.Background(), "cached")
	require.NoError(t, err)

	assert.Equal(t, "42", id1)
	assert.Equal(t, id1, id2)
	assert.Equal(t, int32(1), requestCount.Load(), "second call should hit the in-memory cache")
}

func TestKickClient_EmptySlug(t *testing.T) {
	client := NewKickClient(zaptest.NewLogger(t))
	_, err := client.GetUserID(context.Background(), "  ")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty")
}
