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

// The loopback redirect validator for device linking (ADR-0049, RFC 8252 §7.3).
//
// This is the one place a device-token authorization code can be sent, so it is a
// dedicated, deliberately narrow rule and NOT a call into the general user-facing
// redirect guard. ADR-0049 is explicit about why:
//
//	Do not route this through isAllowedExternalRedirect. That guard exists for
//	user-facing navigation, and it already had to be fixed once for backslash
//	normalisation (audit M1). A redirect that hands over a credential deserves a narrow
//	rule that cannot be widened by an unrelated change to the general one.
//
// The closest existing thing in the tree is the ALLOWED_OPENER_ORIGINS allowlist in
// frontend/src/app/chat/auth-success/page.tsx, which guards a postMessage target. That
// is a different rule for a different job and is deliberately not reused either.
//
// THE RULE, in full:
//
//	scheme   exactly "http"           (https on a loopback interface has no CA that can
//	                                   issue for it; RFC 8252 §7.3 expects plain http)
//	host     exactly 127.0.0.1 or ::1 (literal addresses only — see below)
//	port     any, or absent           (the plugin cannot reserve a port in advance and a
//	                                   fixed one collides; safe only because the host is
//	                                   pinned)
//	path     exactly LoopbackPath     (one fixed path, so the plugin's tiny listener has
//	                                   exactly one route to serve)
//	userinfo, query, fragment: rejected outright
//	anything needing normalisation to look safe: rejected outright
//
// `localhost` is REJECTED. It is a name, resolved through DNS, /etc/hosts, a search
// domain or a malicious resolver, so "localhost" can point somewhere that is not this
// machine. The literal addresses cannot. This is the single most important assertion in
// the whole feature and it has its own test.

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
)

// LoopbackPath is the ONE path a device-link redirect may target. Fixing it means the
// plugin's ephemeral listener serves exactly one route and the validator has one string
// to compare, instead of a path-matching rule that could be tricked by traversal,
// duplicate slashes or encoded separators.
//
// Part of the frozen contract: both plugins bind this path.
const LoopbackPath = "/allchat/device-callback"

// loopbackHosts is the closed set of acceptable hosts, as literal addresses.
//
// `[::1]` arrives from url.Parse's Hostname() with the brackets already stripped, so
// the bare form is what we compare. "localhost" is deliberately absent and must stay
// absent; see the file comment.
var loopbackHosts = map[string]bool{
	"127.0.0.1": true,
	"::1":       true,
}

// Errors are distinct values so the caller can log which rule fired without echoing an
// attacker-supplied URI back to the client. The endpoint reports one generic message.
var (
	// ErrLoopbackEmpty is returned for an absent redirect_uri.
	ErrLoopbackEmpty = errors.New("redirect_uri is required for the loopback flow")
	// ErrLoopbackUnparseable is returned when the value is not a URI at all.
	ErrLoopbackUnparseable = errors.New("redirect_uri is not a valid URI")
	// ErrLoopbackScheme is returned for any scheme other than plain http.
	ErrLoopbackScheme = errors.New("redirect_uri scheme must be http")
	// ErrLoopbackHost is returned for any host other than 127.0.0.1 / [::1] — including
	// localhost, which is a DNS name and can be pointed elsewhere.
	ErrLoopbackHost = errors.New("redirect_uri host must be the literal 127.0.0.1 or [::1]")
	// ErrLoopbackPort is returned for a syntactically invalid port.
	ErrLoopbackPort = errors.New("redirect_uri port is not a valid number")
	// ErrLoopbackPath is returned when the path is not exactly LoopbackPath.
	ErrLoopbackPath = errors.New("redirect_uri path must be exactly " + LoopbackPath)
	// ErrLoopbackUserinfo is returned when the authority carries user[:password].
	ErrLoopbackUserinfo = errors.New("redirect_uri must not contain userinfo")
	// ErrLoopbackQuery is returned for a query string; we add code and state ourselves.
	ErrLoopbackQuery = errors.New("redirect_uri must not contain a query string")
	// ErrLoopbackFragment is returned for a fragment.
	ErrLoopbackFragment = errors.New("redirect_uri must not contain a fragment")
	// ErrLoopbackNotNormalised is returned when the URI only looks safe after
	// normalisation — encoded separators, a backslash, whitespace, dot segments.
	ErrLoopbackNotNormalised = errors.New("redirect_uri must be given in normalised form")
)

