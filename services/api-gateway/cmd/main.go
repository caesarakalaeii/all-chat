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

package main

// Updated: 2025-12-19 20:57 - Viewer JWT checked first in middleware

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/caesar/all-chat/services/api-gateway/handlers"
	localmiddleware "github.com/caesar/all-chat/services/api-gateway/middleware"
	"github.com/caesar/all-chat/services/api-gateway/models"
	"github.com/caesar/all-chat/services/api-gateway/replay"
	"github.com/caesar/all-chat/services/api-gateway/subscription"
	wsconn "github.com/caesar/all-chat/services/api-gateway/websocket"
	sharedAuth "github.com/caesar/all-chat/shared/auth"
	"github.com/caesar/all-chat/shared/database"
	"github.com/caesar/all-chat/shared/logger"
	"github.com/caesar/all-chat/shared/metrics"
	sharedmiddleware "github.com/caesar/all-chat/shared/middleware"
	"github.com/caesar/all-chat/shared/ratelimit"
	sharedredis "github.com/caesar/all-chat/shared/redis"
	"github.com/caesar/all-chat/shared/tracing"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

func main() {
	// Initialize logger
	logLevel := getEnvOrDefault("LOG_LEVEL", "info")
	log := logger.NewLogger("api-gateway", logLevel)
	defer log.Sync()

	log.Info("Starting API Gateway",
		zap.String("version", getEnvOrDefault("APP_VERSION", "dev")),
	)

	// Wire the shared-middleware revocation logger (L1) so JWT blacklist-check
	// failures surface instead of being silently dropped (the old code did
	// `_ = err` despite the comment claiming it logged).
	sharedmiddleware.SetLogger(log)

	// Initialize OpenTelemetry tracing
	tracingEnabled := getEnvOrDefault("OTEL_ENABLED", "false") == "true"
	if tracingEnabled {
		tracingCfg := tracing.Config{
			ServiceName:    "api-gateway",
			ServiceVersion: getEnvOrDefault("APP_VERSION", "dev"),
			Environment:    getEnvOrDefault("ENVIRONMENT", "development"),
			OTLPEndpoint:   getEnvOrDefault("OTEL_EXPORTER_OTLP_ENDPOINT", "localhost:4317"),
			Enabled:        true,
		}

		shutdownTracer, err := tracing.InitTracer(tracingCfg, log)
		if err != nil {
			log.Error("Failed to initialize tracer (continuing without tracing)", zap.Error(err))
		} else {
			defer shutdownTracer(context.Background())
			log.Info("OpenTelemetry tracing enabled")
		}
	}

	ctx := context.Background()

	// Connect to PostgreSQL (for WebSocket overlay verification)
	dbHost := getEnvOrDefault("DATABASE_HOST", "localhost")
	dbPort := getEnvOrDefault("DATABASE_PORT", "5432")
	dbUser := getEnvOrDefault("DATABASE_USER", "allchat")
	dbPassword := getEnvOrDefault("DATABASE_PASSWORD", "")
	if dbPassword == "" {
		log.Fatal("DATABASE_PASSWORD must be set")
	}
	dbName := getEnvOrDefault("DATABASE_NAME", "allchat")

	connString := fmt.Sprintf("postgres://%s:%s@%s:%s/%s",
		dbUser, dbPassword, dbHost, dbPort, dbName)

	db, err := database.NewPostgresPool(connString)
	if err != nil {
		log.Fatal("Failed to connect to database", zap.Error(err))
	}
	defer db.Close()

	log.Info("Connected to PostgreSQL")

	// Connect to Redis (for WebSocket Pub/Sub), retrying with backoff so a
	// transient Redis outage (e.g. the pod being rescheduled onto another node)
	// does not crash-loop this service. The retry is cancelled on shutdown
	// signals so SIGTERM still terminates the process promptly while it is
	// waiting for Redis.
	redisHost := getEnvOrDefault("REDIS_HOST", "localhost")
	redisPort := getEnvOrDefault("REDIS_PORT", "6379")
	redisAddr := sharedredis.BuildDSN(redisHost, redisPort)

	startupCtx, stopStartup := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	redisClient, err := sharedredis.NewClientWithRetry(startupCtx, redisAddr, getEnvOrDefault("REDIS_PASSWORD", ""), tracingEnabled,
		sharedredis.DefaultRetryOptions(),
		func(attempt int, err error, backoff time.Duration) {
			log.Warn("Redis not reachable, retrying with backoff",
				zap.Int("attempt", attempt),
				zap.Duration("backoff", backoff),
				zap.Error(err),
			)
		})
	stopStartup()
	if err != nil {
		log.Fatal("Failed to connect to Redis", zap.Error(err))
	}
	defer redisClient.Close()

	log.Info("Connected to Redis")

	// Migrate legacy connection tracking (one-time cleanup)
	migrateLegacyConnectionTracking(ctx, redisClient, log)

	// Initialize rate limiter (configurable via env vars)
	rateLimitPerMin := getEnvAsIntOrDefault("RATE_LIMIT_PER_MINUTE", 300) // Default: 300 requests/min per IP/user
	rateLimiter := ratelimit.NewRateLimiter(ratelimit.Config{
		RequestsPerMinute: rateLimitPerMin,
		KeyPrefix:         "api_gateway",
		RedisClient:       redisClient,
		Logger:            log,
	})
	log.Info("Initialized rate limiter",
		zap.Int("requests_per_minute", rateLimitPerMin),
	)

	// Stricter rate limiter for auth-sensitive routes (M7). Prevents brute-force /
	// credential-stuffing on login, refresh, and token exchange. Applied per
	// endpoint via MiddlewareScoped (audit #14) so automatic /auth/refresh traffic
	// cannot exhaust the interactive login/exchange budget for a shared egress IP
	// (CGNAT/corporate NAT). Default raised to 20/min/IP per bucket to tolerate
	// concurrent users behind one NAT. Fails CLOSED (audit #15): during a Redis
	// outage these endpoints deny rather than silently drop the brute-force defense.
	authRateLimitPerMin := getEnvAsIntOrDefault("AUTH_RATE_LIMIT_PER_MINUTE", 20)
	authRateLimiter := ratelimit.NewRateLimiter(ratelimit.Config{
		RequestsPerMinute: authRateLimitPerMin,
		KeyPrefix:         "api_gateway:auth",
		RedisClient:       redisClient,
		Logger:            log,
		FailClosed:        getEnvAsBoolOrDefault("AUTH_RATE_LIMIT_FAIL_CLOSED", true),
	})
	log.Info("Initialized auth rate limiter",
		zap.Int("requests_per_minute", authRateLimitPerMin),
	)

	// Initialize metrics (available via /metrics endpoint)
	gatewayMetrics := metrics.NewGatewayMetrics()
	log.Info("Initialized Prometheus metrics")

	// Load service registry from environment
	registry, err := models.NewServiceRegistry()
	if err != nil {
		log.Fatal("Failed to initialize service registry", zap.Error(err))
	}

	log.Info("Service registry initialized",
		zap.Int("services", len(registry.Services)),
	)

	// Create WebSocket components
	wsManager := wsconn.NewManager(log, gatewayMetrics, redisClient, db)

	// Initialize deletion replay buffer
	replayBuffer := replay.NewRedisDeletionReplayBuffer(redisClient, 60*time.Second)
	log.Info("Initialized deletion replay buffer", zap.Duration("ttl", 60*time.Second))

	// Initialize chat replay buffer.
	// Sliding window covers brief WebSocket disconnects so reconnecting clients
	// receive messages that arrived while their socket was down. TTL matches the
	// youtube-listener demand-stop debounce so a hiccup ≤5min produces no gap.
	// MaxEntries caps memory at ~500 messages per overlay (well below Redis
	// memory pressure even with hundreds of active overlays).
	chatReplayTTL := 5 * time.Minute
	if envVal := os.Getenv("CHAT_REPLAY_TTL_SECONDS"); envVal != "" {
		if seconds, err := strconv.Atoi(envVal); err == nil && seconds > 0 {
			chatReplayTTL = time.Duration(seconds) * time.Second
		}
	}
	chatReplayMax := 500
	if envVal := os.Getenv("CHAT_REPLAY_MAX_ENTRIES"); envVal != "" {
		if n, err := strconv.Atoi(envVal); err == nil && n > 0 {
			chatReplayMax = n
		}
	}
	chatReplayBuffer := replay.NewRedisChatReplayBuffer(redisClient, chatReplayTTL, chatReplayMax)
	log.Info("Initialized chat replay buffer",
		zap.Duration("ttl", chatReplayTTL),
		zap.Int("max_entries", chatReplayMax))

	// The seeded public test-stream overlay is fed a continuous flood of
	// synthetic chat by the test-stream generator. Replaying it on reconnect has
	// no value (the data is throwaway and regenerated constantly), so skip
	// buffering it — pure Redis-write savings. Same overlay id as
	// message-processor's TEST_STREAM_OVERLAY_ID.
	testStreamOverlayID := getEnvOrDefault("TEST_STREAM_OVERLAY_ID", "00000000-0000-4000-8000-000000000a11")

	// Create WebSocket health checker for state reconciliation
	healthChecker := wsconn.NewHealthChecker(wsManager, redisClient, log, gatewayMetrics)
	healthChecker.Start()
	defer healthChecker.Stop()

	// Create Redis Pub/Sub subscriber with message handler
	messageHandler := func(overlayID string, channel string, message []byte) {
		// Determine message type based on channel
		// Main channel: overlay:{id} -> regular messages/events
		// Update channel: overlay:{id}:updates -> TikTok like aggregate updates
		msgType := models.WSMessageTypeChatMessage
		if len(channel) > 8 && channel[len(channel)-8:] == ":updates" {
			msgType = models.WSMessageTypeMessageUpdate
		}

		// Check if this is a deletion event for replay buffer
		// Parse the message to detect deletion events and to extract the stable
		// message ID used for cross-pod buffer dedup.
		var unifiedMsg struct {
			ID       string `json:"id"`
			Platform string `json:"platform"`
			Event    *struct {
				Type     string                 `json:"type"`
				Metadata map[string]interface{} `json:"metadata"`
			} `json:"event"`
		}
		if err := json.Unmarshal(message, &unifiedMsg); err == nil {
			// Record message received from Redis pub/sub
			platform := unifiedMsg.Platform
			if platform == "" {
				platform = "unknown"
			}
			gatewayMetrics.RecordMessageReceived("api-gateway", overlayID, platform)
		}

		if err := json.Unmarshal(message, &unifiedMsg); err == nil && unifiedMsg.Event != nil && unifiedMsg.Event.Type == "message_deletion" {
			// Add deletion event to replay buffer (best-effort, don't fail broadcast)
			deletionEvent := &replay.DeletionEvent{
				Platform:  unifiedMsg.Platform,
				Timestamp: time.Now().UTC(),
			}

			// Extract deletion type and target from metadata
			if delType, ok := unifiedMsg.Event.Metadata["deletion_type"].(string); ok {
				deletionEvent.DeletionType = delType
			}
			if targetUUID, ok := unifiedMsg.Event.Metadata["target_uuid"].(string); ok {
				deletionEvent.TargetUUID = targetUUID
			}
			if targetUserID, ok := unifiedMsg.Event.Metadata["target_user_id"].(string); ok {
				deletionEvent.TargetUserID = targetUserID
			}

			if err := replayBuffer.Add(context.Background(), overlayID, deletionEvent); err != nil {
				log.Error("Failed to add deletion to replay buffer",
					zap.String("overlay_id", overlayID),
					zap.Error(err),
				)
				// Continue - Pub/Sub broadcast is more critical than replay buffer
			} else {
				log.Debug("Added deletion to replay buffer",
					zap.String("overlay_id", overlayID),
					zap.String("type", deletionEvent.DeletionType),
				)
			}
		}

		// Wrap the unified message in a WebSocket message envelope
		wsMsg := models.WSMessage{
			Type:      msgType,
			Data:      json.RawMessage(message), // Use RawMessage to avoid re-parsing
			Timestamp: time.Now().UTC(),
		}

		// Convert to JSON
		wsJSON, err := wsMsg.ToJSON()
		if err != nil {
			log.Error("Failed to wrap message in WebSocket envelope",
				zap.String("overlay_id", overlayID),
				zap.Error(err),
			)
			return
		}

		// Broadcast wrapped message to all live connections in this overlay.
		count := wsManager.BroadcastToOverlay(overlayID, wsJSON)
		log.Debug("Broadcast message to overlay",
			zap.String("overlay_id", overlayID),
			zap.Int("connections", count),
		)

		// Record per-connection delivery metrics
		for i := 0; i < count; i++ {
			gatewayMetrics.RecordMessageSent("api-gateway", overlayID, "success")
		}

		// Buffer when there are no live connections on this pod, regardless of
		// whether another replica has a live connection. The pod-local Pub/Sub
		// subscription lingers for a window after the last connection drops, so
		// messages keep flowing into the buffer while the client reconnects.
		// AddOnce uses a stable per-message SETNX marker so multi-pod writes
		// converge on a single buffer entry — no cross-pod duplicates.
		if count == 0 && overlayID != testStreamOverlayID {
			added, err := chatReplayBuffer.AddOnce(context.Background(), overlayID, unifiedMsg.ID, wsJSON, wsMsg.Timestamp)
			if err != nil {
				log.Warn("Failed to add message to chat replay buffer",
					zap.String("overlay_id", overlayID),
					zap.Error(err),
				)
			}
			if added {
				gatewayMetrics.RecordMessageDropped("api-gateway", "buffered_no_clients")
			} else {
				gatewayMetrics.RecordMessageDropped("api-gateway", "buffered_dedup_skip")
			}
		}
	}

	subscriber := subscription.NewSubscriber(redisClient, log, messageHandler, gatewayMetrics)
	defer subscriber.Stop()

	// Create repository for overlay verification
	subRepo := subscription.NewRepository(db)

	// Create status subscriber for platform connection status
	statusSubscriber := subscription.NewStatusSubscriber(redisClient, wsManager, log, gatewayMetrics)
	statusSubscriber.SetSourceResolver(subRepo)
	if err := statusSubscriber.Start(ctx); err != nil {
		log.Fatal("Failed to start status subscriber", zap.Error(err))
	}
	defer statusSubscriber.Stop()

	// Build JWT KeyChains from versioned env vars
	userKeyChain, err := sharedAuth.NewKeyChainFromEnv("JWT_SECRET")
	if err != nil {
		log.Fatal("JWT key chain init failed (JWT_SECRET_V1 must be set)", zap.Error(err))
	}
	log.Info("JWT key chain initialized", zap.String("latest_kid", userKeyChain.LatestKid()))

	serviceKeyChain, err := sharedAuth.NewKeyChainFromEnv("SERVICE_JWT_SECRET")
	if err != nil {
		log.Fatal("service JWT key chain init failed (SERVICE_JWT_SECRET_V1 must be set)", zap.Error(err))
	}
	log.Info("service JWT key chain initialized", zap.String("latest_kid", serviceKeyChain.LatestKid()))

	// Get Twitch API credentials for badge fetching
	// For automatic token refresh, use TWITCH_CLIENT_SECRET (recommended)
	// Otherwise, provide TWITCH_ACCESS_TOKEN manually
	twitchClientID := strings.TrimSpace(os.Getenv("TWITCH_CLIENT_ID"))
	twitchClientSecret := strings.TrimSpace(os.Getenv("TWITCH_CLIENT_SECRET"))
	if twitchClientID == "" {
		log.Fatal("TWITCH_CLIENT_ID environment variable is required")
	}
	if twitchClientSecret == "" {
		log.Warn("TWITCH_CLIENT_SECRET not set - badge API will not auto-refresh tokens and may fail")
	}

	// Create handlers
	proxyHandler := handlers.NewProxyHandler(registry)
	healthHandler := handlers.NewHealthHandler(registry)
	badgeHandler := handlers.NewTwitchBadgeHandler(log, twitchClientID, twitchClientSecret)
	avatarProxyHandler := handlers.NewAvatarProxyHandler(redisClient, log)
	statsHandler := handlers.NewStatsHandler(redisClient)
	wsHandler := handlers.NewWebSocketHandler(wsManager, subscriber, subRepo, statusSubscriber, userKeyChain, replayBuffer, chatReplayBuffer, redisClient, log)

	// Create viewer WebSocket handler (same origin policy as owner handler)
	viewerWsHandler := handlers.NewViewerWebSocketHandler(
		wsManager,
		subscriber,
		subRepo,
		userKeyChain,
		replayBuffer,
		chatReplayBuffer,
		log,
	)

	// Set Gin mode
	if logLevel == "debug" {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	// Create router
	router := gin.New()

	// M3: restrict trusted proxies so c.ClientIP() (used by the global + auth
	// rate limiters) reflects the real edge IP, not an attacker-spoofed
	// X-Forwarded-For. Default trusts RFC1918 private ranges (the ingress/LB
	// hops in a typical k8s cluster behind ingress-nginx); override via
	// TRUSTED_PROXIES (comma-separated CIDRs/IPs). Set TRUSTED_PROXIES="" to
	// trust none (ClientIP = direct RemoteAddr).
	trustedProxies := getEnvOrDefault("TRUSTED_PROXIES", "10.0.0.0/8,172.16.0.0/12,192.168.0.0/16")
	var proxyList []string
	if trustedProxies != "" {
		for _, p := range strings.Split(trustedProxies, ",") {
			if p = strings.TrimSpace(p); p != "" {
				proxyList = append(proxyList, p)
			}
		}
	}
	if err := router.SetTrustedProxies(proxyList); err != nil {
		log.Fatal("Failed to set trusted proxies", zap.Error(err))
	}
	log.Info("Trusted proxies configured", zap.Strings("proxies", proxyList))

	// Apply global middleware
	router.Use(gin.Recovery()) // Panic recovery
	router.Use(sharedmiddleware.SecurityHeaders())
	router.Use(sharedmiddleware.BodyLimit(2 << 20)) // 2 MB max request body
	router.Use(localmiddleware.Logging(log))

	// Add tracing middleware if enabled
	if tracingEnabled {
		router.Use(tracing.GinMiddleware("api-gateway"))
	}

	// CORS middleware - but skip for WebSocket routes
	router.Use(func(c *gin.Context) {
		// Skip CORS for WebSocket routes (they handle origin checking in upgrader)
		if strings.HasPrefix(c.Request.URL.Path, "/ws/") {
			c.Next()
			return
		}
		localmiddleware.CORS()(c)
	}) // TODO: Update to CORSFromEnv() after shared module rebuild

	// Rate limiting middleware - skip for health, metrics, WebSocket, and static files
	router.Use(func(c *gin.Context) {
		path := c.Request.URL.Path
		// Skip rate limiting for:
		// - Health checks (monitoring)
		// - Metrics (monitoring)
		// - WebSocket connections (different connection model)
		// - Static files (legal pages, OBS overlays)
		if path == "/health" || path == "/metrics" ||
			strings.HasPrefix(path, "/ws/") ||
			strings.HasPrefix(path, "/legal/") ||
			path == "/obs-badge" {
			c.Next()
			return
		}
		rateLimiter.Middleware()(c)
	})

	// Health check endpoint (no auth required)
	router.GET("/health", healthHandler.CheckHealth)

	// Prometheus metrics endpoint. NOT admin-gated (audit B2): Prometheus
	// presents no JWT, so the M6 admin-JWT chain made /metrics return 401 and
	// silently broke scraping (dashboards/alerts went blind). Scrape access is
	// already restricted at the network level — the monitoring namespace is
	// allowed to reach 8080 via NetworkPolicy, and the metrics surface carries
	// no secrets.
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// Static pages (no auth required)
	router.StaticFile("/legal/terms", "./static/legal/terms.html")
	router.StaticFile("/legal/privacy", "./static/legal/privacy.html")
	router.StaticFile("/obs-badge", "./static/obs-badge.html")

	// WebSocket endpoint for overlay owners/OBS (triggers YouTube polling)
	router.GET("/ws/overlay/:overlay_id", wsHandler.HandleOverlayConnection)

	// WebSocket endpoint for viewers (does NOT trigger polling, no overlay ID exposed)
	router.GET("/ws/chat/:streamer_username", viewerWsHandler.HandleViewerChatConnection)

	// API routes - Group by authentication requirements

	// Public routes (no auth required)
	publicAPI := router.Group("/api/v1")
	{
		// Platform message stats (last 24h)
		publicAPI.GET("/stats", statsHandler.GetPlatformStats)

		// Auth service routes
		publicAPI.POST("/auth/login", authRateLimiter.MiddlewareScoped("login"), proxyHandler.ForwardRequest)
		publicAPI.GET("/auth/login", authRateLimiter.MiddlewareScoped("login"), proxyHandler.ForwardRequest)
		publicAPI.GET("/auth/callback", proxyHandler.ForwardRequest)
		publicAPI.POST("/auth/refresh", authRateLimiter.MiddlewareScoped("refresh"), localmiddleware.AuthCookieForward(), sharedmiddleware.OriginCheck(localmiddleware.LoadHTTPAllowedOrigins()), proxyHandler.ForwardRequest)

		// Streamer/admin single-use code → cookie exchange (audit H3 / C1). The
		// frontend OAuth callback POSTs the one-time code here; auth-service sets
		// the httpOnly access+refresh cookies and returns the non-secret user
		// payload. This route was MISSING, so every fresh streamer/admin login
		// 404'd at the gateway before reaching the proxy (Set-Cookie never reached
		// the browser). Rate-limited like the other auth-sensitive endpoints; no
		// cookie/origin middleware needed (no cookie yet at exchange time — the
		// client sends a JSON {code} body; the proxy passes Set-Cookie back).
		publicAPI.POST("/auth/exchange", authRateLimiter.MiddlewareScoped("exchange"), proxyHandler.ForwardRequest)

		// Platform-specific OAuth routes
		publicAPI.GET("/auth/twitch/login", proxyHandler.ForwardRequest)
		publicAPI.GET("/auth/twitch/callback", proxyHandler.ForwardRequest)
		publicAPI.GET("/auth/twitch/add-source/:overlay_id", proxyHandler.ForwardRequest)
		// Opt-in moderation re-consent (ADR-0017); auth-service applies its own JWT middleware.
		publicAPI.GET("/auth/twitch/moderation/:overlay_id", proxyHandler.ForwardRequest)
		publicAPI.GET("/auth/youtube/login", proxyHandler.ForwardRequest)
		publicAPI.GET("/auth/youtube/callback", proxyHandler.ForwardRequest)
		publicAPI.GET("/auth/youtube/add-source/:overlay_id", proxyHandler.ForwardRequest)
		// Opt-in moderation re-consent (ADR-0017); auth-service applies its own JWT middleware.
		publicAPI.GET("/auth/youtube/moderation/:overlay_id", proxyHandler.ForwardRequest)
		publicAPI.GET("/auth/kick/login", proxyHandler.ForwardRequest)
		publicAPI.GET("/auth/kick/callback", proxyHandler.ForwardRequest)
		publicAPI.GET("/auth/kick/add-source/:overlay_id", proxyHandler.ForwardRequest)
		// Opt-in moderation re-consent (ADR-0017); auth-service applies its own JWT middleware.
		publicAPI.GET("/auth/kick/moderation/:overlay_id", proxyHandler.ForwardRequest)
		publicAPI.GET("/auth/tiktok/login", proxyHandler.ForwardRequest)
		publicAPI.GET("/auth/tiktok/callback", proxyHandler.ForwardRequest)
		publicAPI.GET("/auth/tiktok/add-source/:overlay_id", proxyHandler.ForwardRequest)
		publicAPI.GET("/auth/discord/callback", proxyHandler.ForwardRequest)

		// Viewer OAuth routes (for sending messages)
		publicAPI.GET("/auth/viewer/twitch/login", proxyHandler.ForwardRequest)
		publicAPI.GET("/auth/viewer/twitch/callback", proxyHandler.ForwardRequest)
		publicAPI.POST("/auth/viewer/twitch/exchange", authRateLimiter.MiddlewareScoped("viewer_exchange"), proxyHandler.ForwardRequest)
		publicAPI.GET("/auth/viewer/youtube/login", proxyHandler.ForwardRequest)
		publicAPI.GET("/auth/viewer/youtube/callback", proxyHandler.ForwardRequest)
		publicAPI.POST("/auth/viewer/youtube/exchange", authRateLimiter.MiddlewareScoped("viewer_exchange"), proxyHandler.ForwardRequest)
		publicAPI.GET("/auth/viewer/kick/login", proxyHandler.ForwardRequest)
		publicAPI.GET("/auth/viewer/kick/callback", proxyHandler.ForwardRequest)
		publicAPI.POST("/auth/viewer/kick/exchange", authRateLimiter.MiddlewareScoped("viewer_exchange"), proxyHandler.ForwardRequest)

		// Auth code exchange (viewer trades code for JWT)
		publicAPI.POST("/auth/viewer/token/exchange", authRateLimiter.MiddlewareScoped("viewer_exchange"), proxyHandler.ForwardRequest)

		// Streamer info (public)
		publicAPI.GET("/auth/streamers/:username", proxyHandler.ForwardRequest)

		// Emote service routes (public)
		publicAPI.GET("/emotes/*path", proxyHandler.ForwardRequest)

		// Overlay config for public overlays
		publicAPI.GET("/overlays/public/:id/config", proxyHandler.ForwardRequest)
		publicAPI.GET("/overlays/public/:id/event-settings", proxyHandler.ForwardRequest)
		publicAPI.GET("/overlays/public/:id/creditroll", proxyHandler.ForwardRequest)
		publicAPI.GET("/overlays/public/:id/credit-roll", proxyHandler.ForwardRequest)

		// Phase 13: TTS streaming proxy — uses per-overlay tts_token JWT (not user JWT).
		// Auth is enforced downstream in overlay-manager via the tts_token query param.
		// NOTE (L15 defense-in-depth): this endpoint is public at the gateway level
		// (no user JWT). The downstream tts_token provides the only auth layer, and
		// that token is currently passed via URL query (see M17). Consider adding a
		// gateway-level check or moving the token to a header as a future hardening step.
		publicAPI.POST("/overlays/:id/tts", proxyHandler.ForwardRequest)

		// Viewer cosmetics catalog (public — no auth required)
		publicAPI.GET("/auth/viewer/catalog/frames", proxyHandler.ForwardRequest) // -> auth-service
		publicAPI.GET("/auth/viewer/catalog/flairs", proxyHandler.ForwardRequest) // -> auth-service

		// Public test-stream generator (proxied to message-processor). Drives fake
		// chat/votes/events onto the fixed test overlay so external tools can be
		// tested against the WebSocket feed. Gated by TEST_STREAM_ENABLED on the
		// message-processor side; only ever targets the fixed test overlay.
		publicAPI.POST("/test-stream/start", proxyHandler.ForwardRequest)
		publicAPI.POST("/test-stream/stop", proxyHandler.ForwardRequest)
		publicAPI.GET("/test-stream/status", proxyHandler.ForwardRequest)

		// Payment service (ADR-0018) — public surfaces. The Patreon webhook is
		// authenticated by its HMAC signature; the OAuth callback by a one-time
		// Redis state. Both validated inside payment-service, so no gateway JWT.
		publicAPI.POST("/webhooks/patreon", proxyHandler.ForwardRequest)
		publicAPI.GET("/payment/patreon/callback", proxyHandler.ForwardRequest)
	}

	// Twitch badge proxy endpoints (public, but not part of /api/v1 service registry)
	router.GET("/api/twitch/badges/global", badgeHandler.GetGlobalBadges)
	router.GET("/api/twitch/badges/channels/:room_id", badgeHandler.GetChannelBadges)

	// Avatar proxy endpoint (serves cached avatar images for platforms with expiring CDN URLs)
	router.GET("/api/avatars/:platform/:user_id", avatarProxyHandler.GetAvatar)

	// Protected routes (JWT auth required for streamers/admins and viewers)
	protectedAPI := router.Group("/api/v1")
	protectedAPI.Use(
		localmiddleware.CookieToBearer(),
		sharedmiddleware.JWTAuthWithRevocation(userKeyChain, redisClient),
		sharedmiddleware.OriginCheck(localmiddleware.LoadHTTPAllowedOrigins()),
	)
	{
		// Auth service - protected routes
		protectedAPI.GET("/auth/me", proxyHandler.ForwardRequest)
		protectedAPI.GET("/auth/me/data-export", proxyHandler.ForwardRequest) // DSGVO Art. 20 data portability
		protectedAPI.POST("/auth/logout", localmiddleware.AuthCookieForward(), proxyHandler.ForwardRequest)
		protectedAPI.POST("/auth/stop-impersonation", localmiddleware.AuthCookieForward(), proxyHandler.ForwardRequest)
		protectedAPI.DELETE("/auth/me", localmiddleware.AuthCookieForward(), proxyHandler.ForwardRequest)

		// Streamer chat send (monitor view sends using the streamer's own OAuth
		// tokens) -> auth-service POST /chat/send.
		protectedAPI.POST("/auth/chat/send", proxyHandler.ForwardRequest)

		// Discord guild management
		protectedAPI.GET("/auth/discord/connect", proxyHandler.ForwardRequest)
		protectedAPI.GET("/auth/guilds", proxyHandler.ForwardRequest)
		protectedAPI.GET("/auth/guilds/:guild_id/channels", proxyHandler.ForwardRequest)
		protectedAPI.DELETE("/auth/guilds/:guild_id", proxyHandler.ForwardRequest)

		// Viewer protected routes
		protectedAPI.GET("/auth/viewer/me", proxyHandler.ForwardRequest)
		// AuthCookieForward so the handler can blacklist the viewer token on logout
		// (audit #18); harmless when the viewer authenticated via the Authorization
		// bearer header (the common case — viewer tokens live in localStorage).
		protectedAPI.POST("/auth/viewer/logout", localmiddleware.AuthCookieForward(), proxyHandler.ForwardRequest)
		protectedAPI.POST("/auth/viewer/chat/send", proxyHandler.ForwardRequest)
		protectedAPI.GET("/auth/viewer/cosmetics", proxyHandler.ForwardRequest)
		protectedAPI.PATCH("/auth/viewer/cosmetics", proxyHandler.ForwardRequest)
		protectedAPI.GET("/auth/viewer/linked-platforms", proxyHandler.ForwardRequest)
		protectedAPI.DELETE("/auth/viewer/linked-platforms/:platform", proxyHandler.ForwardRequest)

		// Overlay manager routes (all protected)
		protectedAPI.GET("/overlays", proxyHandler.ForwardRequest)
		protectedAPI.POST("/overlays", proxyHandler.ForwardRequest)
		protectedAPI.GET("/overlays/:id", proxyHandler.ForwardRequest)
		protectedAPI.PUT("/overlays/:id", proxyHandler.ForwardRequest)
		protectedAPI.DELETE("/overlays/:id", proxyHandler.ForwardRequest)
		protectedAPI.POST("/overlays/:id/clone", proxyHandler.ForwardRequest)
		protectedAPI.GET("/overlays/:id/config", proxyHandler.ForwardRequest)
		protectedAPI.PUT("/overlays/:id/config", proxyHandler.ForwardRequest)
		protectedAPI.POST("/overlays/:id/config/seventv/resolve", proxyHandler.ForwardRequest)
		protectedAPI.GET("/overlays/:id/sources", proxyHandler.ForwardRequest)
		protectedAPI.POST("/overlays/:id/sources", proxyHandler.ForwardRequest)
		protectedAPI.POST("/overlays/:id/mock-messages", proxyHandler.ForwardRequest)
		protectedAPI.PUT("/overlays/:id/sources/:source_id", proxyHandler.ForwardRequest)
		protectedAPI.PATCH("/overlays/:id/sources/:source_id", proxyHandler.ForwardRequest)
		protectedAPI.DELETE("/overlays/:id/sources/:source_id", proxyHandler.ForwardRequest)
		protectedAPI.GET("/overlays/:id/event-settings", proxyHandler.ForwardRequest)
		protectedAPI.PUT("/overlays/:id/event-settings", proxyHandler.ForwardRequest)
		protectedAPI.GET("/overlays/:id/creditroll", proxyHandler.ForwardRequest)
		protectedAPI.POST("/overlays/:id/creditroll", proxyHandler.ForwardRequest)
		protectedAPI.GET("/overlays/:id/credit-roll", proxyHandler.ForwardRequest)

		// Phase 13: TTS endpoints on overlay-manager.
		// POST /overlays/:id/tts is NOT listed here — it uses per-overlay tts_token JWT
		// (not user JWT) and must be forwarded as a public endpoint (added above).
		protectedAPI.GET("/overlays/:id/tts-config", proxyHandler.ForwardRequest)
		protectedAPI.POST("/overlays/:id/tts-config", proxyHandler.ForwardRequest)
		protectedAPI.PATCH("/overlays/:id/tts-config/voice", proxyHandler.ForwardRequest)
		protectedAPI.DELETE("/overlays/:id/tts-config", proxyHandler.ForwardRequest)
		protectedAPI.POST("/overlays/:id/tts-config/rotate-token", proxyHandler.ForwardRequest)
		protectedAPI.POST("/overlays/:id/tts-config/test", proxyHandler.ForwardRequest)
		protectedAPI.GET("/overlays/:id/tts-voices", proxyHandler.ForwardRequest)
		protectedAPI.POST("/overlays/:id/tts-voices/preview", proxyHandler.ForwardRequest)

		// YouTube resolver routes (protected)
		protectedAPI.POST("/youtube/resolve", proxyHandler.ForwardRequest)
		protectedAPI.POST("/overlays/youtube/resolve", proxyHandler.ForwardRequest)

		// Internal API routes — moved to /internal group (L14): this was service-to-
		// service (called by auth-service) but incorrectly sat under protectedAPI
		// (user JWT). It is now in the internal group with service JWT auth.

		// Moderation service routes (ADR-0017) — owner-only chat moderation.
		// Ownership/source authorization is enforced again in moderation-service.
		protectedAPI.GET("/moderation/overlays/:id/capabilities", proxyHandler.ForwardRequest)
		protectedAPI.POST("/moderation/overlays/:id/delete", proxyHandler.ForwardRequest)
		protectedAPI.POST("/moderation/overlays/:id/timeout", proxyHandler.ForwardRequest)
		protectedAPI.POST("/moderation/overlays/:id/ban", proxyHandler.ForwardRequest)
		protectedAPI.POST("/moderation/overlays/:id/unban", proxyHandler.ForwardRequest)
		// Owner-triggered YouTube stream re-discovery (served by moderation-service, but
		// owner-gated only — not a premium moderation action). Recovers chat when YouTube
		// keeps reporting an ended/crashed stream as live.
		protectedAPI.POST("/moderation/overlays/:id/youtube/rediscover", proxyHandler.ForwardRequest)

		// Share service routes (all protected - require JWT auth)
		protectedAPI.GET("/users/search", proxyHandler.ForwardRequest)              // -> share-service
		protectedAPI.GET("/shares/incoming", proxyHandler.ForwardRequest)           // -> share-service
		protectedAPI.GET("/shares/accepted", proxyHandler.ForwardRequest)           // -> share-service (Phase 16)
		protectedAPI.GET("/shares/unseen-acceptances", proxyHandler.ForwardRequest) // -> share-service
		protectedAPI.POST("/shares", proxyHandler.ForwardRequest)                   // -> share-service
		protectedAPI.POST("/shares/:id/accept", proxyHandler.ForwardRequest)        // -> share-service
		protectedAPI.POST("/shares/:id/reject", proxyHandler.ForwardRequest)        // -> share-service
		protectedAPI.POST("/shares/:id/revoke", proxyHandler.ForwardRequest)        // -> share-service
		protectedAPI.POST("/shares/:id/mark-seen", proxyHandler.ForwardRequest)     // -> share-service

		// Payment service (ADR-0018) — authenticated surfaces (-> payment-service).
		protectedAPI.GET("/payment/patreon/connect", proxyHandler.ForwardRequest)
		protectedAPI.GET("/payment/status", proxyHandler.ForwardRequest)
		protectedAPI.DELETE("/payment/patreon/connection", proxyHandler.ForwardRequest)
		// Viewer premium (ADR-0019) — viewer-JWT authenticated (JWTAuth accepts both
		// user and viewer tokens; payment-service reads viewer_id).
		protectedAPI.GET("/payment/viewer/patreon/connect", proxyHandler.ForwardRequest)
		protectedAPI.GET("/payment/viewer/status", proxyHandler.ForwardRequest)
		protectedAPI.DELETE("/payment/viewer/patreon/connection", proxyHandler.ForwardRequest)
		// User-facing upcoming maintenance (-> overlay-manager)
		protectedAPI.GET("/maintenance/upcoming", proxyHandler.ForwardRequest)
	}

	// Admin routes (require JWT + admin role — defense-in-depth at gateway level)
	adminAPI := router.Group("/api/v1/admin")
	adminAPI.Use(
		localmiddleware.CookieToBearer(),
		sharedmiddleware.JWTAuthWithRevocation(userKeyChain, redisClient),
		sharedmiddleware.OriginCheck(localmiddleware.LoadHTTPAllowedOrigins()),
		sharedmiddleware.AdminOnly(),
	)
	{
		adminAPI.POST("/premium/users/:id", proxyHandler.ForwardRequest) // -> share-service

		// Beta-tester role management (-> share-service, ADR-0020)
		adminAPI.POST("/beta-tester/users/:id", proxyHandler.ForwardRequest)

		// Feature gate management (-> share-service)
		adminAPI.GET("/feature-gates", proxyHandler.ForwardRequest)
		adminAPI.PATCH("/feature-gates/:key", proxyHandler.ForwardRequest)

		// User management (-> auth-service)
		adminAPI.GET("/users", proxyHandler.ForwardRequest)
		adminAPI.GET("/users/:id", proxyHandler.ForwardRequest)
		adminAPI.POST("/users/:id/impersonate", proxyHandler.ForwardRequest)
		adminAPI.POST("/users/:id/ban", proxyHandler.ForwardRequest)
		adminAPI.POST("/users/:id/unban", proxyHandler.ForwardRequest)
		adminAPI.GET("/users/banned", proxyHandler.ForwardRequest)

		// Stats (-> auth-service)
		adminAPI.GET("/stats", proxyHandler.ForwardRequest)

		// Viewer management (-> auth-service)
		adminAPI.GET("/viewers", proxyHandler.ForwardRequest)
		adminAPI.POST("/viewers/:session_id/ban", proxyHandler.ForwardRequest)
		adminAPI.POST("/viewers/:session_id/unban", proxyHandler.ForwardRequest)
		adminAPI.POST("/viewers/:session_id/premium", proxyHandler.ForwardRequest)

		// Overlay management (-> overlay-manager)
		adminAPI.GET("/overlays", proxyHandler.ForwardRequest)
		adminAPI.GET("/overlays/active", statsHandler.GetActiveOverlays) // local: Redis scan
		adminAPI.GET("/overlays/:id/sources", proxyHandler.ForwardRequest)
		adminAPI.GET("/user-overlays/:id", proxyHandler.ForwardRequest) // overlays for a specific user
		adminAPI.GET("/sources", proxyHandler.ForwardRequest)

		// Maintenance windows (-> overlay-manager)
		adminAPI.POST("/maintenance", proxyHandler.ForwardRequest)
		adminAPI.GET("/maintenance", proxyHandler.ForwardRequest)
		adminAPI.DELETE("/maintenance/:id", proxyHandler.ForwardRequest)

		// Cosmetics catalog (-> auth-service)
		adminAPI.GET("/cosmetics/frames", proxyHandler.ForwardRequest)
		adminAPI.POST("/cosmetics/frames", proxyHandler.ForwardRequest)
		adminAPI.DELETE("/cosmetics/frames/:id", proxyHandler.ForwardRequest)
		adminAPI.GET("/cosmetics/flairs", proxyHandler.ForwardRequest)
		adminAPI.POST("/cosmetics/flairs", proxyHandler.ForwardRequest)
		adminAPI.DELETE("/cosmetics/flairs/:id", proxyHandler.ForwardRequest)
	}

	// Internal routes (service-to-service, requires service JWT)
	internal := router.Group("/internal")
	internal.Use(sharedmiddleware.ServiceJWTAuth(serviceKeyChain, "share-service", "overlay-manager", "auth-service"))
	{
		internal.POST("/ws/notify", wsHandler.NotifyUser)
		// Auto-source-activation: called by auth-service after OAuth callback to
		// add a chat source. Proxied to overlay-manager (service JWT, not user JWT).
		internal.POST("/overlays/:id/sources/auto", proxyHandler.ForwardRequest)
	}

	// Get port from environment
	port := getEnvOrDefault("PORT", "8080")

	// Create HTTP server
	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      router,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in goroutine
	go func() {
		log.Info("API Gateway listening",
			zap.String("port", port),
		)

		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("Failed to start server", zap.Error(err))
		}
	}()

	// Wait for interrupt signal for graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("Shutting down server...")

	// Give outstanding requests 25 seconds to complete
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()

	// Shutdown WebSocket manager first (cleans up Redis state)
	if err := wsManager.Shutdown(shutdownCtx); err != nil {
		log.Error("WebSocket manager shutdown error", zap.Error(err))
	}

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("Server forced to shutdown", zap.Error(err))
	}

	log.Info("Server exited")
}

