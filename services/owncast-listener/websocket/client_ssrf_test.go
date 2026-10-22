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

package websocket

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/caesar/all-chat/shared/safehttp"
)

// The chat socket is dialed at the user-supplied instance URL, so it must never
// reach a non-public address (ADR-0080).
func TestConnectRefusesNonPublicInstance(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Error("socket dial reached a loopback instance")
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "tk", func(string, *OwncastChatMessage) {}, nil)
	if err := c.Connect("tk"); !errors.Is(err, safehttp.ErrNotPublic) {
		t.Fatalf("Connect error = %v, want ErrNotPublic", err)
	}
}
