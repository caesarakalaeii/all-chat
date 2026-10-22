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
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeFetch serves {urlfetch} from a map and records every URL asked for.
func fakeFetch(bodies map[string]string, asked *[]string) func(string) (string, error) {
	return func(u string) (string, error) {
		*asked = append(*asked, u)
		if b, ok := bodies[u]; ok {
			return b, nil
		}
		return "", errors.New("HTTP 404")
	}
}

func TestRender_URLFetch(t *testing.T) {
	var asked []string
	v := sampleVars()
	v.Fetch = fakeFetch(map[string]string{
		"https://api.example.com/joke":         "  a joke \n",
		"https://api.example.com/q?u=%40Bob+x": "found",
		"https://api.example.com/dog":          `{"message":"https://img.example.com/dog.jpg"}`,
	}, &asked)

	cases := []struct{ tpl, want string }{
		{"{urlfetch https://api.example.com/joke}", "a joke"},
		// Every value inside the URL is query-escaped, so chat text cannot add
		// parameters, change the path or the host.
		{"{urlfetch https://api.example.com/q?u={query}}", "found"},
		// Inside {js} the body arrives as a JS string literal.
		{"{js JSON.parse({urlfetch https://api.example.com/dog}).message}", "https://img.example.com/dog.jpg"},
	}
	for _, c := range cases {
		got, errs := Render(c.tpl, v)
		assert.Empty(t, errs, c.tpl)
		assert.Equal(t, c.want, got, c.tpl)
	}
}

func TestRender_URLFetchBodyIsDataNotCode(t *testing.T) {
	var asked []string
	v := sampleVars()
	v.Fetch = fakeFetch(map[string]string{"https://evil.example.com/": `"; throw new Error("pwned"); "`}, &asked)
	got, errs := Render("{js {urlfetch https://evil.example.com/}}", v)
	assert.Empty(t, errs)
	assert.Equal(t, `"; throw new Error("pwned"); "`, got)
}

func TestRender_URLFetchRefusals(t *testing.T) {
	cases := []string{
		"{urlfetch http://api.example.com/}",       // not https
		"{urlfetch https://api.example.com:8443/}", // not port 443
		"{urlfetch https://user:pw@api.example.com/}",
		"{urlfetch https:///nohost}",
		"{urlfetch {1}}", // the chatter cannot pick the URL: "%40Bob" is no URL
		"{urlfetch https://api.example.com/missing}",
	}
	for _, tpl := range cases {
		var asked []string
		v := sampleVars()
		v.Fetch = fakeFetch(nil, &asked)
		got, errs := Render(tpl, v)
		assert.Equal(t, fetchErrorMarker, got, tpl)
		assert.Len(t, errs, 1, tpl)
		if !strings.Contains(tpl, "missing") {
			assert.Empty(t, asked, "refused before fetching: %s", tpl)
		}
	}
}

func TestRender_URLFetchInJSFailsToNull(t *testing.T) {
	var asked []string
	v := sampleVars()
	v.Fetch = fakeFetch(nil, &asked)
	got, errs := Render("{js {urlfetch https://api.example.com/x} ?? 'offline'}", v)
	assert.Equal(t, "offline", got)
	assert.Len(t, errs, 1)
}

func TestRender_URLFetchLimitPerRender(t *testing.T) {
	var asked []string
	v := sampleVars()
	v.Fetch = fakeFetch(map[string]string{"https://api.example.com/": "x"}, &asked)
	got, errs := Render(strings.Repeat("{urlfetch https://api.example.com/}", maxFetches+2), v)
	assert.Len(t, asked, maxFetches)
	assert.Equal(t, strings.Repeat("x", maxFetches)+strings.Repeat(fetchErrorMarker, 2), got)
	assert.Len(t, errs, 2)
}

func TestRender_URLFetchWithoutFetcherStaysLiteral(t *testing.T) {
	got, errs := Render("{urlfetch https://api.example.com/}", sampleVars())
	assert.Empty(t, errs)
	assert.Equal(t, "{urlfetch https://api.example.com/}", got)
}

// Fetch time is not JS time: a slow API before a {js} block must not use up the
// sandbox's CPU budget.
func TestRender_FetchDoesNotSpendJSBudget(t *testing.T) {
	v := sampleVars()
	v.Fetch = func(string) (string, error) {
		<-time.After(2 * jsBudget)
		return "1", nil
	}
	got, errs := Render("{js var a = 1}{urlfetch https://api.example.com/}{js a + 1}", v)
	assert.Empty(t, errs)
	assert.Equal(t, "12", got)
}

func TestImageURL(t *testing.T) {
	var asked []string
	v := sampleVars()
	v.Fetch = fakeFetch(map[string]string{"https://dog.example.com/api": `{"message":"https://img.example.com/d.jpg"}`}, &asked)

	cases := []struct{ tpl, want string }{
		{"https://img.example.com/hype.gif", "https://img.example.com/hype.gif"},
		{" {pick https://a.example.com/1.gif | https://a.example.com/2.gif} ", ""}, // checked below
		{"{js JSON.parse({urlfetch https://dog.example.com/api}).message}", "https://img.example.com/d.jpg"},
		// Chatter input never reaches an image: what appears on stream is the
		// streamer's choice alone.
		{"https://img.example.com/{1}.gif", "https://img.example.com/.gif"},
		{"{query}", ""},
		{"{js args.join('') || user}", ""},
		{"http://img.example.com/x.gif", ""},
		{"javascript:alert(1)", ""},
		{"", ""},
	}
	for _, c := range cases {
		got, _ := ImageURL(c.tpl, v)
		if strings.Contains(c.tpl, "pick") {
			assert.Contains(t, []string{"https://a.example.com/1.gif", "https://a.example.com/2.gif"}, got)
			continue
		}
		assert.Equal(t, c.want, got, c.tpl)
	}
}

func TestHTTPFetch(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/ok":
			assert.Contains(t, r.Header.Get("User-Agent"), "All-Chat")
			fmt.Fprint(w, "hello")
		case "/big":
			_, _ = io.WriteString(w, strings.Repeat("a", 2*maxFetchBytes))
		default:
			http.Error(w, "nope", http.StatusNotFound)
		}
	}))
	defer srv.Close()
	fetch := HTTPFetch(t.Context(), srv.Client())

	body, err := fetch(srv.URL + "/ok")
	require.NoError(t, err)
	assert.Equal(t, "hello", body)

	body, err = fetch(srv.URL + "/big")
	require.NoError(t, err)
	assert.Len(t, body, maxFetchBytes)

	_, err = fetch(srv.URL + "/missing")
	assert.ErrorContains(t, err, "404")
}

// A redirect must not lead {urlfetch} anywhere its own URL rules refuse.
func TestFetchClientChecksRedirects(t *testing.T) {
	c := FetchClient()
	for target, ok := range map[string]bool{
		"https://api.example.com/next":      true,
		"http://api.example.com/next":       false,
		"https://api.example.com:10250/pod": false,
	} {
		req, _ := http.NewRequest(http.MethodGet, target, nil)
		err := c.CheckRedirect(req, []*http.Request{{}})
		assert.Equal(t, ok, err == nil, target)
	}
}
