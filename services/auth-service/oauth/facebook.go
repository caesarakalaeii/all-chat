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

package oauth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/oauth2"
)

// Graph version pinned per ADR-0060. Graph versions live two years past the
// next release, so a pinned path is stable; an unversioned call would track
// whatever the app dashboard has configured.
const FacebookAPIVersion = "v26.0"

// Facebook OAuth scope strings (see
// https://developers.facebook.com/docs/permissions):
//
//   - pages_show_list          — list the streamer's Pages (dependency of both
//     other scopes; needed to resolve WHICH Page to connect)
//   - pages_read_engagement    — read Page content incl. live videos and
//     comments on them (the ingest path)
//   - pages_manage_engagement  — create/edit/delete Page comments (the
//     moderation write path; requested from the start per ADR-0060 so the
//     streamer consents once)
//
// Meta's consent screen is granular: a user can approve some scopes and
// decline others, so the callback must verify both engagement scopes were
// actually granted (mirrors the YouTube readonly check).
const (
	FacebookPagesShowListScope         = "pages_show_list"
	FacebookPagesReadEngagementScope   = "pages_read_engagement"
	FacebookPagesManageEngagementScope = "pages_manage_engagement"
)

// FacebookScopes is the exact set requested at login AND add-source: Facebook
// has no incremental-consent concept like Google's, and re-consent would cost
// a second App Review round-trip, so both engagement scopes ride along from
// the start (ADR-0060).
func FacebookScopes() []string {
	return []string{
		FacebookPagesShowListScope,
		FacebookPagesReadEngagementScope,
		FacebookPagesManageEngagementScope,
	}
}

// FacebookOAuth handles the Facebook Login OAuth flow.
//
// The token chain differs from Twitch/YouTube: the authorization code is
// exchanged for a short-lived USER token, which we exchange for a long-lived
// user token, which is then exchanged for a PAGE token via /me/accounts (see
// ExchangePageToken). The stored credential is the Page token: per Meta's
// docs, a Page access token obtained from a long-lived user token "does not
// have an expiration date", so the token-refresh-service has nothing to
// refresh for Facebook (ADR-0060).
type FacebookOAuth struct {
	appID       string
	appSecret   string
	redirectURL string
	graphBase   string // overridable in tests
	dialogBase  string // overridable in tests
	httpClient  *http.Client
}

// NewFacebookOAuth creates a new Facebook OAuth handler.
func NewFacebookOAuth(appID, appSecret, redirectURL string) *FacebookOAuth {
	return &FacebookOAuth{
		appID:       appID,
		appSecret:   appSecret,
		redirectURL: redirectURL,
		graphBase:   "https://graph.facebook.com/" + FacebookAPIVersion,
		dialogBase:  "https://www.facebook.com/" + FacebookAPIVersion + "/dialog/oauth",
		httpClient:  &http.Client{Timeout: 15 * time.Second},
	}
}

// WithRedirectURL returns a copy that redirects to redirectURL instead. Used
// to point one deployment's OAuth flow at a second frontend origin whose
// callback URI is registered separately with Meta.
func (f *FacebookOAuth) WithRedirectURL(redirectURL string) *FacebookOAuth {
	c := *f
	c.redirectURL = redirectURL
	return &c
}

// GetAuthURL generates the OAuth authorization URL. Facebook's dialog is
// state-based only (no PKCE for server-side web flow).
func (f *FacebookOAuth) GetAuthURL(state string) string {
	params := url.Values{}
	params.Set("client_id", f.appID)
	params.Set("redirect_uri", f.redirectURL)
	params.Set("state", state)
	params.Set("response_type", "code")
	params.Set("scope", strings.Join(FacebookScopes(), ","))
	return f.dialogBase + "?" + params.Encode()
}

// facebookTokenResponse is the /oauth/access_token payload.
type facebookTokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int64  `json:"expires_in"` // absent for non-expiring tokens
}

