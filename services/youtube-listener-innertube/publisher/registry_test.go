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

package publisher

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/caesar/all-chat/services/youtube-listener-innertube/innertube"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// fakeRegistry records Add() calls so tests can assert the publisher
// populates the message-ID registry before publishing to Redis Streams.
type fakeRegistry struct {
	mu       sync.Mutex
	addErr   error
	recorded []registryAddCall
}

type registryAddCall struct {
	platform      string
	channelID     string
	platformMsgID string
	internalUUID  string
}

func (f *fakeRegistry) Add(_ context.Context, platform, channelID, platformMsgID, internalUUID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.recorded = append(f.recorded, registryAddCall{platform, channelID, platformMsgID, internalUUID})
	return f.addErr
}

func (f *fakeRegistry) Lookup(_ context.Context, _, _, _ string) (string, error) {
	return "", errors.New("not implemented in fake")
}

func (f *fakeRegistry) Remove(_ context.Context, _, _, _ string) error {
	return nil
}

func (f *fakeRegistry) calls() []registryAddCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]registryAddCall, len(f.recorded))
	copy(out, f.recorded)
	return out
}

// TestPublish_RegistersYouTubeMessageID verifies that publishing a chat message
// with Tags["youtube_message_id"] registers a platform→internal UUID mapping so
// later deletion events can resolve the InnerTube TargetItemID to our internal
// UUID. Without this call, deletions are silently dropped (#284).
func TestPublish_RegistersYouTubeMessageID(t *testing.T) {
	publishFn := func(_ context.Context, _ []byte) error { return nil }
	pub := newStreamPublisherWithRingBuffer(publishFn, zap.NewNop(), nil, nil, prometheus.NewRegistry())
	defer pub.Stop()

	reg := &fakeRegistry{}
	pub.SetMessageIDRegistry(reg)

	msg := &innertube.RawChatMessage{
		MessageID: "internal-uuid-1",
		Platform:  "youtube",
		ChannelID: "UCabc123",
		Timestamp: time.Now().UTC(),
		Tags: map[string]string{
			"youtube_message_id": "renderer-id-xyz",
		},
	}

	require.NoError(t, pub.Publish(context.Background(), msg))

	calls := reg.calls()
	require.Len(t, calls, 1, "expected exactly one registry.Add call")
	assert.Equal(t, "youtube", calls[0].platform)
	assert.Equal(t, "UCabc123", calls[0].channelID)
	assert.Equal(t, "renderer-id-xyz", calls[0].platformMsgID)
	assert.Equal(t, "internal-uuid-1", calls[0].internalUUID)
}

// TestPublish_NoYouTubeMessageID_SkipsRegistry asserts that when a message has
// no youtube_message_id tag (e.g., a deletion or system event), the registry is
// not touched.
func TestPublish_NoYouTubeMessageID_SkipsRegistry(t *testing.T) {
	publishFn := func(_ context.Context, _ []byte) error { return nil }
	pub := newStreamPublisherWithRingBuffer(publishFn, zap.NewNop(), nil, nil, prometheus.NewRegistry())
	defer pub.Stop()

	reg := &fakeRegistry{}
	pub.SetMessageIDRegistry(reg)

	msg := &innertube.RawChatMessage{
		MessageID: "internal-uuid-2",
		Platform:  "youtube",
		ChannelID: "UCdef",
		Timestamp: time.Now().UTC(),
		Tags:      map[string]string{},
	}

	require.NoError(t, pub.Publish(context.Background(), msg))
	assert.Empty(t, reg.calls())
}

// TestPublish_NoRegistryConfigured_DoesNotPanic asserts the publisher works
// even when no registry has been wired (e.g., older deployments, tests that
// don't care about deletion routing).
func TestPublish_NoRegistryConfigured_DoesNotPanic(t *testing.T) {
	publishFn := func(_ context.Context, _ []byte) error { return nil }
	pub := newStreamPublisherWithRingBuffer(publishFn, zap.NewNop(), nil, nil, prometheus.NewRegistry())
	defer pub.Stop()

	msg := &innertube.RawChatMessage{
		MessageID: "internal-uuid-3",
		Platform:  "youtube",
		ChannelID: "UCghi",
		Timestamp: time.Now().UTC(),
		Tags: map[string]string{
			"youtube_message_id": "renderer-id-abc",
		},
	}

	require.NoError(t, pub.Publish(context.Background(), msg))
}
