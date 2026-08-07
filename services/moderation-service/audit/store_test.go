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

package audit

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

const (
	ownerID = "11111111-1111-1111-1111-111111111111"
	adminID = "99999999-9999-9999-9999-999999999999"
	overlay = "aaaaaaaa-1111-1111-1111-111111111111"
)

func TestRecord_NormalActionHasNullImpersonatedBy(t *testing.T) {
	store, pool, cleanup := setupAuditDB(t)
	defer cleanup()
	ctx := context.Background()

	err := store.Record(ctx, Entry{
		OverlayID:    overlay,
		ActorUserID:  ownerID,
		Platform:     "twitch",
		ChannelID:    "somestreamer",
		Action:       "ban",
		TargetUserID: "12345",
		Reason:       "spam",
		Outcome:      OutcomeSuccess,
	})
	require.NoError(t, err)

	var actor string
	var impersonatedBy *string
	var outcome string
	err = pool.QueryRow(ctx, `SELECT actor_user_id, impersonated_by, outcome FROM moderation_actions WHERE overlay_id=$1`, overlay).
		Scan(&actor, &impersonatedBy, &outcome)
	require.NoError(t, err)
	assert.Equal(t, ownerID, actor)
	assert.Nil(t, impersonatedBy, "non-impersonated actions must store NULL impersonated_by (UUID column)")
	assert.Equal(t, OutcomeSuccess, outcome)
}

func TestRecord_ImpersonatedActionAttributesAdmin(t *testing.T) {
	store, pool, cleanup := setupAuditDB(t)
	defer cleanup()
	ctx := context.Background()

	// An admin moderating while impersonating the owner: the action runs as the
	// owner (their token), but the audit row must name the real admin.
	err := store.Record(ctx, Entry{
		OverlayID:      overlay,
		ActorUserID:    ownerID,
		ImpersonatedBy: adminID,
		Platform:       "twitch",
		ChannelID:      "somestreamer",
		Action:         "delete",
		Outcome:        OutcomeDryRun,
	})
	require.NoError(t, err)

	var actor string
	var impersonatedBy *string
	err = pool.QueryRow(ctx, `SELECT actor_user_id, impersonated_by FROM moderation_actions WHERE overlay_id=$1`, overlay).
		Scan(&actor, &impersonatedBy)
	require.NoError(t, err)
	assert.Equal(t, ownerID, actor, "actor is the impersonated owner whose token acts")
	require.NotNil(t, impersonatedBy)
	assert.Equal(t, adminID, *impersonatedBy, "the real admin must be recorded")
}

// setupAuditDB spins up Postgres and applies the moderation_actions schema.
func setupAuditDB(t *testing.T) (*Store, *pgxpool.Pool, func()) {
	t.Helper()
	ctx := context.Background()

	req := testcontainers.ContainerRequest{
		Image:        "postgres:16-alpine",
		ExposedPorts: []string{"5432/tcp"},
		Env: map[string]string{
			"POSTGRES_USER":     "testuser",
			"POSTGRES_PASSWORD": "testpass",
			"POSTGRES_DB":       "testdb",
		},
		WaitingFor: wait.ForLog("database system is ready to accept connections").
			WithOccurrence(2).
			WithStartupTimeout(60 * time.Second),
	}
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	require.NoError(t, err)

	host, err := container.Host(ctx)
	require.NoError(t, err)
	port, err := container.MappedPort(ctx, "5432")
	require.NoError(t, err)

	pool, err := pgxpool.New(ctx, "postgres://testuser:testpass@"+host+":"+port.Port()+"/testdb?sslmode=disable")
	require.NoError(t, err)

	// Matches migrations/060_moderation_actions.sql plus 080's attribution columns. Those are
	// UUID columns, which is what makes the empty-string-to-NULL mapping load-bearing rather
	// than cosmetic: an owner action carries no grant, and "" would be a cast error.
	const schema = `
		CREATE TABLE moderation_actions (
			id                   UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			overlay_id           UUID NOT NULL,
			actor_user_id        UUID NOT NULL,
			impersonated_by      UUID,
			platform             VARCHAR(50)  NOT NULL,
			channel_id           VARCHAR(100) NOT NULL,
			action               VARCHAR(20)  NOT NULL,
			target_user_id       VARCHAR(100),
			target_message_id    VARCHAR(200),
			reason               TEXT,
			outcome              VARCHAR(30)  NOT NULL,
			platform_status      TEXT,
			actor_role           VARCHAR(24),
			on_behalf_of_user_id UUID,
			credential_user_id   UUID,
			platform_actor_id    VARCHAR(100),
			grant_id             UUID,
			created_at           TIMESTAMP NOT NULL DEFAULT NOW()
		);`
	_, err = pool.Exec(ctx, schema)
	require.NoError(t, err)

	return New(pool), pool, func() {
		pool.Close()
		_ = container.Terminate(ctx)
	}
}

