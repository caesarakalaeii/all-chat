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

package channels

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/caesar/all-chat/shared/safehttp"
)

// The instance URL is user input, and the cluster leaves egress open: registering
// with an instance must never reach a non-public address (ADR-0080).
func TestRegisterRefusesNonPublicInstance(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Error("register reached a loopback instance")
	}))
	defer srv.Close()

	m := NewManager(nil, nil, nil, nil, nil, "test", nil)
	defer m.cancel()
	if _, err := m.register(srv.URL); !errors.Is(err, safehttp.ErrNotPublic) {
		t.Fatalf("register(%s) error = %v, want ErrNotPublic", srv.URL, err)
	}
}
