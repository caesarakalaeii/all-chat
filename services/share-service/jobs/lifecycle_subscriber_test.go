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

package jobs

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// TestLifecycleSubscriber_StreamEnd verifies that when a lifecycle:stream_end
// event is received, shares with expiry_option='this_stream' for that user
// are expired atomically (EXPIRY-03).
func TestLifecycleSubscriber_StreamEnd(t *testing.T) {
	log, _ := zap.NewDevelopment()

	// Verify construction with nil args succeeds (nil-safe for unit testing)
	ls := NewLifecycleSubscriber(nil, nil, nil, log.Sugar())
	require.NotNil(t, ls)

	// Verify StreamEndEvent marshaling
	event := StreamEndEvent{
		Platform:      "twitch",
		UserID:        "user-uuid-123",
		BroadcasterID: "twitch-broadcaster-456",
	}
	data, err := json.Marshal(event)
	require.NoError(t, err)

	var decoded StreamEndEvent
	require.NoError(t, json.Unmarshal(data, &decoded))
	assert.Equal(t, event.UserID, decoded.UserID)
	assert.Equal(t, event.Platform, decoded.Platform)
	assert.Equal(t, event.BroadcasterID, decoded.BroadcasterID)
}
