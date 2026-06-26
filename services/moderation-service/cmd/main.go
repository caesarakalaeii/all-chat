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

// Command moderation-service exposes the chat-moderation write endpoints (ADR-0017).
// It authorizes each command (owner-only) against the shared database, performs the
// platform action (Phase 0: dry-run / reflect-back only), audits it, and publishes a
// message_deletion onto chat:raw so overlays and the dashboard update live.
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	sharedAuth "github.com/caesar/all-chat/shared/auth"
	"github.com/caesar/all-chat/shared/database"
	"github.com/caesar/all-chat/shared/encryption"
	"github.com/caesar/all-chat/shared/featuregates"
	"github.com/caesar/all-chat/shared/logger"
	"github.com/caesar/all-chat/shared/middleware"
	"github.com/caesar/all-chat/shared/quota"
	"github.com/caesar/all-chat/shared/ratelimit"

	"github.com/caesar/all-chat/services/moderation-service/audit"
	"github.com/caesar/all-chat/services/moderation-service/clients"
	"github.com/caesar/all-chat/services/moderation-service/dispatch"
	"github.com/caesar/all-chat/services/moderation-service/handler"
	"github.com/caesar/all-chat/services/moderation-service/publisher"
	"github.com/caesar/all-chat/services/moderation-service/repository"
	"github.com/caesar/all-chat/services/moderation-service/tokens"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

