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

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/caesar/all-chat/services/message-processor/cache"
	"github.com/caesar/all-chat/services/message-processor/consumer"
	"github.com/caesar/all-chat/services/message-processor/dedup"
	"github.com/caesar/all-chat/services/message-processor/enricher"
	"github.com/caesar/all-chat/services/message-processor/filter"
	"github.com/caesar/all-chat/services/message-processor/handlers"
	"github.com/caesar/all-chat/services/message-processor/models"
	"github.com/caesar/all-chat/services/message-processor/normalizer"
	"github.com/caesar/all-chat/services/message-processor/publisher"
	"github.com/caesar/all-chat/services/message-processor/registry"
	"github.com/caesar/all-chat/services/message-processor/router"
	"github.com/caesar/all-chat/services/message-processor/sessions"
	"github.com/caesar/all-chat/services/message-processor/seventv"
	"github.com/caesar/all-chat/services/message-processor/testgen"
	"github.com/caesar/all-chat/shared/database"
	"github.com/caesar/all-chat/shared/logger"
	"github.com/caesar/all-chat/shared/metrics"
	"github.com/caesar/all-chat/shared/sendall"
	"github.com/caesar/all-chat/shared/tracing"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

func main() {
	// Initialize logger
	logLevel := getEnvOrDefault("LOG_LEVEL", "info")
	log := logger.NewLogger("message-processor", logLevel)
	defer log.Sync()

	log.Info("Starting Message Processor",
		zap.String("version", getEnvOrDefault("APP_VERSION", "dev")),
	)

	// Initialize OpenTelemetry tracing
	tracingEnabled := getEnvOrDefault("OTEL_ENABLED", "false") == "true"
	if tracingEnabled {
		tracingCfg := tracing.Config{
			ServiceName:    "message-processor",
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

	// Parse message age cutoff (default 60 seconds)
	messageAgeCutoffSeconds := 60
	if cutoffStr := getEnvOrDefault("MESSAGE_AGE_CUTOFF_SECONDS", "60"); cutoffStr != "" {
		if parsed, err := time.ParseDuration(cutoffStr + "s"); err == nil {
			messageAgeCutoffSeconds = int(parsed.Seconds())
		} else {
			log.Warn("Invalid MESSAGE_AGE_CUTOFF_SECONDS, using default",
				zap.String("value", cutoffStr),
				zap.Int("default", 60),
			)
		}
	}
	messageAgeCutoff := time.Duration(messageAgeCutoffSeconds) * time.Second
	log.Info("Message age cutoff configured",
		zap.Duration("cutoff", messageAgeCutoff),
	)

	ctx := context.Background()

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
	processorMetrics := metrics.NewProcessorMetrics()
	log.Info("Initialized Prometheus metrics")

	// Initialize Message ID Registry
	msgIDRegistry := registry.NewRedisRegistry(redisClient, 1*time.Hour)
	log.Info("Initialized Message ID Registry", zap.Duration("ttl", 1*time.Hour))

	// Initialize Deletion Buffer (60 second TTL per requirements)
	deletionBuffer := registry.NewRedisDeletionBuffer(redisClient, 60*time.Second)
	log.Info("Initialized Deletion Buffer", zap.Duration("ttl", 60*time.Second))

	// Initialize components
	twitchNormalizer := normalizer.NewTwitchNormalizer()
	youtubeNormalizer := normalizer.NewYouTubeNormalizer()
	tiktokNormalizer := normalizer.NewTikTokNormalizer()
	kickNormalizer := normalizer.NewKickNormalizer()
	systemNormalizer := normalizer.NewSystemNormalizer()
	discordNormalizer := normalizer.NewDiscordNormalizer()

	// Map of platform-specific normalizers
	normalizers := map[string]normalizer.Normalizer{
		"twitch":  twitchNormalizer,
		"youtube": youtubeNormalizer,
		"tiktok":  tiktokNormalizer,
		"kick":    kickNormalizer,
		"system":  systemNormalizer,
		"discord": discordNormalizer,
	}

	emoteServiceURL := getEnvOrDefault("EMOTE_SERVICE_URL", "http://localhost:8083")
	emoteClient := enricher.NewHTTPEmoteClient(emoteServiceURL, log)
	emoteCacheStore := cache.NewEmoteCache(redisClient, log, 0)
	emoteEnricher := enricher.NewEnricher(emoteClient, emoteCacheStore, log)
	emoteEnricher.SetMetrics(processorMetrics)

	// Initialize cheermote enricher (Twitch bits visual emotes)
	cheermoteClient := enricher.NewHTTPCheermoteClient(emoteServiceURL, log)
	cheermoteEnricher := enricher.NewCheermoteEnricher(cheermoteClient, redisClient, log)

	// Initialize 7TV event manager for real-time emote updates
	seventvManager := seventv.NewManager(emoteCacheStore, log)
	if err := seventvManager.Start(ctx); err != nil {
		log.Warn("Failed to start 7TV event manager, continuing without real-time updates",
			zap.Error(err),
		)
	} else {
		log.Info("7TV event manager started successfully")
	}
	defer seventvManager.Stop()

	// Avatar enricher for Twitch and TikTok users
	twitchClientID := getEnvOrDefault("TWITCH_CLIENT_ID", "")
	twitchClientSecret := getEnvOrDefault("TWITCH_CLIENT_SECRET", "")
	gatewayBaseURL := getEnvOrDefault("GATEWAY_BASE_URL", "http://api-gateway:8080")
	avatarEnricher := enricher.NewAvatarEnricher(redisClient, twitchClientID, twitchClientSecret, gatewayBaseURL, log)
	badgeEnricher := enricher.NewBadgeEnricher(redisClient, twitchClientID, twitchClientSecret, log)
	viewerBadgeEnricher := enricher.NewViewerBadgeEnricher(redisClient, db, log)
	log.Info("Initialized ViewerBadge enricher")

	pronounEnricher := enricher.NewPronounEnricher(redisClient, log)
	log.Info("Initialized Pronoun enricher")

	overlayRepo := router.NewRepository(db)
	overlayRouter := router.NewRouter(overlayRepo, log)

	pubsubPublisher := publisher.NewPubSubPublisher(redisClient, log, processorMetrics)

	// Create deduplicator to prevent duplicate message publishing
	deduplicator := dedup.NewDeduplicator(redisClient, log)

	// Create event filter to check if event types are enabled per overlay
	eventFilter := filter.NewEventFilter(db, log)

	// Public test-stream generator: drives fake chat/events onto a single fixed
	// overlay (migration 058) so external tools can be tested against the
	// WebSocket feed without any real platform. Targeting one fixed overlay
	// bounds the blast radius of the unauthenticated trigger endpoint.
	testStreamOverlayID := getEnvOrDefault("TEST_STREAM_OVERLAY_ID", "00000000-0000-4000-8000-000000000a11")
	testStreamEnabled := getEnvOrDefault("TEST_STREAM_ENABLED", "true") == "true"
	publicWSBaseURL := getEnvOrDefault("PUBLIC_WS_BASE_URL", "")
	testStreamGenerator := testgen.NewGenerator(testStreamOverlayID, pubsubPublisher, emoteEnricher, cheermoteEnricher, log)

	// Auto-start the generator while a WebSocket client is connected to the test
	// overlay (reuses the api-gateway connection-presence signal), so external
	// tools just connect and see traffic — no manual trigger needed. The HTTP
	// endpoints below remain for explicit/dev control (e.g. no gateway present).
	testStreamWatcherCtx, testStreamWatcherCancel := context.WithCancel(context.Background())
	defer testStreamWatcherCancel()
	if testStreamEnabled && getEnvOrDefault("TEST_STREAM_AUTOSTART", "true") == "true" {
		watcher := testgen.NewDemandWatcher(testStreamOverlayID, redisClient, testStreamGenerator, testgen.ConfigFromEnv(), log)
		go watcher.Run(testStreamWatcherCtx)
	}

	// Create event capture for credit roll sessions
	eventCapture := sessions.NewEventCapture(redisClient, log)

	// Define message handler
	messageHandler := func(ctx context.Context, rawMsg *models.RawChatMessage) error {
		// Filter out old messages based on timestamp
		messageAge := time.Since(rawMsg.Timestamp)
		if messageAge > messageAgeCutoff {
			log.Debug("Ignoring old message",
				zap.String("message_id", rawMsg.MessageID),
				zap.String("platform", rawMsg.Platform),
				zap.String("channel_id", rawMsg.ChannelID),
				zap.Duration("message_age", messageAge),
				zap.Duration("cutoff", messageAgeCutoff),
				zap.Time("timestamp", rawMsg.Timestamp),
			)
			processorMetrics.RecordMessageProcessed("message-processor", rawMsg.Platform, "filtered_old", "success")
			return nil
		}

		// Note: Deduplication moved to per-overlay loop (Phase 15-03)
		// This prevents duplicate messages from overlapping sources (e.g., Twitch Shared Chat)
		// while allowing same message to be delivered to different overlays

		// Find target overlays for this message
		var overlays []models.OverlayTarget
		if rawMsg.OverlayID != "" {
			overlays = []models.OverlayTarget{
				{OverlayID: rawMsg.OverlayID},
			}
		} else {
			routed, err := overlayRouter.Route(ctx, rawMsg.Platform, rawMsg.ChannelID)
			if err != nil {
				return fmt.Errorf("failed to route message: %w", err)
			}
			overlays = routed
		}

		if len(overlays) == 0 {
			log.Debug("No overlays found for message, skipping",
				zap.String("platform", rawMsg.Platform),
				zap.String("channel", rawMsg.ChannelID),
			)
			return nil
		}

		// Get the appropriate normalizer for this platform
		platformNormalizer, ok := normalizers[rawMsg.Platform]
		if !ok {
			log.Warn("Unknown platform, skipping message",
				zap.String("platform", rawMsg.Platform),
				zap.String("message_id", rawMsg.MessageID),
			)
			return nil
		}

		// Track channel for 7TV real-time emote updates (fire-and-forget)
		go func() {
			if err := seventvManager.TrackChannel(context.Background(), rawMsg.Platform, rawMsg.ChannelID); err != nil {
				log.Debug("Failed to track channel for 7TV updates",
					zap.String("platform", rawMsg.Platform),
					zap.String("channel_id", rawMsg.ChannelID),
					zap.Error(err),
				)
			}
		}()

		// Track user for 7TV real-time user emote updates (fire-and-forget)
		if rawMsg.UserID != "" {
			go func() {
				if err := seventvManager.TrackUser(context.Background(), rawMsg.Platform, rawMsg.UserID); err != nil {
					log.Debug("Failed to track user for 7TV updates",
						zap.String("platform", rawMsg.Platform),
						zap.String("user_id", rawMsg.UserID),
						zap.Error(err),
					)
				}
			}()
		}

		// Send-to-all detection (once per raw message): if this is a streamer's own
		// message echoed back after a "send to all", auth-service pre-registered it so
		// the N platform echoes collapse into one combined-pill message. Only chat
		// messages participate — events never fan out this way.
		var sendAllGroup *sendall.Registration
		if rawMsg.EventType == "" || rawMsg.EventType == "chat" {
			if reg, lookupErr := deduplicator.LookupSendAllGroup(ctx, rawMsg.Platform, rawMsg.UserID, rawMsg.Text); lookupErr != nil {
				log.Debug("send-to-all lookup failed, treating as ordinary message", zap.Error(lookupErr))
			} else {
				sendAllGroup = reg
			}
		}

		// Process message for each overlay.
		// publishedToAnyOverlay tracks whether the message was successfully
		// delivered to at least one overlay so the stats counter is only
		// incremented once per unique message, not once per overlay.
		publishedToAnyOverlay := false
		for _, overlay := range overlays {
			var unified *models.UnifiedChatMessage
			var err error
			isEvent := rawMsg.EventType != "" && rawMsg.EventType != "chat"

			if isEvent {
				// DELETION EVENT PATH: Handle message deletions specially
				if rawMsg.EventType == "message_deletion" {
					// Normalize deletion event (platform-agnostic)
					startNormalize := time.Now()
					unified = normalizer.NormalizeDeletion(rawMsg)
					unified.OverlayID = overlay.OverlayID // Set overlay ID for this target
					processorMetrics.RecordMessageProcessed("message-processor", rawMsg.Platform, "normalized_deletion", "success")
					processorMetrics.StageDuration.WithLabelValues("message-processor", rawMsg.Platform, "deletion_normalization").Observe(time.Since(startNormalize).Seconds())

					// Deletions don't need enrichment - go straight to publishing
					goto publish
				}

				// EVENT PATH: Check if event type is enabled for this overlay
				enabled, filterErr := eventFilter.IsEventEnabled(ctx, overlay.OverlayID, rawMsg.Platform, rawMsg.EventType)
				if filterErr != nil {
					log.Warn("Failed to check event filter, allowing event",
						zap.String("overlay_id", overlay.OverlayID),
						zap.String("event_type", rawMsg.EventType),
						zap.Error(filterErr),
					)
					// Fail open - allow event if filter check fails
					enabled = true
				}

				if !enabled {
					log.Debug("Event type disabled for overlay, skipping",
						zap.String("overlay_id", overlay.OverlayID),
						zap.String("platform", rawMsg.Platform),
						zap.String("event_type", rawMsg.EventType),
					)
					processorMetrics.RecordMessageProcessed("message-processor", rawMsg.Platform, "filtered_event", "skipped")
					continue
				}

				// Normalize event using platform-specific event normalizer
				startNormalize := time.Now()

				// Type assert to get NormalizeEvent method
				switch platformNormalizer.(type) {
				case *normalizer.TwitchNormalizer:
					unified, err = platformNormalizer.(*normalizer.TwitchNormalizer).NormalizeEvent(rawMsg, overlay.OverlayID)
				case *normalizer.YouTubeNormalizer:
					unified, err = platformNormalizer.(*normalizer.YouTubeNormalizer).NormalizeEvent(rawMsg, overlay.OverlayID)
				case *normalizer.TikTokNormalizer:
					unified, err = platformNormalizer.(*normalizer.TikTokNormalizer).NormalizeEvent(rawMsg, overlay.OverlayID)
				default:
					log.Warn("Platform normalizer does not support events, skipping",
						zap.String("platform", rawMsg.Platform),
						zap.String("event_type", rawMsg.EventType),
					)
					continue
				}

				if err != nil {
					log.Warn("Failed to normalize event",
						zap.String("message_id", rawMsg.MessageID),
						zap.String("platform", rawMsg.Platform),
						zap.String("event_type", rawMsg.EventType),
						zap.Error(err),
					)
					processorMetrics.RecordMessageProcessed("message-processor", rawMsg.Platform, "normalized_event", "failed")
					continue
				}
				processorMetrics.RecordMessageProcessed("message-processor", rawMsg.Platform, "normalized_event", "success")
				processorMetrics.StageDuration.WithLabelValues("message-processor", rawMsg.Platform, "event_normalization").Observe(time.Since(startNormalize).Seconds())

				// Enrich events with user identity (avatars, badges)
				// Some events have user-defined text (Super Chat, resubs, channel points with input)
				// which may contain emotes, so we enrich those conditionally

				// Enrich with avatars (Twitch only, cached in Redis)
				startAvatar := time.Now()
				if err := avatarEnricher.Enrich(ctx, unified); err != nil {
					log.Warn("Failed to enrich avatar for event",
						zap.String("message_id", rawMsg.MessageID),
						zap.String("event_type", rawMsg.EventType),
						zap.Error(err),
					)
					// Continue even if enrichment fails
				}
				processorMetrics.StageDuration.WithLabelValues("message-processor", rawMsg.Platform, "event_avatar_enrichment").Observe(time.Since(startAvatar).Seconds())

				// Enrich with badge icons (Twitch only, cached in Redis)
				startBadge := time.Now()
				if err := badgeEnricher.Enrich(ctx, unified); err != nil {
					log.Warn("Failed to enrich badges for event",
						zap.String("message_id", rawMsg.MessageID),
						zap.String("event_type", rawMsg.EventType),
						zap.Error(err),
					)
					// Continue even if enrichment fails
				}
				processorMetrics.StageDuration.WithLabelValues("message-processor", rawMsg.Platform, "event_badge_enrichment").Observe(time.Since(startBadge).Seconds())

				// Conditionally enrich with emotes if event has user-defined text
				// Examples: Super Chat messages, resub messages, channel point redemptions with user input
				// Skip for system-only events: raids, follows, subscriptions without messages
				if unified.Message.Text != "" {
					// For non-Twitch events, look up sibling Twitch source for 7TV channel emotes
					if unified.Platform != "twitch" {
						if twitchCh, err := overlayRouter.TwitchChannelForOverlay(ctx, overlay.OverlayID); err == nil && twitchCh != "" {
							if unified.Metadata == nil {
								unified.Metadata = make(map[string]interface{})
							}
							unified.Metadata["twitch_channel_hint"] = twitchCh
						}
					}
					// Per-overlay 7TV emote-set override applies to all platforms.
					if setID, err := overlayRouter.SevenTVEmoteSetIDForOverlay(ctx, overlay.OverlayID); err == nil && setID != "" {
						if unified.Metadata == nil {
							unified.Metadata = make(map[string]interface{})
						}
						unified.Metadata["seventv_emote_set_id"] = setID
					}

					startEmote := time.Now()
					if err := emoteEnricher.Enrich(ctx, unified); err != nil {
						log.Warn("Failed to enrich emotes for event",
							zap.String("message_id", rawMsg.MessageID),
							zap.String("event_type", rawMsg.EventType),
							zap.Error(err),
						)
						// Continue even if enrichment fails
					}
					processorMetrics.StageDuration.WithLabelValues("message-processor", rawMsg.Platform, "event_emote_enrichment").Observe(time.Since(startEmote).Seconds())

					// Enrich with cheermotes for Twitch events (if message contains bits)
					if unified.Platform == "twitch" {
						startCheer := time.Now()
						if err := cheermoteEnricher.Enrich(ctx, unified); err != nil {
							log.Warn("Failed to enrich cheermotes for event",
								zap.String("message_id", rawMsg.MessageID),
								zap.String("event_type", rawMsg.EventType),
								zap.Error(err),
							)
							// Continue even if cheermote enrichment fails
						}
						processorMetrics.StageDuration.WithLabelValues("message-processor", rawMsg.Platform, "event_cheermote_enrichment").Observe(time.Since(startCheer).Seconds())
					}
				}

				// Enrich with viewer identity (name_color from All-Chat account, all platforms)
				startViewer := time.Now()
				if err := viewerBadgeEnricher.Enrich(ctx, unified); err != nil {
					log.Warn("Failed to enrich viewer identity for event",
						zap.String("message_id", rawMsg.MessageID),
						zap.Error(err),
					)
					// Continue even if enrichment fails
				}
				processorMetrics.StageDuration.WithLabelValues("message-processor", rawMsg.Platform, "viewer_identity_enrichment").Observe(time.Since(startViewer).Seconds())

			} else {
				// CHAT PATH (existing logic)
				// Normalize message using platform-specific normalizer
				startNormalize := time.Now()
				unified, err = platformNormalizer.Normalize(rawMsg, overlay.OverlayID)
				if err != nil {
					log.Warn("Failed to normalize message",
						zap.String("message_id", rawMsg.MessageID),
						zap.String("platform", rawMsg.Platform),
						zap.Error(err),
					)
					processorMetrics.RecordMessageProcessed("message-processor", rawMsg.Platform, "normalized", "failed")
					continue
				}
				processorMetrics.RecordMessageProcessed("message-processor", rawMsg.Platform, "normalized", "success")
				processorMetrics.StageDuration.WithLabelValues("message-processor", rawMsg.Platform, "normalization").Observe(time.Since(startNormalize).Seconds())

				// Enrich with avatars (Twitch only, cached in Redis)
				startAvatar := time.Now()
				if err := avatarEnricher.Enrich(ctx, unified); err != nil {
					log.Warn("Failed to enrich avatar",
						zap.String("message_id", rawMsg.MessageID),
						zap.Error(err),
					)
					// Continue even if enrichment fails
				}
				processorMetrics.StageDuration.WithLabelValues("message-processor", rawMsg.Platform, "avatar_enrichment").Observe(time.Since(startAvatar).Seconds())

				// Enrich with badge icons (Twitch only, cached in Redis)
				startBadge := time.Now()
				if err := badgeEnricher.Enrich(ctx, unified); err != nil {
					log.Warn("Failed to enrich badges",
						zap.String("message_id", rawMsg.MessageID),
						zap.Error(err),
					)
					// Continue even if enrichment fails
				}
				processorMetrics.StageDuration.WithLabelValues("message-processor", rawMsg.Platform, "badge_enrichment").Observe(time.Since(startBadge).Seconds())

				// For non-Twitch messages, look up sibling Twitch source on the same overlay
				// to enable 7TV channel emote enrichment via Twitch channel hint
				if unified.Platform != "twitch" {
					if twitchCh, err := overlayRouter.TwitchChannelForOverlay(ctx, overlay.OverlayID); err == nil && twitchCh != "" {
						if unified.Metadata == nil {
							unified.Metadata = make(map[string]interface{})
						}
						unified.Metadata["twitch_channel_hint"] = twitchCh
					}
				}
				// Per-overlay 7TV emote-set override applies to all platforms.
				if setID, err := overlayRouter.SevenTVEmoteSetIDForOverlay(ctx, overlay.OverlayID); err == nil && setID != "" {
					if unified.Metadata == nil {
						unified.Metadata = make(map[string]interface{})
					}
					unified.Metadata["seventv_emote_set_id"] = setID
				}

				// Enrich with emotes
				startEmote := time.Now()
				if err := emoteEnricher.Enrich(ctx, unified); err != nil {
					log.Warn("Failed to enrich message",
						zap.String("message_id", rawMsg.MessageID),
						zap.Error(err),
					)
					processorMetrics.RecordMessageProcessed("message-processor", rawMsg.Platform, "enriched", "failed")
					// Continue even if enrichment fails
				} else {
					processorMetrics.RecordMessageProcessed("message-processor", rawMsg.Platform, "enriched", "success")
				}
				processorMetrics.StageDuration.WithLabelValues("message-processor", rawMsg.Platform, "emote_enrichment").Observe(time.Since(startEmote).Seconds())

				// Enrich with cheermotes (Twitch only)
				if unified.Platform == "twitch" {
					startCheer := time.Now()
					if err := cheermoteEnricher.Enrich(ctx, unified); err != nil {
						log.Warn("Failed to enrich cheermotes",
							zap.String("message_id", rawMsg.MessageID),
							zap.Error(err),
						)
						// Continue even if cheermote enrichment fails
					}
					processorMetrics.StageDuration.WithLabelValues("message-processor", rawMsg.Platform, "cheermote_enrichment").Observe(time.Since(startCheer).Seconds())
				}

				// Enrich with viewer identity (name_color from All-Chat account, all platforms)
				startViewer := time.Now()
				if err := viewerBadgeEnricher.Enrich(ctx, unified); err != nil {
					log.Warn("Failed to enrich viewer identity",
						zap.String("message_id", rawMsg.MessageID),
						zap.Error(err),
					)
					// Continue even if enrichment fails
				}
				processorMetrics.StageDuration.WithLabelValues("message-processor", rawMsg.Platform, "viewer_identity_enrichment").Observe(time.Since(startViewer).Seconds())

				// Phase 9: Enrich with pronouns (after viewer identity for TwitchUsername availability)
				startPronoun := time.Now()
				if err := pronounEnricher.Enrich(ctx, unified); err != nil {
					log.Warn("Failed to enrich pronouns",
						zap.String("message_id", rawMsg.MessageID),
						zap.Error(err),
					)
					// Continue even if enrichment fails — D-05
				}
				processorMetrics.StageDuration.WithLabelValues("message-processor", rawMsg.Platform, "pronoun_enrichment").Observe(time.Since(startPronoun).Seconds())
			}

		publish:
			// Streamer send-to-all: collapse the per-platform echoes into one combined
			// message. Reuse the shared group id (so all echoes share an id), attach the
			// full platform set for the combined pill, and publish at most once per
			// overlay — the first echo to arrive wins, later siblings are dropped here.
			if sendAllGroup != nil {
				unified.ID = sendAllGroup.GroupID
				unified.Platforms = sendAllGroup.Platforms
				if len(sendAllGroup.Platforms) > 0 {
					unified.Platform = sendAllGroup.Platforms[0]
				}
				if won, claimErr := deduplicator.ClaimSendAllOverlay(ctx, overlay.OverlayID, sendAllGroup.GroupID); claimErr == nil && !won {
					log.Debug("send-to-all echo already published to overlay, skipping",
						zap.String("overlay_id", overlay.OverlayID),
						zap.String("group_id", sendAllGroup.GroupID),
						zap.String("platform", rawMsg.Platform),
					)
					processorMetrics.RecordMessageProcessed("message-processor", rawMsg.Platform, "sendall_deduplicated", "skipped")
					continue
				}
			}

			// Check for duplicate messages per overlay (Phase 15-03: overlay-specific deduplication)
			// Prevents duplicates from overlapping sources (e.g., Twitch Shared Chat)
			// Extract platform message ID from tags for better fingerprinting
			platformMsgID := rawMsg.Tags["id"]
			if platformMsgID == "" {
				platformMsgID = rawMsg.MessageID
			}

			isDup, dedupErr := deduplicator.IsDuplicateForOverlay(
				ctx,
				overlay.OverlayID,
				rawMsg.Platform,
				rawMsg.ChannelID,
				platformMsgID,
				rawMsg.UserID,
				rawMsg.Text,
				rawMsg.Timestamp,
			)

			if dedupErr != nil {
				log.Warn("Deduplication check failed, processing message anyway",
					zap.Error(dedupErr),
					zap.String("message_id", rawMsg.MessageID),
					zap.String("overlay_id", overlay.OverlayID),
				)
				// Continue processing on error (fail open)
			} else if isDup {
				log.Debug("Duplicate message detected for overlay, skipping",
					zap.String("overlay_id", overlay.OverlayID),
					zap.String("platform", rawMsg.Platform),
					zap.String("channel", rawMsg.ChannelID),
					zap.String("message_id", platformMsgID),
				)
				processorMetrics.RecordMessageProcessed("message-processor", rawMsg.Platform, "deduplicated", "skipped")
				continue // Skip publishing to this overlay
			}

			// Capture event for credit roll if applicable
			if unified.Event != nil {
				if err := eventCapture.CaptureIfActive(ctx, unified); err != nil {
					log.Warn("Failed to capture event for credit roll",
						zap.String("overlay_id", overlay.OverlayID),
						zap.String("event_type", unified.Event.Type),
						zap.Error(err),
					)
					// Continue - event capture failure shouldn't break message delivery
				}
			}

			// Publish to overlay channel
			startPublish := time.Now()
			if err := pubsubPublisher.Publish(ctx, overlay.OverlayID, unified); err != nil {
				log.Error("Failed to publish to overlay",
					zap.String("overlay_id", overlay.OverlayID),
					zap.String("message_id", rawMsg.MessageID),
					zap.Error(err),
				)
				processorMetrics.RecordMessagePublished("message-processor", overlay.OverlayID, rawMsg.Platform, "failed")
				continue
			}
			processorMetrics.RecordMessagePublished("message-processor", overlay.OverlayID, rawMsg.Platform, "success")
			processorMetrics.FanoutDuration.WithLabelValues("message-processor").Observe(time.Since(startPublish).Seconds())
			publishedToAnyOverlay = true
		}

		// Increment daily platform message counter once per unique message.
		// Placed after the overlay loop so multi-overlay fanout counts as one message.
		// Uses daily buckets (chat:stats:daily:{platform}:{YYYY-MM-DD}) so the
		// API can sum the last 7 days for a stable rolling window.
		if publishedToAnyOverlay {
			day := time.Now().UTC().Format("2006-01-02")
			statsKey := "chat:stats:daily:" + rawMsg.Platform + ":" + day
			if redisClient.Incr(ctx, statsKey).Err() == nil {
				// Each daily bucket expires after 8 days (7d window + 1d grace).
				redisClient.ExpireNX(ctx, statsKey, 8*24*time.Hour)
			}
		}

		return nil
	}

	// MP-01: Resolve unique consumer name from hostname for per-pod identification
	hostname, err := os.Hostname()
	if err != nil {
		hostname = fmt.Sprintf("processor-unknown-%d", time.Now().UnixNano())
		log.Warn("Failed to get hostname, using fallback consumer name",
			zap.String("consumer_name", hostname),
			zap.Error(err),
		)
	}
	log.Info("Consumer name resolved", zap.String("consumer_name", hostname))

	// Create and start stream consumer
	streamConsumer := consumer.NewStreamConsumer(redisClient, log, processorMetrics, messageHandler, msgIDRegistry, deletionBuffer, hostname)
	// Collapse the IRC↔EventSub Twitch handoff overlap (and Twitch webhook retries) by the native
	// message id before enrichment, so viewers never see doubled chat (ADR-0015).
	streamConsumer.SetNativeDeduplicator(deduplicator)
	if err := streamConsumer.Start(ctx); err != nil {
		log.Fatal("Failed to start stream consumer", zap.Error(err))
	}

	log.Info("Message processor started")

	// Set up HTTP server for health checks
	if logLevel == "debug" {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(gin.Logger())

	// Add tracing middleware if enabled
	if tracingEnabled {
		router.Use(tracing.GinMiddleware("message-processor"))
	}

	// Health check endpoints
	router.GET("/health/live", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "alive"})
	})

	router.GET("/health/ready", func(c *gin.Context) {
		// Check Redis connection
		if err := redisClient.Ping(ctx).Err(); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"status": "not_ready",
				"error":  "redis connection failed",
			})
			return
		}

		// Check database connection
		if err := db.Ping(ctx); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"status": "not_ready",
				"error":  "database connection failed",
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{"status": "ready"})
	})

	router.GET("/status", func(c *gin.Context) {
		pendingCount, _ := streamConsumer.GetPendingCount(ctx)

		c.JSON(http.StatusOK, gin.H{
			"status": "ok",
			"consumer": gin.H{
				"pending_messages": pendingCount,
			},
		})
	})

	// DQ-03: Admin DLQ replay endpoint
	router.POST("/admin/dlq/replay", handlers.HandleDLQReplay(redisClient, log))

	// Prometheus metrics endpoint
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	mockAPIKey := getEnvOrDefault("MOCK_MESSAGE_API_KEY", "")
	router.POST("/internal/mock-messages", func(c *gin.Context) {
		if mockAPIKey == "" {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "mock messaging disabled"})
			return
		}

		if token := c.GetHeader("X-Internal-Token"); token == "" || token != mockAPIKey {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		var req mockMessageRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		normalizeMockRequest(&req)
		msg := buildMockMessage(&req)

		// Check if event is enabled for this overlay (if this is an event message)
		if msg.Event != nil && msg.Event.Type != "" {
			enabled, filterErr := eventFilter.IsEventEnabled(ctx, req.OverlayID, msg.Platform, msg.Event.Type)
			if filterErr != nil {
				log.Warn("Failed to check event filter for mock message, allowing event",
					zap.String("overlay_id", req.OverlayID),
					zap.String("event_type", msg.Event.Type),
					zap.Error(filterErr),
				)
				// Fail open - allow event if filter check fails
				enabled = true
			}

			if !enabled {
				log.Debug("Event type disabled for overlay, skipping mock event",
					zap.String("overlay_id", req.OverlayID),
					zap.String("platform", msg.Platform),
					zap.String("event_type", msg.Event.Type),
				)
				c.JSON(http.StatusOK, gin.H{
					"status":  "filtered",
					"message": "event type disabled for this overlay",
				})
				return
			}
		}

		log.Info("Enriching mock message",
			zap.String("channel_id", msg.ChannelID),
			zap.String("platform", msg.Platform),
			zap.String("text", msg.Message.Text),
		)
		if err := emoteEnricher.Enrich(ctx, msg); err != nil {
			log.Warn("Failed to enrich mock message", zap.Error(err))
		}
		log.Info("Mock message enriched",
			zap.Int("emote_count", len(msg.Message.Emotes)),
		)

		// Enrich cheermotes for Twitch mock messages
		if msg.Platform == "twitch" {
			if err := cheermoteEnricher.Enrich(ctx, msg); err != nil {
				log.Warn("Failed to enrich mock message cheermotes", zap.Error(err))
			}
		}

		if err := pubsubPublisher.Publish(ctx, req.OverlayID, msg); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to publish mock message"})
			return
		}

		c.JSON(http.StatusAccepted, gin.H{
			"status":     "queued",
			"message_id": msg.ID,
		})
	})

	// Public, unauthenticated test-stream control endpoints. They only ever
	// target the fixed test overlay, so there is no overlay_id to spoof.
	// wsURL builds the connectable WebSocket URL for the test overlay.
	wsURL := func() string {
		if publicWSBaseURL == "" {
			return fmt.Sprintf("/ws/overlay/%s", testStreamOverlayID)
		}
		return fmt.Sprintf("%s/ws/overlay/%s", strings.TrimRight(publicWSBaseURL, "/"), testStreamOverlayID)
	}

	requireTestStream := func(c *gin.Context) bool {
		if !testStreamEnabled {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "test stream disabled"})
			return false
		}
		return true
	}

	router.POST("/public/test-stream/start", func(c *gin.Context) {
		if !requireTestStream(c) {
			return
		}
		var cfg testgen.Config
		// Empty/invalid body is fine — the generator falls back to sensible defaults.
		_ = c.ShouldBindJSON(&cfg)

		status, err := testStreamGenerator.Start(cfg)
		if err != nil {
			c.JSON(http.StatusConflict, gin.H{
				"error":  err.Error(),
				"status": status,
				"ws_url": wsURL(),
			})
			return
		}
		c.JSON(http.StatusAccepted, gin.H{
			"status": status,
			"ws_url": wsURL(),
		})
	})

	router.POST("/public/test-stream/stop", func(c *gin.Context) {
		if !requireTestStream(c) {
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": testStreamGenerator.Stop()})
	})

	router.GET("/public/test-stream/status", func(c *gin.Context) {
		if !requireTestStream(c) {
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"status": testStreamGenerator.Status(),
			"ws_url": wsURL(),
		})
	})

	// Get port
	port := getEnvOrDefault("PORT", "8087")

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

	// Stop 7TV event manager
	if err := seventvManager.Stop(); err != nil {
		log.Error("Failed to stop 7TV event manager", zap.Error(err))
	}

	// Stop stream consumer
	streamConsumer.Stop()

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

