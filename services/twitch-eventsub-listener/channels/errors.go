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

package channels

import "errors"

// ErrChatScopesMissing wraps the Twitch 403 ("missing proper authorization") a
// channel.chat.message creation returns when the owner's chat-scope grant is
// gone. Defined here (not in cmd) so both the producer (cmd/main.go's
// subscription callback) and the consumer (Manager) can identify it — the
// manager publishes the re-auth-hinted offline status ONLY for this failure.
// A plain error (429, 5xx, network) must not surface as "Auth Required" to the
// streamer: re-consenting cannot fix a Twitch outage.
var ErrChatScopesMissing = errors.New("chat scopes missing")

// errChatScopesMissingMessage is the overlay-visible hint published with the
// "offline" platform:status for a channel whose chat-scope grant is gone. It names
// OAuth so the frontend's auth-detection (PlatformStatusIndicators renders the red
// Auth Required indicator when the message contains "oauth" or "token") routes the
// streamer at the right fix: re-authorize, not reconnect.
const errChatScopesMissingMessage = "OAuth re-authorization required (chat scopes revoked)"
