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
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoadFirstPartyWSOrigins(t *testing.T) {
	t.Run("FRONTEND_URL alone", func(t *testing.T) {
		t.Setenv("FRONTEND_URL", "https://allch.at")
		t.Setenv("FRONTEND_URLS", "")
		assert.Equal(t, []string{"https://allch.at"}, loadFirstPartyWSOrigins())
	})

	t.Run("FRONTEND_URLS adds the beta origin", func(t *testing.T) {
		t.Setenv("FRONTEND_URL", "https://allch.at")
		t.Setenv("FRONTEND_URLS", "https://beta.allch.at")
		assert.Equal(t,
			[]string{"https://allch.at", "https://beta.allch.at"},
			loadFirstPartyWSOrigins())
	})

	t.Run("trailing slashes and spacing are normalised", func(t *testing.T) {
		t.Setenv("FRONTEND_URL", "https://allch.at/")
		t.Setenv("FRONTEND_URLS", " https://beta.allch.at/ , https://staging.allch.at")
		assert.Equal(t,
			[]string{"https://allch.at", "https://beta.allch.at", "https://staging.allch.at"},
			loadFirstPartyWSOrigins())
	})

	t.Run("unset FRONTEND_URL yields no first-party origins", func(t *testing.T) {
		t.Setenv("FRONTEND_URL", "")
		t.Setenv("FRONTEND_URLS", "")
		assert.Nil(t, loadFirstPartyWSOrigins())
	})
}
