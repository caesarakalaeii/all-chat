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

package consumer

import (
	"testing"

	"github.com/caesar/all-chat/services/engagement-service/models"
	mpmodels "github.com/caesar/all-chat/services/message-processor/models"
	"github.com/stretchr/testify/assert"
)

func TestNativePollState(t *testing.T) {
	assert.Equal(t, models.PollActive, nativePollState(mpmodels.NativeEventBegin))
	assert.Equal(t, models.PollActive, nativePollState(mpmodels.NativeEventProgress))
	assert.Equal(t, models.PollClosed, nativePollState(mpmodels.NativeEventEnd))
	// A phase we never subscribe to for polls still resolves to a live state
	// rather than accidentally closing the round.
	assert.Equal(t, models.PollActive, nativePollState("unknown"))
}

func TestNativePredictionState(t *testing.T) {
	tests := []struct {
		name   string
		phase  string
		status string
		want   string
	}{
		{"begin is active", mpmodels.NativeEventBegin, "", models.PredActive},
		{"progress is active", mpmodels.NativeEventProgress, "", models.PredActive},
		{"lock is locked", mpmodels.NativeEventLock, "", models.PredLocked},
		{"end resolved", mpmodels.NativeEventEnd, "resolved", models.PredResolved},
		{"end canceled", mpmodels.NativeEventEnd, "canceled", models.PredCanceled},
		{"end cancelled (British)", mpmodels.NativeEventEnd, "cancelled", models.PredCanceled},
		{"end status case-insensitive", mpmodels.NativeEventEnd, "CANCELED", models.PredCanceled},
		{"end unknown status defaults resolved", mpmodels.NativeEventEnd, "weird", models.PredResolved},
		{"end empty status defaults resolved", mpmodels.NativeEventEnd, "", models.PredResolved},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, nativePredictionState(tt.phase, tt.status))
		})
	}
}
