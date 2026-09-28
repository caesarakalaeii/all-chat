// This file is part of All-Chat.
// Copyright (C) 2026 caesarakalaeii
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published
// by the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

package subscription

import (
	"context"
	"errors"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/caesar/all-chat/services/api-gateway/models"
	"github.com/caesar/all-chat/services/api-gateway/sessions"
	"github.com/caesar/all-chat/services/api-gateway/websocket"
	"github.com/caesar/all-chat/shared/metrics"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// Pentest F2a: a status frame belongs on an overlay only when the overlay has
// that exact platform+channel configured. Platform-only matching broadcast
// every other streamer's statuses cross-tenant.

// fakeResolver hands per-overlay sources back without a database.
type fakeResolver struct {
	sources map[string][]OverlaySource
	err     map[string]error
}

func (f *fakeResolver) GetOverlaySources(_ context.Context, overlayID string) ([]OverlaySource, error) {
	if err, ok := f.err[overlayID]; ok {
		return nil, err
	}
	return f.sources[overlayID], nil
}

// newSubscriberTestManager wires a real Manager over miniredis, with only the
// DB nil. AddConnection's DB writes all run behind nil guards or on the
// EnsureSession fast path, so tests must seed an active session per overlay
// (seedActiveSession) before attaching connections. The heartbeat goroutine is
// stopped via t.Cleanup.
func newSubscriberTestManager(t *testing.T) (*websocket.Manager, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	m := websocket.NewManager(zap.NewNop(), metrics.NewGatewayMetricsForTest(), rdb, nil)
	t.Cleanup(func() { _ = m.Shutdown(context.Background()) })
	return m, mr
}

// seedActiveSession puts a live session hash in Redis so EnsureSession takes
// its exists-and-ACTIVE fast path — addConnection then never reaches the nil
// pgxpool.
func seedActiveSession(mr *miniredis.Miniredis, overlayID string) {
	mr.HSet(sessions.SessionKeyPrefix+overlayID, "state", "ACTIVE")
}

// subscriberTestConnection builds a Connection without a real socket. The
// broadcast path only touches the classification accessors and Send(), so a
// buffered send channel and a nop logger suffice (as in pool_test.go).
func subscriberTestConnection(overlayID string) *websocket.Connection {
	return websocket.NewConnection(nil, overlayID, "owner-user", nil, zap.NewNop())
}

// statusFrameJSON marshals a platform_status frame the way
// handleStatusMessage does before broadcasting.
func statusFrameJSON(t *testing.T, statusData models.PlatformStatusData) []byte {
	t.Helper()
	msgJSON, err := models.NewPlatformStatus(statusData).ToJSON()
	require.NoError(t, err)
	return msgJSON
}

// TestBroadcastStatusToRelevantOverlays_RoutesOnlyMatchingOverlays is the
// wiring-level pin for the F2a fix: broadcastStatusToRelevantOverlays must
// actually consult the resolver's platform+channel conjunct and the viewer
// exclusion filter. Rewiring broadcastStatusToRelevantOverlays to skip
// overlayHasSource (or to broadcast untargeted) must fail this test, not
// merely the pure-function table below.
func TestBroadcastStatusToRelevantOverlays_RoutesOnlyMatchingOverlays(t *testing.T) {
	m, mr := newSubscriberTestManager(t)
	ctx := context.Background()
	seedActiveSession(mr, "ov-matching")
	seedActiveSession(mr, "ov-other")

	matchingOwner := subscriberTestConnection("ov-matching")
	m.AddConnection(ctx, matchingOwner)
	otherOwner := subscriberTestConnection("ov-other")
	m.AddConnection(ctx, otherOwner)
	otherViewer := websocket.NewViewerConnection(nil, "ov-other", "viewer-user", nil, zap.NewNop())
	m.AddConnectionNoDemand(ctx, otherViewer)

	ss := NewStatusSubscriber(nil, m, zap.NewNop(), nil)
	ss.SetSourceResolver(&fakeResolver{sources: map[string][]OverlaySource{
		"ov-matching": {{Platform: "twitch", ChannelID: "111", ChannelName: "mine"}},
		"ov-other":    {{Platform: "twitch", ChannelID: "999", ChannelName: "theirs"}},
	}})

	statusData := models.PlatformStatusData{Platform: "twitch", ChannelID: "111", Status: "connected"}
	msgJSON := statusFrameJSON(t, statusData)

	// Three sockets are attached, so the sent count discriminates every
	// regression: an untargeted broadcast sends 3, a platform-only match
	// sends 2 (both overlays have a twitch source), a broken viewer
	// exclusion sends 2 (owner + viewer in ov-other). Only the
	// platform+channel conjunct with ExcludeViewers sends exactly 1.
	sent := ss.broadcastStatusToRelevantOverlays(ctx, statusData, msgJSON)
	require.Equal(t, 1, sent, "only the matching overlay's owner socket may receive the frame")
}

// TestBroadcastStatusToRelevantOverlays_ResolverErrorSkipsOverlay pins the
// resolver-error branch: a source lookup that fails must skip the overlay
// entirely — sending anyway would route a foreign channel's status to an
// unrelated overlay.
func TestBroadcastStatusToRelevantOverlays_ResolverErrorSkipsOverlay(t *testing.T) {
	m, mr := newSubscriberTestManager(t)
	ctx := context.Background()
	seedActiveSession(mr, "ov-broken")
	owner := subscriberTestConnection("ov-broken")
	m.AddConnection(ctx, owner)

	ss := NewStatusSubscriber(nil, m, zap.NewNop(), nil)
	ss.SetSourceResolver(&fakeResolver{err: map[string]error{
		"ov-broken": errors.New("db down"),
	}})

	statusData := models.PlatformStatusData{Platform: "twitch", ChannelID: "111", Status: "connected"}
	msgJSON := statusFrameJSON(t, statusData)

	sent := ss.broadcastStatusToRelevantOverlays(ctx, statusData, msgJSON)
	require.Zero(t, sent, "a failed source lookup must not fall back to sending")
}

// TestOverlayHasSource is the pure-function half of the F2a pin: the match
// must be platform AND channel. The wiring half above proves the broadcast
// path actually consults it.
func TestOverlayHasSource(t *testing.T) {
	// Cases with sources=nil/empty pin that an overlay with nothing
	// configured must never match.
	configured := []OverlaySource{
		{Platform: "twitch", ChannelID: "111", ChannelName: "mine"},
		{Platform: "youtube", ChannelID: "UC-mine", ChannelName: "mine-yt"},
	}

	cases := []struct {
		name     string
		sources  []OverlaySource
		platform string
		channel  string
		want     bool
	}{
		{"exact platform+channel match", configured, "twitch", "111", true},
		{"second source matches", configured, "youtube", "UC-mine", true},
		{"same platform, different channel must not match", configured, "twitch", "999", false},
		{"different platform, same channel id must not match", configured, "kick", "111", false},
		{"no sources configured", nil, "", "", false},
		{"empty sources never matches", []OverlaySource{}, "twitch", "111", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := overlayHasSource(tc.sources, models.PlatformStatusData{Platform: tc.platform, ChannelID: tc.channel})
			if got != tc.want {
				t.Errorf("overlayHasSource(platform=%q, channel=%q) = %v, want %v",
					tc.platform, tc.channel, got, tc.want)
			}
		})
	}
}
