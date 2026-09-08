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

package handlers

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/caesar/all-chat/services/auth-service/oauth"
	"github.com/gin-gonic/gin"
)

// canonicalFrontendURL returns FRONTEND_URL, the default origin every OAuth
// flow returns to when the request carries no allowlisted origin of its own.
func canonicalFrontendURL() string {
	return strings.TrimSuffix(getEnvOrDefault("FRONTEND_URL", "http://localhost:3000"), "/")
}

// allowedFrontendOrigins is the set of origins an OAuth flow may redirect back
// to: the canonical FRONTEND_URL plus every entry in FRONTEND_URLS (comma-
// separated). Normalised to scheme://host so a trailing slash in either env
// var cannot split the set.
func allowedFrontendOrigins() map[string]struct{} {
	allowed := map[string]struct{}{canonicalFrontendURL(): {}}
	for _, raw := range strings.Split(getEnvOrDefault("FRONTEND_URLS", ""), ",") {
		origin := normalizeOrigin(raw)
		if origin != "" {
			allowed[origin] = struct{}{}
		}
	}
	return allowed
}

// normalizeOrigin reduces an origin to scheme://host, dropping path, trailing
// slash and casing differences in the host. Returns "" for anything that does
// not parse to an absolute http(s) URL.
func normalizeOrigin(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" {
		return ""
	}
	return u.Scheme + "://" + strings.ToLower(u.Host)
}

// requestFrontendOrigin resolves which frontend origin started this OAuth
// flow. The browser's Host never reaches the auth-service: the api-gateway
// builds a fresh backend request and stamps the original host on
// X-Forwarded-Host (see api-gateway proxy.go). A host that is not on the
// allowlist falls back to the canonical FRONTEND_URL, so the value returned
// here is always safe to redirect to.
func requestFrontendOrigin(c *gin.Context) string {
	host := c.Request.Host
	if forwarded := c.GetHeader("X-Forwarded-Host"); forwarded != "" {
		// First entry wins; the gateway sets exactly one, anything beyond it
		// came from the client and is untrusted.
		host, _, _ = strings.Cut(forwarded, ",")
		host = strings.TrimSpace(host)
	}
	if origin := normalizeOrigin("https://" + host); origin != "" {
		if _, ok := allowedFrontendOrigins()[origin]; ok {
			return origin
		}
	}
	return canonicalFrontendURL()
}

// stateOrigin returns the origin recorded in a verified OAuth state, falling
// back to the canonical frontend for states written before Origin existed.
// The state is already trusted at this point (the callback byte-compares it
// against the Redis copy); the allowlist re-check guards against an allowlist
// that shrank between login and callback.
func stateOrigin(state *oauth.OAuthState) string {
	if state == nil {
		return canonicalFrontendURL()
	}
	if origin := allowlistedOrigin(state.Origin); origin != "" {
		return origin
	}
	return canonicalFrontendURL()
}

// allowlistedOrigin returns raw as a normalized origin when it is currently
// allowlisted, "" otherwise. The shared re-check behind stateOrigin and the
// Discord flow-origin lookup: a stored origin is server-written, but the
// allowlist may have shrunk between authorize and callback.
func allowlistedOrigin(raw string) string {
	if origin := normalizeOrigin(raw); origin != "" {
		if _, ok := allowedFrontendOrigins()[origin]; ok {
			return origin
		}
	}
	return ""
}

// originCallbackURL is the auth callback URI registered at the platform for
// origin: every allowlisted frontend origin has its own callback registration
// there, at the same path on its own host.
func originCallbackURL(origin string, platform oauth.Platform) string {
	return fmt.Sprintf("%s/api/v1/auth/%s/callback", origin, platform)
}

// providerForOrigin returns a provider whose redirect_uri points at origin's
// auth callback, so the authorize URL and the code exchange agree with the
// callback URI registered at the platform for that origin. The canonical
// origin returns the provider unchanged — one instance, no copies, for the
// overwhelmingly common case.
func providerForOrigin(provider oauth.OAuthProvider, platform oauth.Platform, origin string) oauth.OAuthProvider {
	if origin == canonicalFrontendURL() {
		return provider
	}
	redirectURL := originCallbackURL(origin, platform)
	switch p := provider.(type) {
	case *oauth.TwitchOAuth:
		return p.WithRedirectURL(redirectURL)
	case *oauth.YouTubeOAuth:
		return p.WithRedirectURL(redirectURL)
	case *oauth.KickOAuth:
		return p.WithRedirectURL(redirectURL)
	default:
		return provider
	}
}
