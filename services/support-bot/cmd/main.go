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

// Command support-bot is the All-Chat Discord support/admin agent. It answers codebase
// and operational questions over the Claude Code CLI (subscription OAuth), Claude via
// the Anthropic Messages API, or an OpenAI-compatible gateway, with two code-enforced
// access modes: SUPPORT
// (read-only, output redacted) for everyone and ADMIN (GitHub write: branch+PR+review)
// for an allow-list of maintainer Discord UIDs. Cluster access is read-only in both
// modes; database writes and secret reads are never exposed.
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/caesar/all-chat/services/support-bot/access"
	"github.com/caesar/all-chat/services/support-bot/agent"
	"github.com/caesar/all-chat/services/support-bot/config"
	"github.com/caesar/all-chat/services/support-bot/discord"
	"github.com/caesar/all-chat/services/support-bot/ghclient"
	"github.com/caesar/all-chat/services/support-bot/grafana"
	"github.com/caesar/all-chat/services/support-bot/handlers"
	"github.com/caesar/all-chat/services/support-bot/llm"
	"github.com/caesar/all-chat/services/support-bot/memory"
	"github.com/caesar/all-chat/services/support-bot/redact"
	"github.com/caesar/all-chat/services/support-bot/tool"
	"github.com/caesar/all-chat/services/support-bot/tools"
	"github.com/caesar/all-chat/shared/database"
	"github.com/caesar/all-chat/shared/logger"
	"github.com/caesar/all-chat/shared/middleware"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

func main() {
	cfg := config.Load()

	log := logger.NewLogger("support-bot", cfg.LogLevel)
	defer func() { _ = log.Sync() }()

	// Fail-fast on the truly required secrets.
	if cfg.DiscordToken == "" {
		log.Fatal("DISCORD_BOT_TOKEN must be set")
	}
	llmClient, model, maxTokens, llmTarget := buildLLM(cfg, log)

	log.Info("starting support-bot",
		zap.String("llm", llmTarget),
		zap.String("llm_model", model),
		zap.Int("admin_uids", len(cfg.AdminDiscordIDs)),
		zap.Bool("grafana", cfg.GrafanaEnabled()),
		zap.Strings("repos", cfg.RepoPaths()),
	)

	// Bot memory database (optional: the bot still answers without it).
	dbPool := connectDB(cfg, log)
	if dbPool != nil {
		defer dbPool.Close()
	}
	var memRepo *memory.Repository
	if dbPool != nil {
		memRepo = memory.NewRepository(dbPool)
	}

	// Optional backends.
	var gh *ghclient.Client
	if cfg.GitHubToken != "" {
		gh = ghclient.New(cfg.GitHubToken, log)
	} else {
		log.Warn("GITHUB_TOKEN not set; GitHub tools disabled")
	}
	var gf *grafana.Client
	if cfg.GrafanaEnabled() {
		gf = grafana.New(cfg.GrafanaURL, cfg.GrafanaToken, log)
	}

	reg := tool.NewRegistry()
	tools.RegisterAll(reg, tools.Deps{
		GH:             gh,
		GHOwner:        cfg.GitHubOwner,
		GHBotLogin:     cfg.GitHubBotLogin,
		BlockedRepos:   nil,
		Grafana:        gf,
		GrafanaLokiDS:  cfg.GrafanaLokiDS,
		GrafanaPromDS:  cfg.GrafanaPromDS,
		GrafanaLogTail: cfg.GrafanaLogTail,
		Memory:         memRepo,
		ScreenshotDir:  filepath.Join(cfg.AllChatRepoPath, tools.ScreenshotDir),
	})

	policy := access.NewPolicy(cfg.AdminDiscordIDs)
	redactor := redact.NewRedactor()
	agentCfg := agent.Config{
		Model:            model,
		MaxTokens:        maxTokens,
		MaxIterations:    cfg.MaxIterations,
		PerCallTimeout:   cfg.PerCallTimeout,
		MaxParallelTools: cfg.MaxParallelTool,
		BusyRetryDelay:   cfg.BusyRetryDelay,
		BusyMaxWaits:     cfg.BusyMaxWaits,
		Log:              log,
	}

	// Claude Code on the subscription: the CLI runs its own loop and calls the
	// registry over a localhost MCP endpoint scoped to each requester's mode.
	var cc *agent.ClaudeCode
	if cfg.ClaudeCodeOAuthToken != "" {
		var err error
		cc, err = agent.NewClaudeCode(agent.ClaudeCodeConfig{
			Bin:      cfg.ClaudeCodeBin,
			Token:    cfg.ClaudeCodeOAuthToken,
			Model:    cfg.ClaudeCodeModel,
			MaxTurns: cfg.MaxIterations,
			Home:     cfg.ClaudeCodeHome,
			Log:      log,
		}, reg)
		if err != nil {
			log.Fatal("failed to start the Claude Code runner", zap.Error(err))
		}
		defer func() { _ = cc.Close() }()
	}

	bot, err := discord.New(discord.Deps{
		Config:     cfg,
		Policy:     policy,
		Registry:   reg,
		LLM:        llmClient,
		ClaudeCode: cc,
		Memory:     memRepo,
		Redactor:   redactor,
		AgentCfg:   agentCfg,
		Log:        log,
	})
	if err != nil {
		log.Fatal("failed to build Discord bot", zap.Error(err))
	}
	if err := bot.Start(); err != nil {
		log.Fatal("failed to start Discord bot", zap.Error(err))
	}
	defer func() { _ = bot.Close() }()
	log.Info("Discord bot connected")

	srv := startHealthServer(cfg, dbPool, log)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Info("shutting down...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("health server forced to shutdown", zap.Error(err))
	}
	log.Info("support-bot exited")
}

