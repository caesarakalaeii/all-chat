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

// Low-level protobuf encoding helpers and chat-type constants for live-chat
// continuation tokens.
//
// Note: the listener does NOT hand-roll continuation tokens from scratch. That
// approach (GenerateLiveChatContinuation, removed 2026-07-17) produced tokens
// that get_live_chat accepted with HTTP 200 but answered with ZERO actions,
// leaving the listener blind on active streams. Instead GetInitialContinuation
// uses YouTube's own /next continuation token and rewrites only its chat-type
// via forceChatTypeAll (see continuation_rewrite.go). These encoders remain for
// that rewrite path and its tests.

// ChatType controls whether the continuation token fetches all messages or
// only the algorithmically filtered "Top Chat".
type ChatType int

const (
	// ChatTypeAll fetches all messages (unfiltered "Live chat").
	ChatTypeAll ChatType = 1
	// ChatTypeTop fetches only top/filtered messages.
	ChatTypeTop ChatType = 4
)

// --- Raw protobuf encoding helpers ---

func appendVarint(buf []byte, v uint64) []byte {
	for v >= 0x80 {
		buf = append(buf, byte(v)|0x80)
		v >>= 7
	}
	return append(buf, byte(v))
}

func appendTag(buf []byte, fieldNumber uint64, wireType byte) []byte {
	return appendVarint(buf, (fieldNumber<<3)|uint64(wireType))
}

func appendVarintField(buf []byte, fieldNumber uint64, value uint64) []byte {
	buf = appendTag(buf, fieldNumber, 0) // wire type 0 = VARINT
	return appendVarint(buf, value)
}

func appendLenDelimited(buf []byte, fieldNumber uint64, data []byte) []byte {
	buf = appendTag(buf, fieldNumber, 2) // wire type 2 = LEN
	buf = appendVarint(buf, uint64(len(data)))
	return append(buf, data...)
}
