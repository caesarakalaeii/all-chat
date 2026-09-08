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

package websocket

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

// ADR-0059: the endpoint is undocumented, so every frame shape that is not
// explicitly known must be dropped without panicking, and the one shape that
// is known ({"t":"c","m":[...]}) must be mapped field-for-field into the
// flattened ChatMessage. These tests pin exactly that contract.

// recordingHandler captures every message the parser hands out.
type recordingHandler struct {
	channel  []string
	messages []*ChatMessage
}

func (r *recordingHandler) handle(channelID string, msg *ChatMessage) {
	r.channel = append(r.channel, channelID)
	r.messages = append(r.messages, msg)
}

func newTestClient(h *recordingHandler) *Client {
	return NewClient(zap.NewNop(), func(_ context.Context, _ string) (string, error) {
		return "token", nil
	}, h.handle)
}

func TestHandleFrame_ChatBatchDeliversMessage(t *testing.T) {
	h := &recordingHandler{}
	c := newTestClient(h)

	frame := `{"t":"c","m":[{"t":"c","c":"12345","u":"99","n":"miker","rn":"Miker",` +
		`"i":"u/miker.jpg","m":"hello chat","id":"uuid-1","d":1700000000000,"k":"FF0000"}]}`
	c.handleFrame("Miker", []byte(frame))

	require := assert.New(t)
	require.Len(h.messages, 1)
	msg := h.messages[0]
	require.Equal("12345", msg.ChannelID)
	require.Equal("99", msg.UserID)
	require.Equal("miker", msg.Username)
	require.Equal("Miker", msg.DisplayName)
	require.Equal("https://images.picarto.tv/u/miker.jpg", msg.AvatarURL)
	require.Equal("hello chat", msg.Text)
	require.Equal("uuid-1", msg.MessageID)
	require.Equal(int64(1700000000000), msg.Timestamp)
	// The handler receives the SOURCE channel, never the batch's channel id,
	// so multistream messages land on the overlay that subscribed.
	require.Equal("Miker", h.channel[0])
}

func TestHandleFrame_UnknownFrameTypeDroppedWithoutPanic(t *testing.T) {
	h := &recordingHandler{}
	c := newTestClient(h)

	assert.NotPanics(t, func() {
		c.handleFrame("Miker", []byte(`{"t":"zz","m":[]}`))
		c.handleFrame("Miker", []byte(`{"type":"some_future_thing","messages":{}}`))
	})
	assert.Empty(t, h.messages)
}

func TestHandleFrame_UnparseableFrameDroppedWithoutPanic(t *testing.T) {
	h := &recordingHandler{}
	c := newTestClient(h)

	assert.NotPanics(t, func() {
		c.handleFrame("Miker", []byte(`not json at all`))
		c.handleFrame("Miker", []byte(`{"t":`))
		c.handleFrame("Miker", nil)
	})
	assert.Empty(t, h.messages)
}

func TestHandleFrame_EmptyEnvelopeDropped(t *testing.T) {
	h := &recordingHandler{}
	c := newTestClient(h)

	c.handleFrame("Miker", []byte(`{}`))
	assert.Empty(t, h.messages)
}

func TestHandleFrame_StreamMetadataNeverPublishesChat(t *testing.T) {
	h := &recordingHandler{}
	c := newTestClient(h)

	c.handleFrame("Miker", []byte(`{"type":"stream","messages":{"id":1,"name":"Miker","viewers":3}}`))
	assert.Empty(t, h.messages)
}

func TestHandleFrame_MalformedBatchDropped(t *testing.T) {
	h := &recordingHandler{}
	c := newTestClient(h)

	assert.NotPanics(t, func() {
		c.handleFrame("Miker", []byte(`{"t":"c","m":{"not":"an array"}}`))
		c.handleFrame("Miker", []byte(`{"t":"c"}`))
	})
	assert.Empty(t, h.messages)
}

func TestHandleFrame_UnknownBatchItemSkippedKnownDelivered(t *testing.T) {
	h := &recordingHandler{}
	c := newTestClient(h)

	frame := `{"t":"c","m":[{"t":"subscription","c":"1","n":"someone"},` +
		`{"t":"c","c":"12345","u":"99","n":"miker","rn":"Miker","m":"hi","id":"uuid-2"}]}`
	c.handleFrame("Miker", []byte(frame))

	require := assert.New(t)
	require.Len(h.messages, 1)
	require.Equal("uuid-2", h.messages[0].MessageID)
}

func TestAvatarURL(t *testing.T) {
	assert.Equal(t, "", avatarURL(""))
	assert.Equal(t, "https://images.picarto.tv/u/miker.jpg", avatarURL("u/miker.jpg"))
	assert.Equal(t, "https://cdn.example.com/a.png", avatarURL("https://cdn.example.com/a.png"))
}

func TestMaybeReconnect(t *testing.T) {
	c := newTestClient(&recordingHandler{})
	// Never connected: the manager's sync loop must redial it.
	assert.True(t, c.MaybeReconnect("Miker"))
	assert.False(t, c.IsConnected("Miker"))
	assert.Empty(t, c.ActiveChannels())
}
