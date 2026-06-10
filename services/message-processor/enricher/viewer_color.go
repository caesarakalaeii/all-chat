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

package enricher

import "hash/fnv"

// autoColorPalette is a 16-color palette of bright, saturated colors chosen to
// stay legible on dark overlays (similar in spirit to Twitch's native username
// palette). Viewers with no platform-native color and no All-Chat cosmetic
// color are deterministically assigned one of these so they remain visually
// distinct from one another instead of all collapsing to the overlay's CSS
// fallback (#FFFFFF). Keep the length a power of two only by coincidence — the
// modulo in AutoColor works for any length.
var autoColorPalette = []string{
	"#FF4C4C", // red
	"#FF7F50", // coral
	"#FF8C00", // dark orange
	"#FFB300", // amber
	"#FFD400", // gold
	"#9ACD32", // yellow-green
	"#4CD964", // green
	"#2ECC71", // emerald
	"#1ABC9C", // teal
	"#5FD0E0", // cyan
	"#1E90FF", // dodger blue
	"#5B8DEF", // periwinkle
	"#9B7BFF", // violet
	"#C77DFF", // purple
	"#FF6FD8", // pink
	"#FF69B4", // hot pink
}

// AutoColor returns a deterministic palette color for the given identity key.
// The same key always maps to the same color (FNV-32a hash of the key, mod
// palette length), so a viewer keeps a stable color — within a platform when
// keyed by "platform:userID", and across every platform when keyed by their
// All-Chat viewer UUID. Nothing is persisted; the color is computed on demand.
func AutoColor(id string) string {
	h := fnv.New32a()
	// hash.Hash32.Write never returns an error, so the result is safe to ignore.
	_, _ = h.Write([]byte(id))
	return autoColorPalette[h.Sum32()%uint32(len(autoColorPalette))]
}
