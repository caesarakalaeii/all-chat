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
// and operational questions over a locally hosted OpenAI-compatible LLM (replacing the
// former Claude-Code-CLI workaround), with two code-enforced access modes: SUPPORT
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
	if cfg.LLMModel == "" {
		log.Fatal("LOCAL_LLM_MODEL must be set")
	}

	log.Info("starting support-bot",
		zap.String("llm_base_url", cfg.LLMBaseURL),
		zap.String("llm_model", cfg.LLMModel),
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

	// Local LLM client.
	llmClient, err := llm.New(llm.Config{
		BaseURL:        cfg.LLMBaseURL,
		APIKey:         cfg.LLMAPIKey,
		Model:          cfg.LLMModel,
		RequestTimeout: 90 * time.Second,
	}, log)
	if err != nil {
		log.Fatal("failed to build LLM client", zap.Error(err))
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

	// Tool registry.
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
	})

	policy := access.NewPolicy(cfg.AdminDiscordIDs)
	redactor := redact.NewRedactor()

	agentCfg := agent.Config{
		Model:            cfg.LLMModel,
		MaxTokens:        cfg.LLMMaxTokens,
		MaxIterations:    cfg.MaxIterations,
		PerCallTimeout:   cfg.PerCallTimeout,
		MaxParallelTools: cfg.MaxParallelTool,
		Log:              log,
	}

	bot, err := discord.New(discord.Deps{
		Config:   cfg,
		Policy:   policy,
		Registry: reg,
		LLM:      llmClient,
		Memory:   memRepo,
		Redactor: redactor,
		AgentCfg: agentCfg,
		Log:      log,
	})
	if err != nil {
		log.Fatal("failed to build Discord bot", zap.Error(err))
	}
	if err := bot.Start(); err != nil {
		log.Fatal("failed to start Discord bot", zap.Error(err))
	}
	defer func() { _ = bot.Close() }()
	log.Info("Discord bot connected")

	// Health server.
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
	router.Use(gin.LoggerWithConfig(gin.LoggerConfig{SkipPaths: []string{"/health/live", "/health/ready"}}))
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