// facebookGraphError is the error envelope Graph returns on >=400.
type facebookGraphError struct {
	Error struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func decodeFacebookError(body []byte, status int) error {
	var env facebookGraphError
	if err := json.Unmarshal(body, &env); err == nil && env.Error.Message != "" {
		return fmt.Errorf("facebook: graph error %d: %s", env.Error.Code, env.Error.Message)
	}
	return fmt.Errorf("facebook: api returned status %d", status)
}

// get issues an authorized GET and decodes JSON into out.
func (f *FacebookOAuth) get(ctx context.Context, path string, params url.Values, out interface{}) error {
	endpoint := f.graphBase + path
	if len(params) > 0 {
		endpoint += "?" + params.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return fmt.Errorf("facebook: build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := f.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("facebook: request failed: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return fmt.Errorf("facebook: read body: %w", err)
	}
	if resp.StatusCode >= 400 {
		return decodeFacebookError(body, resp.StatusCode)
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("facebook: decode response: %w", err)
	}
	return nil
}

// ExchangeCode exchanges the authorization code for a short-lived user token,
// then immediately exchanges it for a long-lived user token (Meta deprecates
// the short-lived variant server-side; the long-lived exchange needs the app
// secret, so it runs here and nowhere else).
// See https://developers.facebook.com/docs/facebook-login/guides/access-tokens/get-long-lived.
func (f *FacebookOAuth) ExchangeCode(ctx context.Context, code string) (*oauth2.Token, error) {
	params := url.Values{}
	params.Set("client_id", f.appID)
	params.Set("client_secret", f.appSecret)
	params.Set("redirect_uri", f.redirectURL)
	params.Set("code", code)

	var short facebookTokenResponse
	if err := f.get(ctx, "/oauth/access_token", params, &short); err != nil {
		return nil, fmt.Errorf("exchange code: %w", err)
	}

	longLived, err := f.exchangeLongLived(ctx, short.AccessToken)
	if err != nil {
		return nil, err
	}
	return longLived, nil
}

// exchangeLongLived trades a short-lived user token for a ~60-day one via the
// fb_exchange_token grant.
func (f *FacebookOAuth) exchangeLongLived(ctx context.Context, shortToken string) (*oauth2.Token, error) {
	params := url.Values{}
	params.Set("grant_type", "fb_exchange_token")
	params.Set("client_id", f.appID)
	params.Set("client_secret", f.appSecret)
	params.Set("fb_exchange_token", shortToken)

	var resp facebookTokenResponse
	if err := f.get(ctx, "/oauth/access_token", params, &resp); err != nil {
		return nil, fmt.Errorf("exchange long-lived token: %w", err)
	}

	token := &oauth2.Token{
		AccessToken: resp.AccessToken,
		TokenType:   "Bearer",
	}
	if resp.ExpiresIn > 0 {
		token.Expiry = time.Now().Add(time.Duration(resp.ExpiresIn) * time.Second)
	}
	return token, nil
}

// FacebookPage is one entry of GET /me/accounts: the streamer's Page with its
// own non-expiring page token and the task list the user holds on it.
type FacebookPage struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	AccessToken string   `json:"access_token"`
	Tasks       []string `json:"tasks"`
}

// HasModerateTask reports whether the user can moderate on this Page
// (MODERATE) or administer it (MANAGE) — required for the write path.
func (p *FacebookPage) HasModerateTask() bool {
	for _, t := range p.Tasks {
		if t == "MODERATE" || t == "MANAGE" {
			return true
		}
	}
	return false
}

// GetPrimaryPage returns the streamer's Pages, preferring one on which they
// hold a moderation-capable task. The page token inside is the long-lived
// credential the listener and moderation-service use.
func (f *FacebookOAuth) GetPrimaryPage(ctx context.Context, longLivedUserToken string) (*FacebookPage, error) {
	var out struct {
		Data []FacebookPage `json:"data"`
	}
	params := url.Values{}
	params.Set("access_token", longLivedUserToken)
	params.Set("fields", "id,name,access_token,tasks")
	params.Set("limit", "100")
	if err := f.get(ctx, "/me/accounts", params, &out); err != nil {
		return nil, fmt.Errorf("fetch pages: %w", err)
	}
	if len(out.Data) == 0 {
		return nil, fmt.Errorf("facebook: no pages available for this account (a facebook source requires a Page you manage)")
	}
	for _, p := range out.Data {
		if p.HasModerateTask() {
			return &p, nil
		}
	}
	return nil, fmt.Errorf("facebook: you hold no moderation-capable task (MODERATE/MANAGE) on any of your Pages")
}

// FacebookUserInfo is the /me identity of the streamer (app-scoped id).
type FacebookUserInfo struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Picture string `json:"-"`
}

