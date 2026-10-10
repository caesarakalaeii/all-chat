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

package youtube

import "testing"

func TestParseVideoID(t *testing.T) {
	const id = "dQw4w9WgXcQ"
	tests := []struct {
		name  string
		input string
		want  string
		ok    bool
	}{
		{"bare id", id, id, true},
		{"bare id with whitespace", "  " + id + "\n", id, true},
		{"watch url", "https://www.youtube.com/watch?v=" + id, id, true},
		{"watch url other param first", "https://www.youtube.com/watch?feature=share&v=" + id, id, true},
		{"watch url with timestamp", "https://www.youtube.com/watch?v=" + id + "&t=42s", id, true},
		{"watch url no www", "https://youtube.com/watch?v=" + id, id, true},
		{"watch url mobile", "https://m.youtube.com/watch?v=" + id, id, true},
		{"watch url no scheme", "www.youtube.com/watch?v=" + id, id, true},
		{"watch url bare host no scheme", "youtube.com/watch?v=" + id, id, true},
		{"watch url http", "http://www.youtube.com/watch?v=" + id, id, true},
		{"short url", "https://youtu.be/" + id, id, true},
		{"short url with share param", "https://youtu.be/" + id + "?si=AbCdEf", id, true},
		{"short url no scheme", "youtu.be/" + id, id, true},
		{"live url", "https://www.youtube.com/live/" + id, id, true},
		{"live url with share param", "https://www.youtube.com/live/" + id + "?feature=shared", id, true},
		{"live url trailing slash", "https://youtube.com/live/" + id + "/", id, true},

		{"empty", "", "", false},
		{"channel url", "https://www.youtube.com/channel/UCxxxxxxxxxxxxxxxxxxxxxx", "", false},
		{"handle url", "https://www.youtube.com/@somestreamer", "", false},
		{"channel live tab", "https://www.youtube.com/@somestreamer/live", "", false},
		{"playlist", "https://www.youtube.com/playlist?list=PLxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx", "", false},
		{"watch with only playlist", "https://www.youtube.com/watch?list=PLxxxxxxxxxxxxxxxx", "", false},
		{"bare 10-char id", "dQw4w9WgXc", "", false},
		{"bare 12-char id", "dQw4w9WgXcQQ", "", false},
		{"watch 10-char id", "https://www.youtube.com/watch?v=dQw4w9WgXc", "", false},
		{"watch 12-char id", "https://www.youtube.com/watch?v=dQw4w9WgXcQQ", "", false},
		{"short 12-char id", "https://youtu.be/dQw4w9WgXcQQ", "", false},
		{"live 12-char id", "https://www.youtube.com/live/dQw4w9WgXcQQ", "", false},
		{"bare id illegal char", "dQw4w9WgX!Q", "", false},
		{"other host", "https://vimeo.com/watch?v=" + id, "", false},
		{"lookalike host", "https://notyoutube.com/watch?v=" + id, "", false},
		{"suffix host", "https://youtube.com.example.org/watch?v=" + id, "", false},
		{"userinfo host trick", "https://youtube.com@example.org/watch?v=" + id, "", false},
		{"short url extra segment", "https://youtu.be/" + id + "/more", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := ParseVideoID(tt.input)
			if ok != tt.ok || got != tt.want {
				t.Errorf("ParseVideoID(%q) = (%q, %v), want (%q, %v)", tt.input, got, ok, tt.want, tt.ok)
			}
		})
	}
}
