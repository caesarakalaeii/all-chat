// This file is part of All-Chat.
// Copyright (C) 2026 caesarakalaeii
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

package repository

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

// Empty list results must marshal as [] — never null. The translate page uses
// null as its loading sentinel, so a {"locales":null} response from a fresh
// (empty) localization_locales table leaves the UI skeleton up forever. This
// pins the marshaled wire shape, not the in-memory slice, because that is
// what the frontend consumes.
func TestListApprovedLocales_EmptyTableMarshalsAsArray(t *testing.T) {
	pool, cleanup := setupAmbassadorTestDB(t)
	defer cleanup()

	repo := NewLocalizationRepository(pool, zap.NewNop())
	locales, err := repo.ListApprovedLocales(t.Context())
	assert.NoError(t, err)

	b, err := json.Marshal(map[string]any{"locales": locales})
	assert.NoError(t, err)
	assert.Equal(t, `{"locales":[]}`, string(b))
}
