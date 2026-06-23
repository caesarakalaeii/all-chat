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

package channels

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

// mockNoticePublisher captures every deprecation notice publish call.
type mockNoticePublisher struct {
	mu    sync.Mutex
	calls [][2]string // {overlayID, channel}
	err   error
}

func (m *mockNoticePublisher) PublishDeprecationNotice(_ context.Context, overlayID, channel string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = append(m.calls, [2]string{overlayID, channel})
	return m.err
}

func (m *mockNoticePublisher) Calls() [][2]string {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([][2]string, len(m.calls))
	copy(out, m.calls)
	return out
}

func TestParseDeprecationMode(t *testing.T) {
	cases := map[string]DeprecationMode{
		"":        DeprecationOff,
		"off":     DeprecationOff,
		"garbage": DeprecationOff,
		"warn":    DeprecationWarn,
		"WARN":    DeprecationWarn,
		" soft ":  DeprecationWarn,
		"enforce": DeprecationEnforce,
		"Enforce": DeprecationEnforce,
		"hard":    DeprecationEnforce,
		"block":   DeprecationEnforce,
	}
	for input, want := range cases {
		assert.Equalf(t, want, ParseDeprecationMode(input), "ParseDeprecationMode(%q)", input)
	}
}

func TestDeprecationMode_String(t *testing.T) {
	assert.Equal(t, "off", DeprecationOff.String())
	assert.Equal(t, "warn", DeprecationWarn.String())
	assert.Equal(t, "enforce", DeprecationEnforce.String())
}

// In ENFORCE mode the listener must not issue a single JOIN, no matter how many
// channels the database lists.
func TestManager_SyncChannels_EnforceMode_NoJoins(t *testing.T) {
	ctx := context.Background()
	logger := zaptest.NewLogger(t)

	repo := &MockRepository{channels: []string{"xqc", "summit1g", "shroud"}}
	mockJP := NewMockJoinParter()
	manager := NewManager(repo, mockJP, nil, nil, nil, nil, "", logger, nil)
	manager.SetDeprecationConfig(DeprecationConfig{Mode: DeprecationEnforce}, nil)

	require.NoError(t, manager.SyncChannels(ctx))

	assert.Empty(t, mockJP.GetJoined(), "enforce mode must not join any channel")
	assert.Equal(t, 0, manager.GetActiveChannelCount())
}

// ENFORCE mode must also PART channels that were joined while the pod was still
// serving (defensive: a desired set that empties out triggers the normal PART path).
func TestManager_SyncChannels_EnforceMode_PartsExisting(t *testing.T) {
	ctx := context.Background()
	logger := zaptest.NewLogger(t)

	repo := &MockRepository{channels: []string{"xqc", "summit1g"}}
	mockJP := NewMockJoinParter()
	manager := NewManager(repo, mockJP, nil, nil, nil, nil, "", logger, nil)

	// First sync in normal mode joins both channels.
	require.NoError(t, manager.SyncChannels(ctx))
	require.Equal(t, 2, manager.GetActiveChannelCount())

	// Flip to enforce and re-sync: both must be parted.
	manager.SetDeprecationConfig(DeprecationConfig{Mode: DeprecationEnforce}, nil)
	require.NoError(t, manager.SyncChannels(ctx))

	assert.Equal(t, 0, manager.GetActiveChannelCount())
	departed := mockJP.GetDeparted()
	assert.Contains(t, departed, "xqc")
	assert.Contains(t, departed, "summit1g")
}

// In WARN mode the listener keeps serving chat (channels still joined).
func TestManager_SyncChannels_WarnMode_StillJoins(t *testing.T) {
	ctx := context.Background()
	logger := zaptest.NewLogger(t)

	repo := &MockRepository{channels: []string{"xqc", "summit1g"}}
	mockJP := NewMockJoinParter()
	manager := NewManager(repo, mockJP, nil, nil, nil, nil, "", logger, nil)
	manager.SetDeprecationConfig(DeprecationConfig{Mode: DeprecationWarn}, &mockNoticePublisher{})

	require.NoError(t, manager.SyncChannels(ctx))

	assert.ElementsMatch(t, []string{"xqc", "summit1g"}, mockJP.GetJoined())
	assert.Equal(t, 2, manager.GetActiveChannelCount())
}

// publishDeprecationNotices must emit exactly one notice per (overlay, connected
// channel) pair.
func TestManager_PublishDeprecationNotices(t *testing.T) {
	ctx := context.Background()
	logger := zaptest.NewLogger(t)

	repo := &MockRepository{channels: []string{"xqc", "summit1g"}}
	mockJP := NewMockJoinParter()
	pub := &mockNoticePublisher{}
	manager := NewManager(repo, mockJP, nil, nil, nil, nil, "", logger, nil)
	manager.SetDeprecationConfig(DeprecationConfig{Mode: DeprecationWarn}, pub)

	// Join the channels so they count as "connected sources".
	require.NoError(t, manager.SyncChannels(ctx))

	manager.publishDeprecationNotices(ctx)

	// MockRepository.GetOverlayIDsForChannel returns exactly one overlay per channel.
	assert.ElementsMatch(t, [][2]string{
		{"test-overlay-xqc", "xqc"},
		{"test-overlay-summit1g", "summit1g"},
	}, pub.Calls())
}

// With no connected channels there is nothing to notify.
func TestManager_PublishDeprecationNotices_NoActiveChannels(t *testing.T) {
	ctx := context.Background()
	logger := zaptest.NewLogger(t)

	repo := &MockRepository{channels: []string{}}
	pub := &mockNoticePublisher{}
	manager := NewManager(repo, NewMockJoinParter(), nil, nil, nil, nil, "", logger, nil)
	manager.SetDeprecationConfig(DeprecationConfig{Mode: DeprecationWarn}, pub)

	manager.publishDeprecationNotices(ctx)
	assert.Empty(t, pub.Calls())
}

// SetDeprecationConfig must default a non-positive interval rather than spin a
// zero-duration ticker.
func TestSetDeprecationConfig_DefaultsInterval(t *testing.T) {
	logger := zaptest.NewLogger(t)
	manager := NewManager(&MockRepository{}, NewMockJoinParter(), nil, nil, nil, nil, "", logger, nil)
	manager.SetDeprecationConfig(DeprecationConfig{Mode: DeprecationWarn}, &mockNoticePublisher{})
	assert.Equal(t, DefaultDeprecationNoticeInterval, manager.deprecation.NoticeInterval)

	manager.SetDeprecationConfig(DeprecationConfig{Mode: DeprecationWarn, NoticeInterval: 30 * time.Second}, &mockNoticePublisher{})
	assert.Equal(t, 30*time.Second, manager.deprecation.NoticeInterval)
}
