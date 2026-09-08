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

// This file is the SINGLE point of contact with Rumble's undocumented chat
// protocol (ADR-0061): endpoint construction, SSE framing, and payload
// decoding all live here. When rumble.com changes its chat wire format, this
// is the one file to fix.

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

const (
	// chatAPIBase is the base of Rumble's internal chat API. The official
	// chat pop-up page (rumble.com/chat/popup/<chat_id>) bootstraps its
	// client with exactly this base plus the numeric chat id.
	chatAPIBase = "https://web7.rumble.com/chat/api"

	// chatStreamPath builds the SSE chat stream for one chat room:
	// {base}/chat/{chat_id}/stream.
	chatStreamPath = chatAPIBase + "/chat/%s/stream"
)

// validateChatID reports whether the channel value is a numeric Rumble chat
// id. Sources are stored as the chat id string (see README "Channel
// Discovery"); anything else is rejected before it reaches the wire.
func validateChatID(channelID string) bool {
	if channelID == "" || channelID[0] == '0' {
		return false
	}
	for _, r := range channelID {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// chatStreamURL builds the SSE endpoint for a chat id against the given
// base (empty = production base). Returns an error for anything that is not
// a numeric chat id.
func chatStreamURL(base, chatID string) (string, error) {
	if !validateChatID(chatID) {
		return "", fmt.Errorf("invalid rumble chat id %q: must be numeric", chatID)
	}
	if base == "" {
		base = chatAPIBase
	}
	return base + "/chat/" + chatID + "/stream", nil
}

// parseSSEFrame splits one SSE frame (the bytes up to a blank line) into its
// joined data payload. Comment/keepalive lines (`: -1`) and non-data fields
// are ignored; multiple data lines are joined per the SSE spec.
func parseSSEFrame(frame string) (string, bool) {
	var datas []string
	for _, line := range strings.Split(frame, "\n") {
		switch {
		case strings.HasPrefix(line, "data:"):
			datas = append(datas, strings.TrimPrefix(strings.TrimPrefix(line, "data:"), " "))
		default:
			// comment (": -1" keepalives), "event:" and other fields: ignored.
			// Rumble carries the event type inside the JSON payload.
		}
	}
	if len(datas) == 0 {
		return "", false
	}
	return strings.Join(datas, "\n"), true
}

// decodeStreamEvent parses one SSE data payload into a typed event. Returns
// nil for payloads that are not JSON objects (the rumble.com chat client does
// the same: `/^\s*{/` guard) — callers log and drop.
func decodeStreamEvent(data string) *SSEStreamEvent {
	trimmed := strings.TrimSpace(data)
	if !strings.HasPrefix(trimmed, "{") {
		return nil
	}
	var ev SSEStreamEvent
	if err := json.Unmarshal([]byte(trimmed), &ev); err != nil {
		return nil
	}
	return &ev
}

// decodeMessagesData parses the payload of the "messages" event (and the
// history slice of "init", which shares the shape).
func decodeMessagesData(raw json.RawMessage) (*SSEMessagesData, error) {
	var d SSEMessagesData
	if err := json.Unmarshal(raw, &d); err != nil {
		return nil, err
	}
	return &d, nil
}

// decodeInitData parses the payload of the "init" event.
func decodeInitData(raw json.RawMessage) (*SSEInitData, error) {
	var d SSEInitData
	if err := json.Unmarshal(raw, &d); err != nil {
		return nil, err
	}
	return &d, nil
}

// streamChatConfig holds the parsed chat configuration delivered at init.
type streamChatConfig struct {
	Badges map[string]SSEBadgeDef
}

// messageText returns the best available text for a message: the pre-joined
// Text field, else the concatenation of the text blocks.
func messageText(msg *SSEMessage) string {
	if msg.Text != "" {
		return msg.Text
	}
	if len(msg.Blocks) == 0 {
		return ""
	}
	var blocks []struct {
		Type string `json:"type"`
		Data struct {
			Text string `json:"text"`
		} `json:"data"`
	}
	if err := json.Unmarshal(msg.Blocks, &blocks); err != nil {
		return ""
	}
	var b strings.Builder
	for _, block := range blocks {
		if block.Data.Text != "" {
			if b.Len() > 0 {
				b.WriteByte(' ')
			}
			b.WriteString(block.Data.Text)
		}
	}
	return b.String()
}

// StreamCallbacks receives decoded chat activity from a StreamClient.
type StreamCallbacks struct {
	// OnMessage fires for every chat message in both init history and live
	// message batches. chatID is the numeric chat room id.
	OnMessage func(chatID string, msg SSEMessage)
	// OnInit fires once per successful connection with the chat config
	// (badge catalog). Currently informational; kept so a future badge
	// consumer does not need to touch the framing code.
	OnInit func(chatID string, cfg streamChatConfig)
}

// StreamClient reads one Rumble chat SSE stream and multiplexes decoded
// messages to the callbacks. Reconnects are owned by the caller
// (cmd/main.go), which holds the backoff schedule — matching kick-listener.
type StreamClient struct {
	log  zapShim
	base string // chat API base; empty = chatAPIBase
}

// zapShim is the minimal logger surface the parser needs, satisfied by
// *zap.Logger through loggerAdapter.
type zapShim interface {
	Info(msg string, fields ...Field)
	Warn(msg string, fields ...Field)
}

// Field is a typed key/value log pair.
type Field struct {
	Key   string
	Value interface{}
}

func str(key, value string) Field { return Field{Key: key, Value: value} }

func num(key string, v int) Field { return Field{Key: key, Value: v} }

func chatIDField(id string) []Field { return []Field{kvStr("chat_id", id)} }

func kvStr(key, value string) Field { return Field{Key: key, Value: value} }

func kvInt(key string, v int) Field { return Field{Key: key, Value: v} }

// NewStreamClient creates a stream reader wrapping a logger shim. base
// overrides the chat API endpoint (tests); empty uses the production base.
func NewStreamClient(log zapShim, base string) *StreamClient {
	return &StreamClient{log: log, base: base}
}

// streamResult reports how a stream session ended.
type streamResult int

const (
	streamStopped  streamResult = iota // context cancelled: caller wants out
	streamConnLost                     // connection error or clean EOF; caller reconnects
)

// Run connects to the chat's SSE stream and pumps events into cb until ctx is
// cancelled or the connection fails. It never panics on malformed payloads:
// anything undecodable is logged and dropped.
func (s *StreamClient) Run(ctx context.Context, httpClient *http.Client, chatID, sessionCookie string, cb StreamCallbacks) streamResult {
	url, err := chatStreamURL(s.base, chatID)
	if err != nil {
		s.log.Warn("invalid chat id, not connecting", chatIDField(chatID)...)
		return streamStopped
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		s.log.Warn("failed to build chat stream request", chatIDField(chatID)...)
		return streamConnLost
	}
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-store")
	// Anonymous reads work (spike, 2026-09-08): the official pop-up sends
	// credentials for send-message privileges, but the stream delivers all
	// chat without them. When a session cookie IS provided, forward it so
	// follower/subscriber-scoped detail is not filtered.
	if sessionCookie != "" {
		req.Header.Set("Cookie", sessionCookie)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		// Context cancellation means shutdown, not a lost connection.
		if ctx.Err() != nil {
			return streamStopped
		}
		s.log.Warn("chat stream request failed", chatIDField(chatID)...)
		return streamConnLost
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		s.log.Warn("chat stream returned non-200", append(chatIDField(chatID), kvInt("status", resp.StatusCode))...)
		return streamConnLost
	}

	return s.readBody(ctx, chatID, resp.Body, cb)
}

// readBody parses the SSE byte stream. Malformed frames are logged and
// dropped; the connection persists until it errors or the context ends.
func (s *StreamClient) readBody(ctx context.Context, chatID string, body io.Reader, cb StreamCallbacks) streamResult {
	reader := bufio.NewReaderSize(body, 64*1024)
	var frame strings.Builder

	for {
		if ctx.Err() != nil {
			return streamStopped
		}
		line, err := reader.ReadString('\n')
		if err != nil {
			if ctx.Err() != nil {
				return streamStopped
			}
			s.log.Warn("chat stream read ended", chatIDField(chatID)...)
			return streamConnLost
		}

		trimmed := strings.TrimRight(line, "\r\n")
		if trimmed == "" {
			if frame.Len() > 0 {
				s.handleFrame(chatID, frame.String(), cb)
				frame.Reset()
			}
			continue
		}
		frame.WriteString(trimmed)
		frame.WriteByte('\n')
	}
}

// handleFrame decodes one SSE frame and dispatches it. Unknown event types
// and undecodable payloads are logged and dropped, never propagated
// (defensive parser requirement, ADR-0061).
func (s *StreamClient) handleFrame(chatID, frame string, cb StreamCallbacks) {
	data, ok := parseSSEFrame(frame)
	if !ok {
		return
	}
	ev := decodeStreamEvent(data)
	if ev == nil {
		s.log.Warn("dropping non-JSON SSE payload", chatIDField(chatID)...)
		return
	}

	switch ev.Type {
	case "init":
		initData, err := decodeInitData(ev.Data)
		if err != nil {
			s.log.Warn("dropping undecodable init payload", chatIDField(chatID)...)
			return
		}
		if cb.OnInit != nil {
			cb.OnInit(chatID, streamChatConfig{Badges: initData.Config.Badges})
		}
		if cb.OnMessage != nil {
			for i := range initData.Messages {
				cb.OnMessage(chatID, initData.Messages[i])
			}
		}
	case "messages":
		msgData, err := decodeMessagesData(ev.Data)
		if err != nil {
			s.log.Warn("dropping undecodable messages payload", chatIDField(chatID)...)
			return
		}
		if cb.OnMessage != nil {
			for i := range msgData.Messages {
				cb.OnMessage(chatID, msgData.Messages[i])
			}
		}
	default:
		// Unknown event types (pinned messages, raids, gift events, ...):
		// log and drop so a rumble.com format change degrades to missing
		// extra features, never to a crash.
		s.log.Info("dropping unknown SSE event type", append(chatIDField(chatID), kvStr("event_type", ev.Type))...)
	}
}
