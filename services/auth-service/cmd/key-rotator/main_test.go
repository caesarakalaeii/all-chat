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

package main

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMain_FlagsParse verifies that all four flags are parsed correctly.
func TestMain_FlagsParse(t *testing.T) {
	args := []string{
		"--dry-run",
		"--batch-size=50",
		"--batch-delay-ms=10",
		"--skip-table=tiktok_oauth_tokens",
		"--skip-table=viewer_sessions",
	}
	// Provide valid env so run() gets past env validation.
	env := map[string]string{
		"DATABASE_URL":              "postgres://u:p@localhost/db",
		"TOKEN_ENCRYPTION_KEY_V1":   "dGVzdC1rZXktdjEtMzJieXRlcy1wYWRkaW5neA==", // 32-byte base64
	}

	opts, err := parseFlags(args, env)
	require.NoError(t, err)

	assert.True(t, opts.dryRun, "dry-run flag")
	assert.Equal(t, 50, opts.batchSize, "batch-size flag")
	assert.Equal(t, 10, opts.batchDelayMs, "batch-delay-ms flag")
	assert.Contains(t, opts.skipTables, "tiktok_oauth_tokens", "skip-table flag #1")
	assert.Contains(t, opts.skipTables, "viewer_sessions", "skip-table flag #2")
}

// TestMain_RequiresDatabaseURL verifies exit with error when DATABASE_URL is unset.
func TestMain_RequiresDatabaseURL(t *testing.T) {
	env := map[string]string{
		// DATABASE_URL intentionally absent
		"TOKEN_ENCRYPTION_KEY_V1": "dGVzdC1rZXktdjEtMzJieXRlcy1wYWRkaW5neA==",
	}
	code := run(context.Background(), []string{}, env)
	assert.NotEqual(t, 0, code, "should exit non-zero when DATABASE_URL is missing")
}

// TestMain_RequiresEncryptionKey verifies exit with error when no versioned key is configured.
func TestMain_RequiresEncryptionKey(t *testing.T) {
	env := map[string]string{
		"DATABASE_URL": "postgres://u:p@localhost/db",
		// No TOKEN_ENCRYPTION_KEY_V1 — should fail encryption init
	}
	code := run(context.Background(), []string{}, env)
	assert.NotEqual(t, 0, code, "should exit non-zero when no TOKEN_ENCRYPTION_KEY_Vn is set")
}

// TestMain_HelpFlag verifies --help produces usage without error (exit 0 for flag.ErrHelp path).
// Note: flag.Parse() calls os.Exit(2) on error in production but run() intercepts this.
// We test the flag set directly via parseFlags.
func TestMain_HelpFlag(t *testing.T) {
	env := map[string]string{}
	// --help triggers flag.ErrHelp which parseFlags should return as a non-nil error.
	_, err := parseFlags([]string{"--help"}, env)
	// flag returns ErrHelp for --help; we just check it's an error (not a crash).
	assert.Error(t, err, "parseFlags should return error for --help")
	assert.True(t, strings.Contains(err.Error(), "help requested") ||
		err.Error() == "flag: help requested",
		"error should mention help: %v", err)
}
