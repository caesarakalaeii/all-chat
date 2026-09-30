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

package agent

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/caesar/all-chat/services/support-bot/llm"
	"github.com/caesar/all-chat/services/support-bot/tool"
	"go.uber.org/zap"
)

// busyClient returns scripted errors then responses, one per call.
type busyClient struct {
	calls int32
	errs  []error
	resps []*llm.ChatResponse
}

func (f *busyClient) Chat(_ context.Context, _ llm.ChatRequest) (*llm.ChatResponse, error) {
	i := int(atomic.AddInt32(&f.calls, 1)) - 1
	if i < len(f.errs) {
		return nil, f.errs[i]
	}
	j := i - len(f.errs)
	if j >= len(f.resps) {
		j = len(f.resps) - 1
	}
	return f.resps[j], nil
}

// Mid-session busy: the loop waits and re-issues the SAME transcript. A 429 after
// a tool call must not discard the tool result — the next successful request
// still carries it, and the tool is not invoked a second time.
func TestRunWaitsOnBusyMidSession(t *testing.T) {
	var invoked int32
	reg := tool.NewRegistry()
	reg.Register(&echoTool{invoked: &invoked})

	busy := &llm.APIError{Kind: llm.KindBusy, Status: 429, Message: "busy"}
	client := &busyClient{
		errs:  []error{busy, busy},
		resps: []*llm.ChatResponse{toolCallResp("c1", "echo", "{}"), stopResp("done")},
	}

	cfg := Config{Model: "m", BusyRetryDelay: time.Millisecond, BusyMaxWaits: 3, Log: zap.NewNop()}
	res, err := Run(context.Background(), cfg, client, reg, supportCtx(),
		[]llm.Message{llm.TextMessage(llm.RoleUser, "hi")})
	if err != nil {
		t.Fatal(err)
	}
	if res.Text != "done" {
		t.Fatalf("text = %q, want done", res.Text)
	}
	if atomic.LoadInt32(&invoked) != 1 {
		t.Fatalf("echo invoked %d times, want 1 (busy must not replay tools)", invoked)
	}
	if n := atomic.LoadInt32(&client.calls); n != 4 {
		t.Fatalf("expected 4 llm calls (1 tool, 2 busy, 1 done), got %d", n)
	}
}

// Busy beyond the wait budget: the error surfaces with the transcript attached,
// so a caller can park and resume the same session.
func TestRunReturnsTranscriptWhenBusyOutlastsWaits(t *testing.T) {
	busy := &llm.APIError{Kind: llm.KindBusy, Status: 429, Message: "busy"}
	client := &busyClient{errs: []error{busy, busy, busy, busy}}

	cfg := Config{Model: "m", BusyRetryDelay: time.Millisecond, BusyMaxWaits: 2, Log: zap.NewNop()}
	res, err := Run(context.Background(), cfg, client, reg(), supportCtx(),
		[]llm.Message{llm.TextMessage(llm.RoleUser, "hi")})
	if err == nil {
		t.Fatal("expected the busy error to surface")
	}
	if len(res.Messages) == 0 {
		t.Fatal("expected the transcript on the result for parking")
	}
}

func reg() *tool.Registry {
	r := tool.NewRegistry()
	var invoked int32
	r.Register(&echoTool{invoked: &invoked})
	return r
}