// ValidateLoopbackRedirect checks a plugin-supplied redirect URI against the rule above
// and returns the exact string that may be redirected to, or an error naming the rule
// that rejected it.
//
// The returned value is rebuilt from the parsed parts rather than echoed, so whatever
// is stored in device_link_requests.redirect_uri and later used in a Location header is
// a string this function constructed, not one an attacker chose.
func ValidateLoopbackRedirect(raw string) (string, error) {
	if strings.TrimSpace(raw) == "" {
		return "", ErrLoopbackEmpty
	}

	// Reject before parsing anything that a parser might normalise into looking safe.
	// url.Parse is lenient by design (it exists to make messy real-world URLs usable),
	// so the cheapest correct defence is to refuse input that needs any interpretation.
	if err := rejectPreParseTricks(raw); err != nil {
		return "", err
	}

	u, err := url.Parse(raw)
	if err != nil {
		return "", ErrLoopbackUnparseable
	}

	// Scheme: exactly http, lowercase. url.Parse lowercases the scheme, so a
	// case-variant spelling is caught by rejectPreParseTricks instead — checked there so
	// "HTTP://..." is rejected rather than silently accepted after normalisation.
	if u.Scheme != "http" {
		return "", ErrLoopbackScheme
	}
	if u.Opaque != "" {
		return "", ErrLoopbackUnparseable
	}
	if u.User != nil {
		return "", ErrLoopbackUserinfo
	}
	if u.RawQuery != "" || u.ForceQuery {
		return "", ErrLoopbackQuery
	}
	if u.Fragment != "" || u.RawFragment != "" {
		return "", ErrLoopbackFragment
	}

	host := u.Hostname()
	if !loopbackHosts[host] {
		// This is where `localhost` dies. Anything that is not one of the two literals
		// is refused, including a name that would resolve to one of them right now: the
		// resolution is not ours and can change between validation and use.
		return "", ErrLoopbackHost
	}

	// Any port is allowed, but it must be a port. url.Port() returns "" both for "no
	// port" and (with a preceding validity check by url.Parse) never for a bad one — so
	// re-validate explicitly rather than trusting the empty string.
	port := u.Port()
	if port != "" {
		if !isNumericPort(port) {
			return "", ErrLoopbackPort
		}
	}

	if u.EscapedPath() != LoopbackPath || u.Path != LoopbackPath {
		// Compared in BOTH forms: EscapedPath() catches an encoded separator that
		// decodes to the right string, u.Path catches a raw one that escapes to it.
		return "", ErrLoopbackPath
	}

	// Rebuilt, not echoed.
	rebuilt := url.URL{Scheme: "http", Host: hostPort(host, port), Path: LoopbackPath}
	return rebuilt.String(), nil
}

// rejectPreParseTricks refuses raw input that would need normalisation to look safe.
//
// Each rule here exists because a permissive parser somewhere has historically been
// talked into accepting one of these. We are not trying to out-parse an attacker: we
// are refusing anything that is not already in the one shape a plugin should send.
func rejectPreParseTricks(raw string) error {
	// A control character or any whitespace is never legitimate in a URI, and several
	// parsers strip them (which is exactly how "http://127.0.0.1\n@evil" gets through
	// something). Refuse instead.
	for _, r := range raw {
		if r <= 0x20 || r == 0x7f {
			return ErrLoopbackNotNormalised
		}
	}
	// A backslash is not a path separator in a URI, but browsers and some parsers treat
	// it as one — the exact class of bug the general redirect guard had to be fixed for
	// (audit M1). There is no reason for one to appear here.
	if strings.Contains(raw, `\`) {
		return ErrLoopbackNotNormalised
	}
	// Percent-encoding anywhere in the authority or path means the value is not in
	// normalised form. The one legitimate URI we accept contains no character that needs
	// encoding, so a `%` is either an evasion attempt or a client bug; both are refused.
	if strings.Contains(raw, "%") {
		return ErrLoopbackNotNormalised
	}
	// Dot segments would let a non-matching path normalise onto LoopbackPath.
	if strings.Contains(raw, "/./") || strings.Contains(raw, "/../") ||
		strings.HasSuffix(raw, "/..") || strings.HasSuffix(raw, "/.") {
		return ErrLoopbackNotNormalised
	}
	// Duplicate slashes in the path can change how a listener routes, and "//host" is a
	// protocol-relative authority. The prefix check below pins the exact scheme spelling
	// so a case variant ("HTTP://") is a rejection rather than a normalisation.
	if !strings.HasPrefix(raw, "http://") {
		return ErrLoopbackScheme
	}
	rest := strings.TrimPrefix(raw, "http://")
	if strings.Contains(rest, "//") {
		return ErrLoopbackNotNormalised
	}
	// An authority ending in a bare colon ("127.0.0.1:/path") parses as "no port", so
	// url.Port() cannot see it. It is a client bug — almost always a port variable that
	// was empty when the URI was composed — and accepting it would silently redirect to
	// port 80 instead of the listener the plugin actually bound.
	authority := rest
	if slash := strings.IndexByte(rest, '/'); slash >= 0 {
		authority = rest[:slash]
	}
	if strings.HasSuffix(authority, ":") {
		return ErrLoopbackPort
	}
	return nil
}

// isNumericPort reports whether s is a decimal port in range. net.SplitHostPort does
// not validate the numeric range, and a port outside it means the client is confused.
func isNumericPort(s string) bool {
	if s == "" || len(s) > 5 {
		return false
	}
	n := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
		n = n*10 + int(r-'0')
	}
	// Port 0 is "pick one for me" at bind time; it can never be a live listener, so a
	// redirect to it is a client bug.
	return n > 0 && n <= 65535
}

// hostPort re-assembles the authority, bracketing an IPv6 literal.
func hostPort(host, port string) string {
	if strings.Contains(host, ":") {
		host = "[" + host + "]"
	}
	if port == "" {
		return host
	}
	return net.JoinHostPort(strings.Trim(host, "[]"), port)
}

// BuildLoopbackRedirect appends the authorization code and the plugin's state to a
// redirect URI that ALREADY passed ValidateLoopbackRedirect.
//
// It re-validates rather than trusting the stored value. The row was written from a
// validated string, so this should never fail — but "should never" is not a property
// worth betting a credential delivery on, and the cost is one function call.
func BuildLoopbackRedirect(validated, code, state string) (string, error) {
	clean, err := ValidateLoopbackRedirect(validated)
	if err != nil {
		return "", fmt.Errorf("stored redirect_uri no longer validates: %w", err)
	}
	u, err := url.Parse(clean)
	if err != nil {
		return "", ErrLoopbackUnparseable
	}
	q := url.Values{}
	q.Set("code", code)
	// `state` is generated by the plugin, echoed here, and compared by the plugin. The
	// server never interprets it; it is the plugin's CSRF check on its own listener.
	if state != "" {
		q.Set("state", state)
	}
	u.RawQuery = q.Encode()
	return u.String(), nil
}
