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

func TestGenerateLiveChatContinuation_ProducesValidBase64(t *testing.T) {
	token := GenerateLiveChatContinuation("dQw4w9WgXcQ", "UCuAXFkgsw1L7xaCfnd5JJOw", ChatTypeAll)
	if token == "" {
		t.Fatal("expected non-empty token")
	}
	// Must be valid base64url
	decoded, err := base64.URLEncoding.DecodeString(token)
	if err != nil {
		t.Fatalf("token is not valid base64url: %v", err)
	}
	if len(decoded) < 50 {
		t.Errorf("decoded token suspiciously short: %d bytes", len(decoded))
	}
}

func TestGenerateLiveChatContinuation_DifferentChatTypes(t *testing.T) {
	allToken := GenerateLiveChatContinuation("dQw4w9WgXcQ", "UCuAXFkgsw1L7xaCfnd5JJOw", ChatTypeAll)
	topToken := GenerateLiveChatContinuation("dQw4w9WgXcQ", "UCuAXFkgsw1L7xaCfnd5JJOw", ChatTypeTop)

	if allToken == topToken {
		t.Error("ChatTypeAll and ChatTypeTop should produce different tokens")
	}
}

func TestGenerateLiveChatContinuation_HeaderContainsVideoID(t *testing.T) {
	videoID := "XSXEaikz0Bc"
	channelID := "UCSJ4gkVC6NrvII8umztf0Ow"

	// The header is built separately and contains the video ID directly
	header := buildHeader(videoID, channelID)
	if !containsBytes(header, []byte(videoID)) {
		t.Error("header does not contain video ID")
	}
	if !containsBytes(header, []byte(channelID)) {
		t.Error("header does not contain channel ID")
	}
}

func TestAppendVarint(t *testing.T) {
	tests := []struct {
		value    uint64
		expected []byte
	}{
		{0, []byte{0}},
		{1, []byte{1}},
		{127, []byte{127}},
		{128, []byte{0x80, 0x01}},
		{300, []byte{0xAC, 0x02}},
	}

	for _, tt := range tests {
		result := appendVarint(nil, tt.value)
		if len(result) != len(tt.expected) {
			t.Errorf("appendVarint(%d): got %v, want %v", tt.value, result, tt.expected)
			continue
		}
		for i := range result {
			if result[i] != tt.expected[i] {
				t.Errorf("appendVarint(%d): byte %d: got 0x%02x, want 0x%02x", tt.value, i, result[i], tt.expected[i])
			}
		}
	}
}

func TestAppendTag_LargeFieldNumber(t *testing.T) {
	// Field 119693434 wire type 2 should produce the tag d2 87 cc c8 03
	tag := appendTag(nil, 119693434, 2)
	expected := []byte{0xd2, 0x87, 0xcc, 0xc8, 0x03}
	if len(tag) != len(expected) {
		t.Fatalf("tag length: got %d, want %d (got %v)", len(tag), len(expected), tag)
	}
	for i := range tag {
		if tag[i] != expected[i] {
			t.Errorf("tag byte %d: got 0x%02x, want 0x%02x", i, tag[i], expected[i])
		}
	}
}

func containsBytes(haystack, needle []byte) bool {
	for i := 0; i <= len(haystack)-len(needle); i++ {
		match := true
		for j := range needle {
			if haystack[i+j] != needle[j] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}
