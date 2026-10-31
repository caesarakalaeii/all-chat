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
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func sampleVars() Vars {
	return Vars{
		User:     "Alice",
		Args:     []string{"@Bob", "x"},
		Query:    "@Bob x",
		Platform: "twitch",
		Channel:  "caesar",
		Count:    3,
		Role:     "moderator",
		Now:      time.Date(2026, 10, 21, 18, 5, 0, 0, time.UTC),
	}
}

func TestRender_Variables(t *testing.T) {
	cases := []struct{ tpl, want string }{
		{"Hello {user}!", "Hello Alice!"},
		{"hi {touser}", "hi Bob"},
		{"{query}", "@Bob x"},
		{"{1}-{2}-{3}", "@Bob-x-"},
		{"{platform}/{channel}", "twitch/caesar"},
		{"used {count} times", "used 3 times"},
		{"{time}", "18:05"},
		{"{time Europe/Berlin}", "20:05"},
		{"{unknown} stays", "{unknown} stays"},
		{"it's {it's} {user}", "it's {it's} Alice"},
		{"unclosed {user", "unclosed {user"},
		{"{time Not/AZone}", "{time Not/AZone}"},
		{"no tokens", "no tokens"},
	}
	for _, c := range cases {
		got, errs := Render(c.tpl, sampleVars())
		assert.Empty(t, errs, c.tpl)
		assert.Equal(t, c.want, got, c.tpl)
	}
}

func TestRender_ToUserFallsBackToInvoker(t *testing.T) {
	v := sampleVars()
	v.Args = nil
	got, _ := Render("{touser}", v)
	assert.Equal(t, "Alice", got)
}

func TestRender_Clip(t *testing.T) {
	v := sampleVars()
	v.Clip = "https://clips.twitch.tv/abc"
	got, _ := Render("Clip: {clip}", v)
	assert.Equal(t, "Clip: https://clips.twitch.tv/abc", got)
}

func TestRender_RandomAndPick(t *testing.T) {
	for i := 0; i < 50; i++ {
		got, _ := Render("{random 3 5}", sampleVars())
		n, err := strconv.Atoi(got)
		require.NoError(t, err)
		assert.True(t, n >= 3 && n <= 5, got)

		got, _ = Render("{random}", sampleVars())
		n, err = strconv.Atoi(got)
		require.NoError(t, err)
		assert.True(t, n >= 1 && n <= 100, got)

		got, _ = Render("{pick heads | tails}", sampleVars())
		assert.Contains(t, []string{"heads", "tails"}, got)
	}
	// Bad bounds are left literal rather than guessed at.
	got, _ := Render("{random 9 1}", sampleVars())
	assert.Equal(t, "{random 9 1}", got)
}

func TestRender_JS(t *testing.T) {
	cases := []struct{ tpl, want string }{
		{"{js user.toUpperCase()}", "ALICE"},
		{"{js const o = {a: 1}; o.a + count}", "4"},
		{"{js '}' + user}", "}Alice"},
		{"{js args.length}", "2"},
		{"{js role === 'moderator' ? 'mod' : 'pleb'}", "mod"},
		{"{js ({n: 1})}", `{"n":1}`},
		{"{js undefined}", ""},
		{"[{js var x = 2}][{js x * 21}]", "[][42]"},
		{"{js `tpl ${platform}`}", "tpl twitch"},
	}
	for _, c := range cases {
		got, errs := Render(c.tpl, sampleVars())
		assert.Empty(t, errs, c.tpl)
		assert.Equal(t, c.want, got, c.tpl)
	}
}

// Viewer text reaches JS as data, never as code: a query that is itself
// valid JS must come back verbatim, not executed.
func TestRender_JSInputIsDataNotCode(t *testing.T) {
	v := sampleVars()
	v.Query = `"; throw new Error("pwned"); "`
	got, errs := Render("{js query}", v)
	assert.Empty(t, errs)
	assert.Equal(t, v.Query, got)
}

func TestRender_JSSandbox(t *testing.T) {
	cases := []string{
		"{js while(true){}}",
		"{js 'x'.repeat(1<<30)}",
		"{js let a=[]; while(true) a.push('xxxxxxxxxxxxxxxx'+a.length)}",
		"{js throw new Error('boom')}",
		"{js require('fs')}",
		"{js fetch('http://example.com')}",
	}
	for _, tpl := range cases {
		start := time.Now()
		got, errs := Render(tpl, sampleVars())
		assert.Equal(t, jsErrorMarker, got, tpl)
		assert.Len(t, errs, 1, tpl)
		assert.Less(t, time.Since(start), 2*time.Second, tpl)
	}
}

// Many JS blocks share one time budget, so a template cannot multiply the
// per-render CPU cap by repeating a slow block.
func TestRender_JSBudgetIsPerRender(t *testing.T) {
	tpl := strings.Repeat("{js while(true){}}", 20)
	start := time.Now()
	_, errs := Render(tpl, sampleVars())
	assert.Less(t, time.Since(start), 2*jsBudget+time.Second)
	assert.Len(t, errs, 20)
}

func TestLine(t *testing.T) {
	assert.Equal(t, "a b c", Line(" a\n b\t\tc "))
	long := Line(strings.Repeat("é", 400))
	assert.LessOrEqual(t, len(long), MaxReplyBytes)
	assert.True(t, strings.HasSuffix(long, "é"), "never split a rune")
}
