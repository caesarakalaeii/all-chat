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

package altstream

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// dataLines returns the payloads of the `data:` lines in the captured SSE
// fixture, skipping the `: ping` comment.
func dataLines(t *testing.T) []string {
	t.Helper()
	raw, err := os.ReadFile("testdata/sse_frames.txt")
	require.NoError(t, err)
	var out []string
	for _, line := range strings.Split(string(raw), "\n") {
		if payload, ok := strings.CutPrefix(line, "data:"); ok {
			out = append(out, payload)
		}
	}
	return out
}

func TestParseFrame_CapturedSSEFrames(t *testing.T) {
	frames := dataLines(t)
	require.Len(t, frames, 4)

	// Frame 1 is the catch-up replay in the REST shape: two events, no
	// authorUsername.
	evs, err := parseFrame(frames[0])
	require.NoError(t, err)
	require.Len(t, evs, 2)
	assert.Equal(t, "739958863", evs[0].ID)
	assert.Equal(t, "746166481", evs[1].ID)
	assert.Equal(t, TypeMessage, evs[1].Type)
	assert.Equal(t, "LEMONPOPPYSEEDGAMES", evs[1].Author)
	assert.Empty(t, evs[1].AuthorUsername)
	assert.Equal(t, int64(1793462378000), evs[1].SentAt)

	// Frame 2 carries the richer live shape.
	evs, err = parseFrame(frames[1])
	require.NoError(t, err)
	require.Len(t, evs, 1)
	assert.Equal(t, "lemonpoppyseedgames", evs[0].AuthorUsername)
	assert.Equal(t, "i forget how prominent soy is", evs[0].ChatText())

	// Raw keeps the event verbatim for the normalizer.
	var arr []json.RawMessage
	require.NoError(t, json.Unmarshal([]byte(frames[1]), &arr))
	assert.JSONEq(t, string(arr[0]), string(evs[0].Raw))

	evs, err = parseFrame(frames[3])
	require.NoError(t, err)
	require.Len(t, evs, 1)
	assert.Equal(t, "dyamode96", evs[0].AuthorUsername)
	assert.Equal(t, "funny xd", evs[0].Text)
}

func TestParseFrame_EveryEventTypeFixture(t *testing.T) {
	raw, err := os.ReadFile("testdata/rest_events_by_type.json")
	require.NoError(t, err)
	var byType map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(raw, &byType))

	for typ, ev := range byType {
		evs, err := parseFrame("[" + string(ev) + "]")
		require.NoError(t, err, typ)
		require.Len(t, evs, 1, typ)
		assert.Equal(t, typ, evs[0].Type)
	}

	evs, _ := parseFrame("[" + string(byType["highlight"]) + "]")
	assert.Equal(t, "Hi!", evs[0].ChatText(), "highlight text lives in data.text")
	assert.Equal(t, "[HIGHLIGHT]:{\"from\":\"copygirl\",\"rewardName\":\"Test Highlight\",\"cost\":1,\"pointsName\":null,\"text\":\"Hi!\"}", evs[0].Text)

	evs, _ = parseFrame("[" + string(byType["delete"]) + "]")
	assert.Equal(t, "840417196", evs[0].TargetID)

	evs, _ = parseFrame("[" + string(byType["message"]) + "]")
	assert.Equal(t, "choo choo", evs[0].ChatText())
}

func TestParseFrame_MalformedInputNeverPanics(t *testing.T) {
	for _, bad := range []string{"", "not json", `{"type":"message"}`, `[`, `[1,2]`} {
		assert.NotPanics(t, func() { _, _ = parseFrame(bad) }, bad)
	}

	_, err := parseFrame(`{"type":"message","id":"1"}`)
	assert.Error(t, err, "a frame is a JSON array")

	// One undecodable element does not sink the rest of the batch.
	evs, err := parseFrame(` [{"type":"message","id":7},{"type":"message","id":"8","text":"ok"}]`)
	require.NoError(t, err)
	require.Len(t, evs, 1)
	assert.Equal(t, "8", evs[0].ID)
}
