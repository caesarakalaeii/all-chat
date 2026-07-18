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

import "errors"

// ErrNotFound signals that a provider has no emotes for the requested channel — an HTTP
// 404 / unknown-channel response. This is the NORMAL case for most channels on BTTV and
// FFZ (few streamers have those sets) and for channels without a 7TV set, so callers
// classify it as a benign "not_found" miss rather than a real API failure. Wrap it with
// %w so callers can detect it via errors.Is.
var ErrNotFound = errors.New("emotes not found for channel")
