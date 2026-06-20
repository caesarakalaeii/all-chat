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

package patreon

import (
	"crypto/hmac"
	"crypto/md5" //nolint:gosec // tests compute the provider-mandated HMAC-MD5 webhook signature
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSubscriptionStatusFor(t *testing.T) {
	const minCents = 500
	cases := []struct {
		name string
		snap MembershipSnapshot
		want string
	}{
		{"active at threshold", MembershipSnapshot{PatronStatus: patronActive, EntitledCents: 500}, StatusActive},
		{"active above threshold", MembershipSnapshot{PatronStatus: patronActive, EntitledCents: 1000}, StatusActive},
		{"active below threshold", MembershipSnapshot{PatronStatus: patronActive, EntitledCents: 300}, StatusExpired},
		{"declined", MembershipSnapshot{PatronStatus: patronDeclined, EntitledCents: 500}, StatusDeclined},
		{"former", MembershipSnapshot{PatronStatus: patronFormer}, StatusFormer},
		{"no membership", MembershipSnapshot{PatronStatus: ""}, StatusNone},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, SubscriptionStatusFor(tc.snap, minCents))
		})
	}
}

func TestVerifyWebhookSignature(t *testing.T) {
	secret := "whsec_test"
	body := []byte(`{"data":{"type":"member"}}`)

	mac := hmac.New(md5.New, []byte(secret))
	mac.Write(body)
	valid := hex.EncodeToString(mac.Sum(nil))

	assert.True(t, VerifyWebhookSignature(secret, body, valid), "valid signature must pass")
	assert.False(t, VerifyWebhookSignature(secret, body, "deadbeef"), "wrong signature must fail")
	assert.False(t, VerifyWebhookSignature(secret, []byte(`{"tampered":true}`), valid), "tampered body must fail")
	assert.False(t, VerifyWebhookSignature("", body, valid), "empty secret must fail")
	assert.False(t, VerifyWebhookSignature(secret, body, ""), "empty header must fail")
}

func TestIsMemberEvent(t *testing.T) {
	assert.True(t, IsMemberEvent(EventMembersCreate))
	assert.True(t, IsMemberEvent(EventMembersUpdate))
	assert.True(t, IsMemberEvent(EventMembersDelete))
	assert.False(t, IsMemberEvent("posts:publish"))
	assert.False(t, IsMemberEvent(""))
}

func TestParseMemberEvent(t *testing.T) {
	body := []byte(`{
		"data": {
			"type": "member",
			"id": "member-1",
			"attributes": {
				"patron_status": "active_patron",
				"currently_entitled_amount_cents": 500,
				"last_charge_status": "Paid",
				"next_charge_date": "2026-07-01T00:00:00.000+00:00"
			},
			"relationships": {
				"user": { "data": { "type": "user", "id": "patreon-user-9" } },
				"currently_entitled_tiers": { "data": [ { "type": "tier", "id": "tier-42" } ] }
			}
		},
		"included": [
			{ "type": "user", "id": "patreon-user-9", "attributes": { "email": "fan@example.com" } }
		]
	}`)

	snap, err := ParseMemberEvent(body)
	require.NoError(t, err)
	assert.Equal(t, "patreon-user-9", snap.PatreonUserID)
	assert.Equal(t, "fan@example.com", snap.Email)
	assert.Equal(t, patronActive, snap.PatronStatus)
	assert.Equal(t, 500, snap.EntitledCents)
	assert.Equal(t, "Paid", snap.LastChargeStatus)
	assert.Equal(t, "tier-42", snap.TierID)
	require.NotNil(t, snap.NextChargeDate)

	// A delete payload with no user relationship is an error (can't map to a user).
	_, err = ParseMemberEvent([]byte(`{"data":{"type":"member","id":"m","relationships":{}}}`))
	assert.Error(t, err)
}

func TestParseIdentity(t *testing.T) {
	const ourCampaign = "campaign-allchat"
	body := []byte(`{
		"data": {
			"type": "user",
			"id": "patreon-user-9",
			"attributes": { "email": "fan@example.com", "full_name": "A Fan" },
			"relationships": {
				"memberships": { "data": [ { "type": "member", "id": "member-1" } ] }
			}
		},
		"included": [
			{
				"type": "member",
				"id": "member-1",
				"attributes": {
					"patron_status": "active_patron",
					"currently_entitled_amount_cents": 1000,
					"last_charge_status": "Paid",
					"next_charge_date": "2026-07-01T00:00:00.000+00:00"
				},
				"relationships": {
					"campaign": { "data": { "type": "campaign", "id": "campaign-allchat" } },
					"currently_entitled_tiers": { "data": [ { "type": "tier", "id": "tier-7" } ] }
				}
			}
		]
	}`)

	snap, err := parseIdentity(body, ourCampaign)
	require.NoError(t, err)
	assert.Equal(t, "patreon-user-9", snap.PatreonUserID)
	assert.Equal(t, "fan@example.com", snap.Email)
	assert.Equal(t, patronActive, snap.PatronStatus)
	assert.Equal(t, 1000, snap.EntitledCents)
	assert.Equal(t, "tier-7", snap.TierID)
	assert.Equal(t, StatusActive, SubscriptionStatusFor(*snap, 500))
}

func TestParseIdentity_MembershipToDifferentCampaign(t *testing.T) {
	// The patron backs some OTHER creator, not all-chat → no membership to our
	// campaign → StatusNone (no premium).
	body := []byte(`{
		"data": { "type": "user", "id": "patreon-user-9", "attributes": { "email": "fan@example.com" } },
		"included": [
			{
				"type": "member",
				"id": "member-1",
				"attributes": { "patron_status": "active_patron", "currently_entitled_amount_cents": 1000 },
				"relationships": { "campaign": { "data": { "type": "campaign", "id": "some-other-campaign" } } }
			}
		]
	}`)

	snap, err := parseIdentity(body, "campaign-allchat")
	require.NoError(t, err)
	assert.Equal(t, "patreon-user-9", snap.PatreonUserID)
	assert.Equal(t, "", snap.PatronStatus)
	assert.Equal(t, StatusNone, SubscriptionStatusFor(*snap, 500))
}
