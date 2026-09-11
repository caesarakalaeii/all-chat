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

package channels

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// ADR-0058: an unreachable instance is a normal state. The retry decision is
// a pure function so the pacing contract here is testable without a socket.
func TestNextBackoff_CapsAtFiveMinutes(t *testing.T) {
	assert.Equal(t, 2*time.Second, nextBackoff(0))
	assert.Equal(t, 4*time.Second, nextBackoff(1))
	assert.Equal(t, 8*time.Second, nextBackoff(2))
	assert.Equal(t, 16*time.Second, nextBackoff(3))
	assert.Equal(t, 32*time.Second, nextBackoff(4))
	assert.Equal(t, 64*time.Second, nextBackoff(5))
	assert.Equal(t, 128*time.Second, nextBackoff(6))
	assert.Equal(t, 256*time.Second, nextBackoff(7))
	// Attempt 8 would be 512s; the cap binds at 5 minutes (external plan
	// requirement: slow retry, never crash-loop).
	assert.Equal(t, 300*time.Second, nextBackoff(8))
	assert.Equal(t, 300*time.Second, nextBackoff(9))
	assert.Equal(t, 300*time.Second, nextBackoff(300))
}

func TestNextBackoff_NegativeAttemptClampsToZero(t *testing.T) {
	assert.Equal(t, backoffBase, nextBackoff(-5))
}

func TestNormalizeInstanceURL(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    string
		wantErr bool
	}{
		{"bare https host", "https://watch.example.org", "https://watch.example.org", false},
		{"uppercase host lowercased", "https://WATCH.Example.ORG", "https://watch.example.org", false},
		{"trailing slash stripped", "https://watch.example.org/", "https://watch.example.org", false},
		{"http allowed", "http://localhost:8080", "http://localhost:8080", false},
		{"path rejected", "https://watch.example.org/embed", "", true},
		{"fragment rejected", "https://watch.example.org/#chat", "", true},
		{"missing scheme", "watch.example.org", "", true},
		{"missing host", "https://", "", true},
		{"empty", "   ", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NormalizeInstanceURL(tt.in)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Empty(t, got)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
