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

package consumer

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseCommand(t *testing.T) {
	tests := []struct {
		name   string
		text   string
		kind   cmdKind
		idx    int
		amount int64
		wantOK bool
	}{
		{"explicit vote", "!vote 2", cmdVote, 2, 0, true},
		{"short vote alias", "!v 3", cmdVote, 3, 0, true},
		{"bare number vote", "2", cmdVote, 2, 0, true},
		{"bare number padded", "  1  ", cmdVote, 1, 0, true},
		{"explicit predict", "!predict 1 500", cmdWager, 1, 500, true},
		{"bet alias", "!bet 2 100", cmdWager, 2, 100, true},

		{"plain chat", "hello world", cmdNone, 0, 0, false},
		{"vote without arg", "!vote", cmdNone, 0, 0, false},
		{"predict without amount", "!predict 1", cmdNone, 0, 0, false},
		{"predict non-positive amount", "!predict 1 -5", cmdNone, 0, 0, false},
		{"vote option zero", "!vote 0", cmdNone, 0, 0, false},
		{"missing bang", "vote 2", cmdNone, 0, 0, false},
		{"two numbers not bare vote", "2 3", cmdNone, 0, 0, false},
		{"large number ignored", "12345", cmdNone, 0, 0, false},
		{"empty", "", cmdNone, 0, 0, false},

		// P2-5: "+1"/"-1"/"+2" are chat-agreement idioms, NOT votes. strconv.Atoi would
		// otherwise parse the leading sign and silently count them as option votes.
		{"plus-one idiom not a vote", "+1", cmdNone, 0, 0, false},
		{"plus-two idiom not a vote", "+2", cmdNone, 0, 0, false},
		{"minus-one not a vote", "-1", cmdNone, 0, 0, false},
		{"signed explicit vote rejected", "!vote +2", cmdNone, 0, 0, false},
		{"signed wager option rejected", "!predict +1 500", cmdNone, 0, 0, false},
		{"signed wager amount rejected", "!bet 1 +500", cmdNone, 0, 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kind, idx, amount, ok := parseCommand(tt.text)
			assert.Equal(t, tt.wantOK, ok)
			if tt.wantOK {
				assert.Equal(t, tt.kind, kind)
				assert.Equal(t, tt.idx, idx)
				assert.Equal(t, tt.amount, amount)
			}
		})
	}
}
