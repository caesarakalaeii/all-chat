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
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"golang.org/x/oauth2"
)

// identityURL returns the current patron's identity plus their memberships, with
// the member fields we need to decide entitlement. We filter the returned
// memberships to all-chat's campaign in parseIdentity.
const identityURL = "https://www.patreon.com/api/oauth2/v2/identity"

// patreonEndpoint is the OAuth 2.0 endpoint. golang.org/x/oauth2 has no built-in
// Patreon endpoint, so we set the URLs literally.
var patreonEndpoint = oauth2.Endpoint{
	AuthURL:  "https://www.patreon.com/oauth2/authorize",
	TokenURL: "https://www.patreon.com/api/oauth2/token",
}

// identityScopes are the minimal scopes needed to read the logged-in user's own
// membership to our campaign: their identity and verified email.
var identityScopes = []string{"identity", "identity[email]"}

// OAuth wraps the Patreon OAuth 2.0 flow and the identity API call.
type OAuth struct {
	config *oauth2.Config
	client *http.Client
}

// NewOAuth builds a Patreon OAuth client.
func NewOAuth(clientID, clientSecret, redirectURL string) *OAuth {
	return &OAuth{
		config: &oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			RedirectURL:  redirectURL,
			Scopes:       identityScopes,
			Endpoint:     patreonEndpoint,
		},
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

// GetAuthURL builds the consent URL for the given opaque state.
func (o *OAuth) GetAuthURL(state string) string {
	return o.config.AuthCodeURL(state, oauth2.AccessTypeOffline)
}

// Scopes returns the scopes requested by this client.
func (o *OAuth) Scopes() []string {
	return o.config.Scopes
}

// ExchangeCode exchanges an authorization code for tokens.
func (o *OAuth) ExchangeCode(ctx context.Context, code string) (*oauth2.Token, error) {
	token, err := o.config.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("failed to exchange code: %w", err)
	}
	return token, nil
}

// RefreshToken refreshes an access token using a refresh token.
func (o *OAuth) RefreshToken(ctx context.Context, refreshToken string) (*oauth2.Token, error) {
	src := o.config.TokenSource(ctx, &oauth2.Token{RefreshToken: refreshToken})
	token, err := src.Token()
	if err != nil {
		return nil, fmt.Errorf("failed to refresh token: %w", err)
	}
	return token, nil
}

// GetIdentityWithMembership fetches the logged-in patron's identity and their
// membership to campaignID, normalized into a MembershipSnapshot. If the user has
// no membership to that campaign, the snapshot carries their identity with an empty
// PatronStatus (which maps to StatusNone).
func (o *OAuth) GetIdentityWithMembership(ctx context.Context, accessToken, campaignID string) (*MembershipSnapshot, error) {
	q := url.Values{}
	q.Set("include", "memberships.currently_entitled_tiers")
	q.Set("fields[member]", "patron_status,currently_entitled_amount_cents,last_charge_status,next_charge_date")
	q.Set("fields[user]", "email,full_name")

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, identityURL+"?"+q.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to build identity request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := o.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch identity: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read identity response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("patreon identity API returned status %d: %s", resp.StatusCode, string(body))
	}

	snap, err := parseIdentity(body, campaignID)
	if err != nil {
		return nil, fmt.Errorf("failed to parse identity: %w", err)
	}
	return snap, nil
}

// parseIdentity extracts the patron identity and their membership to campaignID
// from an identity API document. data is the "user"; the member(s) are in included.
func parseIdentity(body []byte, campaignID string) (*MembershipSnapshot, error) {
	var doc apiDocument
	if err := json.Unmarshal(body, &doc); err != nil {
		return nil, fmt.Errorf("unmarshal identity: %w", err)
	}

	snap := &MembershipSnapshot{
		PatreonUserID: doc.Data.ID,
		Email:         doc.Data.Attributes.Email,
	}

	for _, inc := range doc.Included {
		if inc.Type != "member" {
			continue
		}
		if inc.Relationships.Campaign == nil || inc.Relationships.Campaign.Data == nil {
			continue
		}
		if inc.Relationships.Campaign.Data.ID != campaignID {
			continue
		}
		memberToSnapshot(inc, snap)
		break
	}

	return snap, nil
}
