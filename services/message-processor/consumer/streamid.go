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
	"strconv"
	"strings"
	"time"
)

// StreamIDTime returns the time Redis assigned to a stream entry, in UTC.
// Entry IDs have the format "{unix_ms}-{sequence}", where the millisecond part
// is the server clock at XADD.
//
// Returns false when the id is malformed, so a caller can leave an ingest
// timestamp unset rather than stamp the Unix epoch.
func StreamIDTime(id string) (time.Time, bool) {
	msPart, _, _ := strings.Cut(id, "-")
	ms, err := strconv.ParseInt(msPart, 10, 64)
	if err != nil {
		return time.Time{}, false
	}
	return time.UnixMilli(ms).UTC(), true
}
