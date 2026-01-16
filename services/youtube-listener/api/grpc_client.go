package api

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/caesar/all-chat/services/youtube-listener/api/proto"
	"github.com/caesar/all-chat/services/youtube-listener/quota"
	"go.uber.org/zap"
	"google.golang.org/api/youtube/v3"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const (
	youtubeGRPCEndpoint = "youtube.googleapis.com:443"
)

// GRPCStreamClient manages gRPC streaming connections to YouTube Live Chat API
type GRPCStreamClient struct {
	conn         *grpc.ClientConn
	stub         proto.V3DataLiveChatMessageServiceClient
	quotaTracker *quota.Tracker
	logger       *zap.Logger
	tokenSource  credentials.PerRPCCredentials
}

// NewGRPCStreamClient creates a new gRPC streaming client for YouTube Live Chat
// This uses the official gRPC API for true server-side streaming
// Accepts oauth2.TokenSource which will be wrapped for gRPC
func NewGRPCStreamClient(ctx context.Context, tokenSource credentials.PerRPCCredentials, quotaTracker *quota.Tracker, logger *zap.Logger) (*GRPCStreamClient, error) {
	// Create TLS credentials for secure connection
	tlsCreds := credentials.NewTLS(nil)

	// Establish gRPC connection to YouTube API with keepalive settings
	// These prevent the server from closing the connection due to inactivity
	// Based on research: YouTube Live Chat has "quiet periods" where no messages arrive.
	// Google's GFE closes idle connections after ~10 seconds. Solution: aggressive keepalive pings.
	// Reference: https://github.com/grpc/grpc/blob/master/doc/keepalive.md
	logger.Info("Configuring gRPC connection with debug settings",
		zap.String("endpoint", youtubeGRPCEndpoint),
		zap.Duration("keepalive_time", 5*time.Second),
		zap.Duration("keepalive_timeout", 2*time.Second),
		zap.Int("initial_window_size", 4<<20),
	)

	conn, err := grpc.NewClient(
		youtubeGRPCEndpoint,
		grpc.WithTransportCredentials(tlsCreds),
		grpc.WithPerRPCCredentials(tokenSource),
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                5 * time.Second,  // Send ping every 5s to prevent 10s idle timeout
			Timeout:             2 * time.Second,  // Wait 2s for ping ack (matching Python recommendation)
			PermitWithoutStream: true,             // Allow pings even when no active RPCs (critical for idle periods)
		}),
		// EXPERIMENT: Try different options to keep stream alive longer
		grpc.WithDefaultCallOptions(
			grpc.MaxCallRecvMsgSize(10*1024*1024), // 10MB max message size
			grpc.MaxCallSendMsgSize(10*1024*1024), // 10MB max message size
		),
		// Allow unlimited keepalive pings without data (equivalent to Python's max_pings_without_data: 0)
		// This prevents the HTTP/2 layer from blocking pings during quiet chat periods
		//
		// CRITICAL: Use 4MB window for high-volume streams (e.g., Ludwig's chat with 75+ msgs/batch)
		// Prevents flow control stalling when message processing (Redis publish) blocks receive loop
		// If receive buffer fills (no WINDOW_UPDATE sent), YouTube kills connection after ~10s
		grpc.WithInitialWindowSize(4 << 20),     // 4MB initial window (high-volume streams)
		grpc.WithInitialConnWindowSize(4 << 20), // 4MB initial connection window
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create gRPC connection: %w", err)
	}

	// Create the service client stub
	stub := proto.NewV3DataLiveChatMessageServiceClient(conn)

	logger.Info("Created gRPC streaming client",
		zap.String("endpoint", youtubeGRPCEndpoint),
	)

	return &GRPCStreamClient{
		conn:         conn,
		stub:         stub,
		quotaTracker: quotaTracker,
		logger:       logger,
		tokenSource:  tokenSource,
	}, nil
}

// Close closes the gRPC connection
func (g *GRPCStreamClient) Close() error {
	if g.conn != nil {
		return g.conn.Close()
	}
	return nil
}

