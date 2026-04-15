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
	"math/rand/v2"
	"time"
)

// ChatType controls whether the continuation token fetches all messages or
// only the algorithmically filtered "Top Chat".
type ChatType int

const (
	// ChatTypeAll fetches all messages (unfiltered "Live chat").
	ChatTypeAll ChatType = 1
	// ChatTypeTop fetches only top/filtered messages.
	ChatTypeTop ChatType = 4
)

// GenerateLiveChatContinuation builds a continuation token from scratch using
// hand-rolled protobuf encoding. This avoids depending on YouTube's response
// structure (subMenuItems) which changes frequently and whose tokens may be
// rejected by the get_live_chat endpoint.
//
// The structure is reverse-engineered from pytchat (taizan-hokuto/pytchat)
// and has been stable since ~2020.
func GenerateLiveChatContinuation(videoID, channelID string, chatType ChatType) string {
	now := time.Now()
	nowMicro := now.UnixMicro()

	// Build header (innermost layer)
	header := buildHeader(videoID, channelID)
	headerB64 := base64.URLEncoding.EncodeToString(header)

	// Build entity (middle layer)
	entity := buildEntity(headerB64, nowMicro, int(chatType))

	// Build continuation (outermost wrapper)
	var cont []byte
	cont = appendLenDelimited(cont, 119693434, entity)

	return base64.URLEncoding.EncodeToString(cont)
}

func buildHeader(videoID, channelID string) []byte {
	// field 1: chat identifier submessage
	var field1Inner []byte
	// field 1.3: video reference submessage
	var videoRef []byte
	videoRef = appendLenDelimited(videoRef, 1, []byte(videoID))
	field1Inner = appendLenDelimited(field1Inner, 3, videoRef)
	// field 1.5: channel+video reference submessage
	var chanVideoRef []byte
	chanVideoRef = appendLenDelimited(chanVideoRef, 1, []byte(channelID))
	chanVideoRef = appendLenDelimited(chanVideoRef, 2, []byte(videoID))
	field1Inner = appendLenDelimited(field1Inner, 5, chanVideoRef)

	var header []byte
	header = appendLenDelimited(header, 1, field1Inner)

	// field 3: live chat params submessage
	var field3Inner []byte
	// field 3.48687757: magic field
	var magicInner []byte
	magicInner = appendLenDelimited(magicInner, 1, []byte(videoID))
	field3Inner = appendLenDelimited(field3Inner, 48687757, magicInner)
	header = appendLenDelimited(header, 3, field3Inner)

	// field 4: constant 1
	header = appendVarintField(header, 4, 1)

	return header
}

func buildEntity(headerB64 string, nowMicro int64, chatType int) []byte {
	var entity []byte

	// field 3: base64-encoded header (as a string, not submessage)
	entity = appendLenDelimited(entity, 3, []byte(headerB64))

	// field 5: ts1 — request time with 0-3s jitter
	ts1 := nowMicro - int64(rand.Float64()*3*1e6)
	entity = appendVarintField(entity, 5, uint64(ts1))

	// fields 6, 7: zeros
	entity = appendVarintField(entity, 6, 0)
	entity = appendVarintField(entity, 7, 0)

	// field 8: constant 1
	entity = appendVarintField(entity, 8, 1)

	// field 9: body submessage
	entity = appendLenDelimited(entity, 9, buildBody(nowMicro))

	// field 10: ts3 — history start (~now for live-only)
	ts3 := nowMicro - int64(rand.Float64()*1e6)
	entity = appendVarintField(entity, 10, uint64(ts3))

	// field 11: ts4 — older reference (10-60 min ago)
	ts4 := nowMicro - int64((600+rand.Float64()*2400)*1e6)
	entity = appendVarintField(entity, 11, uint64(ts4))

	// field 13: chat type (1=all, 4=top)
	entity = appendVarintField(entity, 13, uint64(chatType))

	// field 16: submessage with chat type
	var chatTypeSub []byte
	chatTypeSub = appendVarintField(chatTypeSub, 1, uint64(chatType))
	entity = appendLenDelimited(entity, 16, chatTypeSub)

	// field 17: zero
	entity = appendVarintField(entity, 17, 0)

	// field 19: submessage with zero
	var field19 []byte
	field19 = appendVarintField(field19, 1, 0)
	entity = appendLenDelimited(entity, 19, field19)

	// field 20: ts5 — sub-second jitter
	ts5 := nowMicro - int64((0.01+rand.Float64()*0.98)*1e6)
	entity = appendVarintField(entity, 20, uint64(ts5))

	return entity
}

func buildBody(nowMicro int64) []byte {
	var body []byte
	// fields 1-4: zeros
	body = appendVarintField(body, 1, 0)
	body = appendVarintField(body, 2, 0)
	body = appendVarintField(body, 3, 0)
	body = appendVarintField(body, 4, 0)
	// field 7: empty string
	body = appendLenDelimited(body, 7, nil)
	// field 8: zero
	body = appendVarintField(body, 8, 0)
	// field 9: empty string
	body = appendLenDelimited(body, 9, nil)
	// field 10: ts2 — sub-second jitter
	ts2 := nowMicro - int64((0.01+rand.Float64()*0.98)*1e6)
	body = appendVarintField(body, 10, uint64(ts2))
	// field 11: constant 3
	body = appendVarintField(body, 11, 3)
	// field 15: zero
	body = appendVarintField(body, 15, 0)
	return body
}

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