// GetUserInfoFacebook fetches the streamer's identity.
func (f *FacebookOAuth) GetUserInfoFacebook(ctx context.Context, accessToken string) (*FacebookUserInfo, error) {
	var out FacebookUserInfo
	params := url.Values{}
	params.Set("access_token", accessToken)
	params.Set("fields", "id,name")
	if err := f.get(ctx, "/me", params, &out); err != nil {
		return nil, fmt.Errorf("fetch user info: %w", err)
	}
	return &out, nil
}

// --- OAuthProvider interface implementation --------------------------------

// facebookUserInfoWrapper adapts FacebookUserInfo to PlatformUserInfo.
type facebookUserInfoWrapper struct{ u *FacebookUserInfo }

func (w *facebookUserInfoWrapper) GetID() string              { return w.u.ID }
func (w *facebookUserInfoWrapper) GetUsername() string        { return w.u.ID } // no login name on Facebook
func (w *facebookUserInfoWrapper) GetDisplayName() string     { return w.u.Name }
func (w *facebookUserInfoWrapper) GetProfileImageURL() string { return "" }
func (w *facebookUserInfoWrapper) GetPlatform() Platform      { return PlatformFacebook }

// GetUserInfo implements the generic provider interface.
func (f *FacebookOAuth) GetUserInfo(ctx context.Context, accessToken string) (PlatformUserInfo, error) {
	u, err := f.GetUserInfoFacebook(ctx, accessToken)
	if err != nil {
		return nil, err
	}
	return &facebookUserInfoWrapper{u: u}, nil
}

// GetPlatform returns the platform identifier.
func (f *FacebookOAuth) GetPlatform() Platform { return PlatformFacebook }

// RefreshToken is a no-op: the stored credential is a Page token obtained from
// a long-lived user token, and per Meta's docs such page tokens do not expire
// (they are only invalidated by password change, permission revocation or
// deauthorization — all of which surface as API errors at use time, prompting
// the streamer to reconnect). A user-token refresh path would add an expiry we
// cannot renew without the original short-lived token, so Facebook is wired as
// refresh-free (ADR-0060). The error return keeps the OAuthProvider contract.
func (f *FacebookOAuth) RefreshToken(ctx context.Context, refreshToken string) (*oauth2.Token, error) {
	return nil, fmt.Errorf("facebook: page tokens do not expire; re-consent via the normal login flow instead")
}

// ExtractGrantedFacebookScopes reads which of the requested permissions the
// streamer actually granted. Facebook returns the granted set in the token
// exchange response only as `scope` on some flows; the reliable mechanism is
// the debug_token endpoint, which needs an app token. We derive an app token
// from app id|secret (documented app-access-token format) and check.
// A declined pages_manage_engagement degrades gracefully: the listener works
// (read scope) and moderation reports missing scope.
func (f *FacebookOAuth) ExtractGrantedFacebookScopes(ctx context.Context, userToken string) []string {
	appToken := f.appID + "|" + f.appSecret
	var out struct {
		Data struct {
			Scopes []string `json:"scopes"`
		} `json:"data"`
	}
	params := url.Values{}
	params.Set("input_token", userToken)
	params.Set("access_token", appToken)
	if err := f.get(ctx, "/debug_token", params, &out); err != nil {
		return nil
	}
	return out.Data.Scopes
}
