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

import "encoding/json"

// Envelope is a frame received on the Picarto chat WebSocket
// (wss://chat.picarto.tv/chat/token=<jwt>).
//
// The protocol recognises two families of frames. Control frames use a
// single-letter "t" discriminator with the payload in "m"; the chat message
// batch is {"t":"c","m":[ChatEnvelope,...]}. Legacy server frames use a
// full-word "type" field ({"type":"stream",...}). Both are parsed and
// anything unrecognised is logged at Debug and dropped (ADR-0059).
type Envelope struct {
	// T is the control discriminator ("c" chat batch, "ur" user list update).
	T string `json:"t"`
	// Type is the legacy full-word discriminator ("stream", "chat", ...).
	Type string `json:"type"`
	// M carries the payload: a chat batch when T=="c".
	M json.RawMessage `json:"m,omitempty"`
	// Messages is the legacy payload field used alongside Type.
	Messages json.RawMessage `json:"messages,omitempty"`
}

// ChatEnvelope is one message inside a "t":"c" batch. Field names are the
// server's documented short keys (PicaBot adapter, github.com/NobreHD/PicaBot):
//
//	c = channel id, u = user id, n = username, rn = display name,
//	i = avatar path, y = ..., m = message text, id = message UUID,
//	d = unix-millisecond timestamp, k = user colour, rc = channel colour.
type ChatEnvelope struct {
	// Ct is the per-item type; "c" is a chat message.
	Ct string `json:"t"`
	// ChannelID is the chatroom the message was posted in.
	ChannelID string `json:"c"`
	// UserID is the sender's platform user id.
	UserID string `json:"u"`
	// User is the sender's username.
	User string `json:"n"`
	// DisplayName is the SEND-AS name (for multistream it is the target
	// channel the message is being shown in).
	DisplayName string `json:"rn"`
	// Avatar is a relative images.picarto.tv path; "" when absent.
	Avatar string `json:"i"`
	// Message is the raw text including :emote-name: shortcodes.
	Message string `json:"m"`
	// ID is the platform message UUID.
	ID string `json:"id"`
	// D is the unix-millisecond timestamp; empty when absent.
	D int64 `json:"d"`
	// K is the sender's nickname colour as 6 hex digits (no "#").
	K string `json:"k"`
}

// StreamInfo is the body of a legacy "type":"stream" frame. It carries channel
// metadata (id, name, online, viewers, multistream members). Only ChannelID
// and Name are read; everything else is captured in the raw payload.
type StreamInfo struct {
	ChannelID int64  `json:"id"`
	Name      string `json:"name"`
}

// ChatMessage is the flattened, platform-independent view of one received chat
// message that the client hands to its message handler.
type ChatMessage struct {
	ChannelID   string
	UserID      string
	Username    string
	DisplayName string
	AvatarURL   string
	Text        string
	MessageID   string
	Timestamp   int64 // unix milliseconds; 0 when the server sent none
}
