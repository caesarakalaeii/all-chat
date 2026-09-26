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

package subscribers

import (
	"context"
	"encoding/json"
	"time"
)

// RawPublisher publishes a pre-serialised chat:raw payload.
// *publisher.StreamPublisher's ring-buffer Publish path is adapted to this
// by cmd/main.go's announceAdapter; the interface keeps subscribers free of a
// publisher import (the publisher imports innertube, which subscribers
// deliberately does not depend on beyond the wire struct).
type RawPublisher interface {
	PublishRaw(ctx context.Context, payload []byte) error
}

// PublisherAnnouncer adapts a RawPublisher to the Announcer interface by
// building the chat:raw subscriber event via BuildEventMessage and serialising
// it into the innertube.RawChatMessage JSON shape the message-processor
// consumes (the wire struct is byte-compatible).
type PublisherAnnouncer struct {
	Pub RawPublisher
}

// AnnounceSubscriber serialises one subscriber event to chat:raw.
func (a *PublisherAnnouncer) AnnounceSubscriber(ctx context.Context, ev Event) error {
	raw := BuildEventMessage(ev, time.Now())
	payload, err := json.Marshal(raw)
	if err != nil {
		return err
	}
	return a.Pub.PublishRaw(ctx, payload)
}

var _ Announcer = (*PublisherAnnouncer)(nil)
