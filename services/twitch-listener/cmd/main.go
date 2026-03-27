package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/caesar/all-chat/services/message-processor/registry"
	"github.com/caesar/all-chat/services/twitch-listener/channels"
	"github.com/caesar/all-chat/services/twitch-listener/handlers"
	"github.com/caesar/all-chat/services/twitch-listener/irc"
	"github.com/caesar/all-chat/services/twitch-listener/publisher"
	"github.com/caesar/all-chat/services/twitch-listener/status"
	"github.com/caesar/all-chat/shared/coordination"
	"github.com/caesar/all-chat/shared/database"
	"github.com/caesar/all-chat/shared/listener"
	"github.com/caesar/all-chat/shared/logger"
	"github.com/caesar/all-chat/shared/metrics"
	"github.com/caesar/all-chat/shared/sourcemanager"
	"github.com/caesar/all-chat/shared/tracing"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

func main() {
	// Initialize logger
	logLevel := listener.Env("LOG_LEVEL", "info")
	log := logger.NewLogger("twitch-listener", logLevel)
	defer log.Sync()

	// Rebuild trigger
	log.Info("Starting Twitch Listener",
		zap.String("version", listener.Env("APP_VERSION", "dev")),
	)

	// Initialize tracing
	tracingEnabled := listener.Env("OTEL_ENABLED", "false") == "true"
	if tracingEnabled {
		tracingCfg := tracing.Config{
			ServiceName:    "twitch-listener",
			ServiceVersion: listener.Env("APP_VERSION", "dev"),
			Environment:    listener.Env("ENVIRONMENT", "development"),
			OTLPEndpoint:   listener.Env("OTEL_EXPORTER_OTLP_ENDPOINT", "localhost:4317"),
			Enabled:        true,
		}
		shutdownTracer, err := tracing.InitTracer(tracingCfg, log)
		if err != nil {
			log.Error("Failed to initialize tracer", zap.Error(err))
		} else {
			defer shutdownTracer(context.Background())
			log.Info("Tracer initialized", zap.String("service", "twitch-listener"))
		}
	}

	ctx := context.Background()

	// Validate required environment variables
	twitchUsername := os.Getenv("TWITCH_BOT_USERNAME")
	twitchOAuth := os.Getenv("TWITCH_BOT_OAUTH")
	if twitchUsername == "" || twitchOAuth == "" {
		log.Fatal("TWITCH_BOT_USERNAME and TWITCH_BOT_OAUTH are required")
	}

	// Connect to PostgreSQL
	dbHost := listener.Env("DATABASE_HOST", "localhost")
	dbPort := listener.Env("DATABASE_PORT", "5432")
	dbUser := listener.Env("DATABASE_USER", "allchat")
	dbPassword := listener.Env("DATABASE_PASSWORD", "allchat_dev_password")
	dbName := listener.Env("DATABASE_NAME", "allchat")

	connString := fmt.Sprintf("postgres://%s:%s@%s:%s/%s",
		dbUser, dbPassword, dbHost, dbPort, dbName)

	db, err := database.NewPostgresPool(connString)
	if err != nil {
		log.Fatal("Failed to connect to database", zap.Error(err))
	}
	defer db.Close()

	log.Info("Connected to PostgreSQL")

	// Connect to Redis
	redisHost := listener.Env("REDIS_HOST", "localhost")
	redisPort := listener.Env("REDIS_PORT", "6379")
	redisClient := redis.NewClient(&redis.Options{
		Addr: fmt.Sprintf("%s:%s", redisHost, redisPort),
	})
	defer redisClient.Close()

	if err := redisClient.Ping(ctx).Err(); err != nil {
		log.Fatal("Failed to connect to Redis", zap.Error(err))
	}

	log.Info("Connected to Redis")

	// Initialize metrics (available via /metrics endpoint)
	listenerMetrics := metrics.NewListenerMetrics("twitch", "twitch-listener")
	shardMetrics := metrics.NewShardMetrics()
	log.Info("Initialized Prometheus metrics")

	// Get Kubernetes pod name from HOSTNAME environment variable
	podName := os.Getenv("HOSTNAME")
	if podName == "" {
		podName = "twitch-listener-unknown"
		log.Warn("HOSTNAME not set, using default pod name", zap.String("pod_name", podName))
	}

	// Initialize coordinator client
	serviceJWT := os.Getenv("SERVICE_JWT_SECRET")
	if serviceJWT == "" {
		log.Fatal("SERVICE_JWT_SECRET is required for coordinator authentication")
	}

	coordClient := coordination.NewCoordinatorClient(
		listener.Env("COORDINATOR_URL", "http://source-manager:8088"),
		serviceJWT,
		"twitch-listener",
		log,
	)
	log.Info("Initialized coordinator client",
		zap.String("coordinator_url", listener.Env("COORDINATOR_URL", "http://source-manager:8088")),
	)

	// Feature flag for coordinator filtering (allows instant rollback)
	enableFiltering := listener.Env("ENABLE_COORDINATOR_FILTERING", "false") == "true"
	cfg := listener.DefaultConfig()
	cfg.Platform = "twitch"
	cfg.DisableCoordinatorFiltering = !enableFiltering
	cfg.DisableDemandFiltering = true // Twitch IRC always connected to all assigned channels

	// Initialize ListenerBase — owns heartbeat, assignment refresh, migration subscriber, JWT refresh
	base := listener.NewListenerBase(cfg, coordClient, redisClient, podName, log)

	// Initialize IRC components
	parser := irc.NewParser()
	streamPublisher := publisher.NewStreamPublisher(redisClient, log)

	// Initialize message ID registry with 1-hour TTL (per Plan 01-01 decision)
	msgRegistry := registry.NewRedisRegistry(redisClient, 1*time.Hour)
	log.Info("Initialized message ID registry", zap.Duration("ttl", 1*time.Hour))

	ircConfig := irc.Config{
		Username: twitchUsername,
		OAuth:    twitchOAuth,
	}
	ircConn := irc.NewConnectionManager(ircConfig, parser, streamPublisher, msgRegistry, log, listenerMetrics)

	// Twitch Listener does NOT use leadership coordination
	// Twitch IRC is stateless and event-driven (push, not pull like YouTube polling)
	// Multiple IRC clients can connect to the same channels without conflicts
	// Each message gets a unique UUID, and Redis Streams consumer groups handle deduplication
	var leaderCoord *sourcemanager.LeadershipCoordinator = nil
	log.Info("Twitch Listener running without leadership coordination (IRC is stateless)")

	// Initialize status publisher for platform status indicators
	statusPublisher := status.NewPublisher(redisClient, log)
	log.Info("Initialized platform status publisher")

	// Initialize channel manager — pass nil for assignedSourceIDs; SDK calls UpdateAssignedSourceIDs inside base.Start
	channelRepo := channels.NewRepository(db)
	dbConnWrapper := &dbConnWrapper{pool: db}
	channelMgr := channels.NewManager(channelRepo, ircConn, dbConnWrapper, leaderCoord, nil, redisClient, podName, log, listenerMetrics)

	// Inject status publisher into channel manager
	channelMgr.SetStatusPublisher(statusPublisher)

	// Wire firstMessageChan from manager to IRC connection for migration coordination
	ircConn.SetFirstMessageChan(channelMgr.GetFirstMessageChan())

	// Wire status publisher and active channels callback to IRC connection for reconnect
	ircConn.SetActiveChannelsFn(channelMgr.GetActiveChannels, statusPublisher)

	// Wire disconnect callback — clears stale activeChans so next sync re-joins all channels
	ircConn.SetOnDisconnect(channelMgr.ClearActiveChannels)

	// Wire connect callback — clears activeChans again when the new client connects.
	// This handles the race where the periodic sync joins channels on the OLD disconnected
	// client during the reconnect backoff window, leaving them marked active but unjoined
	// in the new IRC session. Clearing on connect ensures a clean re-join on the fresh client.
	ircConn.SetOnConnect(channelMgr.ClearActiveChannels)

	// Connect to Twitch IRC
	if err := ircConn.Connect(ctx); err != nil {
		log.Fatal("Failed to connect to Twitch IRC", zap.Error(err))
	}

	// Wait a bit for IRC connection to establish (IRC-specific, required before Start)
	time.Sleep(2 * time.Second)

	// Start ListenerBase — handles startup jitter, initial assignment query, channelMgr.Start,
	// JWT refresh, and launches heartbeat/assignment-refresh/migration-subscriber goroutines
	if err := base.Start(ctx, channelMgr); err != nil {
		log.Fatal("Failed to start listener base", zap.Error(err))
	}

	// Record per-pod channel count metric (after filtering by coordinator assignments)
	filteredCount := channelMgr.GetFilteredAssignmentCount()
	shardMetrics.PodChannelCount.WithLabelValues(podName).Set(float64(filteredCount))
	log.Info("Recorded channel count metric",
		zap.String("pod_id", podName),
		zap.Int("channel_count", filteredCount),
	)

	// Set up HTTP server for health checks
	if logLevel == "debug" {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()
	router.Use(gin.Recovery())
	if tracingEnabled {
		router.Use(tracing.GinMiddleware("twitch-listener"))
	}

	// Health check handlers
	healthHandler := handlers.NewHealthHandler(ircConn, streamPublisher, channelMgr)
	router.GET("/health/live", healthHandler.LivenessProbe)
	router.GET("/health/ready", healthHandler.ReadinessProbe)
	router.GET("/status", healthHandler.Status)

	// Prometheus metrics endpoint
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// Get port
	port := listener.Env("PORT", "8085")

	// Create HTTP server
	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      router,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start HTTP server in goroutine
	go func() {
		log.Info("HTTP server listening",
			zap.String("port", port),
		)

		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("Failed to start HTTP server", zap.Error(err))
		}
	}()

	// Wait for interrupt signal for graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("Shutting down service...")

	// SDK-owned graceful shutdown: stops base goroutines + channelMgr + IRC + HTTP server
	listener.ShutdownCoordinator(base, channelMgr, func() { _ = ircConn.Disconnect() }, srv, log)

	log.Info("Service exited")
}

// dbConnWrapper wraps pgxpool.Pool to implement DBConnInterface
type dbConnWrapper struct {
	pool *pgxpool.Pool
}

func (w *dbConnWrapper) GetPool() interface{} {
	return w.pool
}
