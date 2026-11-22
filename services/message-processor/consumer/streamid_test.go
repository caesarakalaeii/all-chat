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
	"testing"
	"time"
)

func TestStreamIDTime(t *testing.T) {
	tests := []struct {
		name     string
		streamID string
		want     time.Time
		wantOK   bool
	}{
		{
			name:     "entry id with sequence",
			streamID: "1700000000123-0",
			want:     time.UnixMilli(1700000000123).UTC(),
			wantOK:   true,
		},
		{
			name:     "non-zero sequence does not change the time",
			streamID: "1700000000123-7",
			want:     time.UnixMilli(1700000000123).UTC(),
			wantOK:   true,
		},
		{
			name:     "bare milliseconds without a sequence",
			streamID: "1700000000123",
			want:     time.UnixMilli(1700000000123).UTC(),
			wantOK:   true,
		},
		{name: "empty id", streamID: "", wantOK: false},
		{name: "non-numeric milliseconds", streamID: "abc-0", wantOK: false},
		{name: "missing milliseconds", streamID: "-1", wantOK: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := StreamIDTime(tc.streamID)
			if ok != tc.wantOK {
				t.Fatalf("StreamIDTime(%q) ok = %v, want %v", tc.streamID, ok, tc.wantOK)
			}
			if !tc.wantOK {
				return
			}
			if !got.Equal(tc.want) {
				t.Errorf("StreamIDTime(%q) = %v, want %v", tc.streamID, got, tc.want)
			}
			if got.Location() != time.UTC {
				t.Errorf("StreamIDTime(%q) location = %v, want UTC", tc.streamID, got.Location())
			}
		})
	}
}
