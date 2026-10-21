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

// The chat commands gate ships OPEN (ADR-0079): only YouTube replies are premium, and
// that is a per-send check. A seed typo would leave the key unknown, which the cache
// reads as premium, silently closing the feature.
func TestChatCommandsGateSeed(t *testing.T) {
	assertSeed(t, "113_chat_commands.sql", GateChatCommands, false)
}