func main() {
	log := logger.NewLogger("moderation-service", getEnv("LOG_LEVEL", "info"))
	defer log.Sync()
	log.Info("Starting Moderation Service", zap.String("version", getEnv("APP_VERSION", "0.1.0")))

	cfg := loadConfig()

	// JWT key chain for validating user tokens forwarded by the API gateway.
	userKeyChain, err := sharedAuth.NewKeyChainFromEnv("JWT_SECRET")
	if err != nil {
		log.Fatal("JWT key chain init failed (JWT_SECRET_V1 must be set)", zap.Error(err))
	}
	log.Info("JWT key chain initialized", zap.String("latest_kid", userKeyChain.LatestKid()))

	// PostgreSQL (overlay ownership, source membership, audit log).
	connString := fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=disable",
		cfg.DatabaseUser, cfg.DatabasePassword, cfg.DatabaseHost, cfg.DatabasePort, cfg.DatabaseName,
	)
	dbPool, err := database.NewPostgresPool(connString)
	if err != nil {
		log.Fatal("Failed to connect to database", zap.Error(err))
	}
	defer dbPool.Close()
	log.Info("Connected to PostgreSQL")

	// Redis for publishing reflect-back deletion events to chat:raw.
	redisClient := redis.NewClient(&redis.Options{
		Addr: fmt.Sprintf("%s:%s", getEnv("REDIS_HOST", "localhost"), getEnv("REDIS_PORT", "6379")),
	})
	defer redisClient.Close()
	if err := redisClient.Ping(context.Background()).Err(); err != nil {
		// The ring-buffer publisher tolerates transient failures, but a persistent
		// outage means deletions won't propagate — surface it loudly.
		log.Warn("Redis ping failed; reflect-back deletions will buffer until Redis recovers", zap.Error(err))
	} else {
		log.Info("Connected to Redis")
	}

	deletionPublisher := publisher.NewDeletionPublisher(redisClient, log)
	defer deletionPublisher.Stop()

	repo := repository.New(dbPool)
	auditStore := audit.New(dbPool)

	// Feature-gate cache (ADR-0008): gates the moderation write-path to a rollout
	// cohort. Seeded premium-only (migration 061); flip the gate to is_premium=false
	// to graduate to all users with no redeploy. Refreshed via Redis Pub/Sub + 60s
	// TTL. Unknown keys default to premium-required (safe).
	gateCache := featuregates.NewFeatureGateCache(dbPool, redisClient, log)
	if err := gateCache.Start(context.Background()); err != nil {
		log.Fatal("Failed to start feature gate cache", zap.Error(err))
	}

	// Per-platform moderation is wired into a router (dispatch.Multi) and a matching
	// capability scope-checker (handler.MultiScopeChecker). A platform that is not
	// configured for this deployment is simply absent from both maps: dispatch falls
	// back to dry-run (reflect-back only, no platform API call) and capabilities report
	// it as missing_scope. This keeps local / minimal deployments working; production
	// configures the credentials and gets real platform calls.
	dispatchers := map[string]dispatch.PlatformDispatcher{}
	scopeCheckers := map[string]handler.ScopeChecker{}
	// sendCheckers parallels scopeCheckers: the same per-platform checker instances
	// also report the chat-send capability (can_send) for the monitor view's send bar.
	sendCheckers := map[string]handler.SendChecker{}

	// Twitch (delete/timeout/ban/unban) needs the token cipher (to decrypt broadcaster
	// tokens) AND the Twitch app credentials (Client-Id header + refresh grant).
	cipher, cipherErr := encryption.NewMultiKeyEncryptorFromEnv()
	switch {
	case cipherErr != nil:
		log.Warn("Token cipher unavailable (set TOKEN_ENCRYPTION_KEY_V1); Twitch moderation runs in dry-run", zap.Error(cipherErr))
	case cfg.TwitchClientID == "" || cfg.TwitchClientSecret == "":
		log.Warn("TWITCH_CLIENT_ID/TWITCH_CLIENT_SECRET not set; Twitch moderation runs in dry-run")
	default:
		twitchSource := tokens.NewTwitchSource(dbPool, cipher, cfg.TwitchClientID, cfg.TwitchClientSecret)
		twitchClient := clients.NewTwitchClient(cfg.TwitchClientID)
		dispatchers["twitch"] = dispatch.NewTwitch(twitchSource, twitchClient, log)
		twitchScopes := tokens.NewTwitchScopeChecker(twitchSource)
		scopeCheckers["twitch"] = twitchScopes
		sendCheckers["twitch"] = twitchScopes
		log.Info("Twitch moderation enabled (real Helix calls)")
	}

	// Kick (timeout/ban/unban; no single-message delete) uses the broadcaster's own
	// OAuth token, so it needs the cipher AND the Kick app credentials (refresh grant).
	switch {
	case cipherErr != nil:
		// The cipher warning already fired in the Twitch block; Kick also stays dry-run.
	case cfg.KickClientID == "" || cfg.KickClientSecret == "":
		log.Warn("KICK_CLIENT_ID/KICK_CLIENT_SECRET not set; Kick moderation runs in dry-run")
	default:
		kickSource := tokens.NewKickSource(dbPool, cipher, cfg.KickClientID, cfg.KickClientSecret)
		dispatchers["kick"] = dispatch.NewKick(kickSource, clients.NewKickClient(), log)
		kickScopes := tokens.NewKickScopeChecker(kickSource)
		scopeCheckers["kick"] = kickScopes
		sendCheckers["kick"] = kickScopes
		log.Info("Kick moderation enabled (real Kick API calls)")
	}

	// YouTube (ban-only) uses the broadcaster's own token (cipher + Google OAuth creds);
	// the liveChatId comes from the youtube-listener's Redis cache and quota is reserved
	// against the shared youtube_quota_usage table (ADR-0006). force-ssl scope-gated.
	switch {
	case cipherErr != nil:
		// The cipher warning already fired in the Twitch block; YouTube also stays dry-run.
	case cfg.YouTubeClientID == "" || cfg.YouTubeClientSecret == "":
		log.Warn("YOUTUBE_CLIENT_ID/YOUTUBE_CLIENT_SECRET not set; YouTube moderation runs in dry-run")
	default:
		ytSource := tokens.NewYouTubeSource(dbPool, cipher, cfg.YouTubeClientID, cfg.YouTubeClientSecret)
		ytQuota := quota.NewReserver(dbPool, getEnvInt("YOUTUBE_QUOTA_LIMIT_DAILY", quota.DefaultDailyLimit))
		dispatchers["youtube"] = dispatch.NewYouTube(ytSource, clients.NewYouTubeClient(), clients.NewYouTubeLiveChatResolver(redisClient), ytQuota, log)
		ytScopes := tokens.NewYouTubeScopeChecker(ytSource)
		scopeCheckers["youtube"] = ytScopes
		sendCheckers["youtube"] = ytScopes
		log.Info("YouTube moderation enabled (ban-only, real Data API calls)")
	}

	// Discord (delete/timeout/ban/unban) authenticates with the shared bot token — no
	// per-user OAuth, so no cipher or re-consent. The bot's authority is its GUILD
	// permissions (granted at invite time): capabilities report exactly what the bot can
	// do there, and a missing permission surfaces as a 403 → "re-invite the bot" rather
	// than a false reflect-back. The guild for a channel is resolved (and Redis-cached)
	// from the channel id, since member ops (ban/timeout) are guild-scoped.
	if cfg.DiscordBotToken != "" {
		discordClient := clients.NewDiscordClient(cfg.DiscordBotToken)
		discordGuilds := clients.NewDiscordGuildResolver(discordClient, redisClient)
		dispatchers["discord"] = dispatch.NewDiscord(discordClient, discordGuilds, log)
		// The resolver caches both the channel→guild map AND the per-guild effective
		// permissions, so the capability endpoint stays cheap on dashboard load.
		scopeCheckers["discord"] = handler.NewDiscordScopeChecker(discordGuilds, discordGuilds, log)
		log.Info("Discord moderation enabled (delete/timeout/ban/unban, bot REST)")
	} else {
		log.Warn("DISCORD_BOT_TOKEN not set; Discord moderation runs in dry-run")
	}

	var dispatcher handler.Dispatcher = dispatch.NewMulti(dispatchers)
	var scopeChecker handler.ScopeChecker = handler.MultiScopeChecker(scopeCheckers)

	modHandler := handler.New(repo, deletionPublisher, auditStore, scopeChecker, dispatcher, log)
	// Surface the cohort decision on the capabilities endpoint so the dashboard hides
	// controls for users outside the rollout (the action routes are gated separately).
	modHandler.SetFeatureGate(moderationGate{gates: gateCache, repo: repo})
	// Wire the chat-send capability checker so capabilities report can_send per source.
	modHandler.SetSendChecker(handler.MultiSendChecker(sendCheckers))
	// Wire the YouTube stream re-discovery publisher (owner-triggered recovery from /view).
	modHandler.SetRediscoverPublisher(publisher.NewRediscoverPublisher(redisClient, log))

	if cfg.GinMode == "release" {
		gin.SetMode(gin.ReleaseMode)
	}
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(gin.LoggerWithConfig(gin.LoggerConfig{SkipPaths: []string{"/health/live", "/health/ready"}}))

	router.GET("/health/live", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "alive"})
	})
	router.GET("/health/ready", func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()
		if err := dbPool.Ping(ctx); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "unavailable", "reason": "database"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ready"})
	})
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// Per-user rate limit + idempotency guard these powerful write endpoints.
	rl := ratelimit.NewRateLimiter(ratelimit.Config{
		RequestsPerMinute: getEnvInt("MODERATION_RATE_PER_MIN", 30),
		KeyPrefix:         "modrl",
		RedisClient:       redisClient,
		Logger:            log,
	})
	idempotencyTTL := time.Duration(getEnvInt("MODERATION_IDEMPOTENCY_TTL_SECONDS", 60)) * time.Second

	// All moderation routes require a valid user JWT (forwarded by the gateway).
	api := router.Group("/api/v1/moderation")
	api.Use(middleware.JWTAuth(userKeyChain)) // sets user_id + impersonation provenance
	api.Use(rl.Middleware())                  // per-user rate limit (keys on user_id)
	api.Use(handler.IdempotencyMiddleware(redisClient, idempotencyTTL))

	// Capabilities is ungated: owners outside the cohort still need it to learn the
	// feature is gated (it reports enabled:false) so the dashboard shows the right UX.
	api.GET("/overlays/:id/capabilities", modHandler.HandleCapabilities)

	// Stream re-discovery is a reliability recovery (not a moderation action): it is
	// owner-gated only — available to every overlay owner, not just the premium
	// moderation cohort — so it lives on the base group, outside the premium `actions`
	// group below. Ownership is verified in the handler; the per-channel cooldown lives
	// in the publisher.
	api.POST("/overlays/:id/youtube/rediscover", modHandler.HandleYouTubeRediscover)

	// The write actions are gated to the moderation rollout cohort (ADR-0008): a
	// non-cohort user gets 403 here even though capabilities already hid the controls
	// (defense in depth — never trust the client).
	actions := api.Group("")
	actions.Use(middleware.RequirePremium(dbPool, gateCache, featuregates.GateModeration, log))
	{
		actions.POST("/overlays/:id/delete", modHandler.HandleDelete)
		actions.POST("/overlays/:id/timeout", modHandler.HandleTimeout)
		actions.POST("/overlays/:id/ban", modHandler.HandleBan)
		actions.POST("/overlays/:id/unban", modHandler.HandleUnban)
	}

	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Info("Moderation Service listening", zap.String("port", cfg.Port))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("Failed to start server", zap.Error(err))
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Info("Shutting down...")

	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Error("Server forced to shutdown", zap.Error(err))
	}
	log.Info("Server exited")
}

