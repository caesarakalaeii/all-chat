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
	//nolint:gosec // Patreon mandates HMAC-MD5 for webhook signatures; the algorithm is fixed by the provider, not a security choice of ours.
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

// Webhook event types (the X-Patreon-Event header value).
const (
	EventMembersCreate = "members:create"
	EventMembersUpdate = "members:update"
	EventMembersDelete = "members:delete"
)

// IsMemberEvent reports whether an X-Patreon-Event value is one of the membership
// lifecycle events we act on.
func IsMemberEvent(event string) bool {
	switch event {
	case EventMembersCreate, EventMembersUpdate, EventMembersDelete:
		return true
	default:
		return false
	}
}

// VerifyWebhookSignature reports whether sigHeader (the X-Patreon-Signature value)
// is a valid signature for body under secret.
//
// Patreon signs webhooks with HMAC-MD5 of the raw request body, hex-encoded — NOT
// the HMAC-SHA256-with-timestamp scheme used by Twitch EventSub. The body MUST be
// the exact bytes received (read before any JSON decode). The comparison is
// constant-time.
func VerifyWebhookSignature(secret string, body []byte, sigHeader string) bool {
	if secret == "" || sigHeader == "" {
		return false
	}
	mac := hmac.New(md5.New, []byte(secret))
	mac.Write(body)
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(sigHeader))
}

// ParseMemberEvent normalizes a members:* webhook payload into a MembershipSnapshot.
// data is the "member"; the patron's Patreon user id comes from the member's user
// relationship, and the email (if present) from the included user resource.
func ParseMemberEvent(body []byte) (*MembershipSnapshot, error) {
	var doc apiDocument
	if err := json.Unmarshal(body, &doc); err != nil {
		return nil, fmt.Errorf("unmarshal webhook: %w", err)
	}
	if doc.Data.Type != "member" {
		return nil, fmt.Errorf("unexpected webhook data type %q (want member)", doc.Data.Type)
	}

	snap := &MembershipSnapshot{}
	memberToSnapshot(doc.Data, snap)

	if r := doc.Data.Relationships.User; r != nil && r.Data != nil {
		snap.PatreonUserID = r.Data.ID
	}
	if snap.PatreonUserID == "" {
		return nil, fmt.Errorf("webhook member has no associated user id")
	}

	// Email is best-effort: pull it from the included user resource if present.
	for _, inc := range doc.Included {
		if inc.Type == "user" && inc.ID == snap.PatreonUserID {
			snap.Email = inc.Attributes.Email
			break
		}
	}

	return snap, nil
}
