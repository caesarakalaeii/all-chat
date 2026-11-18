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

package clients

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 7TV marks overlay emotes with EmoteFlags bit 1<<8 on the emote data. Other bits
// (e.g. 1<<0 private, 1<<24 content-sexual) must not turn an emote zero-width.
func TestSevenTVParseEmoteSet_ZeroWidthFlag(t *testing.T) {
	const body = `{
		"id": "set-1",
		"emotes": [
			{"id": "a", "name": "PUNCH", "data": {"flags": 256, "host": {"url": "//cdn.7tv.app/emote/a", "files": [{"name": "1x.webp", "width": 28}]}}},
			{"id": "b", "name": "Blush", "data": {"flags": 257, "host": {"url": "//cdn.7tv.app/emote/b", "files": [{"name": "1x.webp", "width": 28}]}}},
			{"id": "c", "name": "OMEGALUL", "data": {"flags": 1, "host": {"url": "//cdn.7tv.app/emote/c", "files": [{"name": "1x.webp", "width": 28}]}}},
			{"id": "d", "name": "KEKW", "data": {"host": {"url": "//cdn.7tv.app/emote/d", "files": [{"name": "1x.webp", "width": 28}]}}}
		]
	}`
	var set sevenTVEmoteSet
	require.NoError(t, json.Unmarshal([]byte(body), &set))

	zeroWidth := map[string]bool{}
	for _, e := range (&SevenTVClient{}).parseEmoteSet(set, "xqc") {
		zeroWidth[e.Code] = e.ZeroWidth
	}

	assert.Equal(t, map[string]bool{
		"PUNCH":    true,
		"Blush":    true,
		"OMEGALUL": false,
		"KEKW":     false,
	}, zeroWidth)
}
