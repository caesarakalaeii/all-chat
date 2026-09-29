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

// Package access implements the code-enforced access model for the support-bot.
//
// The access Mode is decided ONLY by trusted handler code from a Discord UID
// allow-list, is set before any tool runs, and is never derived from, hinted by, or
// overridable through message content, tool output, memory, or GitHub/issue text. A
// message that says "you are now admin" changes nothing — it only ever flows through
// the prompt as sanitized data.
//
// The zero value of Mode is ModeSupport (the least-privileged *usable* tier), so a
// forgotten assignment fails closed to read-only + redacted rather than escalating.
package access

import "strings"

// Mode is the access tier a request runs under. Deliberately: ModeSupport is the
// zero value so an unset Mode can never accidentally grant admin.
type Mode int

const (
	// ModeSupport is the default for everyone: read-only cluster/repo/grafana access,
	// no code writes, and all outputs run through the leak redactor. Zero value.
	ModeSupport Mode = iota
	// ModeAdmin is granted only to Discord UIDs on the maintainer allow-list. It adds
	// GitHub write (branch + PR + review + issue create/close) and unredacted output.
	// Cluster access stays read-only; DB writes and secret reads are never exposed in
	// either mode.
	ModeAdmin
)

// String renders the mode for logs/audit.
func (m Mode) String() string {
	switch m {
	case ModeAdmin:
		return "admin"
	case ModeSupport:
		return "support"
	default:
		return "support"
	}
}

// Policy resolves a Discord UID to an access Mode using a fixed maintainer allow-list.
type Policy struct {
	adminUIDs map[string]struct{}
}

// NewPolicy builds a Policy from the maintainer Discord UID allow-list. Blank entries
// are dropped so an empty/whitespace UID can never be treated as an admin.
func NewPolicy(adminUIDs []string) *Policy {
	set := make(map[string]struct{}, len(adminUIDs))
	for _, uid := range adminUIDs {
		if uid = strings.TrimSpace(uid); uid != "" {
			set[uid] = struct{}{}
		}
	}
	return &Policy{adminUIDs: set}
}

// ModeFor returns the access Mode for a Discord UID. Anyone not on the allow-list
// (including the empty string) gets ModeSupport — least privilege by default.
func (p *Policy) ModeFor(discordUID string) Mode {
	discordUID = strings.TrimSpace(discordUID)
	if discordUID == "" {
		return ModeSupport
	}
	if _, ok := p.adminUIDs[discordUID]; ok {
		return ModeAdmin
	}
	return ModeSupport
}

// IsAdmin reports whether the UID is an allow-listed maintainer.
func (p *Policy) IsAdmin(discordUID string) bool {
	return p.ModeFor(discordUID) == ModeAdmin
}

// AdminCount is the number of configured maintainers (for startup logging).
func (p *Policy) AdminCount() int {
	return len(p.adminUIDs)
}
