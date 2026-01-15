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
	protobuf "google.golang.org/protobuf/proto"
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
	conn, err := grpc.NewClient(
		youtubeGRPCEndpoint,
		grpc.WithTransportCredentials(tlsCreds),
		grpc.WithPerRPCCredentials(tokenSource),
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                20 * time.Second, // Send keepalive ping every 20 seconds
			Timeout:             10 * time.Second, // Wait 10 seconds for ping ack before considering connection dead
			PermitWithoutStream: true,             // Allow pings even when no active RPCs
		}),
		// EXPERIMENT: Try different options to keep stream alive longer
		grpc.WithDefaultCallOptions(
			grpc.MaxCallRecvMsgSize(10*1024*1024), // 10MB max message size
			grpc.MaxCallSendMsgSize(10*1024*1024), // 10MB max message size
		),
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
	req := &proto.LiveChatMessageListRequest{
		LiveChatId: &liveChatID,
		Part:       []string{"id", "snippet", "authorDetails"},
		// EXPERIMENT: Removed MaxResults since proto says "Not used in the streaming RPC"
		// Maybe setting it causes YouTube to treat this as a batch request?
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
	stream, err := g.stub.StreamList(streamCtx, req)
	if err != nil {
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
	for {
		protoResp, err := stream.Recv()
		if err != nil {
			if err == io.EOF {
				endReason = "eof"
				g.logger.Info("gRPC stream ended normally",
					zap.String("live_chat_id", liveChatID),
					zap.Int("total_responses", responsesReceived),
					zap.Duration("stream_duration", time.Since(start)),
				)
				return nil
			}
			endReason = "recv_error"
			g.logger.Error("gRPC stream error",
				zap.String("live_chat_id", liveChatID),
				zap.Int("responses_before_error", responsesReceived),
				zap.Error(err),
			)
			return fmt.Errorf("gRPC stream error: %w", err)
		}

		responsesReceived++

		// Convert proto response to youtube.LiveChatMessageListResponse
		ytResponse, err := g.convertProtoToYouTube(protoResp)
		if err != nil {
			endReason = "conversion_error"
			return fmt.Errorf("failed to convert proto response: %w", err)
		}

		// DEBUG: Log each response details to understand stream closure pattern
		g.logger.Debug("Received gRPC response",
			zap.String("live_chat_id", liveChatID),
			zap.Int("response_num", responsesReceived),
			zap.Int("messages_count", len(ytResponse.Items)),
			zap.String("next_page_token", ytResponse.NextPageToken),
			zap.Bool("has_next_token", ytResponse.NextPageToken != ""),
			zap.String("offline_at", ytResponse.OfflineAt),
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
