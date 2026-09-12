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

// Instagram API version, pinned like FacebookAPIVersion. The Instagram API
// with Facebook Login is served from graph.facebook.com (see
// https://developers.facebook.com/documentation/instagram-platform/reference),
// so the same Graph version pin applies.
const InstagramAPIVersion = "v26.0"

// Instagram OAuth scope strings (see
// https://developers.facebook.com/docs/permissions/reference#i):
//
//   - instagram_basic           — read the streamer's IG professional account
//     (id, username) through the Page it is linked to
//
//   - instagram_manage_comments — read comments on the IG user's live media
//     (the ingest path); read-only for All-Chat, no moderation endpoints
//     are called
//
//   - pages_show_list           — dependency scope demanded by the account
//     resolution call, requested on top of the two contract scopes. The
//     streamer's IG user id is only obtainable by enumerating their Pages
//     (GET /me/accounts with the instagram_business_account field). Meta's
//     own get-started guide for this API requests exactly
//     instagram_basic + pages_show_list for that step
//     (https://developers.facebook.com/documentation/instagram-platform/instagram-api-with-facebook-login/get-started,
//     steps 2 and 4), and without it /me/accounts returns no Pages for
//     third-party apps in production (Advanced Access), so the IG account
//     could not be resolved at all. The ingest endpoint itself
//     (/{ig-user-id}/live_comments) never needs pages_show_list — it is
//     consent surface only.
const (
	InstagramBasicScope          = "instagram_basic"
	InstagramManageCommentsScope = "instagram_manage_comments"
	InstagramPagesShowListScope  = "pages_show_list"
)

// InstagramScopes is the exact set requested at login AND add-source. The
// Instagram dialog is the same Facebook dialog with these permissions (see
// NewInstagramOAuth), so re-consent costs nothing extra.
func InstagramScopes() []string {
	return []string{
		InstagramBasicScope,
		InstagramManageCommentsScope,
		InstagramPagesShowListScope,
	}
}

// InstagramOAuth handles the Instagram OAuth flow via Facebook Login.
//
// The token chain mirrors Facebook's exactly (ADR-0060): the authorization
// code is exchanged for a short-lived USER token, which is exchanged for a
// long-lived user token (~60 days), which is then exchanged for a PAGE token
// via /me/accounts. The stored credential is the Page token for the Page
// linked to the streamer's Instagram professional account: per Meta's Comment
// Moderation guide (https://developers.facebook.com/documentation/instagram-platform/comment-moderation),
// the "Instagram API with Facebook Login" column requires a "Facebook Page
// access token" for comment endpoints on graph.facebook.com, and a Page token
// obtained from a long-lived user token does not expire — so the
// token-refresh-service has nothing to refresh for Instagram either.
type InstagramOAuth struct {
	appID       string
	appSecret   string
	redirectURL string
	graphBase   string
	dialogBase  string
	httpClient  *http.Client
}

// NewInstagramOAuth creates a new Instagram OAuth handler.
func NewInstagramOAuth(appID, appSecret, redirectURL string) *InstagramOAuth {
	return &InstagramOAuth{
		appID:       appID,
		appSecret:   appSecret,
		redirectURL: redirectURL,
		graphBase:   "https://graph.facebook.com/" + InstagramAPIVersion,
		dialogBase:  "https://www.facebook.com/" + InstagramAPIVersion + "/dialog/oauth",
		httpClient:  &http.Client{Timeout: 15 * time.Second},
	}
}

// WithRedirectURL returns a copy that redirects to redirectURL instead.
func (o *InstagramOAuth) WithRedirectURL(redirectURL string) *InstagramOAuth {
	c := *o
	c.redirectURL = redirectURL
	return &c
}

// WithGraphBase overrides the Graph endpoint and version (INSTAGRAM_GRAPH_URL /
// INSTAGRAM_GRAPH_VERSION seams). Both default to the documented values.
func (o *InstagramOAuth) WithGraphBase(graphURL, version string) *InstagramOAuth {
	if graphURL == "" {
		graphURL = "https://graph.facebook.com"
	}
	if version == "" {
		version = InstagramAPIVersion
	}
	c := *o
	c.graphBase = strings.TrimSuffix(graphURL, "/") + "/" + version
	c.dialogBase = strings.TrimSuffix(graphURL, "/")
	return &c
}

