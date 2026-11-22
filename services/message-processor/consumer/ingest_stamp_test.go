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
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/caesar/all-chat/services/message-processor/models"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

// chatStreamMsg builds a plain chat stream entry carrying the given entry ID.
func chatStreamMsg(t *testing.T, streamID string) redis.XMessage {
	t.Helper()
	raw := &models.RawChatMessage{
		MessageID: "uuid-1",
		Platform:  "kick",
		ChannelID: "somechannel",
		UserID:    "42",
		Username:  "viewer",
		Text:      "hello",
		Timestamp: time.Now().UTC(),
	}
	data, err := json.Marshal(raw)
	require.NoError(t, err)
	return redis.XMessage{ID: streamID, Values: map[string]interface{}{"data": string(data)}}
}

// newIngestTestConsumer returns a consumer whose handler records the raw message it was given.
func newIngestTestConsumer(t *testing.T, seen **models.RawChatMessage) *StreamConsumer {
	t.Helper()
	mr, err := miniredis.Run()
	require.NoError(t, err)
	t.Cleanup(mr.Close)

	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { client.Close() })

	handler := func(ctx context.Context, msg *models.RawChatMessage) error {
		*seen = msg
		return nil
	}
	return NewStreamConsumer(client, zaptest.NewLogger(t), sharedTestMetrics, handler, nil, nil, "host")
}

// The ingest timestamp the gateway reports end-to-end latency against comes from the
// Redis entry ID, not from the platform timestamp inside the message body.
func TestProcessMessage_StampsIngestedAtFromStreamID(t *testing.T) {
	var seen *models.RawChatMessage
	c := newIngestTestConsumer(t, &seen)

	require.NoError(t, c.processMessage(context.Background(), chatStreamMsg(t, "1700000000123-0")))

	require.NotNil(t, seen)
	require.NotNil(t, seen.IngestedAt, "handler received a message with no ingest timestamp")
	want := time.UnixMilli(1700000000123).UTC()
	if !seen.IngestedAt.Equal(want) {
		t.Errorf("IngestedAt = %v, want %v", seen.IngestedAt, want)
	}
}

// A malformed entry id leaves the field nil rather than stamping the Unix epoch,
// which would show up as a 55-year latency in the histogram.
func TestProcessMessage_LeavesIngestedAtNilForMalformedStreamID(t *testing.T) {
	var seen *models.RawChatMessage
	c := newIngestTestConsumer(t, &seen)

	require.NoError(t, c.processMessage(context.Background(), chatStreamMsg(t, "not-an-id")))

	require.NotNil(t, seen)
	require.Nil(t, seen.IngestedAt)
}
