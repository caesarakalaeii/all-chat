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

package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// The database half of ADR-0048's Discord leg.
//
// Discord is the one platform where nothing external re-checks a delegated moderator: every write
// authenticates as the shared bot, so All-Chat's own decision IS the authorization. These two reads
// are its non-live inputs — who someone is on Discord, and whether the overlay owner ever connected
// the guild being acted in. Both answer "no" rather than erroring on an input that cannot match a
// row, because a 500 on the authorization path is a worse failure than a denial.

// DiscordIdentity returns the Discord snowflake linked to an All-Chat account, and whether such a
// link exists at all.
//
// Needed for both roles and for different reasons: a delegated moderator's snowflake is what their
// live guild permissions are read against — the check that stands in for the platform's own — and
// the overlay owner's is what proves they still control the guild on a delegated action.
//
// The table holds no OAuth token, so there is nothing here that could leak a credential: the
// `identify` grant is used once, at link time, and discarded.
func (r *Repository) DiscordIdentity(ctx context.Context, userID string) (string, bool, error) {
	// A caller id that is not a UUID cannot match a row. Normalise rather than letting the cast
	// fail and surface as a 500.
	if _, err := uuid.Parse(userID); err != nil {
		return "", false, nil
	}
	const query = `SELECT discord_user_id FROM discord_identities WHERE user_id = $1`

	var discordUserID string
	err := r.db.QueryRow(ctx, query, userID).Scan(&discordUserID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("read discord identity: %w", err)
	}
	return discordUserID, true, nil
}

// DiscordGuildConnectedBy reports whether this user connected the All-Chat bot to this guild — the
// Discord arm of ADR-0048's owner-reach anchor.
//
// The row is not a weak stand-in for a live permission read: Discord only lets someone add a bot to
// a guild where they hold Manage Server, so a `discord_guilds` row IS Discord's own record that the
// user controlled that guild at invite time. What it cannot see is a later loss of standing, which
// is why a delegated action additionally requires the live read (ADR-0048, "Discord anchor
// strength") and an owner action does not.
//
// Scoped to the user, never to the guild alone: a row belonging to a different streamer would
// otherwise let one overlay's delegation reach another's guild.
func (r *Repository) DiscordGuildConnectedBy(ctx context.Context, userID, guildID string) (bool, error) {
	if _, err := uuid.Parse(userID); err != nil {
		return false, nil
	}
	const query = `SELECT EXISTS (SELECT 1 FROM discord_guilds WHERE user_id = $1 AND guild_id = $2)`

	var connected bool
	if err := r.db.QueryRow(ctx, query, userID, guildID).Scan(&connected); err != nil {
		return false, fmt.Errorf("check discord guild connection: %w", err)
	}
	return connected, nil
}