// GetAuthURL generates the OAuth authorization URL. It is the same
// facebook.com dialog as Facebook's, with the Instagram permissions requested
// via `scope` (Meta's Business Login for Instagram guide builds exactly this
// URL for an app "that relies on the Instagram Messaging API", swapping only
// the permission list:
// https://developers.facebook.com/documentation/instagram-platform/instagram-api-with-facebook-login/business-login-for-instagram).
func (o *InstagramOAuth) GetAuthURL(state string) string {
	params := url.Values{}
	params.Set("client_id", o.appID)
	params.Set("redirect_uri", o.redirectURL)
	params.Set("state", state)
	params.Set("response_type", "code")
	params.Set("scope", strings.Join(InstagramScopes(), ","))
	return o.dialogBase + "?" + params.Encode()
}

// instagramTokenResponse is the /oauth/access_token payload.
type instagramTokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int64  `json:"expires_in"`
}

// instagramGraphError is the error envelope Graph returns on >=400 (same
// shape as Facebook's).
type instagramGraphError struct {
	Error struct {
		Code      int    `json:"code"`
		Message   string `json:"message"`
		FBTraceID string `json:"fbtrace_id"`
	} `json:"error"`
}

func decodeInstagramError(body []byte, status int) error {
	var env instagramGraphError
	if err := json.Unmarshal(body, &env); err == nil && env.Error.Message != "" {
		return fmt.Errorf("instagram: graph error %d: %s", env.Error.Code, env.Error.Message)
	}
	return fmt.Errorf("instagram: api returned status %d", status)
}

