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

package webhooks

import (
	"testing"

	"github.com/caesar/all-chat/services/twitch-eventsub-listener/eventsub"
)

func TestBuildEmotesTag(t *testing.T) {
	tests := []struct {
		name  string
		frags []eventsub.ChatMessageFragment
		want  string
	}{
		{
			name:  "no emotes returns empty",
			frags: []eventsub.ChatMessageFragment{{Type: "text", Text: "hello world"}},
			want:  "",
		},
		{
			name: "single emote after ascii text",
			frags: []eventsub.ChatMessageFragment{
				{Type: "text", Text: "Hello "}, // 6 bytes, offsets 0-5
				{Type: "emote", Text: "Kappa", Emote: &eventsub.ChatEmote{ID: "25"}},
			},
			want: "25:6-10",
		},
		{
			// "日本 " is 3+3+1 = 7 bytes; positions must be BYTE offsets (not runes) to
			// match message-processor's byte slicing of the text.
			name: "byte offsets with multibyte text",
			frags: []eventsub.ChatMessageFragment{
				{Type: "text", Text: "日本 "},
				{Type: "emote", Text: "Kappa", Emote: &eventsub.ChatEmote{ID: "99"}},
			},
			want: "99:7-11",
		},
		{
			name: "repeated emote groups its positions",
			frags: []eventsub.ChatMessageFragment{
				{Type: "emote", Text: "Kappa", Emote: &eventsub.ChatEmote{ID: "25"}}, // 0-4
				{Type: "text", Text: " "}, // 5
				{Type: "emote", Text: "Kappa", Emote: &eventsub.ChatEmote{ID: "25"}}, // 6-10
			},
			want: "25:0-4,6-10",
		},
		{
			name: "distinct emotes preserve first-seen order",
			frags: []eventsub.ChatMessageFragment{
				{Type: "emote", Text: "Kappa", Emote: &eventsub.ChatEmote{ID: "25"}}, // 0-4
				{Type: "text", Text: " "}, // 5
				{Type: "emote", Text: "PogChamp", Emote: &eventsub.ChatEmote{ID: "88"}}, // 6-13
			},
			want: "25:0-4/88:6-13",
		},
		{
			name: "emote fragment with empty id is ignored",
			frags: []eventsub.ChatMessageFragment{
				{Type: "emote", Text: "x", Emote: &eventsub.ChatEmote{ID: ""}},
			},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := buildEmotesTag(tt.frags); got != tt.want {
				t.Fatalf("buildEmotesTag() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBuildChatTags_BadgesColorAndCheer(t *testing.T) {
	event := &eventsub.ChatMessageEvent{
		BroadcasterUserID: "12345",
		ChatterUserName:   "Viewer",
		MessageID:         "msg-abc",
		Color:             "#FF0000",
		Message:           eventsub.ChatMessageBody{Text: "hi"},
		Badges: []eventsub.ChatBadge{
			{SetID: "subscriber", ID: "12", Info: "12"},
			{SetID: "moderator", ID: "1"},
		},
		Cheer: &eventsub.ChatCheer{Bits: 100},
	}

	tags := buildChatTags(event)

	want := map[string]string{
		"room-id":      "12345", // CRITICAL: enrichers key channel lookups on this
		"id":           "msg-abc",
		"display-name": "Viewer",
		"color":        "#FF0000",
		"badges":       "subscriber/12,moderator/1",
		"subscriber":   "1",
		"mod":          "1",
		"turbo":        "0",
		"bits":         "100",
	}
	for k, v := range want {
		if got := tags[k]; got != v {
			t.Errorf("tags[%q] = %q, want %q", k, got, v)
		}
	}
	if _, ok := tags["tmi-sent-ts"]; !ok {
		t.Error("tags missing tmi-sent-ts")
	}
}

func TestBuildChatTags_BitsSummedFromCheermoteFragments(t *testing.T) {
	event := &eventsub.ChatMessageEvent{
		BroadcasterUserID: "1",
		MessageID:         "m",
		ChatterUserLogin:  "v",
		Message: eventsub.ChatMessageBody{
			Text: "Cheer100 Cheer50",
			Fragments: []eventsub.ChatMessageFragment{
				{Type: "cheermote", Text: "Cheer100", Cheermote: &eventsub.ChatCheermote{Prefix: "Cheer", Bits: 100}},
				{Type: "text", Text: " "},
				{Type: "cheermote", Text: "Cheer50", Cheermote: &eventsub.ChatCheermote{Prefix: "Cheer", Bits: 50}},
			},
		},
	}
	if got := buildChatTags(event)["bits"]; got != "150" {
		t.Errorf("bits = %q, want 150", got)
	}
}

func TestBuildChatTags_NoBadgesDefaultsFlagsToZero(t *testing.T) {
	tags := buildChatTags(&eventsub.ChatMessageEvent{
		BroadcasterUserID: "1",
		MessageID:         "m",
		ChatterUserLogin:  "v",
		Message:           eventsub.ChatMessageBody{Text: "hi"},
	})
	for _, k := range []string{"subscriber", "mod", "turbo"} {
		if tags[k] != "0" {
			t.Errorf("tags[%q] = %q, want 0", k, tags[k])
		}
	}
	if _, ok := tags["badges"]; ok {
		t.Error("badges tag should be absent when there are no badges")
	}
	if _, ok := tags["bits"]; ok {
		t.Error("bits tag should be absent when there are no bits")
	}
}

func TestBuildChatTags_SharedChat(t *testing.T) {
	tags := buildChatTags(&eventsub.ChatMessageEvent{
		BroadcasterUserID:       "1",
		MessageID:               "m",
		ChatterUserLogin:        "v",
		Message:                 eventsub.ChatMessageBody{Text: "hi"},
		SourceBroadcasterUserID: "999",
		SourceMessageID:         "src-msg",
		SourceBadges:            []eventsub.ChatBadge{{SetID: "subscriber", ID: "6"}},
	})
	want := map[string]string{
		"source-room-id": "999",
		"source-id":      "src-msg",
		"source-badges":  "subscriber/6",
	}
	for k, v := range want {
		if got := tags[k]; got != v {
			t.Errorf("tags[%q] = %q, want %q", k, got, v)
		}
	}
}
