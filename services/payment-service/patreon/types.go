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

// Package patreon contains the Patreon-specific logic for payment-service:
// the OAuth client, the identity/membership API call, webhook signature
// verification + parsing, and the pure mapping from a Patreon membership to our
// subscription status. Everything here is decoupled from the database and HTTP
// layer so it can be unit-tested without a server or Postgres.
package patreon

import "time"

// Subscription status values stored in premium_subscriptions.status. StatusActive
// MUST equal the literal that shared/premium.Recompute checks ('active') — a row
// grants premium iff its status is StatusActive.
const (
	StatusNone     = "none"
	StatusActive   = "active"
	StatusDeclined = "declined"
	StatusFormer   = "former"
	StatusExpired  = "expired"
)

// Patreon patron_status values (https://docs.patreon.com/#member).
const (
	patronActive   = "active_patron"
	patronDeclined = "declined_patron"
	patronFormer   = "former_patron"
)

// MembershipSnapshot is the normalized view of a patron's membership to all-chat's
// campaign, derived from either the identity API response or a webhook payload.
type MembershipSnapshot struct {
	PatreonUserID    string
	Email            string
	PatronStatus     string // raw Patreon patron_status; "" when the user has no membership to our campaign
	EntitledCents    int    // currently_entitled_amount_cents
	LastChargeStatus string
	NextChargeDate   *time.Time
	TierID           string

	// UnmatchedMembers is how many member resources the identity response carried
	// that could not be attributed EITHER WAY — i.e. they declared no campaign at
	// all. A non-zero value alongside an empty PatronStatus means we discarded
	// membership data we could not rule in or out, so the resulting StatusNone is not
	// trustworthy.
	//
	// It stays 0 for a successful match, for a genuine non-patron (no members at
	// all), and for a patron who backs some OTHER creator — that last case is an
	// explicit, unambiguous negative and is both common and expected, so counting it
	// would keep the signal permanently lit on healthy traffic. Always 0 for webhook
	// payloads, which carry their member directly.
	UnmatchedMembers int
}

// SubscriptionStatusFor maps a membership snapshot to our subscription status.
// minCents is the qualifying threshold (PATREON_MIN_TIER_CENTS).
//
// Patreon keeps patron_status=active_patron and currently_entitled_amount_cents at
// the tier amount throughout its own payment-retry/grace window, so honoring those
// two fields automatically respects Patreon's grace period — we do not maintain a
// separate grace timer.
func SubscriptionStatusFor(snap MembershipSnapshot, minCents int) string {
	switch snap.PatronStatus {
	case patronActive:
		if snap.EntitledCents >= minCents {
			return StatusActive
		}
		return StatusExpired // active patron, but pledged below the qualifying tier
	case patronDeclined:
		return StatusDeclined
	case patronFormer:
		return StatusFormer
	default:
		return StatusNone
	}
}

// The types below mirror Patreon API v2's JSON:API envelope, so their shape is
// fixed by the upstream response rather than by anything all-chat needs.

type apiRelRef struct {
	Type string `json:"type"`
	ID   string `json:"id"`
}

type apiRelOne struct {
	Data *apiRelRef `json:"data"`
}

type apiRelMany struct {
	Data []apiRelRef `json:"data"`
}

// apiAttributes is a union of the user and member attribute fields we read. Absent
// fields simply unmarshal to their zero value, so one struct serves both resource
// types.
type apiAttributes struct {
	// user
	Email    string `json:"email"`
	FullName string `json:"full_name"`
	// member
	PatronStatus           string `json:"patron_status"`
	CurrentlyEntitledCents int    `json:"currently_entitled_amount_cents"`
	LastChargeStatus       string `json:"last_charge_status"`
	NextChargeDate         string `json:"next_charge_date"`
}

type apiRelationships struct {
	Campaign               *apiRelOne  `json:"campaign"`
	CurrentlyEntitledTiers *apiRelMany `json:"currently_entitled_tiers"`
	User                   *apiRelOne  `json:"user"`
	Memberships            *apiRelMany `json:"memberships"`
}

type apiResource struct {
	Type          string           `json:"type"`
	ID            string           `json:"id"`
	Attributes    apiAttributes    `json:"attributes"`
	Relationships apiRelationships `json:"relationships"`
}

type apiDocument struct {
	Data     apiResource   `json:"data"`
	Included []apiResource `json:"included"`
}

// memberToSnapshot fills the membership-specific fields of a snapshot from a
// "member" resource.
func memberToSnapshot(member apiResource, snap *MembershipSnapshot) {
	snap.PatronStatus = member.Attributes.PatronStatus
	snap.EntitledCents = member.Attributes.CurrentlyEntitledCents
	snap.LastChargeStatus = member.Attributes.LastChargeStatus
	snap.NextChargeDate = parsePatreonTime(member.Attributes.NextChargeDate)
	if r := member.Relationships.CurrentlyEntitledTiers; r != nil && len(r.Data) > 0 {
		snap.TierID = r.Data[0].ID
	}
}

// parsePatreonTime parses a Patreon ISO-8601 timestamp, returning nil on empty or
// unparseable input.
func parsePatreonTime(s string) *time.Time {
	if s == "" {
		return nil
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return &t
	}
	return nil
}
