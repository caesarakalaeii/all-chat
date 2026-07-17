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

package innertube

import (
	"encoding/base64"
	"testing"
)

// buildTestToken builds a token mirroring the real YouTube layout: an entity
// carrying an anchor field (5), the chat-type submessage (16 -> {1: chatType})
// and optionally the entity-level chat-type (13).
func buildTestToken(anchor uint64, chatType uint64, withField13 bool) string {
	var entity []byte
	entity = appendVarintField(entity, 5, anchor) // position anchor we must preserve
	sub := appendVarintField(nil, 1, chatType)
	entity = appendLenDelimited(entity, 16, sub)
	if withField13 {
		entity = appendVarintField(entity, 13, chatType)
	}
	outer := appendLenDelimited(nil, 119693434, entity)
	return base64.URLEncoding.EncodeToString(outer)
}

// chatTypeOf decodes a token and returns (field16.1, field13, anchorField5, ok).
func chatTypeOf(t *testing.T, token string) (sub16 uint64, f13 uint64, hasF13 bool, anchor uint64) {
	t.Helper()
	raw, err := decodeBase64URL(token)
	if err != nil {
		t.Fatalf("decode base64: %v", err)
	}
	outer, ok := decodeProto(raw)
	if !ok {
		t.Fatalf("decode outer proto failed")
	}
	for _, of := range outer {
		if of.num != 119693434 {
			continue
		}
		entity, ok := decodeProto(of.data)
		if !ok {
			t.Fatalf("decode entity failed")
		}
		for _, ef := range entity {
			switch ef.num {
			case 5:
				anchor = ef.varint
			case 13:
				f13 = ef.varint
				hasF13 = true
			case 16:
				sub, ok := decodeProto(ef.data)
				if !ok {
					t.Fatalf("decode field16 submsg failed")
				}
				for _, sf := range sub {
					if sf.num == 1 {
						sub16 = sf.varint
					}
				}
			}
		}
	}
	return
}

func TestForceChatTypeAll_FlipsTopToLive(t *testing.T) {
	// YouTube native token: chattype=4 (Top chat) in both field16.1 and field13.
	token := buildTestToken(123456789, uint64(ChatTypeTop), true)

	got := forceChatTypeAll(token)
	if got == token {
		t.Fatal("expected token to be rewritten, got identical token")
	}

	sub16, f13, hasF13, anchor := chatTypeOf(t, got)
	if sub16 != uint64(ChatTypeAll) {
		t.Errorf("field16.1 chattype = %d, want %d (all)", sub16, ChatTypeAll)
	}
	if !hasF13 || f13 != uint64(ChatTypeAll) {
		t.Errorf("field13 chattype = %d (present=%v), want %d (all)", f13, hasF13, ChatTypeAll)
	}
	if anchor != 123456789 {
		t.Errorf("position anchor (field 5) corrupted: got %d, want 123456789", anchor)
	}
}

func TestForceChatTypeAll_NativeWithoutField13(t *testing.T) {
	// Real /next tokens carry chattype only in field16.1 (no field13).
	token := buildTestToken(42, uint64(ChatTypeTop), false)

	got := forceChatTypeAll(token)
	sub16, _, hasF13, anchor := chatTypeOf(t, got)
	if sub16 != uint64(ChatTypeAll) {
		t.Errorf("field16.1 chattype = %d, want %d (all)", sub16, ChatTypeAll)
	}
	if hasF13 {
		t.Error("field13 should not be added when YouTube's token omits it")
	}
	if anchor != 42 {
		t.Errorf("position anchor corrupted: got %d, want 42", anchor)
	}
}

func TestForceChatTypeAll_AlreadyAll_NoStructuralDamage(t *testing.T) {
	// A token already at chattype=1 must remain decodable at chattype=1.
	token := buildTestToken(7, uint64(ChatTypeAll), true)
	got := forceChatTypeAll(token)
	sub16, f13, _, anchor := chatTypeOf(t, got)
	if sub16 != uint64(ChatTypeAll) || f13 != uint64(ChatTypeAll) || anchor != 7 {
		t.Errorf("unexpected mutation: sub16=%d f13=%d anchor=%d", sub16, f13, anchor)
	}
}

func TestForceChatTypeAll_InvalidBase64_ReturnsInput(t *testing.T) {
	in := "!!! not base64 @@@"
	if got := forceChatTypeAll(in); got != in {
		t.Errorf("invalid base64 should be returned unchanged, got %q", got)
	}
}

func TestForceChatTypeAll_NoEntityField_ReturnsInput(t *testing.T) {
	// Well-formed protobuf but missing the entity wrapper (field 119693434).
	other := appendLenDelimited(nil, 1, []byte("unexpected"))
	in := base64.URLEncoding.EncodeToString(other)
	if got := forceChatTypeAll(in); got != in {
		t.Errorf("token without entity field should be returned unchanged")
	}
}

func TestDecodeBase64URL_ToleratesMissingPadding(t *testing.T) {
	raw := []byte{0x08, 0x96, 0x01} // arbitrary bytes
	padded := base64.URLEncoding.EncodeToString(raw)
	unpadded := base64.RawURLEncoding.EncodeToString(raw)

	for _, s := range []string{padded, unpadded} {
		got, err := decodeBase64URL(s)
		if err != nil {
			t.Fatalf("decodeBase64URL(%q) error: %v", s, err)
		}
		if string(got) != string(raw) {
			t.Errorf("decodeBase64URL(%q) = %v, want %v", s, got, raw)
		}
	}
}
