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

package altstream

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/caesar/all-chat/services/altstream-listener/metrics"
)

// Event types the listener acts on. Every other type (redemption, raid,
// system, shoutout, tip, subscription) is dropped by the caller.
const (
	TypeMessage   = "message"
	TypeHighlight = "highlight" // paid highlighted message; its text is data.text
	TypeDelete    = "delete"    // targetId is the deleted message id
	TypeClear     = "clear"     // wipe chat
)

// Event is one chat event from the Altstream SSE stream or the REST history.
// Only the fields the listener reads are typed; Raw keeps the whole event
// verbatim for the message-processor normalizer. The SSE shape is richer than
// REST (authorUsername, authorImage, authorPronouns, badges), so every field
// is optional.
type Event struct {
	Type           string          `json:"type"`
	ID             string          `json:"id"` // numeric string, NOT monotonic
	Author         string          `json:"author"`
	AuthorUsername string          `json:"authorUsername"`
	Text           string          `json:"text"`
	SentAt         int64           `json:"sentAt"` // epoch milliseconds
	TargetID       string          `json:"targetId"`
	StickerURL     string          `json:"stickerUrl"` // a sticker post, sent with empty text
	Data           json.RawMessage `json:"data"`

	Raw    json.RawMessage `json:"-"`
	Emotes []Emote         `json:"-"` // resolved :name: tokens of ChatText, chat types only
}

// Emote is one :name: token the listener resolved to an image.
type Emote struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

// ChatText is the text shown in chat: data.text for a highlight (its text
// field is a "[HIGHLIGHT]:{json}" marker), the text field otherwise.
func (e *Event) ChatText() string {
	if e.Type != TypeHighlight {
		return e.Text
	}
	var d struct {
		Text string `json:"text"`
	}
	_ = json.Unmarshal(e.Data, &d)
	return d.Text
}

// parseFrame decodes the payload of one `data:` line: a JSON array of events.
// An element that does not decode is counted and skipped so it cannot sink
// the rest of the batch.
func parseFrame(data string) ([]*Event, error) {
	var raws []json.RawMessage
	if err := json.Unmarshal([]byte(strings.TrimSpace(data)), &raws); err != nil {
		return nil, fmt.Errorf("frame is not a JSON array: %w", err)
	}
	events := make([]*Event, 0, len(raws))
	for _, raw := range raws {
		ev := &Event{}
		if err := json.Unmarshal(raw, ev); err != nil {
			metrics.IncDropped("malformed_event")
			continue
		}
		ev.Raw = raw
		events = append(events, ev)
	}
	return events, nil
}
