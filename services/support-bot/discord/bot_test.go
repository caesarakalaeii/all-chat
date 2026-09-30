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

package discord

import (
	"context"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/caesar/all-chat/services/support-bot/access"
	"github.com/caesar/all-chat/services/support-bot/agent"
	"github.com/caesar/all-chat/services/support-bot/config"
	"github.com/caesar/all-chat/services/support-bot/llm"
	"github.com/caesar/all-chat/services/support-bot/redact"
	"github.com/caesar/all-chat/services/support-bot/tool"
	"go.uber.org/zap"
)

// scriptedClient returns the scripted results/errors in order, then repeats the
// last one. It lets a test simulate a mid-session busy (429) followed by
// recovery.
type scriptedClient struct {
	mu    atomic.Int32
	resps []resp
}
type resp struct {
	out *llm.ChatResponse
	err error
}

func (s *scriptedClient) Chat(_ context.Context, _ llm.ChatRequest) (*llm.ChatResponse, error) {
	i := int(s.mu.Add(1)) - 1
	if i >= len(s.resps) {
		i = len(s.resps) - 1
	}
	r := s.resps[i]
	return r.out, r.err
}

// failingClient fails the first Chat call with the configured error, so
// agent.Run aborts immediately and answer() hits its error path.
type failingClient struct{ err error }

func (f *failingClient) Chat(context.Context, llm.ChatRequest) (*llm.ChatResponse, error) {
	return nil, f.err
}

func newTestBot(t *testing.T, client llm.ChatClient) *Bot {
	t.Helper()
	b := &Bot{
		cfg:      &config.Config{OverallTimeout: 6 * time.Minute, BusyRetryDelay: time.Millisecond, BusyQueueTTL: time.Hour},
		policy:   access.NewPolicy(nil),
		reg:      tool.NewRegistry(),
		llm:      client,
		redactor: redact.NewRedactor(),
		agentCfg: agent.Config{Model: "test"},
		log:      zap.NewNop(),
		queues:   newSerialQueues(),
	}
	b.busyQ = newBusyQueue(b)
	return b
}

// A deadline-exceeded run must be reported as a timeout with the limit in the
// reply, not as an unknown fault (the failure #818 fixed for the old TS bot).
func TestAnswerReportsTimeoutAsTimeout(t *testing.T) {
	b := newTestBot(t, &failingClient{err: context.DeadlineExceeded})
	got := b.answer(nil, "1", "c", "why did the bot die?", nil)
	if !strings.Contains(got, "6m") {
		t.Fatalf("expected timeout reply naming the limit, got: %q", got)
	}
	if strings.Contains(got, "something went wrong") {
		t.Fatalf("timeout misreported as unknown fault: %q", got)
	}
}

func TestAnswerGenericErrorKeepsGenericReply(t *testing.T) {
	b := newTestBot(t, &failingClient{err: context.Canceled})
	got := b.answer(nil, "1", "c", "question", nil)
	if !strings.Contains(got, "something went wrong") {
		t.Fatalf("expected generic error reply, got: %q", got)
	}
}

// A busy (429) run parks the request instead of dropping it: the user gets the
// queue reply and the busy queue holds the transcript for the worker to resume.
func TestAnswerParksBusyRequest(t *testing.T) {
	b := newTestBot(t, &failingClient{err: &llm.APIError{Kind: llm.KindBusy, Status: 429, Message: "busy"}})
	got := b.answer(nil, "1", "c", "question", nil)
	if !strings.Contains(got, "queued") {
		t.Fatalf("expected the queued reply, got: %q", got)
	}
	b.busyQ.mu.Lock()
	n := len(b.busyQ.q)
	b.busyQ.mu.Unlock()
	if n != 1 {
		t.Fatalf("expected 1 parked request, got %d", n)
	}
}

// drainOnce on a still-busy request re-queues it; once capacity returns the
// resumed transcript is run to completion and the answer delivered.
func TestBusyQueueResumesWhenCapacityReturns(t *testing.T) {
	busyErr := &llm.APIError{Kind: llm.KindBusy, Status: 429, Message: "busy"}
	ok := &llm.ChatResponse{Choices: []llm.Choice{{
		FinishReason: llm.FinishStop,
		Message:      llm.TextMessage(llm.RoleAssistant, "recovered answer"),
	}}}
	b := newTestBot(t, &scriptedClient{resps: []resp{{err: busyErr}, {err: busyErr}, {out: ok}}})

	b.busyQ.park(nil, &parkedRequest{uid: "1", channelID: "c", mode: access.ModeSupport,
		messages: []llm.Message{llm.TextMessage(llm.RoleUser, "q")}})

	b.busyQ.drainOnce(context.Background()) // 1st resume: still busy -> requeued
	if got := len(b.busyQ.snapshot()); got != 1 {
		t.Fatalf("still-busy resume should requeue, queue len %d", got)
	}
	b.busyQ.drainOnce(context.Background()) // 2nd resume: still busy -> requeued again
	b.busyQ.drainOnce(context.Background()) // 3rd resume: capacity -> delivered
	if got := len(b.busyQ.snapshot()); got != 0 {
		t.Fatalf("recovered request should leave the queue empty, got %d", got)
	}
}

// A parked request older than the TTL is dropped with the expiry reply, not
// retried forever.
func TestBusyQueueExpiresStaleRequests(t *testing.T) {
	b := newTestBot(t, &failingClient{err: context.Canceled})
	b.busyQ.park(nil, &parkedRequest{uid: "1", channelID: "c", mode: access.ModeSupport})
	// Age the request past the TTL.
	b.busyQ.mu.Lock()
	b.busyQ.q[0].enqueued = time.Now().Add(-2 * time.Hour)
	b.busyQ.mu.Unlock()

	b.busyQ.drainOnce(context.Background())
	if got := len(b.busyQ.snapshot()); got != 0 {
		t.Fatalf("expired request should be dropped, queue len %d", got)
	}
}
