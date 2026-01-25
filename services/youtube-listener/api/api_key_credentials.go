package api

import (
	"context"

	"google.golang.org/grpc/credentials"
)

// APIKeyCredentials implements credentials.PerRPCCredentials for YouTube API key authentication
// This allows gRPC streaming without OAuth for public streams
type APIKeyCredentials struct {
	apiKey string
}

// NewAPIKeyCredentials creates gRPC credentials from a YouTube API key
func NewAPIKeyCredentials(apiKey string) credentials.PerRPCCredentials {
	return &APIKeyCredentials{apiKey: apiKey}
}

// GetRequestMetadata returns the API key as request metadata
func (c *APIKeyCredentials) GetRequestMetadata(ctx context.Context, uri ...string) (map[string]string, error) {
	return map[string]string{
		"x-goog-api-key": c.apiKey,
	}, nil
}

// RequireTransportSecurity indicates that this credential requires TLS
func (c *APIKeyCredentials) RequireTransportSecurity() bool {
	return true
}
