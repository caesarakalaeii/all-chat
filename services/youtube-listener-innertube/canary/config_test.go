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

package canary

import (
	"testing"
	"time"
)

// envFrom builds the accessor ParseConfig expects out of a plain map, matching
// listener.Env's "missing or empty means fallback" behaviour.
func envFrom(vals map[string]string) func(key, fallback string) string {
	return func(key, fallback string) string {
		if v, ok := vals[key]; ok && v != "" {
			return v
		}
		return fallback
	}
}

func TestParseTargets(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		want    []Target
		wantErr bool
	}{
		{
			name: "two pinned targets",
			raw:  "UCchan1:vid1,UCchan2:vid2",
			want: []Target{{"UCchan1", "vid1"}, {"UCchan2", "vid2"}},
		},
		{
			name: "whitespace and trailing comma tolerated",
			raw:  " UCchan1 : vid1 , ",
			want: []Target{{"UCchan1", "vid1"}},
		},
		{
			name: "empty yields no targets",
			raw:  "",
			want: nil,
		},
		{
			// An unpinned canary would fall back to stream selection, which on
			// a multi-stream 24/7 channel can silently attach to a chat-less
			// simulcast (#473) and recreate the very false alert this replaces.
			// Reject it at startup instead.
			name:    "channel without a pinned video is rejected",
			raw:     "UCchan1",
			wantErr: true,
		},
		{
			name:    "empty video id is rejected",
			raw:     "UCchan1:",
			wantErr: true,
		},
		{
			name:    "empty channel id is rejected",
			raw:     ":vid1",
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseTargets(tc.raw)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("ParseTargets(%q) = %v, want error", tc.raw, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseTargets(%q): %v", tc.raw, err)
			}
			if len(got) != len(tc.want) {
				t.Fatalf("got %d targets (%v), want %d", len(got), got, len(tc.want))
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("target %d = %+v, want %+v", i, got[i], tc.want[i])
				}
			}
		})
	}
}

func TestParseConfigDefaultsToDisabled(t *testing.T) {
	cfg, err := ParseConfig(envFrom(nil))
	if err != nil {
		t.Fatalf("ParseConfig: %v", err)
	}
	if cfg.Enabled {
		t.Error("canary must be off unless explicitly enabled")
	}
	if cfg.PollInterval != DefaultPollInterval {
		t.Errorf("PollInterval = %v, want %v", cfg.PollInterval, DefaultPollInterval)
	}
	if cfg.RediscoverInterval != DefaultRediscoverInterval {
		t.Errorf("RediscoverInterval = %v, want %v", cfg.RediscoverInterval, DefaultRediscoverInterval)
	}
}

func TestParseConfigEnabled(t *testing.T) {
	cfg, err := ParseConfig(envFrom(map[string]string{
		"YOUTUBE_CANARY_ENABLED":             "true",
		"YOUTUBE_CANARY_CHANNELS":            "UCchan1:vid1,UCchan2:vid2",
		"YOUTUBE_CANARY_POLL_INTERVAL":       "3s",
		"YOUTUBE_CANARY_REDISCOVER_INTERVAL": "5m",
	}))
	if err != nil {
		t.Fatalf("ParseConfig: %v", err)
	}
	if !cfg.Enabled {
		t.Error("Enabled = false, want true")
	}
	if len(cfg.Targets) != 2 {
		t.Fatalf("got %d targets, want 2", len(cfg.Targets))
	}
	if cfg.PollInterval != 3*time.Second {
		t.Errorf("PollInterval = %v, want 3s", cfg.PollInterval)
	}
	if cfg.RediscoverInterval != 5*time.Minute {
		t.Errorf("RediscoverInterval = %v, want 5m", cfg.RediscoverInterval)
	}
}

// Enabled with no targets would leave the "who watches the canary" alert firing
// forever with nothing to point at, so it has to fail loudly at startup.
func TestParseConfigEnabledWithoutTargetsIsAnError(t *testing.T) {
	if _, err := ParseConfig(envFrom(map[string]string{
		"YOUTUBE_CANARY_ENABLED": "true",
	})); err == nil {
		t.Fatal("ParseConfig succeeded, want error")
	}
}

func TestParseConfigRejectsBadDurations(t *testing.T) {
	for _, key := range []string{"YOUTUBE_CANARY_POLL_INTERVAL", "YOUTUBE_CANARY_REDISCOVER_INTERVAL"} {
		if _, err := ParseConfig(envFrom(map[string]string{key: "soon"})); err == nil {
			t.Errorf("%s=soon accepted, want error", key)
		}
	}
}