// StreamChatMessagesGRPC opens a true server-streaming gRPC connection and invokes handler per response.
// This is the proper implementation of YouTube's streamList endpoint.
// Costs 5 quota units per connection.
// Uses reserve-confirm-rollback pattern for accurate quota tracking.
func (g *GRPCStreamClient) StreamChatMessagesGRPC(
	ctx context.Context,
	liveChatID string,
	pageToken string,
	audit *quota.AuditContext,
	handler func(*youtube.LiveChatMessageListResponse) error,
) error {
	const cost = quota.QuotaCostLiveChatMessages
	start := time.Now()
	endReason := ""
	responsesReceived := 0

	defer func() {
		duration := time.Since(start)
		if endReason == "" {
			endReason = "completed"
		}
		g.logger.Info("gRPC stream closed",
			zap.String("live_chat_id", liveChatID),
			zap.String("reason", endReason),
			zap.Duration("duration", duration),
			zap.Int("responses_received", responsesReceived),
			zap.Bool("used_page_token", pageToken != ""),
		)
	}()

	// STEP 1: RESERVE quota BEFORE making API call
	var reservationID string
	var err error
	if g.quotaTracker != nil {
		reservationID, err = g.quotaTracker.ReserveQuota(ctx, cost)
		if err != nil {
			endReason = "insufficient_quota"
			return fmt.Errorf("insufficient quota: %w", err)
		}
	}

	// Build the request
	// CRITICAL: Must request BOTH "snippet" AND "authorDetails"
	// Parser requires both fields (parser.go:24) - returns error if either is nil
	// If we only request "snippet", ALL messages are silently rejected → overlay shows nothing!
	req := &proto.LiveChatMessageListRequest{
		LiveChatId: &liveChatID,
		Part:       []string{"snippet", "authorDetails"}, // BOTH required for parser
		// MaxResults: nil,  // Omit - let YouTube decide batch size
	}
	if pageToken != "" {
		req.PageToken = &pageToken
	}

	// Add metadata for logging
	md := metadata.New(map[string]string{
		"x-goog-request-params": fmt.Sprintf("live_chat_id=%s", liveChatID),
	})
	streamCtx := metadata.NewOutgoingContext(ctx, md)

	// Start the gRPC stream
	g.logger.Info("Starting gRPC StreamList call",
		zap.String("live_chat_id", liveChatID),
		zap.Bool("has_page_token", pageToken != ""),
		zap.Int("page_token_length", len(pageToken)),
	)

	stream, err := g.stub.StreamList(streamCtx, req)
	if err != nil {
		g.logger.Error("Failed to start gRPC stream",
			zap.String("live_chat_id", liveChatID),
			zap.Error(err),
			zap.String("grpc_code", status.Code(err).String()),
		)

		if g.quotaTracker != nil && reservationID != "" {
			// Check if we should charge quota for this error
			if shouldChargeQuota(err) {
				if confirmErr := g.quotaTracker.ConfirmReservation(ctx, reservationID, cost); confirmErr != nil {
					g.logger.Warn("Failed to confirm quota reservation", zap.Error(confirmErr))
				}
			} else {
				if rollbackErr := g.quotaTracker.RollbackReservation(ctx, reservationID, cost); rollbackErr != nil {
					g.logger.Warn("Failed to rollback quota reservation", zap.Error(rollbackErr))
				}
			}
		}
		endReason = "stream_start_error"
		return fmt.Errorf("failed to start gRPC stream: %w", err)
	}

	// Get stream metadata/headers for debugging
	streamHeader, err := stream.Header()
	if err != nil {
		g.logger.Warn("Could not get stream header", zap.Error(err))
	} else {
		g.logger.Info("gRPC stream established successfully",
			zap.String("live_chat_id", liveChatID),
			zap.Any("stream_headers", streamHeader),
		)
	}

	// STEP 2: CONFIRM quota - we successfully started the stream
	if g.quotaTracker != nil && reservationID != "" {
		if confirmErr := g.quotaTracker.ConfirmReservation(ctx, reservationID, cost); confirmErr != nil {
			g.logger.Warn("Failed to confirm quota reservation", zap.Error(confirmErr))
		}
	}

	// Log API call for audit trail
	if g.quotaTracker != nil {
		g.quotaTracker.LogAPICall(ctx, "LiveChatMessages.StreamList.gRPC", cost, audit)
	}

	g.logger.Info("gRPC stream started",
		zap.String("live_chat_id", liveChatID),
		zap.Bool("with_page_token", pageToken != ""),
	)

	// STEP 3: Receive streaming responses
	var lastResponseTime time.Time
	var lastPageToken string
	var consecutiveEmptyResponses int
	var totalMessagesReceived int

	// Track connection state periodically
	lastConnStateCheck := time.Now()
	initialConnState := g.conn.GetState()
	g.logger.Info("Initial connection state at stream start",
		zap.String("state", initialConnState.String()),
	)

	for {
		// Periodically check connection state (every 5 responses)
		if responsesReceived > 0 && responsesReceived%5 == 0 {
			currentState := g.conn.GetState()
			if time.Since(lastConnStateCheck) > 3*time.Second {
				g.logger.Debug("Connection state check",
					zap.Int("response_num", responsesReceived),
					zap.String("state", currentState.String()),
					zap.Duration("time_in_state", time.Since(lastConnStateCheck)),
				)
				lastConnStateCheck = time.Now()
			}
		}

		recvStart := time.Now()
		protoResp, err := stream.Recv()
		recvDuration := time.Since(recvStart)

		if err != nil {
			if err == io.EOF {
				endReason = "eof"

				// Get trailer metadata which may contain server-side close reason
				trailer := stream.Trailer()

				// Check underlying connection state
				connState := g.conn.GetState()

				// CRITICAL DEBUGGING: Log detailed EOF context
				g.logger.Warn("gRPC stream EOF received - investigating cause",
					zap.String("live_chat_id", liveChatID),
					zap.Int("total_responses", responsesReceived),
					zap.Int("total_messages", totalMessagesReceived),
					zap.Duration("stream_duration", time.Since(start)),
					zap.Duration("time_since_last_response", time.Since(lastResponseTime)),
					zap.String("last_page_token", lastPageToken),
					zap.Int("consecutive_empty_responses", consecutiveEmptyResponses),
					zap.Duration("recv_duration", recvDuration),
					zap.Any("trailer_metadata", trailer),
					zap.String("connection_state", connState.String()),
					zap.String("initial_state", initialConnState.String()),
				)

				// Check if there's an error in the trailer
				if grpc_status_code := trailer.Get("grpc-status"); len(grpc_status_code) > 0 {
					g.logger.Warn("gRPC trailer contains status",
						zap.Strings("grpc-status", grpc_status_code),
					)
				}
				if grpc_message := trailer.Get("grpc-message"); len(grpc_message) > 0 {
					g.logger.Warn("gRPC trailer contains message",
						zap.Strings("grpc-message", grpc_message),
					)
				}

				return nil
			}
			endReason = "recv_error"

			// Log full error details including gRPC status
			st, ok := status.FromError(err)
			if ok {
				g.logger.Error("gRPC stream error with status",
					zap.String("live_chat_id", liveChatID),
					zap.Int("responses_before_error", responsesReceived),
					zap.String("grpc_code", st.Code().String()),
					zap.String("grpc_message", st.Message()),
					zap.Any("grpc_details", st.Details()),
					zap.Error(err),
				)
			} else {
				g.logger.Error("gRPC stream error",
					zap.String("live_chat_id", liveChatID),
					zap.Int("responses_before_error", responsesReceived),
					zap.Error(err),
				)
			}
			return fmt.Errorf("gRPC stream error: %w", err)
		}

		lastResponseTime = time.Now()

		responsesReceived++

		// Convert proto response to youtube.LiveChatMessageListResponse
		ytResponse, err := g.convertProtoToYouTube(protoResp)
		if err != nil {
			endReason = "conversion_error"
			return fmt.Errorf("failed to convert proto response: %w", err)
		}

		// Track empty responses (might signal stream end)
		messageCount := len(ytResponse.Items)
		totalMessagesReceived += messageCount

		if messageCount == 0 {
			consecutiveEmptyResponses++
		} else {
			consecutiveEmptyResponses = 0
		}

		// Track token changes
		tokenChanged := ytResponse.NextPageToken != lastPageToken
		lastPageToken = ytResponse.NextPageToken

		// Calculate response timing
		timeSinceLastResp := time.Duration(0)
		if !lastResponseTime.IsZero() {
			timeSinceLastResp = time.Since(lastResponseTime)
		}

		// DEBUG: Log each response details to understand stream closure pattern
		g.logger.Debug("Received gRPC response",
			zap.String("live_chat_id", liveChatID),
			zap.Int("response_num", responsesReceived),
			zap.Int("messages_count", messageCount),
			zap.Int("total_messages", totalMessagesReceived),
			zap.String("next_page_token", ytResponse.NextPageToken),
			zap.Bool("has_next_token", ytResponse.NextPageToken != ""),
			zap.Bool("token_changed", tokenChanged),
			zap.Int("consecutive_empty", consecutiveEmptyResponses),
			zap.String("offline_at", ytResponse.OfflineAt),
			zap.Int64("polling_interval_ms", ytResponse.PollingIntervalMillis),
			zap.Duration("time_since_start", time.Since(start)),
			zap.Duration("time_since_last_response", timeSinceLastResp),
			zap.Duration("recv_call_duration", recvDuration),
		)

		// Check if stream went offline
		if ytResponse.OfflineAt != "" {
			endReason = "stream_offline"
			g.logger.Info("Stream went offline",
				zap.String("live_chat_id", liveChatID),
				zap.String("offline_at", ytResponse.OfflineAt),
				zap.Int("responses_received", responsesReceived),
			)
			return fmt.Errorf("liveChatEnded")
		}

		// Invoke handler for this response
		if handler != nil {
			if err := handler(ytResponse); err != nil {
				endReason = "handler_error"
				return fmt.Errorf("handler error: %w", err)
			}
		}

		// Check if context was cancelled
		select {
		case <-ctx.Done():
			endReason = "context_cancelled"
			return ctx.Err()
		default:
		}
	}
}

