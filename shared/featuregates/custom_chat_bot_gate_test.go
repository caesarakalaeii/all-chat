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

package featuregates

import "testing"

// The custom chat bot gate ships premium and early access (ADR-0081): a streamer's
// own bot account is a second platform rate-limit identity and the feature is
// beta-only. A seed with is_premium FALSE would hand every free user a bot
// identity the moment the backend reaches main; a key typo would leave the gate
// unknown (fail-closed but invisible in the admin UI).
func TestCustomChatBotGateSeed(t *testing.T) {
	assertSeed(t, "115_chat_bots.sql", GateCustomChatBot, true, true)
}
