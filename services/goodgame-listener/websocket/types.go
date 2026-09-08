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

// Envelope is the general GoodGame chat frame: every message on the wire,
// both directions, is {"type": "...", "data": {...}}.
// Source: GoodGame/API Chat/protocol.md.
type Envelope struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data,omitempty"`
}

// WelcomeData is the payload of the "welcome" frame sent immediately after
// the WebSocket handshake. protocolVersion 1.1 for the raw WS endpoint.
type WelcomeData struct {
	ProtocolVersion float64 `json:"protocolVersion"`
	ServerIdent     string  `json:"serverIdent"`
}

// JoinRequest is the "join" frame the client sends to enter a channel.
// channel_id is the numeric stream/chat id (as a string per protocol).
type JoinRequest struct {
	ChannelID string `json:"channel_id"`
	Hidden    bool   `json:"hidden"`
}

// UnjoinRequest is the "unjoin" frame.
type UnjoinRequest struct {
	ChannelID string `json:"channel_id"`
}

// PingRequest is the application-level keepalive frame. The GoodGame chat
// server does not use WebSocket protocol-level pings; clients send
// {"type":"ping"} periodically (see peka.online / p3ka clients) and the
// server answers with a matching "pong" envelope.
type PingRequest struct {
	ChannelID string `json:"channel_id,omitempty"`
}

// SuccessJoinData is the "success_join" response confirming membership in a
// channel. Only the fields the listener consumes are decoded.
type SuccessJoinData struct {
	ChannelID        json.Number `json:"channel_id"`
	ChannelName      string      `json:"channel_name"`
	Motd             string      `json:"motd"`
	ClientsInChannel json.Number `json:"clients_in_channel"`
	UsersInChannel   json.Number `json:"users_in_channel"`
	AccessRights     json.Number `json:"access_rights"`
}

// SuccessUnjoinData confirms an unjoin.
type SuccessUnjoinData struct {
	ChannelID json.Number `json:"channel_id"`
}

// PongData echoes the ping payload back.
type PongData struct {
	ChannelID string `json:"channel_id,omitempty"`
}

// ChatMessageData is the "message" frame: a chat message in a channel.
// Field shapes as observed on the live endpoint (2026-09): channel_id and
// user_id come as numbers OR strings depending on the field, message_id is a
// large number, timestamp is unix seconds; rights/premium/staff are numbers.
// json.Number everywhere so both encodings decode losslessly.
type ChatMessageData struct {
	ChannelID  json.Number `json:"channel_id"`
	UserID     json.Number `json:"user_id"`
	UserName   string      `json:"user_name"`
	UserRights json.Number `json:"user_rights"`
	Premium    json.Number `json:"premium"`
	Staff      json.Number `json:"staff"`
	Color      string      `json:"color"`
	Icon       string      `json:"icon"`
	Role       string      `json:"role"`
	Mobile     json.Number `json:"mobile"`
	MessageID  json.Number `json:"message_id"`
	Timestamp  json.Number `json:"timestamp"`
	Text       string      `json:"text"`
	Payments   json.Number `json:"payments"`
	GGPlusTier json.Number `json:"gg_plus_tier"`
}

// RemoveMessageData signals a deleted message.
type RemoveMessageData struct {
	ChannelID json.Number `json:"channel_id"`
	MessageID json.Number `json:"message_id"`
}

// ErrorData reports a protocol-level error.
type ErrorData struct {
	ChannelID json.Number `json:"channel_id"`
	ErrorNum  json.Number `json:"error_num"`
	ErrorMsg  string      `json:"errorMsg"`
}

// ChannelCountersData carries live viewer counters pushed periodically.
type ChannelCountersData struct {
	ChannelID        string      `json:"channel_id"`
	ClientsInChannel json.Number `json:"clients_in_channel"`
	UsersInChannel   json.Number `json:"users_in_channel"`
}
