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

package irc

import (
	"regexp"
	"strings"
)

// twitchLoginRE matches the syntactic shape of a Twitch login (account name).
// Twitch logins are lowercased server-side and constrained to [a-z0-9_]; new
// account creation enforces 4–25 chars but legacy logins as short as 3 chars
// remain valid (e.g. xqc), so the lower bound is intentionally lenient. Display
// names can be Unicode/mixed-case but the wire-level JOIN must use the login.
// Sources stored as display names (e.g. "شوشو", "一代鹹魚") reach the IRC layer
// as JOINs that Twitch silently ignores — no SELFJOIN ack, no NOTICE — which
// historically drove joinAckWatchdog into a reconnect storm.
var twitchLoginRE = regexp.MustCompile(`^[a-z0-9_]{3,25}$`)

// IsValidTwitchLogin reports whether name is a syntactically valid Twitch
// login. The check is deliberately syntactic-only: it does not verify the
// account exists, just that the name *could* be a Twitch login and is worth
// sending a JOIN for. Mixed-case input is accepted; callers that store the
// result should lowercase first to match IRC wire semantics.
func IsValidTwitchLogin(name string) bool {
	return twitchLoginRE.MatchString(strings.ToLower(name))
}
