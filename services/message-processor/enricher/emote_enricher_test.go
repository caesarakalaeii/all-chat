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

package enricher

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"go.uber.org/zap"
)

func TestHTTPEmoteClient_GetEmotesForChannel_EscapesChannelID(t *testing.T) {
	maliciousChannel := "../evil channel"
	var requestedPath string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestedPath = r.URL.EscapedPath()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"channel":"test","emotes":[]}`))
	}))
	defer server.Close()

	client := NewHTTPEmoteClient(server.URL, zap.NewNop())

	if _, err := client.GetEmotesForChannel(context.Background(), maliciousChannel); err != nil {
		t.Fatalf("GetEmotesForChannel returned error: %v", err)
	}

	expectedPath := "/emotes/channel/" + url.PathEscape(maliciousChannel)
	if requestedPath != expectedPath {
		t.Fatalf("expected path %q, got %q", expectedPath, requestedPath)
	}
}
