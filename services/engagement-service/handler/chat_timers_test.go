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

package handler

import (
	"strings"
	"testing"

	"github.com/caesar/all-chat/services/engagement-service/models"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

// The timers editor's 400s mirror the DB CHECK constraints (ADR-0081): name and
// message bounds, min one trigger, the interval and message thresholds.
func TestChatTimerValidation(t *testing.T) {
	uid := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	valid := chatTimerBody{Name: "gg", Message: "gg everyone", IntervalMinutes: new(5)}
	cases := []struct {
		name    string
		mutate  func(b *chatTimerBody)
		wantErr string
	}{
		{"blank name", func(b *chatTimerBody) { b.Name = "  " }, "name must be 1 to 50 characters"},
		{"name too long", func(b *chatTimerBody) { b.Name = strings.Repeat("x", 51) }, "name must be 1 to 50 characters"},
		{"blank message", func(b *chatTimerBody) { b.Message = "  \t " }, "message must not be blank"},
		{"message too long", func(b *chatTimerBody) { b.Message = strings.Repeat("x", 2001) }, "message must be at most 2000 characters"},
		{"interval under bound", func(b *chatTimerBody) { b.IntervalMinutes = new(4) }, "interval_minutes must be between 5 and 1440"},
		{"interval over bound", func(b *chatTimerBody) { b.IntervalMinutes = new(1441) }, "interval_minutes must be between 5 and 1440"},
		{"min messages zero", func(b *chatTimerBody) { b.IntervalMinutes = nil; b.MinMessages = new(0) }, "min_messages must be between 1 and 10000"},
		{"min messages high", func(b *chatTimerBody) { b.IntervalMinutes = nil; b.MinMessages = new(10001) }, "min_messages must be between 1 and 10000"},
		{"no trigger from valid interval", func(b *chatTimerBody) { b.IntervalMinutes = nil }, "an interval or a minimum message count is required"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			b := valid
			tc.mutate(&b)
			_, reason := b.timer(uid)
			assert.Equal(t, tc.wantErr, reason)
		})
	}

	for _, tc := range []struct {
		name string
		body chatTimerBody
	}{
		{"interval only", chatTimerBody{Name: "gg", Message: "hi", IntervalMinutes: new(5)}},
		{"messages only", chatTimerBody{Name: "gg", Message: "hi", MinMessages: new(1)}},
		{"both", chatTimerBody{Name: "gg", Message: "hi", IntervalMinutes: new(1440), MinMessages: new(10000)}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, reason := tc.body.timer(uid)
			assert.Empty(t, reason)
			assert.Equal(t, models.ChatTimer{
				UserID: uid, Name: "gg", Message: "hi",
				IntervalMinutes: tc.body.IntervalMinutes, MinMessages: tc.body.MinMessages,
				Enabled: true,
			}, got)
		})
	}
}
