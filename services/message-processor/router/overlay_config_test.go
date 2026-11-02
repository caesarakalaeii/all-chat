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

package router

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// The pronoun lookup sends the chatter's Twitch login to a third party, so it must
// follow the overlay's own pronoun toggle exactly as the OBS overlay page resolves it
// (frontend/src/app/overlay/[id]/page.tsx): visual_settings.showPronouns wins when
// present, then a boolean display_settings.show_pronouns, then the default (on).
func TestShowPronouns(t *testing.T) {
	cases := []struct {
		name            string
		display, visual string // raw JSON values; "" = key absent
		want            bool
	}{
		{"nothing set defaults on", "", "", true},
		{"display off", "false", "", false},
		{"display on", "true", "", true},
		{"display non-boolean ignored", `"false"`, "", true},
		{"visual none", "", `"none"`, false},
		{"visual inline", "", `"inline"`, true},
		{"visual wins over display off", "false", `"inline"`, true},
		{"visual none wins over display on", "true", `"none"`, false},
		{"visual null counts as set and on", "false", "null", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, showPronouns(rawJSON(tc.display), rawJSON(tc.visual)))
		})
	}
}

func rawJSON(s string) []byte {
	if s == "" {
		return nil
	}
	return []byte(s)
}

// TestOverlayConfigAgainstPostgres runs the real query and scan against Postgres, so the
// JSON paths and the jsonb-to-[]byte scan are proven, not just the resolution above.
// PR CI runs -short and has no database; run it with TEST_DATABASE_URL pointing at a
// disposable Postgres. It works in its own schema and drops it afterwards.
func TestOverlayConfigAgainstPostgres(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if testing.Short() || dsn == "" {
		t.Skip("needs TEST_DATABASE_URL (a disposable Postgres) and no -short")
	}
	ctx := context.Background()

	poolCfg, err := pgxpool.ParseConfig(dsn)
	require.NoError(t, err)
	schema := fmt.Sprintf("mp_overlay_config_test_%d", time.Now().UnixNano())
	poolCfg.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	require.NoError(t, err)
	t.Cleanup(pool.Close)

	_, err = pool.Exec(ctx, "CREATE SCHEMA "+schema)
	require.NoError(t, err)
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), "DROP SCHEMA "+schema+" CASCADE") })

	// The columns the query reads, typed as migrations 001, 041 and 054 create them.
	_, err = pool.Exec(ctx, `CREATE TABLE overlay_configs (
		overlay_id UUID NOT NULL UNIQUE,
		display_settings JSONB DEFAULT '{}'::jsonb,
		visual_settings JSONB NOT NULL DEFAULT '{}'::jsonb,
		seventv_emote_set_id TEXT
	)`)
	require.NoError(t, err)

	const (
		defaults    = "00000000-0000-0000-0000-000000000001"
		displayOff  = "00000000-0000-0000-0000-000000000002"
		visualNone  = "00000000-0000-0000-0000-000000000003"
		visualOn    = "00000000-0000-0000-0000-000000000004"
		nullDisplay = "00000000-0000-0000-0000-000000000005"
		noRow       = "00000000-0000-0000-0000-000000000006"
	)
	_, err = pool.Exec(ctx, `INSERT INTO overlay_configs (overlay_id, display_settings, visual_settings, seventv_emote_set_id) VALUES
		($1, '{}', '{}', NULL),
		($2, '{"show_pronouns": false}', '{"fontSize": "14px"}', '01ABC'),
		($3, '{"show_pronouns": true}', '{"showPronouns": "none"}', NULL),
		($4, '{"show_pronouns": false}', '{"showPronouns": "inline"}', NULL),
		($5, NULL, '{}', NULL)`,
		defaults, displayOff, visualNone, visualOn, nullDisplay)
	require.NoError(t, err)

	r := NewRouter(NewRepository(pool), zap.NewNop())
	cases := []struct {
		overlayID string
		want      OverlayConfig
	}{
		{defaults, OverlayConfig{ShowPronouns: true}},
		{displayOff, OverlayConfig{SevenTVEmoteSetID: "01ABC", ShowPronouns: false}},
		{visualNone, OverlayConfig{ShowPronouns: false}},
		{visualOn, OverlayConfig{ShowPronouns: true}},
		{nullDisplay, OverlayConfig{ShowPronouns: true}},
		{noRow, OverlayConfig{ShowPronouns: true}},
	}
	for _, tc := range cases {
		got, err := r.OverlayConfig(ctx, tc.overlayID)
		require.NoError(t, err, tc.overlayID)
		assert.Equal(t, tc.want, got, tc.overlayID)
	}
}
