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

// Package config loads the support-bot runtime configuration from the environment.
// It replaces the former Claude-Code-CLI bot's config: the LLM is now a locally
// hosted OpenAI-compatible endpoint (LOCAL_LLM_*), and admin access is gated by an
// explicit Discord UID allow-list (SUPPORT_BOT_ADMIN_DISCORD_IDS) rather than being
// a single fixed tool set.
package config

import (
	"os"
	"strings"
	"time"
)

// Config holds runtime configuration. Every field is sourced from the environment
// (the allchat-config ConfigMap + allchat-secrets in production). Secrets are held
// here only so the wiring in main can hand them to the clients that need them; they
// are never placed into the LLM prompt or tool output.
type Config struct {
	// HTTP / observability
	Port     string
	GinMode  string
	LogLevel string

	// Database (bot memory only)
	DatabaseHost     string
	DatabasePort     string
	DatabaseUser     string
	DatabasePassword string
	DatabaseName     string

	// Discord
	DiscordToken      string
	DiscordAppID      string
	DiscordGuildID    string // optional: guild-scoped slash-command registration (instant)
	ModerationGuildID string // optional: enables cross-channel-spam auto-ban in this guild

	// Local LLM (OpenAI-compatible /v1/chat/completions)
	LLMBaseURL   string
	LLMModel     string
	LLMAPIKey    string // optional; many local servers need none
	LLMMaxTokens int

	// GitHub
	GitHubToken    string
	GitHubOwner    string
	GitHubBotLogin string // login the bot authenticates as; used for edit/close authorship checks

	// Repository checkouts the read tools are jailed to
	AllChatRepoPath          string
	AllChatExtensionRepoPath string

	// Grafana (Loki + Prometheus over the HTTP API)
	GrafanaURL     string
	GrafanaToken   string
	GrafanaLokiDS  string        // Loki datasource name
	GrafanaPromDS  string        // Prometheus datasource name
	GrafanaLogTail time.Duration // default look-back window for log queries

	// Kubernetes (read-only)
	KubeNamespace string // namespace kubectl reads are pinned to

	// Access control
	LeadDeveloperDiscordID string   // pinged on infra findings / proposals
	AdminDiscordIDs        []string // maintainer allow-list -> ModeAdmin

	// Agent loop tuning
	MaxIterations   int
	PerCallTimeout  time.Duration
	OverallTimeout  time.Duration
	MaxParallelTool int
}

// Load reads configuration from the environment, applying defaults. Required secrets
// are validated in main (fail-fast via log.Fatal), matching the repo convention.
func Load() *Config {
	return &Config{
		Port:     getEnv("PORT", "8094"),
		GinMode:  getEnv("GIN_MODE", "debug"),
		LogLevel: getEnv("LOG_LEVEL", "info"),

		DatabaseHost:     getEnv("DATABASE_HOST", "localhost"),
		DatabasePort:     getEnv("DATABASE_PORT", "5432"),
		DatabaseUser:     getEnv("DATABASE_USER", "allchat"),
		DatabasePassword: getEnv("DATABASE_PASSWORD", ""),
		DatabaseName:     getEnv("DATABASE_NAME", "allchat"),

		DiscordToken:      getEnv("DISCORD_BOT_TOKEN", ""),
		DiscordAppID:      getEnv("DISCORD_CLIENT_ID", ""),
		DiscordGuildID:    getEnv("DISCORD_GUILD_ID", ""),
		ModerationGuildID: getEnv("MODERATION_GUILD_ID", ""),

		LLMBaseURL:   getEnv("LOCAL_LLM_BASE_URL", "http://localhost:8000"),
		LLMModel:     getEnv("LOCAL_LLM_MODEL", ""),
		LLMAPIKey:    getEnv("LOCAL_LLM_API_KEY", ""),
		LLMMaxTokens: getEnvInt("LOCAL_LLM_MAX_TOKENS", 2048),

		GitHubToken:    getEnv("GITHUB_TOKEN", ""),
		GitHubOwner:    getEnv("GITHUB_OWNER", "caesarakalaeii"),
		GitHubBotLogin: getEnv("GITHUB_BOT_LOGIN", ""),

		AllChatRepoPath:          getEnv("ALL_CHAT_REPO_PATH", "/repos/all-chat"),
		AllChatExtensionRepoPath: getEnv("ALL_CHAT_EXTENSION_REPO_PATH", "/repos/all-chat-extension"),

		GrafanaURL:     getEnv("GRAFANA_URL", ""),
		GrafanaToken:   getEnv("GRAFANA_SERVICE_ACCOUNT_TOKEN", ""),
		GrafanaLokiDS:  getEnv("GRAFANA_LOKI_DATASOURCE", "Loki"),
		GrafanaPromDS:  getEnv("GRAFANA_PROM_DATASOURCE", "Prometheus"),
		GrafanaLogTail: getEnvDuration("GRAFANA_LOG_TAIL", 15*time.Minute),

		KubeNamespace: getEnv("KUBE_NAMESPACE", "allchat"),

		LeadDeveloperDiscordID: getEnv("LEAD_DEVELOPER_DISCORD_ID", ""),
		AdminDiscordIDs:        splitIDs(getEnv("SUPPORT_BOT_ADMIN_DISCORD_IDS", "")),

		MaxIterations:   getEnvInt("SUPPORT_BOT_MAX_ITERATIONS", 16),
		PerCallTimeout:  getEnvDuration("SUPPORT_BOT_PER_CALL_TIMEOUT", 120*time.Second),
		OverallTimeout:  getEnvDuration("SUPPORT_BOT_OVERALL_TIMEOUT", 6*time.Minute),
		MaxParallelTool: getEnvInt("SUPPORT_BOT_MAX_PARALLEL_TOOLS", 6),
	}
}

// RepoPaths returns the read-tool jail roots that are actually configured.
func (c *Config) RepoPaths() []string {
	var paths []string
	for _, p := range []string{c.AllChatRepoPath, c.AllChatExtensionRepoPath} {
		if p != "" {
			paths = append(paths, p)
		}
	}
	return paths
}

// GrafanaEnabled reports whether the Grafana tools should be registered.
func (c *Config) GrafanaEnabled() bool {
	return c.GrafanaURL != "" && c.GrafanaToken != ""
}

// splitIDs parses a comma/space/newline-separated list of Discord UIDs, trimming
// blanks. Order is irrelevant; the access policy uses a set.
func splitIDs(raw string) []string {
	fields := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == ' ' || r == '\n' || r == '\t' || r == ';'
	})
	out := make([]string, 0, len(fields))
	for _, f := range fields {
		if f = strings.TrimSpace(f); f != "" {
			out = append(out, f)
		}
	}
	return out
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getEnvInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		n := 0
		parsed := true
		for _, r := range v {
			if r < '0' || r > '9' {
				parsed = false
				break
			}
			n = n*10 + int(r-'0')
		}
		if parsed && n > 0 {
			return n
		}
	}
	return def
}

func getEnvDuration(key string, def time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			return d
		}
	}
	return def
}
