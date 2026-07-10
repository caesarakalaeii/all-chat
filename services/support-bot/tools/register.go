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

package tools

import (
	"time"

	"github.com/caesar/all-chat/services/support-bot/ghclient"
	"github.com/caesar/all-chat/services/support-bot/grafana"
	"github.com/caesar/all-chat/services/support-bot/memory"
	"github.com/caesar/all-chat/services/support-bot/tool"
)

// Deps carries the static dependencies the tools need (per-request context comes from
// tool.ToolCtx instead).
type Deps struct {
	GH             *ghclient.Client
	GHOwner        string
	GHBotLogin     string
	BlockedRepos   []string
	Grafana        *grafana.Client
	GrafanaLokiDS  string
	GrafanaPromDS  string
	GrafanaLogTail time.Duration
	Memory         *memory.Repository
}

// RegisterAll registers every support-bot tool into the registry. Read-only tools are
// always registered; the Grafana/GitHub/memory tools register only when their backend
// is configured. Access-mode gating is enforced per tool at dispatch time, so it is
// safe to register the admin-only github_write tool unconditionally.
func RegisterAll(reg *tool.Registry, d Deps) {
	reg.Register(ReadFileTool{})
	reg.Register(GlobTool{})
	reg.Register(GrepTool{})
	reg.Register(KubectlTool{})

	if d.Grafana != nil {
		reg.Register(NewGrafanaLogsTool(d.Grafana, d.GrafanaLokiDS, d.GrafanaLogTail))
		reg.Register(NewGrafanaMetricsTool(d.Grafana, d.GrafanaPromDS))
	}
	if d.GH != nil {
		reg.Register(NewGitHubTool(d.GH, d.GHOwner, d.GHBotLogin, d.BlockedRepos))
		reg.Register(NewGitHubWriteTool(d.GH, d.GHOwner, d.GHBotLogin, d.BlockedRepos))
	}
	if d.Memory != nil {
		reg.Register(NewMemoryTool(d.Memory))
	}
}