// moderationGate combines the feature-gate cache with the per-user premium lookup to
// decide whether moderation is enabled for a user (ADR-0008). It mirrors the logic in
// shared/middleware.RequirePremium so the capabilities endpoint and the enforced
// action routes agree on cohort membership: when the gate has graduated to free
// (is_premium=false) everyone is in; otherwise only premium users are.
type moderationGate struct {
	gates *featuregates.FeatureGateCache
	repo  *repository.Repository
}

func (g moderationGate) ModerationEnabled(ctx context.Context, userID string) (bool, error) {
	if !g.gates.IsPremium(featuregates.GateModeration) {
		return true, nil
	}
	return g.repo.IsUserPremium(ctx, userID)
}

// Config holds runtime configuration.
type Config struct {
	Port                string
	GinMode             string
	DatabaseHost        string
	DatabasePort        string
	DatabaseUser        string
	DatabasePassword    string
	DatabaseName        string
	TwitchClientID      string
	TwitchClientSecret  string
	KickClientID        string
	KickClientSecret    string
	YouTubeClientID     string
	YouTubeClientSecret string
	DiscordBotToken     string
}

func loadConfig() *Config {
	return &Config{
		Port:                getEnv("PORT", "8092"),
		GinMode:             getEnv("GIN_MODE", "debug"),
		DatabaseHost:        getEnv("DATABASE_HOST", "localhost"),
		DatabasePort:        getEnv("DATABASE_PORT", "5432"),
		DatabaseUser:        getEnv("DATABASE_USER", "allchat"),
		DatabasePassword:    getEnv("DATABASE_PASSWORD", "allchat_dev_password"),
		DatabaseName:        getEnv("DATABASE_NAME", "allchat"),
		TwitchClientID:      getEnv("TWITCH_CLIENT_ID", ""),
		TwitchClientSecret:  getEnv("TWITCH_CLIENT_SECRET", ""),
		KickClientID:        getEnv("KICK_CLIENT_ID", ""),
		KickClientSecret:    getEnv("KICK_CLIENT_SECRET", ""),
		YouTubeClientID:     getEnv("YOUTUBE_CLIENT_ID", ""),
		YouTubeClientSecret: getEnv("YOUTUBE_CLIENT_SECRET", ""),
		DiscordBotToken:     getEnv("DISCORD_BOT_TOKEN", ""),
	}
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getEnvInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}
