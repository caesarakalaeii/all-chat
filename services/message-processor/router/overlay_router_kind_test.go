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

package router

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Alerts/goal/list overlays carry chat sources too (ADR-0069). Chat must reach
// only chat overlays, through both the direct branch and the share fan-out;
// each branch needs its own filter, so they are pinned separately.
func TestFindOverlaysForMessageRoutesOnlyChatOverlays(t *testing.T) {
	branches := strings.Split(findOverlaysForMessageSQL, "UNION")
	require.Len(t, branches, 2)
	for i, branch := range branches {
		assert.Contains(t, branch, "o.overlay_type = 'chat'", "branch %d", i)
	}
}
