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

// Package chatcmd is the streamer-defined chat command engine (ADR-0079): matching
// a chat message to a command or keyword trigger, the role check, and rendering the
// response template. Its only I/O is {urlfetch}, through the Vars.Fetch the caller
// passes (ADR-0080); the consumer owns delivery.
package chatcmd

import (
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// Vars is everything a response template can read. Every field but Fetch is data
// supplied by the chat message or the command, never template syntax.
type Vars struct {
	User     string   // display name of the chatter who fired the command
	Args     []string // words after the command word ({1}..{9}, {touser})
	Query    string   // text after the command word; the whole message for a keyword
	Platform string
	Channel  string
	Count    int64  // times the command has fired, this use included
	Role     string // the chatter's highest role (see Role)
	Clip     string // URL of the clip this command made; empty when none
	Now      time.Time

	// Fetch serves {urlfetch} (HTTPFetch in production); nil leaves the token as
	// written. It is the one way a render reaches the network.
	Fetch func(url string) (string, error)
}

// Render expands the {variable} tokens in tpl. An unknown or malformed token is left
// as written, so ordinary braces in a response survive. errs carries one entry per
// failed {js} block or {urlfetch} (which render as jsErrorMarker / fetchErrorMarker)
// so the editor preview can show the streamer why; chat delivery ignores them.
func Render(tpl string, v Vars) (out string, errs []string) {
	r := &render{v: v}
	defer func() { r.js.close() }()
	return r.expand(tpl, nil), r.errs
}

// render is the state one Render shares across its tokens, including the tokens
// nested in a {urlfetch} URL: one JS session and one fetch allowance.
type render struct {
	v       Vars
	js      *jsSession
	fetches int
	errs    []string
}

// expand renders tpl. esc, when set, is applied to every substituted value: the
// query escaping of a {urlfetch} URL.
func (r *render) expand(tpl string, esc func(string) string) string {
	var b strings.Builder
	for i := 0; i < len(tpl); {
		if tpl[i] != '{' {
			b.WriteByte(tpl[i])
			i++
			continue
		}
		name := tokenName(tpl[i+1:])
		end := -1
		if name == "js" || name == "urlfetch" {
			end = blockEnd(tpl, i+1+len(name))
		} else if j := strings.IndexByte(tpl[i+1:], '}'); j >= 0 {
			end = i + 1 + j
		}
		if end < 0 {
			b.WriteByte('{')
			i++
			continue
		}
		val, ok := r.token(name, strings.TrimSpace(tpl[i+1+len(name):end]))
		if !ok {
			b.WriteByte('{')
			i++
			continue
		}
		if esc != nil {
			val = esc(val)
		}
		b.WriteString(val)
		i = end + 1
	}
	return b.String()
}

func (r *render) token(name, arg string) (string, bool) {
	switch name {
	case "js":
		if r.js == nil {
			r.js = newJSSession(r.v)
		}
		code := r.inlineFetches(arg)
		val, err := r.js.eval(code)
		if err != nil {
			r.errs = append(r.errs, err.Error())
			return jsErrorMarker, true
		}
		return val, true
	case "urlfetch":
		if r.v.Fetch == nil || arg == "" {
			return "", false
		}
		body, err := r.fetch(arg)
		if err != nil {
			return fetchErrorMarker, true
		}
		return body, true
	}
	return r.v.resolve(name, arg)
}

// fetch renders a {urlfetch} URL template, checks it against the urlfetch rules and
// fetches it. A failure is recorded in errs.
func (r *render) fetch(urlTpl string) (body string, err error) {
	defer func() {
		if err != nil {
			r.errs = append(r.errs, err.Error())
		}
	}()
	if r.fetches >= maxFetches {
		return "", fmt.Errorf("urlfetch: at most %d per reply", maxFetches)
	}
	r.fetches++
	raw := r.expand(urlTpl, url.QueryEscape)
	if err := CheckFetchURL(raw); err != nil {
		return "", err
	}
	if body, err = r.v.Fetch(raw); err != nil {
		return "", fmt.Errorf("urlfetch %s: %w", raw, err)
	}
	return strings.TrimSpace(body), nil
}

// inlineFetches replaces each {urlfetch} in a {js} block's code with the response as
// a JS string literal, or null when the fetch failed. JSON quoting keeps the body
// data: it can never end the literal and become code.
func (r *render) inlineFetches(code string) string {
	const open = "{urlfetch"
	if r.v.Fetch == nil {
		return code
	}
	var b strings.Builder
	for {
		i := strings.Index(code, open)
		if i < 0 || i+len(open) >= len(code) || code[i+len(open)] != ' ' {
			break
		}
		end := blockEnd(code, i+len(open))
		if end < 0 {
			break
		}
		lit := "null"
		if body, err := r.fetch(strings.TrimSpace(code[i+len(open) : end])); err == nil {
			q, _ := json.Marshal(body)
			lit = string(q)
		}
		b.WriteString(code[:i])
		b.WriteString(lit)
		code = code[end+1:]
	}
	b.WriteString(code)
	return b.String()
}

// tokenName returns the leading identifier of a token body ("user" in "user}",
// "random" in "random 1 6}").
func tokenName(s string) string {
	n := 0
	for n < len(s) && (s[n] == '_' || s[n] >= 'a' && s[n] <= 'z' || s[n] >= '0' && s[n] <= '9') {
		n++
	}
	return s[:n]
}

func (v Vars) resolve(name, arg string) (string, bool) {
	switch name {
	case "user":
		return v.User, arg == ""
	case "touser":
		if len(v.Args) > 0 {
			return strings.TrimPrefix(v.Args[0], "@"), arg == ""
		}
		return v.User, arg == ""
	case "query":
		return v.Query, arg == ""
	case "platform":
		return v.Platform, arg == ""
	case "channel":
		return v.Channel, arg == ""
	case "count":
		return strconv.FormatInt(v.Count, 10), arg == ""
	case "clip":
		return v.Clip, arg == ""
	case "time":
		loc := time.UTC
		if arg != "" {
			l, err := time.LoadLocation(arg)
			if err != nil {
				return "", false
			}
			loc = l
		}
		return v.now().In(loc).Format("15:04"), true
	case "random":
		lo, hi := 1, 100
		if arg != "" {
			f := strings.Fields(arg)
			if len(f) != 2 {
				return "", false
			}
			var err1, err2 error
			lo, err1 = strconv.Atoi(f[0])
			hi, err2 = strconv.Atoi(f[1])
			if err1 != nil || err2 != nil || lo > hi {
				return "", false
			}
		}
		return strconv.Itoa(lo + rand.IntN(hi-lo+1)), true
	case "pick":
		if arg == "" {
			return "", false
		}
		opts := strings.Split(arg, "|")
		return strings.TrimSpace(opts[rand.IntN(len(opts))]), true
	}
	if n, err := strconv.Atoi(name); err == nil && n >= 1 && n <= 9 && arg == "" {
		if n <= len(v.Args) {
			return v.Args[n-1], true
		}
		return "", true
	}
	return "", false
}

func (v Vars) now() time.Time {
	if v.Now.IsZero() {
		return time.Now()
	}
	return v.Now
}

// blockEnd finds the '}' closing a {js ...} or {urlfetch ...} block that starts
// scanning at from. It tracks brace depth and skips JS string and template literals,
// so object literals, nested tokens and "}" inside strings do not end the token
// early. -1 when unclosed.
func blockEnd(s string, from int) int {
	depth := 0
	for i := from; i < len(s); i++ {
		switch c := s[i]; c {
		case '\'', '"', '`':
			for i++; i < len(s) && s[i] != c; i++ {
				if s[i] == '\\' {
					i++
				}
			}
		case '{':
			depth++
		case '}':
			if depth == 0 {
				return i
			}
			depth--
		}
	}
	return -1
}

// MaxReplyBytes is the longest reply auth-service will send.
const MaxReplyBytes = 500

// Line makes a rendered response sendable: one line, single spaces, at most
// MaxReplyBytes, never cut inside a UTF-8 sequence.
func Line(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if len(s) <= MaxReplyBytes {
		return s
	}
	s = s[:MaxReplyBytes]
	for !utf8.ValidString(s) {
		s = s[:len(s)-1]
	}
	return s
}