// getEnvOrDefault gets an environment variable or returns a default value
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvAsIntOrDefault gets an environment variable as int or returns a default value
func getEnvAsIntOrDefault(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func getEnvAsBoolOrDefault(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if b, err := strconv.ParseBool(value); err == nil {
			return b
		}
	}
	return defaultValue
}

// migrateLegacyConnectionTracking removes the old Redis SET used for connection tracking
// The system now uses individual TTL keys (overlay:connected:{id}) exclusively
func migrateLegacyConnectionTracking(ctx context.Context, redisClient *redis.Client, log *zap.Logger) {
	legacyKey := "overlay:connected"

	// Check if legacy SET exists
	exists, err := redisClient.Exists(ctx, legacyKey).Result()
	if err != nil {
		log.Error("Failed to check legacy connection SET", zap.Error(err))
		return
	}

	if exists == 0 {
		log.Debug("No legacy connection SET found, migration not needed")
		return
	}

	// Get member count for logging
	count, err := redisClient.SCard(ctx, legacyKey).Result()
	if err != nil {
		log.Warn("Failed to get legacy SET member count", zap.Error(err))
	}

	// Delete the SET
	if err := redisClient.Del(ctx, legacyKey).Err(); err != nil {
		log.Error("Failed to delete legacy connection SET", zap.Error(err))
		return
	}

	log.Info("Removed legacy connection SET (health checker now uses TTL keys)",
		zap.Int64("members_removed", count),
	)
}