// convertProtoToYouTube converts the protobuf response to the standard YouTube API format
// This allows the rest of our code to work without changes
func (g *GRPCStreamClient) convertProtoToYouTube(protoResp *proto.LiveChatMessageListResponse) (*youtube.LiveChatMessageListResponse, error) {
	ytResp := &youtube.LiveChatMessageListResponse{
		Kind:          protoResp.GetKind(),
		Etag:          protoResp.GetEtag(),
		OfflineAt:     protoResp.GetOfflineAt(),
		NextPageToken: protoResp.GetNextPageToken(),
	}

	// Convert page info
	if protoResp.PageInfo != nil {
		ytResp.PageInfo = &youtube.PageInfo{
			TotalResults:   int64(protoResp.PageInfo.GetTotalResults()),
			ResultsPerPage: int64(protoResp.PageInfo.GetResultsPerPage()),
		}
	}

	// Convert items (chat messages)
	ytResp.Items = make([]*youtube.LiveChatMessage, 0, len(protoResp.Items))
	for _, protoMsg := range protoResp.Items {
		ytMsg, err := g.convertProtoMessage(protoMsg)
		if err != nil {
			g.logger.Warn("Failed to convert proto message",
				zap.String("message_id", protoMsg.GetId()),
				zap.Error(err),
			)
			continue
		}
		ytResp.Items = append(ytResp.Items, ytMsg)
	}

	// Convert active poll item if present
	if protoResp.ActivePollItem != nil {
		activePoll, err := g.convertProtoMessage(protoResp.ActivePollItem)
		if err != nil {
			g.logger.Warn("Failed to convert active poll item", zap.Error(err))
		} else {
			ytResp.ActivePollItem = activePoll
		}
	}

	return ytResp, nil
}

