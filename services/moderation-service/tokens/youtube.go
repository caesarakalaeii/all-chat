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

package tokens

import "github.com/caesar/all-chat/shared/youtubetoken"

// The YouTube broadcaster-credential source lives in shared/youtubetoken so that
// auth-service (streamer chat send) and moderation-service (ban dispatch) resolve the
// SAME per-channel token from youtube_oauth_tokens. Previously auth-service read
// users.access_token while moderation read youtube_oauth_tokens, so a Twitch-login
// streamer's YouTube send 401'd. These aliases keep this package's public surface
// stable for moderation's callers (dispatch, scope checker, main wiring) while the
// implementation is shared.
type (
	YouTubeSource     = youtubetoken.YouTubeSource
	YouTubeCredential = youtubetoken.YouTubeCredential
	Option            = youtubetoken.Option
)

var (
	NewYouTubeSource = youtubetoken.NewYouTubeSource
	WithTokenURL     = youtubetoken.WithTokenURL
)
