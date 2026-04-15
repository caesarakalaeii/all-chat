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