// get issues an authorized GET against the Graph endpoint and decodes JSON into out.
func (o *InstagramOAuth) get(ctx context.Context, path string, params url.Values, out interface{}) error {
	endpoint := o.graphBase + path
	if len(params) > 0 {
		endpoint += "?" + params.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return fmt.Errorf("instagram: build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := o.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("instagram: request failed: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return fmt.Errorf("instagram: read body: %w", err)
	}
	if resp.StatusCode >= 400 {
		return decodeInstagramError(body, resp.StatusCode)
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("instagram: decode response: %w", err)
	}
	return nil
}

// ExchangeCode exchanges the authorization code for a short-lived user token,
// then immediately for a long-lived ~60-day user token (same fb_exchange_token
// grant as Facebook; Meta deprecates the short-lived variant server-side).
// See https://developers.facebook.com/docs/facebook-login/guides/access-tokens/get-long-lived.
func (o *InstagramOAuth) ExchangeCode(ctx context.Context, code string) (*oauth2.Token, error) {
	params := url.Values{}
	params.Set("client_id", o.appID)
	params.Set("client_secret", o.appSecret)
	params.Set("redirect_uri", o.redirectURL)
	params.Set("code", code)

	var short instagramTokenResponse
	if err := o.get(ctx, "/oauth/access_token", params, &short); err != nil {
		return nil, fmt.Errorf("exchange code: %w", err)
	}

	return o.exchangeLongLived(ctx, short.AccessToken)
}

// exchangeLongLived trades a short-lived user token for a ~60-day one via the
// fb_exchange_token grant.
func (o *InstagramOAuth) exchangeLongLived(ctx context.Context, shortToken string) (*oauth2.Token, error) {
	params := url.Values{}
	params.Set("grant_type", "fb_exchange_token")
	params.Set("client_id", o.appID)
	params.Set("client_secret", o.appSecret)
	params.Set("fb_exchange_token", shortToken)

	var resp instagramTokenResponse
	if err := o.get(ctx, "/oauth/access_token", params, &resp); err != nil {
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

// InstagramAccount is the streamer's Instagram professional account resolved
// through the Page it is linked to. IGUserID is the app-scoped IG user id the
// ingest endpoints key on; the page token alongside it is the stored
// credential.
type InstagramAccount struct {
	IGUserID   string
	IGUsername string
	PageName   string
	// AccessToken is the Page access token. Non-expiring when derived from a
	// long-lived user token (ADR-0060 pattern, see type comment).
	AccessToken string
}

// instagramPage is one entry of GET /me/accounts with its linked IG account.
type instagramPage struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	AccessToken string `json:"access_token"`
	Instagram   struct {
		ID       string `json:"id"`
		Username string `json:"username"`
	} `json:"instagram_business_account"`
}

// GetStreamersInstagramAccount resolves the streamer's Instagram professional
// account: it enumerates the Pages the user can perform tasks on and picks one
// with a connected IG professional account (the IG business account field on
// the Page is the documented Page→IG mapping,
// https://developers.facebook.com/documentation/instagram-platform/instagram-api-with-facebook-login/get-started,
// step 5). A Page the user can at least CREATE_CONTENT on is preferred so the
// connected account is theirs, not one they merely browse.
func (o *InstagramOAuth) GetStreamersInstagramAccount(ctx context.Context, longLivedUserToken string) (*InstagramAccount, error) {
	var out struct {
		Data []instagramPage `json:"data"`
	}
	params := url.Values{}
	params.Set("access_token", longLivedUserToken)
	params.Set("fields", "id,name,access_token,instagram_business_account{id,username}")
	params.Set("limit", "100")
	if err := o.get(ctx, "/me/accounts", params, &out); err != nil {
		return nil, fmt.Errorf("fetch pages: %w", err)
	}

	var fallback *instagramPage
	for i := range out.Data {
		p := &out.Data[i]
		if p.Instagram.ID == "" {
			continue
		}
		// A connected account on a Page the user only reads is a weak claim
		// (some Media-type Pages expose instagram_business_account); keep it
		// as fallback but prefer a Page the user manages.
		if !p.canManage() {
			if fallback == nil {
				fallback = p
			}
			continue
		}
		return &InstagramAccount{
			IGUserID:    p.Instagram.ID,
			IGUsername:  p.Instagram.Username,
			PageName:    p.Name,
			AccessToken: p.AccessToken,
		}, nil
	}
	if fallback == nil {
		return nil, fmt.Errorf("instagram: no Page with a connected Instagram professional account (an instagram source requires an IG professional/creator account linked to a Facebook Page you manage)")
	}
	return &InstagramAccount{
		IGUserID:    fallback.Instagram.ID,
		IGUsername:  fallback.Instagram.Username,
		PageName:    fallback.Name,
		AccessToken: fallback.AccessToken,
	}, nil
}

// canManage reports whether the Page entry came with its own access token,
// which /me/accounts only returns for Pages the user holds a task on — the
// in-response signal that this is a Page they control (tasks were not
// requested because no IG moderation path exists to key off them).
func (p *instagramPage) canManage() bool {
	return p.AccessToken != ""
}

// InstagramUserInfo is the /me identity of the streamer (app-scoped Facebook
// user id — NOT the IG user id; the IG id is resolved separately and stored
// with the Page credential).
type InstagramUserInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// GetUserInfoInstagram fetches the streamer's identity.
func (o *InstagramOAuth) GetUserInfoInstagram(ctx context.Context, accessToken string) (*InstagramUserInfo, error) {
	var out InstagramUserInfo
	params := url.Values{}
	params.Set("access_token", accessToken)
	params.Set("fields", "id,name")
	if err := o.get(ctx, "/me", params, &out); err != nil {
		return nil, fmt.Errorf("fetch user info: %w", err)
	}
	return &out, nil
}

// instagramUserInfoWrapper adapts InstagramUserInfo to PlatformUserInfo.
type instagramUserInfoWrapper struct{ u *InstagramUserInfo }

func (w *instagramUserInfoWrapper) GetID() string              { return w.u.ID }
func (w *instagramUserInfoWrapper) GetUsername() string        { return w.u.ID } // no login name on Facebook login
func (w *instagramUserInfoWrapper) GetDisplayName() string     { return w.u.Name }
func (w *instagramUserInfoWrapper) GetProfileImageURL() string { return "" }
func (w *instagramUserInfoWrapper) GetPlatform() Platform      { return PlatformInstagram }

// GetUserInfo implements the generic provider interface.
func (o *InstagramOAuth) GetUserInfo(ctx context.Context, accessToken string) (PlatformUserInfo, error) {
	u, err := o.GetUserInfoInstagram(ctx, accessToken)
	if err != nil {
		return nil, err
	}
	return &instagramUserInfoWrapper{u: u}, nil
}

// GetPlatform returns the platform identifier.
func (o *InstagramOAuth) GetPlatform() Platform { return PlatformInstagram }

// RefreshToken is a no-op, mirroring Facebook: the stored credential is a Page
// token obtained from a long-lived user token, and per Meta's docs such page
// tokens do not expire (they are only invalidated by password change,
// permission revocation or deauthorization — all of which surface as API
// errors at use time, prompting the streamer to reconnect). The long-lived
// USER token behind it lasts ~60 days and prevails only for the /me identity
// fetch at re-consent; nothing server-side refreshes it. The error return
// keeps the OAuthProvider contract.
func (o *InstagramOAuth) RefreshToken(ctx context.Context, refreshToken string) (*oauth2.Token, error) {
	return nil, fmt.Errorf("instagram: page tokens do not expire; re-consent via the normal login flow instead")
}
