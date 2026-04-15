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

package listener_test

import (
	"testing"

	"github.com/caesar/all-chat/shared/listener"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
)

func TestLeadershipListener_NilSafe(t *testing.T) {
	defer goleak.VerifyNone(t)
	// Ensure SOURCE_MANAGER_SECRET is absent
	t.Setenv("SOURCE_MANAGER_SECRET", "")

	ll, err := listener.NewLeadershipListenerFromEnv("test-platform", nil, nil)
	require.NoError(t, err, "should not return error when SOURCE_MANAGER_SECRET absent")
	require.NotNil(t, ll, "should return non-nil LeadershipListener")
}

func TestLeadershipListener_NilSafe_LeadershipCoordinator(t *testing.T) {
	defer goleak.VerifyNone(t)
	t.Setenv("SOURCE_MANAGER_SECRET", "")

	ll, err := listener.NewLeadershipListenerFromEnv("test-platform", nil, nil)
	require.NoError(t, err)
	require.NotNil(t, ll)

	// LeadershipCoordinator() should return nil — no panic
	coord := ll.LeadershipCoordinator()
	assert.Nil(t, coord, "coordinator should be nil when SECRET not set")
}

func TestLeadershipListener_NewLeadershipListener_ConfigBased(t *testing.T) {
	defer goleak.VerifyNone(t)

	ll, err := listener.NewLeadershipListener(listener.LeadershipConfig{
		Platform:               "kick",
		DisableDemandFiltering: true,
	}, nil, nil)
	require.NoError(t, err)
	require.NotNil(t, ll)

	// Coordinator and SMClient should be nil (no env reads in config-based constructor)
	assert.Nil(t, ll.LeadershipCoordinator())
	assert.Nil(t, ll.SMClient())
}
