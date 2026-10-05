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

// Package normalizer turns event-typed raw stream messages into the normalized
// EventInfo the alert envelope carries, by dispatching to the
// message-processor's platform normalizers. One direction only: the
// message-processor must never import this service (import cycle).
package normalizer

import (
	"fmt"

	mpmodels "github.com/caesar/all-chat/services/message-processor/models"
	mpnormalizer "github.com/caesar/all-chat/services/message-processor/normalizer"
)

// Normalize converts a raw event-typed message into the normalized event
// payload and sender. The dispatch mirrors the message-processor's
// cmd/main.go event path: platforms whose normalizer has no event branch
// (Kick, Discord) return an error rather than an invented EventInfo.
//
// Errors are deterministic for a given message (pure functions of its fields),
// so callers must treat them as permanent — retrying cannot fix them.
func Normalize(raw *mpmodels.RawChatMessage) (*mpmodels.EventInfo, mpmodels.UserInfo, error) {
	var unified *mpmodels.UnifiedChatMessage
	var err error

	switch raw.Platform {
	case "twitch":
		unified, err = mpnormalizer.NewTwitchNormalizer().NormalizeEvent(raw, raw.OverlayID)
	case "youtube":
		unified, err = mpnormalizer.NewYouTubeNormalizer().NormalizeEvent(raw, raw.OverlayID)
	case "tiktok":
		unified, err = mpnormalizer.NewTikTokNormalizer().NormalizeEvent(raw, raw.OverlayID)
	case "system":
		unified, err = mpnormalizer.NewSystemNormalizer().NormalizeEvent(raw, raw.OverlayID)
	default:
		return nil, mpmodels.UserInfo{}, fmt.Errorf("platform %q has no event normalizer", raw.Platform)
	}
	if err != nil {
		return nil, mpmodels.UserInfo{}, fmt.Errorf("normalize %s/%s event: %w", raw.Platform, raw.EventType, err)
	}

	return unified.Event, unified.User, nil
}