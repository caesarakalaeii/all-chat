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

package enricher

import (
	"context"
	"testing"

	"github.com/caesar/all-chat/services/message-processor/cache"
	"github.com/caesar/all-chat/services/message-processor/models"
	"go.uber.org/zap"
)

// A per-user emote cache key only earns its memory when the chatter can actually
// have personal emotes. 7TV resolves a chatter's personal set through
// /v3/users/{platform}/{id}, and it only answers that for a Twitch identity: a
// TikTok username or a YouTube channel id is a guaranteed 400, which emote-service
// logs as "Failed to fetch user emotes, ignoring user-set". The response is then
// byte-identical to the channel-level one, so a per-user key stores a second copy
// of a 400-1700 emote set (~100-230 KB) for 13h, once per chatter per channel.
//
// That is what filled Redis on 2026-11-18 (AllChatRedisMemoryNearLimit at 89%).
// #1226 removed the synthetic half of the same family; this covers real chatters
// on platforms where the per-user lookup cannot succeed.
func TestNonTwitchChatterSharesChannelCacheKey(t *testing.T) {
	for _, platform := range []string{"tiktok", "youtube", "kick"} {
		store := &mockEmoteCacheStore{getErr: cache.ErrCacheMiss}
		client := &mockEmoteServiceClient{emotes: []EmoteServiceEmote{{Code: "KEKW", Provider: "7tv", URL: "https://example"}}}
		e := NewEnricher(client, store, zap.NewNop())

		msg := &models.UnifiedChatMessage{
			Platform:  platform,
			ChannelID: "bardeado_2001",
			User:      models.UserInfo{ID: "_delci0"},
			Message:   models.MessageInfo{Text: "KEKW"},
			Metadata:  map[string]interface{}{},
		}
		if err := e.Enrich(context.Background(), msg); err != nil {
			t.Fatalf("%s: Enrich returned error: %v", platform, err)
		}

		if store.userCalls != 0 {
			t.Errorf("%s: chatter used %d per-user cache calls, want 0", platform, store.userCalls)
		}
		if store.setCalls != 1 {
			t.Errorf("%s: channel cache must still be filled, got %d sets", platform, store.setCalls)
		}
		if client.lastUserID != "" {
			t.Errorf("%s: sent user %q to the emote service, want none", platform, client.lastUserID)
		}
		if len(msg.Message.Emotes) != 1 {
			t.Errorf("%s: message must still enrich, got %#v", platform, msg.Message.Emotes)
		}
	}
}
