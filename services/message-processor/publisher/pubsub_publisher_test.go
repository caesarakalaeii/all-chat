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
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/caesar/all-chat/services/message-processor/models"
	"github.com/caesar/all-chat/shared/metrics"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

// sharedPubSubTestMetrics avoids promauto duplicate registration across tests.
var sharedPubSubTestMetrics = metrics.NewProcessorMetrics()

func newTestPublisher(t *testing.T, mr *miniredis.Miniredis) *PubSubPublisher {
	t.Helper()
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { client.Close() })
	return NewPubSubPublisher(client, zaptest.NewLogger(t), sharedPubSubTestMetrics)
}

func TestPubSubPublisher_PublishSucceeds(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	p := newTestPublisher(t, mr)

	msg := &models.UnifiedChatMessage{
		ID:       "test-msg-1",
		Platform: "twitch",
		Message:  models.MessageInfo{Text: "hello"},
	}

	err = p.Publish(context.Background(), "overlay-123", msg)
	require.NoError(t, err)
}

func TestPubSubPublisher_PublishToMultiple_IsolatesFailures(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	p := newTestPublisher(t, mr)

	msg := &models.UnifiedChatMessage{
		ID:       "test-msg-2",
		Platform: "twitch",
		Message:  models.MessageInfo{Text: "hello"},
	}

	// Publishing to multiple overlays should work without pipeline
	err = p.PublishToMultiple(context.Background(), []string{"overlay-1", "overlay-2", "overlay-3"}, msg)
	require.NoError(t, err)
}

func TestPubSubPublisher_NewPublisher_AcceptsMetrics(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer client.Close()

	m := sharedPubSubTestMetrics
	p := NewPubSubPublisher(client, zaptest.NewLogger(t), m)
	require.NotNil(t, p)
	assert.NotNil(t, p.metrics)
}

func TestPubSubPublisher_PublishToMultiple_EmptyList(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	p := newTestPublisher(t, mr)

	msg := &models.UnifiedChatMessage{
		ID:       "test-msg-3",
		Platform: "twitch",
	}

	// Empty list should not error
	err = p.PublishToMultiple(context.Background(), []string{}, msg)
	require.NoError(t, err)
}
