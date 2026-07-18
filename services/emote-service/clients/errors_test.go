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
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"go.uber.org/zap"
)

// TestProviderNotFoundWrapsErrNotFound locks the contract the emote handler relies on to
// classify a benign "channel has no emotes here" 404 as not_found rather than a real
// error: each Twitch-keyed provider must wrap ErrNotFound on a 404 so errors.Is detects it.
func TestProviderNotFoundWrapsErrNotFound(t *testing.T) {
	notFound := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"error":"not found"}`))
	}))
	defer notFound.Close()

	t.Run("bttv", func(t *testing.T) {
		c := NewBTTVClient(zap.NewNop())
		c.baseURL = notFound.URL
		if _, err := c.FetchEmotes(context.Background(), "nochannel"); !errors.Is(err, ErrNotFound) {
			t.Fatalf("bttv 404: expected ErrNotFound, got %v", err)
		}
	})

	t.Run("ffz", func(t *testing.T) {
		c := NewFFZClient(zap.NewNop())
		c.baseURL = notFound.URL
		if _, err := c.FetchEmotes(context.Background(), "nochannel"); !errors.Is(err, ErrNotFound) {
			t.Fatalf("ffz 404: expected ErrNotFound, got %v", err)
		}
	})
}
