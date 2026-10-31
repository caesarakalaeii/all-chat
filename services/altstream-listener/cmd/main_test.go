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
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/caesar/all-chat/services/altstream-listener/altstream"
	"github.com/caesar/all-chat/services/altstream-listener/publisher"
	"github.com/caesar/all-chat/shared/listener/testutil/redisutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// TestPipeline_SSEToChatRaw drives the whole listener publish path: a fake
// Altstream (REST bootstrap, emote catalog, SSE stream) feeds the real client,
// whose events go through handleEvent and the real publisher into a Redis
// stream. It pins the exact chat:raw payload the message-processor consumes.
func TestPipeline_SSEToChatRaw(t *testing.T) {
	_, rdb := redisutil.StartTestRedisWithClient(t)
	pub := publisher.NewStreamPublisher(rdb, zap.NewNop())
	defer pub.Stop()

	emotes, err := os.ReadFile("../altstream/testdata/channel_emotes.json")
	require.NoError(t, err)

	sentAt := time.Now().Truncate(time.Second) // Altstream stamps whole seconds
	ms := sentAt.UnixMilli()
	ts := sentAt.UTC().Format(time.RFC3339Nano)

	message := fmt.Sprintf(`{"type":"message","id":"615594708","author":"Vildravn","authorUsername":"vildravn","authorImage":"https://cdn.example/v.png","authorPronouns":"He/Him","text":"choo choo :copyPeek: :nope:","sentAt":%d,"channel":"copygirl","userColor":"#8b5cf6","color":"#8b5cf6","roles":{"broadcaster":false,"moderator":false,"subscriber":true,"supporter":false},"badges":["subscriber"]}`, ms)
	highlight := fmt.Sprintf(`{"type":"highlight","id":"2024786203","author":"copygirl","authorUsername":"copygirl","text":"[HIGHLIGHT]:{}","sentAt":%d,"channel":"copygirl","data":{"from":"copygirl","rewardName":"Test Highlight","cost":1,"pointsName":null,"text":"Hi! :copyHeck:"},"userColor":"#84cc16","roles":{"broadcaster":true,"moderator":false,"subscriber":false,"supporter":false}}`, ms)
	del := fmt.Sprintf(`{"type":"delete","id":"169457509","author":"System","text":"[DELETE]:840417196","sentAt":%d,"channel":"copygirl","targetId":"840417196","source":"system"}`, ms)
	clear := fmt.Sprintf(`{"type":"clear","id":"793486440","author":"System","text":"[CLEAR_CHAT]","sentAt":%d,"channel":"copygirl","source":"system"}`, ms)
	raid := fmt.Sprintf(`{"type":"raid","id":"519160387","author":"fletchmakes","text":"raiding","sentAt":%d,"channel":"copygirl"}`, ms)
	replayed := fmt.Sprintf(`{"type":"clear","id":"1","author":"System","text":"[CLEAR_CHAT]","sentAt":%d,"channel":"copygirl"}`, sentAt.Add(-time.Hour).UnixMilli())

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/chat":
			_, _ = w.Write([]byte(`[{"type":"message","id":"1","text":"old","sentAt":1}]`))
		case "/api/v1/channels/copygirl/emotes":
			_, _ = w.Write(emotes)
		case "/api/chat/stream":
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprintf(w, "data: [%s]\n\n: ping\n\ndata: [%s,%s,%s,%s,%s]\n\n", replayed, message, highlight, del, clear, raid)
			w.(http.Flusher).Flush()
			<-r.Context().Done()
		}
	}))
	defer srv.Close()

	client := altstream.NewClient(srv.URL, func(channel string, ev *altstream.Event) {
		handleEvent(channel, ev, []string{"overlay-1"}, pub, zap.NewNop())
	}, zap.NewNop())
	defer client.DisconnectAll()
	require.NoError(t, client.Connect("copygirl"))

	ctx := context.Background()
	require.Eventually(t, func() bool { return rdb.XLen(ctx, "chat:raw").Val() >= 4 }, 5*time.Second, 20*time.Millisecond)
	time.Sleep(100 * time.Millisecond) // nothing else (raid, replayed clear) may follow
	entries, err := rdb.XRange(ctx, "chat:raw", "-", "+").Result()
	require.NoError(t, err)
	require.Len(t, entries, 4)

	sticker := srv.URL + "/api/media/stickers/"
	withEmotes := func(event, emotes string) string {
		return strings.TrimSuffix(event, "}") + `,"emotes":` + emotes + `}`
	}
	want := []string{
		fmt.Sprintf(`{"message_id":"615594708","platform":"altstream","overlay_id":"overlay-1","channel_id":"copygirl","channel_name":"copygirl",
			"user_id":"vildravn","username":"vildravn","text":"choo choo :copyPeek: :nope:","tags":{"message_type":"message"},
			"raw_message":%s,"timestamp":%q}`,
			withEmotes(message, `[{"name":"copyPeek","url":"`+sticker+`copygirl_1792322555448_b51313fd3353.png"}]`), ts),
		fmt.Sprintf(`{"message_id":"2024786203","platform":"altstream","overlay_id":"overlay-1","channel_id":"copygirl","channel_name":"copygirl",
			"user_id":"copygirl","username":"copygirl","text":"Hi! :copyHeck:","tags":{"message_type":"highlight"},
			"raw_message":%s,"timestamp":%q}`,
			withEmotes(highlight, `[{"name":"copyHeck","url":"`+sticker+`copygirl_1792322574453_5e315331e5cc.png"}]`), ts),
		fmt.Sprintf(`{"message_id":"169457509","platform":"altstream","overlay_id":"overlay-1","channel_id":"copygirl","channel_name":"copygirl",
			"event_type":"message_deletion","event_data":{"deletion_type":"single","target_msg_id":"840417196"},
			"timestamp":%q}`, ts),
		fmt.Sprintf(`{"message_id":"793486440","platform":"altstream","overlay_id":"overlay-1","channel_id":"copygirl","channel_name":"copygirl",
			"event_type":"message_deletion","event_data":{"deletion_type":"clear"},
			"timestamp":%q}`, ts),
	}
	for i, e := range entries {
		assert.JSONEq(t, want[i], e.Values["data"].(string), "entry %d", i)
	}
}

