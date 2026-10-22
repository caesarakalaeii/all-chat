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

package chatcmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/caesar/all-chat/shared/safehttp"
)

// {urlfetch} limits (ADR-0080). The fetch runs on our servers, so it only reaches
// public HTTPS on port 443: safehttp refuses every non-public address, and the port
// rule keeps the nodes' own public IPs to the ingress everyone can already reach.
const (
	maxFetches       = 3 // per render
	maxFetchBytes    = 32 << 10
	maxURLBytes      = 2048
	fetchTimeout     = 3 * time.Second
	fetchErrorMarker = "[urlfetch error]"
	fetchUserAgent   = "All-Chat-URLFetch/1.0 (+https://allch.at)"
)

// CheckFetchURL reports why raw may not be fetched: only absolute https URLs on the
// default port, without credentials.
func CheckFetchURL(raw string) error {
	u, err := checkHTTPS(raw)
	if err != nil {
		return fmt.Errorf("urlfetch: %w", err)
	}
	if p := u.Port(); p != "" && p != "443" {
		return errors.New("urlfetch: only the default https port is allowed")
	}
	return nil
}

func checkHTTPS(raw string) (*url.URL, error) {
	if len(raw) > maxURLBytes {
		return nil, fmt.Errorf("URL is longer than %d bytes", maxURLBytes)
	}
	u, err := url.Parse(raw)
	switch {
	case err != nil || u.Scheme != "https" || u.Hostname() == "":
		return nil, fmt.Errorf("%q is not an absolute https URL", raw)
	case u.User != nil:
		return nil, errors.New("URL must not carry credentials")
	}
	return u, nil
}

// FetchClient is the client {urlfetch} uses: public addresses only (safehttp), and
// CheckFetchURL on every redirect hop too.
func FetchClient() *http.Client {
	c := safehttp.Client(fetchTimeout)
	limit := c.CheckRedirect
	c.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if err := CheckFetchURL(req.URL.String()); err != nil {
			return err
		}
		return limit(req, via)
	}
	return c
}

// HTTPFetch returns a Vars.Fetch that GETs a URL with client and returns at most
// maxFetchBytes of a 2xx body as text.
func HTTPFetch(ctx context.Context, client *http.Client) func(string) (string, error) {
	return func(raw string) (string, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, raw, nil)
		if err != nil {
			return "", err
		}
		req.Header.Set("User-Agent", fetchUserAgent)
		resp, err := client.Do(req)
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode > 299 {
			return "", fmt.Errorf("HTTP %d", resp.StatusCode)
		}
		body, err := io.ReadAll(io.LimitReader(resp.Body, maxFetchBytes))
		if err != nil {
			return "", err
		}
		return strings.ToValidUTF8(string(body), "�"), nil
	}
}

// ImageURL renders a command's image template into the URL an alerts overlay shows
// (ADR-0080), or "" when it renders empty or to anything but an https URL. Chatter
// input is withheld: {user}, {query}, the arguments and their JS globals are empty,
// so what appears on stream is the streamer's choice alone.
func ImageURL(tpl string, v Vars) (string, []string) {
	v.User, v.Args, v.Query = "", nil, ""
	out, errs := Render(tpl, v)
	if out = strings.TrimSpace(out); out == "" {
		return "", errs
	}
	if _, err := checkHTTPS(out); err != nil {
		return "", append(errs, "image: "+err.Error())
	}
	return out, errs
}