// The five identities (ADR-0048). A delegated action has more of them than an owner action does,
// and collapsing any pair destroys the trail the streamer needs to answer "who did this?".
func TestRecord_DelegatedActionKeepsFiveIdentitiesDistinct(t *testing.T) {
	store, pool, cleanup := setupAuditDB(t)
	defer cleanup()
	ctx := context.Background()

	const modID = "33333333-3333-4333-8333-333333333333"
	const grantID = "44444444-4444-4444-8444-444444444444"

	err := store.Record(ctx, Entry{
		OverlayID:        overlay,
		ActorUserID:      modID,
		ActorRole:        "moderator",
		OnBehalfOfUserID: ownerID,
		CredentialUserID: modID,
		PlatformActorID:  "777",
		GrantID:          grantID,
		Platform:         "twitch",
		ChannelID:        "somestreamer",
		Action:           "delete",
		Outcome:          OutcomeSuccess,
	})
	require.NoError(t, err)

	var actor, role, onBehalf, credential, platformActor, grant string
	err = pool.QueryRow(ctx, `
		SELECT actor_user_id, actor_role, on_behalf_of_user_id, credential_user_id,
		       platform_actor_id, grant_id
		FROM moderation_actions WHERE overlay_id=$1`, overlay).
		Scan(&actor, &role, &onBehalf, &credential, &platformActor, &grant)
	require.NoError(t, err)

	assert.Equal(t, modID, actor, "the human who pressed the button")
	assert.Equal(t, "moderator", role)
	assert.Equal(t, ownerID, onBehalf, "the streamer the action was performed for")
	assert.Equal(t, modID, credential,
		"the moderator's OWN token acted — this column is the proof there was no fallback")
	assert.NotEqual(t, ownerID, credential)
	assert.Equal(t, "777", platformActor, "reconcilable against Twitch's own mod log")
	assert.Equal(t, grantID, grant)
}

// An owner action leaves the delegation columns NULL rather than restating the owner in each,
// so "was this delegated?" stays answerable with a single IS NOT NULL.
func TestRecord_OwnerActionLeavesDelegationColumnsNull(t *testing.T) {
	store, pool, cleanup := setupAuditDB(t)
	defer cleanup()
	ctx := context.Background()

	err := store.Record(ctx, Entry{
		OverlayID:        overlay,
		ActorUserID:      ownerID,
		ActorRole:        "owner",
		OnBehalfOfUserID: ownerID,
		CredentialUserID: ownerID,
		PlatformActorID:  "9001",
		Platform:         "twitch",
		ChannelID:        "somestreamer",
		Action:           "ban",
		Outcome:          OutcomeSuccess,
	})
	require.NoError(t, err)

	var grant *string
	var role string
	err = pool.QueryRow(ctx, `SELECT grant_id, actor_role FROM moderation_actions WHERE overlay_id=$1`, overlay).
		Scan(&grant, &role)
	require.NoError(t, err)
	assert.Nil(t, grant, "an owner acts by ownership, not under a grant")
	assert.Equal(t, "owner", role)
}