// convertProtoMessage converts a single protobuf message to YouTube API format
func (g *GRPCStreamClient) convertProtoMessage(protoMsg *proto.LiveChatMessage) (*youtube.LiveChatMessage, error) {
	ytMsg := &youtube.LiveChatMessage{
		Kind: protoMsg.GetKind(),
		Etag: protoMsg.GetEtag(),
		Id:   protoMsg.GetId(),
	}

	// Convert snippet
	if protoMsg.Snippet != nil {
		ytMsg.Snippet = &youtube.LiveChatMessageSnippet{
			Type:              g.convertMessageType(protoMsg.Snippet.GetType()),
			LiveChatId:        protoMsg.Snippet.GetLiveChatId(),
			AuthorChannelId:   protoMsg.Snippet.GetAuthorChannelId(),
			PublishedAt:       protoMsg.Snippet.GetPublishedAt(),
			HasDisplayContent: protoMsg.Snippet.GetHasDisplayContent(),
			DisplayMessage:    protoMsg.Snippet.GetDisplayMessage(),
		}

		// Convert message details based on type (using oneof getters)
		if textDetails := protoMsg.Snippet.GetTextMessageDetails(); textDetails != nil {
			ytMsg.Snippet.TextMessageDetails = &youtube.LiveChatTextMessageDetails{
				MessageText: textDetails.GetMessageText(),
			}
		}

		if superChatDetails := protoMsg.Snippet.GetSuperChatDetails(); superChatDetails != nil {
			ytMsg.Snippet.SuperChatDetails = &youtube.LiveChatSuperChatDetails{
				AmountMicros:        superChatDetails.GetAmountMicros(),
				Currency:            superChatDetails.GetCurrency(),
				AmountDisplayString: superChatDetails.GetAmountDisplayString(),
				UserComment:         superChatDetails.GetUserComment(),
				Tier:                int64(superChatDetails.GetTier()),
			}
		}

		// Add other message detail conversions as needed...
	}

	// Convert author details
	if protoMsg.AuthorDetails != nil {
		ytMsg.AuthorDetails = &youtube.LiveChatMessageAuthorDetails{
			ChannelId:       protoMsg.AuthorDetails.GetChannelId(),
			ChannelUrl:      protoMsg.AuthorDetails.GetChannelUrl(),
			DisplayName:     protoMsg.AuthorDetails.GetDisplayName(),
			ProfileImageUrl: protoMsg.AuthorDetails.GetProfileImageUrl(),
			IsVerified:      protoMsg.AuthorDetails.GetIsVerified(),
			IsChatOwner:     protoMsg.AuthorDetails.GetIsChatOwner(),
			IsChatSponsor:   protoMsg.AuthorDetails.GetIsChatSponsor(),
			IsChatModerator: protoMsg.AuthorDetails.GetIsChatModerator(),
		}
	}

	return ytMsg, nil
}

