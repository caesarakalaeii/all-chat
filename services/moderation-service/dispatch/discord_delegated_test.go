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

package dispatch

import (
	"context"
	"errors"
	"testing"

	"github.com/caesar/all-chat/services/moderation-service/clients"
	"github.com/caesar/all-chat/services/moderation-service/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// Delegated moderation on Discord (ADR-0048's platform-attested leg).
//
// Every other dispatcher can lean on the platform: it hands over the moderator's own token and
// Twitch/Kick/YouTube re-check the role on every call. Discord has no per-user moderation API, so
// the shared bot performs the write and NOTHING external re-checks anything. Whatever these tests
// pin down IS the authorization — there is no backstop behind it, which is why the denial cases
// here are as load-bearing as the success case.

// modUserID and ownerUserID are shared with the Twitch delegated tests (twitch_delegated_test.go).
const (
	guildID    = "guild-1"
	ownerSnow  = "100"
	modSnow    = "200"
	targetSnow = "300"
)

// fakeDiscordStore answers the two database reads the delegated path needs.
type fakeDiscordStore struct {
	identities map[string]string // all-chat user id -> discord snowflake
	guilds     map[string]string // all-chat user id -> the one guild they connected
	identErr   error
	guildErr   error
}

func (f *fakeDiscordStore) DiscordIdentity(_ context.Context, userID string) (string, bool, error) {
	if f.identErr != nil {
		return "", false, f.identErr
	}
	snowflake, ok := f.identities[userID]
	return snowflake, ok, nil
}

func (f *fakeDiscordStore) DiscordGuildConnectedBy(_ context.Context, userID, gid string) (bool, error) {
	if f.guildErr != nil {
		return false, f.guildErr
	}
	return f.guilds[userID] == gid, nil
}

// fakeAuthorityResolver is a guild resolver that also answers live permission reads.
type fakeAuthorityResolver struct {
	guildID  string
	guildErr error

	botPerms   uint64
	botErr     error
	botCalls   int
	members    map[string]clients.DiscordMember // snowflake -> standing
	memberErr  error
	memberCall map[string]int
}

func (f *fakeAuthorityResolver) GuildID(_ context.Context, _ string) (string, error) {
	return f.guildID, f.guildErr
}

func (f *fakeAuthorityResolver) GuildBotPermissions(_ context.Context, _ string) (uint64, error) {
	f.botCalls++
	return f.botPerms, f.botErr
}

func (f *fakeAuthorityResolver) MemberAuthority(_ context.Context, _, userID string) (clients.DiscordMember, error) {
	if f.memberCall == nil {
		f.memberCall = map[string]int{}
	}
	f.memberCall[userID]++
	if f.memberErr != nil {
		return clients.DiscordMember{}, f.memberErr
	}
	return f.members[userID], nil
}

// fullyAuthorized is the happy path every denial test perturbs exactly one field of: the owner
// controls the guild, the bot holds every moderation permission, and the moderator holds them too
// and outranks the target.
func fullyAuthorized() (*fakeDiscordStore, *fakeAuthorityResolver) {
	store := &fakeDiscordStore{
		identities: map[string]string{ownerUserID: ownerSnow, modUserID: modSnow},
		guilds:     map[string]string{ownerUserID: guildID},
	}
	resolver := &fakeAuthorityResolver{
		guildID:  guildID,
		botPerms: models.ModerationBotPermissions,
		members: map[string]clients.DiscordMember{
			ownerSnow:  {InGuild: true, IsGuildOwner: true},
			modSnow:    {InGuild: true, Permissions: models.ModerationBotPermissions, HighestRolePos: 5},
			targetSnow: {InGuild: true, HighestRolePos: 1},
		},
	}
	return store, resolver
}

func delegatedDispatcher(api *fakeDiscordAPI, store *fakeDiscordStore, resolver *fakeAuthorityResolver) *Discord {
	return NewDiscord(api, resolver, store, zap.NewNop())
}

func delegatedActor() models.Actor { return moderator(modUserID, ownerUserID) }

func banRequest() models.DispatchRequest {
	return models.DispatchRequest{Platform: "discord", ChannelID: "chan-1", TargetUserID: targetSnow}
}

func TestDiscordDelegated_PerformsWhenEveryCheckPasses(t *testing.T) {
	api := &fakeDiscordAPI{}
	store, resolver := fullyAuthorized()
	d := delegatedDispatcher(api, store, resolver)

	res, err := d.Dispatch(context.Background(), delegatedActor(), models.ActionBan, banRequest())

	require.NoError(t, err)
	assert.Equal(t, models.DispatchPerformed, res.Outcome)
	assert.Equal(t, 1, api.banCalls)
	assert.Equal(t, guildID, api.guildID)
	assert.Equal(t, targetSnow, api.targetID)
}

// The shared bot is the actor on every Discord write, so there is no per-user credential and no
// platform moderator id to report. Claiming otherwise would put a fiction in the audit row that an
// auditor reads as the no-fallback proof.
func TestDiscordDelegated_ReportsNoCredentialAttribution(t *testing.T) {
	api := &fakeDiscordAPI{}
	store, resolver := fullyAuthorized()
	d := delegatedDispatcher(api, store, resolver)

	res, err := d.Dispatch(context.Background(), delegatedActor(), models.ActionBan, banRequest())

	require.NoError(t, err)
	assert.Empty(t, res.CredentialUserID, "no per-user credential acts on Discord")
	assert.Empty(t, res.PlatformActorID, "nothing is sent as a platform moderator id")
}

func TestDiscordDelegated_UnlinkedModeratorIsRefused(t *testing.T) {
	api := &fakeDiscordAPI{}
	store, resolver := fullyAuthorized()
	delete(store.identities, modUserID)
	d := delegatedDispatcher(api, store, resolver)

	res, err := d.Dispatch(context.Background(), delegatedActor(), models.ActionBan, banRequest())

	require.NoError(t, err)
	assert.Equal(t, models.DispatchModNotLinked, res.Outcome)
	assert.Zero(t, api.banCalls)
}

// Two different people can be un-linked, and only one of them can fix it. Telling a volunteer to
// link their Discord account when it is the streamer who has not is a dead end.
func TestDiscordDelegated_UnlinkedOwnerReadsAsOwnerUnverified(t *testing.T) {
	api := &fakeDiscordAPI{}
	store, resolver := fullyAuthorized()
	delete(store.identities, ownerUserID)
	d := delegatedDispatcher(api, store, resolver)

	res, err := d.Dispatch(context.Background(), delegatedActor(), models.ActionBan, banRequest())

	require.NoError(t, err)
	assert.Equal(t, models.DispatchOwnerUnverified, res.Outcome)
	assert.Zero(t, api.banCalls)
}

func TestDiscordDelegated_GuildTheOwnerNeverConnectedIsRefused(t *testing.T) {
	api := &fakeDiscordAPI{}
	store, resolver := fullyAuthorized()
	store.guilds[ownerUserID] = "some-other-guild"
	d := delegatedDispatcher(api, store, resolver)

	res, err := d.Dispatch(context.Background(), delegatedActor(), models.ActionBan, banRequest())

	require.NoError(t, err)
	assert.Equal(t, models.DispatchOwnerUnverified, res.Outcome)
	assert.Zero(t, api.banCalls)
}

// The row proves the owner controlled the guild at invite time; the live read is what notices they
// have since lost that standing. On the delegated path both are required (ADR-0048, "Discord
// anchor strength") — a third party is acting on the strength of the owner's reach.
func TestDiscordDelegated_OwnerWhoLostControlOfTheGuildIsRefused(t *testing.T) {
	api := &fakeDiscordAPI{}
	store, resolver := fullyAuthorized()
	resolver.members[ownerSnow] = clients.DiscordMember{InGuild: true, Permissions: models.DiscordPermViewChannel}
	d := delegatedDispatcher(api, store, resolver)

	res, err := d.Dispatch(context.Background(), delegatedActor(), models.ActionBan, banRequest())

	require.NoError(t, err)
	assert.Equal(t, models.DispatchOwnerUnverified, res.Outcome)
	assert.Zero(t, api.banCalls)
}

func TestDiscordDelegated_ModeratorNotInGuildIsRefused(t *testing.T) {
	api := &fakeDiscordAPI{}
	store, resolver := fullyAuthorized()
	resolver.members[modSnow] = clients.DiscordMember{} // Discord answered 404
	d := delegatedDispatcher(api, store, resolver)

	res, err := d.Dispatch(context.Background(), delegatedActor(), models.ActionBan, banRequest())

	require.NoError(t, err)
	assert.Equal(t, models.DispatchModNotInGuild, res.Outcome)
	assert.Zero(t, api.banCalls)
}

// The whole point of the bot ∩ moderator intersection: All-Chat must never let someone do through
// the bot what Discord would refuse them directly.
func TestDiscordDelegated_ModeratorWithoutThePermissionCannotBorrowTheBots(t *testing.T) {
	api := &fakeDiscordAPI{}
	store, resolver := fullyAuthorized()
	resolver.members[modSnow] = clients.DiscordMember{
		InGuild: true, Permissions: models.DiscordPermManageMessages, HighestRolePos: 5,
	}
	d := delegatedDispatcher(api, store, resolver)

	res, err := d.Dispatch(context.Background(), delegatedActor(), models.ActionBan, banRequest())

	require.NoError(t, err)
	assert.Equal(t, models.DispatchModLacksPermission, res.Outcome)
	assert.Zero(t, api.banCalls, "the bot holds BAN_MEMBERS, but the moderator does not")
}

// The bot's permissions are the ceiling, so a moderator who holds everything is still bounded by
// what the bot was invited with — and the remedy is the streamer re-inviting it, not a re-consent.
func TestDiscordDelegated_BotCeilingBoundsAnAdministratorModerator(t *testing.T) {
	api := &fakeDiscordAPI{}
	store, resolver := fullyAuthorized()
	resolver.botPerms = models.DiscordPermManageMessages
	resolver.members[modSnow] = clients.DiscordMember{
		InGuild: true, Permissions: models.DiscordPermAdministrator, HighestRolePos: 5,
	}
	d := delegatedDispatcher(api, store, resolver)

	res, err := d.Dispatch(context.Background(), delegatedActor(), models.ActionBan, banRequest())

	require.NoError(t, err)
	assert.Equal(t, models.DispatchBotMissingPermission, res.Outcome)
	assert.Zero(t, api.banCalls)
}

// Discord hierarchy-gates the ACTOR, and the actor here is the bot, which typically sits above
// everyone. Without our own check a delegated moderator could ban a member they cannot touch in
// Discord's own client.
func TestDiscordDelegated_HierarchyRefusesBanOnAHigherMember(t *testing.T) {
	api := &fakeDiscordAPI{}
	store, resolver := fullyAuthorized()
	resolver.members[targetSnow] = clients.DiscordMember{InGuild: true, HighestRolePos: 9}
	d := delegatedDispatcher(api, store, resolver)

	res, err := d.Dispatch(context.Background(), delegatedActor(), models.ActionBan, banRequest())

	require.NoError(t, err)
	assert.Equal(t, models.DispatchModBelowTarget, res.Outcome)
	assert.Zero(t, api.banCalls)
}

// Discord requires strictly-greater, and @everyone is position 0 — so two role-less members
// cannot act on each other.
func TestDiscordDelegated_HierarchyTieRefuses(t *testing.T) {
	api := &fakeDiscordAPI{}
	store, resolver := fullyAuthorized()
	resolver.members[modSnow] = clients.DiscordMember{
		InGuild: true, Permissions: models.ModerationBotPermissions, HighestRolePos: 3,
	}
	resolver.members[targetSnow] = clients.DiscordMember{InGuild: true, HighestRolePos: 3}
	d := delegatedDispatcher(api, store, resolver)

	res, err := d.Dispatch(context.Background(), delegatedActor(), models.ActionTimeout,
		models.DispatchRequest{Platform: "discord", ChannelID: "c", TargetUserID: targetSnow, DurationSeconds: 60})

	require.NoError(t, err)
	assert.Equal(t, models.DispatchModBelowTarget, res.Outcome)
	assert.Zero(t, api.timeoutCalls)
}

// Hierarchy governs member operations only. Applying it to a delete would deny an action the
// moderator can perform natively, which is its own kind of wrong answer.
func TestDiscordDelegated_HierarchyDoesNotGateDelete(t *testing.T) {
	api := &fakeDiscordAPI{}
	store, resolver := fullyAuthorized()
	resolver.members[targetSnow] = clients.DiscordMember{InGuild: true, HighestRolePos: 99}
	d := delegatedDispatcher(api, store, resolver)

	res, err := d.Dispatch(context.Background(), delegatedActor(), models.ActionDelete,
		models.DispatchRequest{Platform: "discord", ChannelID: "chan-1", NativeMessageID: "msg-1"})

	require.NoError(t, err)
	assert.Equal(t, models.DispatchPerformed, res.Outcome)
	assert.Equal(t, 1, api.deleteCalls)
	assert.Zero(t, resolver.memberCall[targetSnow], "a delete target has no member record to rank")
}

// An unban target is by definition not a guild member, so there is nothing to rank there either.
func TestDiscordDelegated_HierarchyDoesNotGateUnban(t *testing.T) {
	api := &fakeDiscordAPI{}
	store, resolver := fullyAuthorized()
	d := delegatedDispatcher(api, store, resolver)

	res, err := d.Dispatch(context.Background(), delegatedActor(), models.ActionUnban,
		models.DispatchRequest{Platform: "discord", ChannelID: "c", TargetUserID: targetSnow})

	require.NoError(t, err)
	assert.Equal(t, models.DispatchPerformed, res.Outcome)
	assert.Equal(t, 1, api.unbanCalls)
}

// On the one platform with no external backstop, a swallowed error is an authorization decision
// made on no information. Every read that feeds the decision must fail closed instead.
func TestDiscordDelegated_EveryReadFailureFailsClosed(t *testing.T) {
	cases := []struct {
		name     string
		sabotage func(*fakeDiscordStore, *fakeAuthorityResolver)
	}{
		{"guild resolution", func(_ *fakeDiscordStore, r *fakeAuthorityResolver) {
			r.guildErr = errors.New("discord down")
		}},
		{"identity lookup", func(s *fakeDiscordStore, _ *fakeAuthorityResolver) {
			s.identErr = errors.New("db down")
		}},
		{"owner guild-row lookup", func(s *fakeDiscordStore, _ *fakeAuthorityResolver) {
			s.guildErr = errors.New("db down")
		}},
		{"member authority read", func(_ *fakeDiscordStore, r *fakeAuthorityResolver) {
			r.memberErr = errors.New("discord down")
		}},
		{"bot permission read", func(_ *fakeDiscordStore, r *fakeAuthorityResolver) {
			r.botErr = errors.New("discord down")
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			api := &fakeDiscordAPI{}
			store, resolver := fullyAuthorized()
			tc.sabotage(store, resolver)
			d := delegatedDispatcher(api, store, resolver)

			res, err := d.Dispatch(context.Background(), delegatedActor(), models.ActionBan, banRequest())

			require.Error(t, err, "a read failure must never be read as permission")
			assert.NotEqual(t, models.DispatchPerformed, res.Outcome)
			assert.Zero(t, api.banCalls)
		})
	}
}

// Every action goes through authorization, not just the member ops. On Discord the bot holds the
// streamer's full guild authority, so a single unchecked verb would hand it to whoever asked.
func TestDiscordDelegated_NoActionReachesTheBotUnauthorized(t *testing.T) {
	for _, action := range []models.Action{
		models.ActionDelete, models.ActionTimeout, models.ActionBan, models.ActionUnban,
	} {
		t.Run(string(action), func(t *testing.T) {
			api := &fakeDiscordAPI{}
			store, resolver := fullyAuthorized()
			delete(store.identities, modUserID)
			d := delegatedDispatcher(api, store, resolver)

			res, err := d.Dispatch(context.Background(), delegatedActor(), action,
				models.DispatchRequest{
					Platform: "discord", ChannelID: "c", NativeMessageID: "m",
					TargetUserID: targetSnow, DurationSeconds: 60,
				})

			require.NoError(t, err)
			assert.Equal(t, models.DispatchModNotLinked, res.Outcome)
			assert.Zero(t, api.deleteCalls+api.timeoutCalls+api.banCalls+api.unbanCalls,
				"the bot must not act for an unauthorized moderator")
		})
	}
}

// A dispatcher wired without the store cannot check anything, and on Discord "cannot check" must
// never degrade into "act with the bot's full guild authority".
func TestDiscordDelegated_NoStoreRefusesRatherThanActs(t *testing.T) {
	api := &fakeDiscordAPI{}
	_, resolver := fullyAuthorized()
	d := NewDiscord(api, resolver, nil, zap.NewNop())

	res, err := d.Dispatch(context.Background(), delegatedActor(), models.ActionBan, banRequest())

	require.Error(t, err)
	assert.NotEqual(t, models.DispatchPerformed, res.Outcome)
	assert.Zero(t, api.banCalls)
}

// The owner path keeps the anchor too, at the strength ADR-0048 fixes for it: the discord_guilds
// row, and no live read. A streamer acting on a guild they never connected is refused.
func TestDiscordOwner_AnchorRequiresTheConnectedGuild(t *testing.T) {
	api := &fakeDiscordAPI{}
	store, resolver := fullyAuthorized()
	store.guilds[ownerUserID] = "some-other-guild"
	d := delegatedDispatcher(api, store, resolver)

	res, err := d.Dispatch(context.Background(), owner(ownerUserID), models.ActionBan, banRequest())

	require.NoError(t, err)
	assert.Equal(t, models.DispatchOwnerUnverified, res.Outcome)
	assert.Zero(t, api.banCalls)
}

// The row alone anchors an owner action. Requiring the live read here would have switched Discord
// moderation off for every existing Discord streamer until each completed a new account link, to
// re-prove something Discord already attested at invite time.
func TestDiscordOwner_NeedsNoDiscordLinkAndNoLiveRead(t *testing.T) {
	api := &fakeDiscordAPI{}
	store, resolver := fullyAuthorized()
	delete(store.identities, ownerUserID)
	d := delegatedDispatcher(api, store, resolver)

	res, err := d.Dispatch(context.Background(), owner(ownerUserID), models.ActionBan, banRequest())

	require.NoError(t, err)
	assert.Equal(t, models.DispatchPerformed, res.Outcome)
	assert.Equal(t, 1, api.banCalls)
	assert.Empty(t, resolver.memberCall, "no live permission read on the owner path")
	assert.Zero(t, resolver.botCalls, "and no bot-ceiling pre-check either")
}