func TestToRawMessage_FallsBackToDisplayName(t *testing.T) {
	// The REST-shaped catch-up events carry no authorUsername.
	ev := &altstream.Event{Type: altstream.TypeMessage, ID: "9", Author: "LEMONPOPPYSEEDGAMES", Text: "hi", Raw: []byte(`{"id":"9"}`)}
	msg, ok := toRawMessage("achie7240", ev)
	require.True(t, ok)
	assert.Equal(t, "lemonpoppyseedgames", msg.UserID)
	assert.Equal(t, "LEMONPOPPYSEEDGAMES", msg.Username)
	assert.JSONEq(t, `{"id":"9","emotes":[]}`, string(msg.RawMessage))

	_, ok = toRawMessage("achie7240", &altstream.Event{Type: altstream.TypeDelete, ID: "10"})
	assert.False(t, ok, "a delete without targetId has nothing to remove")
	_, ok = toRawMessage("achie7240", &altstream.Event{Type: "redemption", ID: "11"})
	assert.False(t, ok)

	// A sticker post has empty text and a stickerUrl the normalizer renders;
	// a message with neither would be a blank line on the overlay.
	_, ok = toRawMessage("achie7240", &altstream.Event{Type: altstream.TypeMessage, ID: "12", Author: "a", StickerURL: "/api/media/stickers/x.png", Raw: []byte(`{"id":"12"}`)})
	assert.True(t, ok)
	_, ok = toRawMessage("achie7240", &altstream.Event{Type: altstream.TypeMessage, ID: "13", Author: "a", Raw: []byte(`{"id":"13"}`)})
	assert.False(t, ok)
}
