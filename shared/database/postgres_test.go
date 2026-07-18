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

package database

import "testing"

const testDSN = "postgres://user:pass@localhost:5432/db"

func TestEnvInt(t *testing.T) {
	cases := []struct {
		name string
		set  bool
		val  string
		def  int
		want int
	}{
		{"unset uses default", false, "", 10, 10},
		{"valid override", true, "5", 10, 5},
		{"empty uses default", true, "", 10, 10},
		{"non-numeric uses default", true, "abc", 10, 10},
		{"zero rejected", true, "0", 10, 10},
		{"negative rejected", true, "-3", 10, 10},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.set {
				t.Setenv("TEST_ENV_INT", tc.val)
			}
			if got := envInt("TEST_ENV_INT_UNSET_"+tc.name, tc.def); !tc.set && got != tc.want {
				t.Fatalf("envInt(unset) = %d, want %d", got, tc.want)
			}
			if tc.set {
				if got := envInt("TEST_ENV_INT", tc.def); got != tc.want {
					t.Fatalf("envInt(%q) = %d, want %d", tc.val, got, tc.want)
				}
			}
		})
	}
}

func TestBuildPoolConfigDefaults(t *testing.T) {
	// Ensure a clean environment.
	t.Setenv("DATABASE_MAX_CONNS", "")
	t.Setenv("DATABASE_MIN_CONNS", "")

	cfg, err := buildPoolConfig(testDSN)
	if err != nil {
		t.Fatalf("buildPoolConfig: %v", err)
	}
	if cfg.MaxConns != defaultMaxConns {
		t.Errorf("MaxConns = %d, want %d", cfg.MaxConns, defaultMaxConns)
	}
	if cfg.MinConns != defaultMinConns {
		t.Errorf("MinConns = %d, want %d", cfg.MinConns, defaultMinConns)
	}
	// Connection-budget guard: the default must stay small so that
	// (instances x MaxConns) fits under the cluster max_connections (ADR-0039).
	if cfg.MaxConns > 10 {
		t.Errorf("default MaxConns %d is too high for the shared connection budget", cfg.MaxConns)
	}
	if cfg.MinConns > 2 {
		t.Errorf("default MinConns %d holds too many idle connections per instance", cfg.MinConns)
	}
}

func TestBuildPoolConfigEnvOverride(t *testing.T) {
	t.Setenv("DATABASE_MAX_CONNS", "6")
	t.Setenv("DATABASE_MIN_CONNS", "2")

	cfg, err := buildPoolConfig(testDSN)
	if err != nil {
		t.Fatalf("buildPoolConfig: %v", err)
	}
	if cfg.MaxConns != 6 {
		t.Errorf("MaxConns = %d, want 6", cfg.MaxConns)
	}
	if cfg.MinConns != 2 {
		t.Errorf("MinConns = %d, want 2", cfg.MinConns)
	}
}

func TestBuildPoolConfigApplicationName(t *testing.T) {
	t.Setenv("DATABASE_APP_NAME", "")
	t.Setenv("OTEL_SERVICE_NAME", "")
	t.Setenv("HOSTNAME", "twitch-eventsub-listener-abc123")

	cfg, err := buildPoolConfig(testDSN)
	if err != nil {
		t.Fatalf("buildPoolConfig: %v", err)
	}
	if got := cfg.ConnConfig.RuntimeParams["application_name"]; got != "twitch-eventsub-listener-abc123" {
		t.Errorf("application_name = %q, want the hostname fallback", got)
	}
}

func TestApplicationNamePrecedence(t *testing.T) {
	t.Setenv("HOSTNAME", "pod-xyz")
	t.Setenv("OTEL_SERVICE_NAME", "otel-name")
	t.Setenv("DATABASE_APP_NAME", "explicit-name")

	if got := applicationName(); got != "explicit-name" {
		t.Errorf("applicationName() = %q, want the explicit override to win", got)
	}
}
