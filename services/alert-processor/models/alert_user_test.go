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

	mpmodels "github.com/caesar/all-chat/services/message-processor/models"
	"github.com/stretchr/testify/assert"
)

// TestAlertUserFrom pins the sender mapping: the envelope carries the display
// name the viewer recognizes, falling back to the raw username when the
// platform supplies none, and the avatar when there is one.
func TestAlertUserFrom(t *testing.T) {
	assert.Equal(t,
		AlertUser{ID: "u1", Name: "Some Chatter", AvatarURL: "https://cdn.example/a.png"},
		AlertUserFrom(mpmodels.UserInfo{
			ID:          "u1",
			Username:    "somechatter",
			DisplayName: "Some Chatter",
			AvatarURL:   "https://cdn.example/a.png",
		}),
		"display name must be preferred")

	assert.Equal(t,
		AlertUser{ID: "u1", Name: "somechatter"},
		AlertUserFrom(mpmodels.UserInfo{ID: "u1", Username: "somechatter"}),
		"display name-less senders must fall back to the raw username")
}
