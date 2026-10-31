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

package chatcmd

import (
	"strings"

	"github.com/caesar/all-chat/services/engagement-service/models"
)

// Role is a chatter's privilege level. Each level includes the ones below it, so a
// moderator may use a VIP-only command.
type Role int

const (
	RoleEveryone Role = iota
	RoleSubscriber
	RoleVIP
	RoleModerator
	RoleBroadcaster
)

var roleNames = [...]string{"everyone", "subscriber", "vip", "moderator", "broadcaster"}

func (r Role) String() string { return roleNames[r] }

// Allows reports whether a chatter with role r may use a command that requires min.
func (r Role) Allows(min Role) bool { return r >= min }

// ParseRole maps a stored min_role back to a Role.
func ParseRole(s string) (Role, bool) {
	for i, n := range roleNames {
		if n == s {
			return Role(i), true
		}
	}
	return RoleEveryone, false
}

// HighestRole folds the role names message-processor derived from the message
// (ChatRoles) into the highest level. Unknown names count as everyone.
func HighestRole(roles []string) Role {
	best := RoleEveryone
	for _, s := range roles {
		if r, ok := ParseRole(s); ok && r > best {
			best = r
		}
	}
	return best
}

// Matched is a command a message fired, with the words the template reads.
type Matched struct {
	Command models.ChatCommand
	Args    []string
	Query   string
}

// Match picks the command a chat message fires. A command whose trigger equals the
// first word wins; otherwise the first keyword trigger the text contains. Matching
// ignores case. At most one command fires per message.
func Match(cmds []models.ChatCommand, text string) (Matched, bool) {
	text = strings.TrimSpace(text)
	first, rest, _ := strings.Cut(text, " ")
	if first == "" {
		return Matched{}, false
	}
	for _, c := range cmds {
		if c.Kind == models.TriggerCommand && strings.EqualFold(c.Trigger, first) {
			rest = strings.TrimSpace(rest)
			return Matched{Command: c, Args: strings.Fields(rest), Query: rest}, true
		}
	}
	lower := strings.ToLower(text)
	for _, c := range cmds {
		if c.Kind == models.TriggerKeyword && c.Trigger != "" && strings.Contains(lower, strings.ToLower(c.Trigger)) {
			return Matched{Command: c, Args: strings.Fields(text), Query: text}, true
		}
	}
	return Matched{}, false
}
