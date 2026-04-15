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
	"io"
	"log"
	"os"
	"time"

	"github.com/caesar/all-chat/services/youtube-listener/api/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	protobuf "google.golang.org/protobuf/proto"
)

// apiKeyCredentials implements credentials.PerRPCCredentials for API key authentication
type apiKeyCredentials struct {
	apiKey string
}

func (c *apiKeyCredentials) GetRequestMetadata(ctx context.Context, uri ...string) (map[string]string, error) {
	return map[string]string{
		"x-goog-api-key": c.apiKey,
	}, nil
}

func (c *apiKeyCredentials) RequireTransportSecurity() bool {
	return true
}

func main() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: go run main.go <API_KEY> <LIVE_CHAT_ID>")
		fmt.Println()
		fmt.Println("To find a live chat ID:")
		fmt.Println("1. Find a live stream on YouTube (e.g., big streamer)")
		fmt.Println("2. Get the video ID from URL (e.g., dQw4w9WgXcQ)")
		fmt.Println("3. Use videos.list API to get liveStreamingDetails.activeLiveChatId")
		fmt.Println()
		fmt.Println("Example popular streams to try:")
		fmt.Println("- Gaming: Ninja, xQc, Shroud")
		fmt.Println("- News: CNN, BBC, Al Jazeera live streams")
		fmt.Println("- Music: Lofi Girl 24/7 streams")
		os.Exit(1)
	}

	apiKey := os.Args[1]
	liveChatID := os.Args[2]

	log.Printf("Testing YouTube gRPC streamList with API key authentication")
	log.Printf("Live Chat ID: %s", liveChatID)
	log.Printf("API Key: %s...", apiKey[:8])

	// Create TLS credentials
	tlsCreds := credentials.NewTLS(nil)

	// Create API key credentials
	apiKeyCreds := &apiKeyCredentials{apiKey: apiKey}

	// Establish gRPC connection
	conn, err := grpc.NewClient(
		"youtube.googleapis.com:443",
		grpc.WithTransportCredentials(tlsCreds),
		grpc.WithPerRPCCredentials(apiKeyCreds),
	)
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	// Create the service client
	client := proto.NewV3DataLiveChatMessageServiceClient(conn)

	ctx := context.Background()

	// Test without pageToken (as per our fix)
	testStream(ctx, client, liveChatID, "")
}

func testStream(ctx context.Context, client proto.V3DataLiveChatMessageServiceClient, liveChatID, pageToken string) {
	req := &proto.LiveChatMessageListRequest{
		LiveChatId: &liveChatID,
		Part:       []string{"id", "snippet", "authorDetails"},
		MaxResults: protobuf.Uint32(2000),
	}

	if pageToken != "" {
		req.PageToken = &pageToken
	}

	// Add metadata for logging
	md := metadata.New(map[string]string{
		"x-goog-request-params": fmt.Sprintf("live_chat_id=%s", liveChatID),
	})
	streamCtx := metadata.NewOutgoingContext(ctx, md)

	log.Printf("Starting gRPC stream (with_page_token=%v)", pageToken != "")
	startTime := time.Now()
	responseCount := 0
	totalMessages := 0

	stream, err := client.StreamList(streamCtx, req)
	if err != nil {
		log.Fatalf("Failed to start stream: %v", err)
	}

	// Receive streaming responses
	for {
		resp, err := stream.Recv()
		if err != nil {
			if err == io.EOF {
				duration := time.Since(startTime)
				log.Printf("✅ Stream ended normally (EOF)")
				log.Printf("   Duration: %v", duration)
				log.Printf("   Responses received: %d", responseCount)
				log.Printf("   Total messages: %d", totalMessages)
				log.Printf("   Messages per response: %.1f", float64(totalMessages)/float64(responseCount))
				return
			}
			log.Fatalf("❌ Stream error: %v", err)
		}

		responseCount++
		messagesInResponse := len(resp.Items)
		totalMessages += messagesInResponse

		log.Printf("Response #%d: %d messages, nextPageToken=%v",
			responseCount, messagesInResponse, resp.NextPageToken != nil && *resp.NextPageToken != "")

		// Show first few messages for verification
		if responseCount == 1 {
			for i, msg := range resp.Items {
				if i >= 3 {
					log.Printf("   ... (%d more messages)", messagesInResponse-3)
					break
				}
				author := "unknown"
				if msg.AuthorDetails != nil && msg.AuthorDetails.DisplayName != nil {
					author = *msg.AuthorDetails.DisplayName
				}
				text := "no text"
				if msg.Snippet != nil && msg.Snippet.GetTextMessageDetails() != nil {
					text = msg.Snippet.GetTextMessageDetails().GetMessageText()
				}
				log.Printf("   Message %d: [%s] %s", i+1, author, text)
			}
		}

		// Check if stream went offline
		if resp.OfflineAt != nil && *resp.OfflineAt != "" {
			log.Printf("⚠️  Stream went offline at: %s", *resp.OfflineAt)
			return
		}
	}
}
