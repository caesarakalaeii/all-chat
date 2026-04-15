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

package main

import (
	"context"
	"testing"

	"github.com/caesar/all-chat/shared/listener"
	"go.uber.org/goleak"
)

// mockChannelManagerForTest is a no-op stub that satisfies listener.ChannelManager.
// It is defined inline here to avoid importing channels.Manager which requires real DB/Redis.
type mockChannelManagerForTest struct{}

func (m *mockChannelManagerForTest) Start(_ context.Context) error                                     { return nil }
func (m *mockChannelManagerForTest) Stop()                                                              {}
func (m *mockChannelManagerForTest) UpdateAssignedSourceIDs(_ map[string]bool)                         {}
func (m *mockChannelManagerForTest) UpdateDemandedSourceIDs(_ map[string]listener.DemandedSource) {}
func (m *mockChannelManagerForTest) GetFilteredAssignmentCount() int                                    { return 0 }
func (m *mockChannelManagerForTest) GetActiveChannels() []string                                        { return nil }
func (m *mockChannelManagerForTest) GetActiveChannelCount() int                                         { return 0 }

func TestLeadershipListener_StartStop_NoGoroutineLeak(t *testing.T) {
	defer goleak.VerifyNone(t)

	ll, err := listener.NewLeadershipListener(listener.LeadershipConfig{
		Platform:               "twitch",
		DisableDemandFiltering: true,
	}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	mgr := &mockChannelManagerForTest{}

	ctx, cancel := context.WithCancel(context.Background())
	if err := ll.Start(ctx, mgr); err != nil {
		t.Fatal(err)
	}
	cancel()
	ll.Stop()
}
