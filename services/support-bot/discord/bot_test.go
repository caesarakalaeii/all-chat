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

// failingClient fails the first Chat call with the configured error, so
// agent.Run aborts immediately and answer() hits its error path.
type failingClient struct{ err error }

func (f *failingClient) Chat(context.Context, llm.ChatRequest) (*llm.ChatResponse, error) {
	return nil, f.err
}

func newTestBot(t *testing.T, client llm.ChatClient) *Bot {
	t.Helper()
	return &Bot{
		cfg:      &config.Config{OverallTimeout: 6 * time.Minute},
		policy:   access.NewPolicy(nil),
		reg:      tool.NewRegistry(),
		llm:      client,
		redactor: redact.NewRedactor(),
		agentCfg: agent.Config{Model: "test"},
		log:      zap.NewNop(),
	}
}

// The gateway 429s hobby-project traffic while production load is high; that is a
// capacity pause, and the reply must say so instead of claiming something broke.
func TestAnswerReportsBusyAsCapacityPause(t *testing.T) {
	b := newTestBot(t, &failingClient{err: &llm.APIError{Kind: llm.KindBusy, Status: 429, Message: "too many requests"}})
	got := b.answer("1", "c", "question", nil)
	if !strings.Contains(got, "under load") {
		t.Fatalf("expected capacity-pause reply, got: %q", got)
	}
	if strings.Contains(got, "something went wrong") {
		t.Fatalf("busy misreported as unknown fault: %q", got)
	}
}

// A deadline-exceeded run must be reported as a timeout with the limit in the
// reply, not as an unknown fault (the failure #818 fixed for the old TS bot).
func TestAnswerReportsTimeoutAsTimeout(t *testing.T) {
	b := newTestBot(t, &failingClient{err: context.DeadlineExceeded})
	got := b.answer("1", "c", "why did the bot die?", nil)
	if !strings.Contains(got, "6m") {
		t.Fatalf("expected timeout reply naming the limit, got: %q", got)
	}
	if strings.Contains(got, "something went wrong") {
		t.Fatalf("timeout misreported as unknown fault: %q", got)
	}
}

func TestAnswerGenericErrorKeepsGenericReply(t *testing.T) {
	b := newTestBot(t, &failingClient{err: context.Canceled})
	got := b.answer("1", "c", "question", nil)
	if !strings.Contains(got, "something went wrong") {
		t.Fatalf("expected generic error reply, got: %q", got)
	}
}
