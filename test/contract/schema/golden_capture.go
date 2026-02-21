package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// RawChatMessage matches the official youtube-listener output format
type RawChatMessage struct {
	MessageID string            `json:"message_id"`
	Platform  string            `json:"platform"`
	ChannelID string            `json:"channel_id"`
	StreamID  string            `json:"stream_id"`
	UserID    string            `json:"user_id"`
	Username  string            `json:"username"`
	Text      string            `json:"text"`
	Timestamp time.Time         `json:"timestamp"`
	Tags      map[string]string `json:"tags"`

	EventType string                 `json:"event_type,omitempty"`
	EventData map[string]interface{} `json:"event_data,omitempty"`
}

type captureStats struct {
	mu                sync.Mutex
	total             int
	textMessage       int
	superChat         int
	superSticker      int
	memberJoined      int
	memberMilestone   int
	other             int
}

func (s *captureStats) increment(eventType string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.total++

	switch eventType {
	case "":
		s.textMessage++
	case "super_chat":
		s.superChat++
	case "super_sticker":
		s.superSticker++
	case "member_joined":
		s.memberJoined++
	case "member_milestone":
		s.memberMilestone++
	default:
		s.other++
	}
}

func (s *captureStats) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return fmt.Sprintf("Captured %d messages (text: %d, super_chat: %d, super_sticker: %d, member_joined: %d, member_milestone: %d, other: %d)",
		s.total, s.textMessage, s.superChat, s.superSticker, s.memberJoined, s.memberMilestone, s.other)
}

func main() {
	streamURL := flag.String("stream-url", "", "YouTube live stream URL (required)")
	duration := flag.Duration("duration", 5*time.Minute, "Capture duration")
	outputDir := flag.String("output-dir", "./golden", "Output directory for golden files")
	listenerBinary := flag.String("listener-binary", "../../../services/youtube-listener/youtube-listener", "Path to official youtube-listener binary")
	redisHost := flag.String("redis-host", "localhost:6379", "Redis host:port")
	redisPassword := flag.String("redis-password", "", "Redis password (optional)")
	flag.Parse()

	if *streamURL == "" {
		fmt.Println("Error: -stream-url is required")
		flag.Usage()
		os.Exit(1)
	}

	// Initialize logger
	logger, err := zap.NewDevelopment()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create logger: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync()

	// Verify youtube-listener binary exists
	if _, err := os.Stat(*listenerBinary); os.IsNotExist(err) {
		logger.Fatal("youtube-listener binary not found",
			zap.String("path", *listenerBinary),
			zap.String("hint", "Build it first: cd services/youtube-listener && go build"))
	}

	// Create output directory
	if err := os.MkdirAll(*outputDir, 0755); err != nil {
		logger.Fatal("Failed to create output directory", zap.Error(err))
	}

	// Connect to Redis
	ctx := context.Background()
	rdb := redis.NewClient(&redis.Options{
		Addr:     *redisHost,
		Password: *redisPassword,
		DB:       0,
	})
	defer rdb.Close()

	// Test Redis connection
	if err := rdb.Ping(ctx).Err(); err != nil {
		logger.Fatal("Failed to connect to Redis",
			zap.String("host", *redisHost),
			zap.Error(err))
	}
	logger.Info("Connected to Redis", zap.String("host", *redisHost))

	// Extract stream name from URL for file naming
	streamName := extractStreamName(*streamURL)
	logger.Info("Starting capture",
		zap.String("stream", streamName),
		zap.Duration("duration", *duration),
		zap.String("output_dir", *outputDir))

	// Start official youtube-listener in background
	// Note: In production, the listener would connect to Redis automatically
	// For this tool, we just need it to publish to chat:raw stream
	cmd := exec.Command(*listenerBinary)
	cmd.Env = append(os.Environ(),
		"REDIS_HOST="+strings.Split(*redisHost, ":")[0],
		"REDIS_PORT="+strings.Split(*redisHost, ":")[1],
		fmt.Sprintf("YOUTUBE_STREAM_URL=%s", *streamURL),
	)

	if err := cmd.Start(); err != nil {
		logger.Fatal("Failed to start youtube-listener", zap.Error(err))
	}
	defer func() {
		if cmd.Process != nil {
			cmd.Process.Kill()
			cmd.Wait()
		}
	}()
	logger.Info("Started youtube-listener subprocess", zap.Int("pid", cmd.Process.Pid))

	// Wait a moment for listener to initialize
	time.Sleep(2 * time.Second)

	// Create consumer group for capturing
	groupName := "golden_capture"
	streamKey := "chat:raw"
	consumerName := fmt.Sprintf("capture-%d", time.Now().Unix())

	// Try to create consumer group (ignore BUSYGROUP error)
	_ = rdb.XGroupCreateMkStream(ctx, streamKey, groupName, "0").Err()

	// Start capturing messages
	stats := &captureStats{}
	stopChan := make(chan struct{})

	// Handle Ctrl+C
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Start capture goroutine
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		captureMessages(ctx, rdb, streamKey, groupName, consumerName, *outputDir, streamName, stats, stopChan, logger)
	}()

	// Wait for duration or interrupt
	select {
	case <-time.After(*duration):
		logger.Info("Capture duration expired")
	case <-sigChan:
		logger.Info("Received interrupt signal")
	}

	close(stopChan)
	wg.Wait()

	logger.Info("Capture complete", zap.String("stats", stats.String()))
}

