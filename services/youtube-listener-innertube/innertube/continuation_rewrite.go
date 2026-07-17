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

import "encoding/base64"

// The get_live_chat continuation is a nested protobuf. YouTube's /next endpoint
// hands back a token whose chat-type is "Top chat" (chattype=4) by default —
// polling it silently omits the messages YouTube deems low-priority. We keep
// YouTube's token (its live-position anchor is valid, unlike a hand-rolled one
// which the endpoint answers with zero actions) but flip the chat-type to
// "Live chat" (all messages, chattype=1).
//
// Layout (only the fields we touch):
//
//	outer   { 119693434: <entity> }
//	entity  { 13: <chatType varint>, 16: { 1: <chatType varint> }, ... }
//
// forceChatTypeAll returns the input token unchanged if it cannot be parsed as
// this structure — a safe degradation to the working Top-chat token rather than
// a broken one.

// pbField is a decoded protobuf field that preserves its wire type and raw
// payload so the message can be re-encoded losslessly.
type pbField struct {
	num    uint64
	wire   byte
	varint uint64 // wire 0
	data   []byte // wire 2 (length-delimited); or wire 1/5 raw fixed bytes
}

// decodeProto parses a protobuf message, preserving field order and unknown
// fields. Returns false if the bytes are not a well-formed protobuf message.
func decodeProto(b []byte) ([]pbField, bool) {
	var fields []pbField
	i := 0
	for i < len(b) {
		tag, n := decodeVarint(b[i:])
		if n == 0 {
			return nil, false
		}
		i += n
		f := pbField{num: tag >> 3, wire: byte(tag & 0x7)}
		switch f.wire {
		case 0: // varint
			v, m := decodeVarint(b[i:])
			if m == 0 {
				return nil, false
			}
			f.varint = v
			i += m
		case 2: // length-delimited
			ln, m := decodeVarint(b[i:])
			if m == 0 {
				return nil, false
			}
			i += m
			if ln > uint64(len(b)-i) {
				return nil, false
			}
			f.data = b[i : i+int(ln)]
			i += int(ln)
		case 5: // 32-bit
			if len(b)-i < 4 {
				return nil, false
			}
			f.data = b[i : i+4]
			i += 4
		case 1: // 64-bit
			if len(b)-i < 8 {
				return nil, false
			}
			f.data = b[i : i+8]
			i += 8
		default:
			return nil, false // groups (3,4) and unknown wire types are unexpected here
		}
		fields = append(fields, f)
	}
	return fields, true
}

// encodeProto re-serialises fields produced by decodeProto.
func encodeProto(fields []pbField) []byte {
	var out []byte
	for _, f := range fields {
		out = appendTag(out, f.num, f.wire)
		switch f.wire {
		case 0:
			out = appendVarint(out, f.varint)
		case 2:
			out = appendVarint(out, uint64(len(f.data)))
			out = append(out, f.data...)
		default: // wire 1/5: raw fixed-width bytes
			out = append(out, f.data...)
		}
	}
	return out
}

// decodeVarint decodes a base-128 varint. Returns (value, bytesConsumed); the
// count is 0 if the buffer holds a truncated or over-long varint.
func decodeVarint(b []byte) (uint64, int) {
	var v uint64
	var s uint
	for i := 0; i < len(b); i++ {
		if i >= 10 { // varints are at most 10 bytes for uint64
			return 0, 0
		}
		v |= uint64(b[i]&0x7f) << s
		if b[i]&0x80 == 0 {
			return v, i + 1
		}
		s += 7
	}
	return 0, 0
}

// forceChatTypeAll rewrites a YouTube live-chat continuation token so it
// requests "Live chat" (all messages) instead of the default "Top chat".
// It flips the chat-type submessage (entity field 16, inner field 1) and, when
// present, the entity-level chat-type (field 13) to ChatTypeAll. The token is
// returned unchanged if it does not decode to the expected nested structure.
func forceChatTypeAll(token string) string {
	raw, err := decodeBase64URL(token)
	if err != nil {
		return token
	}
	outer, ok := decodeProto(raw)
	if !ok {
		return token
	}

	changed := false
	for oi := range outer {
		if outer[oi].num != 119693434 || outer[oi].wire != 2 {
			continue
		}
		entity, ok := decodeProto(outer[oi].data)
		if !ok {
			return token
		}
		for ei := range entity {
			switch {
			case entity[ei].num == 16 && entity[ei].wire == 2:
				sub, ok := decodeProto(entity[ei].data)
				if !ok {
					return token
				}
				for si := range sub {
					if sub[si].num == 1 && sub[si].wire == 0 {
						sub[si].varint = uint64(ChatTypeAll)
						changed = true
					}
				}
				entity[ei].data = encodeProto(sub)
			case entity[ei].num == 13 && entity[ei].wire == 0:
				// Only rewrite the entity-level chat-type when YouTube already
				// includes it; do not add a field YouTube's own token omits.
				entity[ei].varint = uint64(ChatTypeAll)
				changed = true
			}
		}
		outer[oi].data = encodeProto(entity)
	}

	if !changed {
		return token
	}
	return base64.URLEncoding.EncodeToString(encodeProto(outer))
}

// decodeBase64URL decodes a URL-safe base64 string, tolerating missing padding
// (YouTube continuation tokens are sometimes served unpadded).
func decodeBase64URL(s string) ([]byte, error) {
	if pad := len(s) % 4; pad != 0 {
		s += "===="[:4-pad]
	}
	return base64.URLEncoding.DecodeString(s)
}
