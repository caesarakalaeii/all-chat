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
	"strings"
	"testing"

	"github.com/caesar/all-chat/services/support-bot/access"
)

// Both modes may hand a confirmed code defect to Caterpillar, so both prompts must
// say how: dedupe first, follow the agent-task template, set agent_task.
func TestBuildSystemPrompt_TellsBothModesHowToFileAgentTasks(t *testing.T) {
	for _, mode := range []access.Mode{access.ModeSupport, access.ModeAdmin} {
		p := BuildSystemPrompt(mode, nil, false)
		for _, want := range []string{"agent_task", "search_issues", ".github/ISSUE_TEMPLATE/agent_task.md", "Caterpillar"} {
			if !strings.Contains(p, want) {
				t.Errorf("mode %v: prompt does not mention %q", mode, want)
			}
		}
	}
}

// The bot must understand a report before it answers or files anything: ask the
// reporter what only they know, play back its understanding, and get it confirmed
// before an agent task goes to Caterpillar.
func TestBuildSystemPrompt_UnderstandsBeforeActing(t *testing.T) {
	for _, mode := range []access.Mode{access.ModeSupport, access.ModeAdmin} {
		p := BuildSystemPrompt(mode, nil, false)
		for _, want := range []string{
			"UNDERSTAND THE PROBLEM FIRST",
			"clarifying questions",
			"what they see",
			"confirm",
			"Do not file an agent task until",
		} {
			if !strings.Contains(p, want) {
				t.Errorf("mode %v: prompt does not mention %q", mode, want)
			}
		}
	}
}

// UI how-to answers should show the screen, not only describe it.
func TestBuildSystemPrompt_ShowsScreensForUIQuestions(t *testing.T) {
	p := BuildSystemPrompt(access.ModeSupport, nil, false)
	if !strings.Contains(p, "send_image") {
		t.Error("prompt does not tell the bot to attach screenshots for UI questions")
	}
}