// connectDB attempts to connect to Postgres for the bot memory store. A failure is
// logged as a warning (the bot runs without memory) rather than being fatal.
func connectDB(cfg *config.Config, log *zap.Logger) *pgxpool.Pool {
	if cfg.DatabasePassword == "" {
		log.Warn("DATABASE_PASSWORD not set; bot memory disabled")
		return nil
	}
	dsn := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		cfg.DatabaseUser, cfg.DatabasePassword, cfg.DatabaseHost, cfg.DatabasePort, cfg.DatabaseName)
	pool, err := database.NewPostgresPool(dsn)
	if err != nil {
		log.Warn("failed to connect to database; bot memory disabled", zap.Error(err))
		return nil
	}
	log.Info("connected to PostgreSQL (bot memory)")
	return pool
}

func startHealthServer(cfg *config.Config, dbPool *pgxpool.Pool, log *zap.Logger) *http.Server {
	if cfg.GinMode == "release" {
		gin.SetMode(gin.ReleaseMode)
	}
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(middleware.GinAccessLog())
	health := handlers.NewHealthHandler(dbPool)
	router.GET("/health/live", health.Live)
	router.GET("/health/ready", health.Ready)

	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
	go func() {
		log.Info("health server listening", zap.String("port", cfg.Port))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("health server failed", zap.Error(err))
		}
	}()
	return srv
}

// buildLLM picks the model backend. With CLAUDE_CODE_OAUTH_TOKEN set the Claude Code
// runner answers instead (built in main once the registry exists), so no client is
// built. Otherwise: Claude when ANTHROPIC_API_KEY is set, else
// the OpenAI-compatible gateway, whose model id is resolved at boot (a pin that is
// no longer served falls back to discovery). It returns the client, the model id,
// the per-reply token ceiling, and a loggable name for the target.
func buildLLM(cfg *config.Config, log *zap.Logger) (llm.ChatClient, string, int, string) {
	if cfg.ClaudeCodeOAuthToken != "" {
		return nil, cfg.ClaudeCodeModel, 0, "claude-code"
	}
	if cfg.AnthropicAPIKey != "" {
		return llm.NewAnthropic(llm.AnthropicConfig{
			APIKey:     cfg.AnthropicAPIKey,
			Model:      cfg.AnthropicModel,
			Effort:     cfg.AnthropicEffort,
			MaxRetries: 3,
		}), cfg.AnthropicModel, cfg.AnthropicMaxTokens, "anthropic"
	}

	model, err := llm.DiscoverModel(context.Background(), llm.Config{
		BaseURL: cfg.LLMBaseURL,
		APIKey:  cfg.LLMAPIKey,
		Model:   cfg.LLMModel,
	}, log)
	if err != nil {
		log.Fatal("no usable LLM model", zap.Error(err))
	}
	client, err := llm.New(llm.Config{
		BaseURL:        cfg.LLMBaseURL,
		APIKey:         cfg.LLMAPIKey,
		Model:          model,
		RequestTimeout: 90 * time.Second,
	}, log)
	if err != nil {
		log.Fatal("failed to build LLM client", zap.Error(err))
	}
	return client, model, cfg.LLMMaxTokens, cfg.LLMBaseURL
}