// convertMessageType converts protobuf message type to string
func (g *GRPCStreamClient) convertMessageType(t proto.LiveChatMessageSnippet_TypeWrapper_Type) string {
	switch t {
	case proto.LiveChatMessageSnippet_TypeWrapper_TEXT_MESSAGE_EVENT:
		return "textMessageEvent"
	case proto.LiveChatMessageSnippet_TypeWrapper_TOMBSTONE:
		return "tombstone"
	case proto.LiveChatMessageSnippet_TypeWrapper_FAN_FUNDING_EVENT:
		return "fanFundingEvent"
	case proto.LiveChatMessageSnippet_TypeWrapper_CHAT_ENDED_EVENT:
		return "chatEndedEvent"
	case proto.LiveChatMessageSnippet_TypeWrapper_SPONSOR_ONLY_MODE_STARTED_EVENT:
		return "sponsorOnlyModeStartedEvent"
	case proto.LiveChatMessageSnippet_TypeWrapper_SPONSOR_ONLY_MODE_ENDED_EVENT:
		return "sponsorOnlyModeEndedEvent"
	case proto.LiveChatMessageSnippet_TypeWrapper_NEW_SPONSOR_EVENT:
		return "newSponsorEvent"
	case proto.LiveChatMessageSnippet_TypeWrapper_MEMBER_MILESTONE_CHAT_EVENT:
		return "memberMilestoneChatEvent"
	case proto.LiveChatMessageSnippet_TypeWrapper_MEMBERSHIP_GIFTING_EVENT:
		return "membershipGiftingEvent"
	case proto.LiveChatMessageSnippet_TypeWrapper_GIFT_MEMBERSHIP_RECEIVED_EVENT:
		return "giftMembershipReceivedEvent"
	case proto.LiveChatMessageSnippet_TypeWrapper_MESSAGE_DELETED_EVENT:
		return "messageDeletedEvent"
	case proto.LiveChatMessageSnippet_TypeWrapper_MESSAGE_RETRACTED_EVENT:
		return "messageRetractedEvent"
	case proto.LiveChatMessageSnippet_TypeWrapper_USER_BANNED_EVENT:
		return "userBannedEvent"
	case proto.LiveChatMessageSnippet_TypeWrapper_SUPER_CHAT_EVENT:
		return "superChatEvent"
	case proto.LiveChatMessageSnippet_TypeWrapper_SUPER_STICKER_EVENT:
		return "superStickerEvent"
	case proto.LiveChatMessageSnippet_TypeWrapper_POLL_EVENT:
		return "pollEvent"
	default:
		return "invalidType"
	}
}