func captureMessages(ctx context.Context, rdb *redis.Client, streamKey, groupName, consumerName, outputDir, streamName string, stats *captureStats, stopChan chan struct{}, logger *zap.Logger) {
	messageSequence := make(map[string]int) // Track sequence per message type

	for {
		select {
		case <-stopChan:
			return
		default:
		}

		// Read messages from stream (block for 1 second)
		streams, err := rdb.XReadGroup(ctx, &redis.XReadGroupArgs{
			Group:    groupName,
			Consumer: consumerName,
			Streams:  []string{streamKey, ">"},
			Count:    10,
			Block:    1 * time.Second,
		}).Result()

		if err != nil {
			if err == redis.Nil {
				continue // No messages
			}
			logger.Error("Failed to read from Redis stream", zap.Error(err))
			continue
		}

		for _, stream := range streams {
			for _, message := range stream.Messages {
				// Parse message data
				dataJSON, ok := message.Values["data"].(string)
				if !ok {
					logger.Warn("Message missing data field", zap.String("id", message.ID))
					continue
				}

				var rawMsg RawChatMessage
				if err := json.Unmarshal([]byte(dataJSON), &rawMsg); err != nil {
					logger.Error("Failed to parse message JSON", zap.Error(err))
					continue
				}

				// Determine message type for file naming
				msgType := "text_message"
				if rawMsg.EventType != "" {
					msgType = rawMsg.EventType
				}

				// Increment sequence counter for this type
				messageSequence[msgType]++
				seq := messageSequence[msgType]

				// Save to file: golden/{stream_name}_{message_type}_{sequence}.json
				filename := fmt.Sprintf("%s_%s_%03d.json", streamName, msgType, seq)
				filepath := filepath.Join(outputDir, filename)

				// Marshal with pretty printing
				jsonData, err := json.MarshalIndent(rawMsg, "", "  ")
				if err != nil {
					logger.Error("Failed to marshal message", zap.Error(err))
					continue
				}

				if err := os.WriteFile(filepath, jsonData, 0644); err != nil {
					logger.Error("Failed to write golden file",
						zap.String("path", filepath),
						zap.Error(err))
					continue
				}

				// Update stats
				stats.increment(rawMsg.EventType)

				logger.Debug("Captured message",
					zap.String("type", msgType),
					zap.String("file", filename),
					zap.String("username", rawMsg.Username))

				// Acknowledge message
				rdb.XAck(ctx, streamKey, groupName, message.ID)
			}
		}
	}
}

func extractStreamName(url string) string {
	// Extract video ID from URL
	// Examples:
	// - https://www.youtube.com/watch?v=VIDEO_ID
	// - https://youtu.be/VIDEO_ID

	if strings.Contains(url, "watch?v=") {
		parts := strings.Split(url, "watch?v=")
		if len(parts) == 2 {
			videoID := strings.Split(parts[1], "&")[0]
			return videoID
		}
	}

	if strings.Contains(url, "youtu.be/") {
		parts := strings.Split(url, "youtu.be/")
		if len(parts) == 2 {
			videoID := strings.Split(parts[1], "?")[0]
			return videoID
		}
	}

	// Fallback: use timestamp
	return fmt.Sprintf("stream_%d", time.Now().Unix())
}