type mockMessageRequest struct {
	OverlayID   string                 `json:"overlay_id" binding:"required"`
	Platform    string                 `json:"platform"`
	ChannelID   string                 `json:"channel_id"`
	ChannelName string                 `json:"channel_name"`
	UserID      string                 `json:"user_id"`
	Username    string                 `json:"username"`
	DisplayName string                 `json:"display_name"`
	AvatarURL   string                 `json:"avatar_url"`
	Color       string                 `json:"color"`
	Badges      []models.Badge         `json:"badges"`
	Event       *models.EventInfo      `json:"event"`
	Text        string                 `json:"text" binding:"required"`
	Metadata    map[string]interface{} `json:"metadata"`
}

func normalizeMockRequest(req *mockMessageRequest) {
	req.Platform = strings.ToLower(strings.TrimSpace(req.Platform))
	if req.Platform == "" {
		req.Platform = "twitch"
	}

	req.ChannelID = strings.TrimSpace(req.ChannelID)
	req.ChannelName = strings.TrimSpace(req.ChannelName)
	if req.ChannelID == "" {
		req.ChannelID = req.ChannelName
	}
	if req.ChannelID == "" {
		req.ChannelID = "mock-channel"
	}
	if req.ChannelName == "" {
		req.ChannelName = req.ChannelID
	}

	req.Username = strings.TrimSpace(req.Username)
	if req.Username == "" {
		req.Username = "mockuser"
	}
	req.DisplayName = strings.TrimSpace(req.DisplayName)
	if req.DisplayName == "" {
		req.DisplayName = req.Username
	}
	req.UserID = strings.TrimSpace(req.UserID)
	if req.UserID == "" {
		req.UserID = "mock-user"
	}

	if req.Metadata == nil {
		req.Metadata = map[string]interface{}{}
	}
}

func buildMockMessage(req *mockMessageRequest) *models.UnifiedChatMessage {
	metadata := map[string]interface{}{
		"mock": true,
	}
	for k, v := range req.Metadata {
		metadata[k] = v
	}

	return &models.UnifiedChatMessage{
		ID:        fmt.Sprintf("mock-%d", time.Now().UnixNano()),
		OverlayID: req.OverlayID,
		Platform:  req.Platform,
		ChannelID: req.ChannelID,
		ChannelName: func() string {
			if req.ChannelName != "" {
				return req.ChannelName
			}
			return req.ChannelID
		}(),
		User: models.UserInfo{
			ID:          req.UserID,
			Username:    req.Username,
			DisplayName: req.DisplayName,
			AvatarURL:   req.AvatarURL,
			Badges:      req.Badges,
			Color:       req.Color,
		},
		Message: models.MessageInfo{
			Text: req.Text,
		},
		Event:     req.Event,
		Timestamp: time.Now().UTC(),
		Metadata:  metadata,
	}
}
