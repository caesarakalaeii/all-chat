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

package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// validOverlay returns the smallest Overlay that passes Validate, so each
// case below varies only the field under test.
func validOverlay() *Overlay {
	return &Overlay{
		UserID: "11111111-1111-1111-1111-111111111111",
		Name:   "My Overlay",
	}
}

func TestOverlayValidateOverlayType(t *testing.T) {
	supported := []string{OverlayTypeChat, OverlayTypeAlerts, OverlayTypeGoal, OverlayTypeList}

	for _, overlayType := range supported {
		t.Run("accepts "+overlayType, func(t *testing.T) {
			overlay := validOverlay()
			overlay.OverlayType = overlayType

			assert.NoError(t, overlay.Validate())
			assert.Equal(t, overlayType, overlay.OverlayType, "a supported type must survive validation unchanged")
		})
	}

	t.Run("absent type resolves to chat", func(t *testing.T) {
		overlay := validOverlay()

		assert.NoError(t, overlay.Validate())
		assert.Equal(t, OverlayTypeChat, overlay.OverlayType, "an overlay created before the column existed must validate as chat")
	})

	t.Run("rejects unknown type", func(t *testing.T) {
		overlay := validOverlay()
		overlay.OverlayType = "webcam"

		err := overlay.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "overlay_type")
		assert.Contains(t, err.Error(), "webcam")
	})

	t.Run("rejects case variant", func(t *testing.T) {
		overlay := validOverlay()
		overlay.OverlayType = "Chat"

		assert.Error(t, overlay.Validate(), "types are wire values, not free text: 'Chat' must not silently become a chat overlay")
	})
}
