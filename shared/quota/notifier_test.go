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

package quota

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestNewNotifier_DefaultsChannel(t *testing.T) {
	n := NewNotifier(nil, zap.NewNop(), false, "")
	assert.Equal(t, DefaultAlertChannel, n.channel)

	n2 := NewNotifier(nil, zap.NewNop(), false, "custom:chan")
	assert.Equal(t, "custom:chan", n2.channel)
}

// TestNotifier_DisabledIsNoOp verifies a disabled notifier short-circuits before
// touching Redis — a nil client must not panic.
func TestNotifier_DisabledIsNoOp(t *testing.T) {
	n := NewNotifier(nil, zap.NewNop(), false, "")
	ctx := context.Background()

	require.NoError(t, n.NotifyStateTransition(ctx, QuotaStateHealthy, QuotaStateDegraded, 72, 720, 1000))
	require.NoError(t, n.NotifyThresholdCrossed(ctx, QuotaStateExhausted, 95, 96, 960, 1000))
	require.NoError(t, n.NotifyQuotaExhausted(ctx, 96, 960, 1000, nil))
	require.NoError(t, n.NotifyQuotaDepleted(ctx, 1000, 1000))
	require.NoError(t, n.NotifyQuotaRecovered(ctx, 10, 100, 1000))
	require.NoError(t, n.NotifyChannelQuotaExceeded(ctx, "UC1", 60, 50))
}
