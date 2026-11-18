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
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// Provider token revocation endpoints, used when a streamer disconnects a platform
// or a viewer unlinks one. Meta (Facebook, Instagram) has no endpoint that revokes
// the stored Page token, so those platforms have no RevokeToken.
const (
	twitchRevokeURL = "https://id.twitch.tv/oauth2/revoke"   // form: client_id, token
	googleRevokeURL = "https://oauth2.googleapis.com/revoke" // form: token; revokes the whole grant
	kickRevokeURL   = "https://id.kick.com/oauth/revoke"     // query: token
)

// postRevoke sends one revocation request and reports a non-2xx answer as an error.
// The RevokeToken methods treat an empty token as a no-op before calling it.
func postRevoke(ctx context.Context, client *http.Client, endpoint string, form url.Values) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("revoke at %s returned %d", endpoint, resp.StatusCode)
	}
	return nil
}

// RevokeToken revokes an access or refresh token at Twitch.
func (t *TwitchOAuth) RevokeToken(ctx context.Context, token string) error {
	if token == "" {
		return nil
	}
	return postRevoke(ctx, t.client, twitchRevokeURL, url.Values{"client_id": {t.config.ClientID}, "token": {token}})
}

// RevokeToken revokes a token at Google, which also revokes the grant behind it.
func (y *YouTubeOAuth) RevokeToken(ctx context.Context, token string) error {
	if token == "" {
		return nil
	}
	return postRevoke(ctx, y.client, googleRevokeURL, url.Values{"token": {token}})
}

// RevokeToken revokes a viewer's token at Google.
func (y *ViewerYouTubeOAuth) RevokeToken(ctx context.Context, token string) error {
	if token == "" {
		return nil
	}
	return postRevoke(ctx, y.client, googleRevokeURL, url.Values{"token": {token}})
}

// RevokeToken revokes an access or refresh token at Kick.
func (k *KickOAuth) RevokeToken(ctx context.Context, token string) error {
	return revokeKick(ctx, k.client, token)
}

// RevokeToken revokes a viewer's access or refresh token at Kick.
func (k *ViewerKickOAuth) RevokeToken(ctx context.Context, token string) error {
	return revokeKick(ctx, k.client, token)
}

func revokeKick(ctx context.Context, client *http.Client, token string) error {
	if token == "" {
		return nil
	}
	return postRevoke(ctx, client, kickRevokeURL+"?"+url.Values{"token": {token}}.Encode(), url.Values{})
}
