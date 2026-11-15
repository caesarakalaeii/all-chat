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
