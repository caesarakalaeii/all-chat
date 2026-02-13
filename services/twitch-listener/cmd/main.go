package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/caesar/all-chat/services/twitch-listener/channels"
	"github.com/caesar/all-chat/services/twitch-listener/handlers"
	"github.com/caesar/all-chat/services/twitch-listener/irc"
	"github.com/caesar/all-chat/services/twitch-listener/publisher"
	"github.com/caesar/all-chat/shared/database"
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
	logLevel := getEnvOrDefault("LOG_LEVEL", "info")
	log := logger.NewLogger("twitch-listener", logLevel)
	defer log.Sync()

	log.Info("Starting Twitch Listener",
		zap.String("version", getEnvOrDefault("APP_VERSION", "dev")),
	)

	// Initialize tracing
	tracingEnabled := getEnvOrDefault("OTEL_ENABLED", "false") == "true"
	if tracingEnabled {
		tracingCfg := tracing.Config{
			ServiceName:    "twitch-listener",
			ServiceVersion: getEnvOrDefault("APP_VERSION", "dev"),
			Environment:    getEnvOrDefault("ENVIRONMENT", "development"),
			OTLPEndpoint:   getEnvOrDefault("OTEL_EXPORTER_OTLP_ENDPOINT", "localhost:4317"),
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
	dbHost := getEnvOrDefault("DATABASE_HOST", "localhost")
	dbPort := getEnvOrDefault("DATABASE_PORT", "5432")
	dbUser := getEnvOrDefault("DATABASE_USER", "allchat")
	dbPassword := getEnvOrDefault("DATABASE_PASSWORD", "allchat_dev_password")
	dbName := getEnvOrDefault("DATABASE_NAME", "allchat")

	connString := fmt.Sprintf("postgres://%s:%s@%s:%s/%s",
		dbUser, dbPassword, dbHost, dbPort, dbName)

	db, err := database.NewPostgresPool(connString)
	if err != nil {
		log.Fatal("Failed to connect to database", zap.Error(err))
	}
	defer db.Close()

	log.Info("Connected to PostgreSQL")

	// Connect to Redis
	redisHost := getEnvOrDefault("REDIS_HOST", "localhost")
	redisPort := getEnvOrDefault("REDIS_PORT", "6379")
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
	log.Info("Initialized Prometheus metrics")

	// Initialize components
	parser := irc.NewParser()
	streamPublisher := publisher.NewStreamPublisher(redisClient, log)

	ircConfig := irc.Config{
		Username: twitchUsername,
		OAuth:    twitchOAuth,
	}
	ircConn := irc.NewConnectionManager(ircConfig, parser, streamPublisher, log, listenerMetrics)

	// Twitch Listener does NOT use leadership coordination
	// Twitch IRC is stateless and event-driven (push, not pull like YouTube polling)
	// Multiple IRC clients can connect to the same channels without conflicts
	// Each message gets a unique UUID, and Redis Streams consumer groups handle deduplication
	var leaderCoord *sourcemanager.LeadershipCoordinator = nil
	log.Info("Twitch Listener running without leadership coordination (IRC is stateless)")

	// Initialize channel manager
	channelRepo := channels.NewRepository(db)
	dbConnWrapper := &dbConnWrapper{pool: db}
	channelMgr := channels.NewManager(channelRepo, ircConn, dbConnWrapper, leaderCoord, log, listenerMetrics)

	// Connect to Twitch IRC
	if err := ircConn.Connect(ctx); err != nil {
		log.Fatal("Failed to connect to Twitch IRC", zap.Error(err))
	}

	// Wait a bit for IRC connection to establish
	time.Sleep(2 * time.Second)

	// Start channel manager (will sync and join channels)
	if err := channelMgr.Start(ctx); err != nil {
		log.Fatal("Failed to start channel manager", zap.Error(err))
	}

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
	port := getEnvOrDefault("PORT", "8085")

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

	// Stop channel manager
	channelMgr.Stop()

	// Disconnect from IRC
	if err := ircConn.Disconnect(); err != nil {
		log.Error("Error disconnecting from IRC", zap.Error(err))
	}

	// Shutdown HTTP server
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("HTTP server forced to shutdown", zap.Error(err))
	}

	log.Info("Service exited")
}

// getEnvOrDefault gets an environment variable or returns a default value
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// dbConnWrapper wraps pgxpool.Pool to implement DBConnInterface
type dbConnWrapper struct {
	pool *pgxpool.Pool
}

func (w *dbConnWrapper) GetPool() interface{} {
	return w.pool
}
