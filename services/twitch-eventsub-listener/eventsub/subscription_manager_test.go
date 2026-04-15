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

package eventsub

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// TestSubscribeToStreamOffline verifies that SubscribeToStreamOffline can
// be called on the SubscriptionManager (EXPIRY-02).
// Wave 0: RED stub — method does not exist yet.
func TestSubscribeToStreamOffline(t *testing.T) {
	// RED: SubscribeToStreamOffline method does not exist yet.
	log, _ := zap.NewDevelopment()
	sm := NewSubscriptionManager("client-id", "client-secret", "webhook-secret", "https://example.com/callback", log)
	require.NotNil(t, sm)

	// This will fail to compile until Wave 2 adds the method.
	_, err := sm.SubscribeToStreamOffline(context.Background(), "broadcaster-123")
	// We expect an error (no real Twitch API in test), but the call must exist.
	_ = err
}
