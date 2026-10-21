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
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"modernc.org/quickjs"
)

// Sandbox limits for streamer-written {js} blocks (ADR-0079). The engine is QuickJS
// with no host functions registered: no require, fetch, timers, file or network
// access exist inside it. The limits bound what is left, CPU and memory.
const (
	jsBudget      = 100 * time.Millisecond // CPU wall time for ALL {js} blocks of one render
	jsMemoryLimit = 16 << 20
	jsStackLimit  = 256 << 10
	jsErrorMarker = "[js error]"
)

// jsSession is one QuickJS VM shared by the {js} blocks of a single render, so a
// variable set in one block is visible in the next. Never reused across renders.
type jsSession struct {
	vm       *quickjs.VM
	err      error
	deadline time.Time
}

func newJSSession(v Vars) *jsSession {
	s := &jsSession{deadline: time.Now().Add(jsBudget)}
	vm, err := quickjs.NewVM()
	if err != nil {
		s.err = fmt.Errorf("start js engine: %w", err)
		return s
	}
	s.vm = vm
	vm.SetMemoryLimit(jsMemoryLimit)
	vm.SetMaxStackSize(jsStackLimit)

	// The template variables enter the VM as one JSON object literal, so chat text is
	// only ever data. json.Marshal escapes every character that could end the literal.
	args := v.Args
	if args == nil {
		args = []string{}
	}
	globals, _ := json.Marshal(map[string]any{
		"user": v.User, "touser": v.touser(), "query": v.Query, "args": args,
		"platform": v.Platform, "channel": v.Channel, "count": v.Count, "role": v.Role, "clip": v.Clip,
	})
	if _, err := s.run("Object.assign(globalThis, " + string(globals) + ")"); err != nil {
		s.err = fmt.Errorf("seed js globals: %w", err)
	}
	return s
}

// eval runs one block and returns its completion value as text: undefined and null
// render empty, objects as JSON. Indirect eval keeps `var` declarations global, so
// they carry over to later blocks of the same render.
func (s *jsSession) eval(code string) (string, error) {
	if s.err != nil {
		return "", s.err
	}
	quoted, _ := json.Marshal(code)
	r, err := s.run(`(() => { const r = (0, eval)(` + string(quoted) + `);
		return r == null ? "" : typeof r === "object" ? JSON.stringify(r) : String(r) })()`)
	if err != nil {
		return "", err
	}
	out, ok := r.(string)
	if !ok {
		return "", errors.New("js result is not a string")
	}
	return out, nil
}

func (s *jsSession) run(code string) (any, error) {
	left := time.Until(s.deadline)
	if left <= 0 {
		return nil, errors.New("js time budget exhausted")
	}
	if err := s.vm.SetEvalTimeout(left); err != nil {
		return nil, err
	}
	return s.vm.Eval(code, quickjs.EvalGlobal)
}

func (s *jsSession) close() {
	if s != nil && s.vm != nil {
		s.vm.Close()
	}
}

func (v Vars) touser() string {
	val, _ := v.resolve("touser", "")
	return val
}
