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

package middleware

import (
	"net/url"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// sensitiveQueryKeys are query parameters that must never reach access logs:
// JWTs (token/tts_token/access_token/refresh_token) and OAuth handshake values
// (code/state). See audit H5/#24/#25.
var sensitiveQueryKeys = []string{"token", "tts_token", "access_token", "refresh_token", "code", "state"}

// RedactQuery returns rawQuery with the values of any known-sensitive parameters
// replaced by [REDACTED], on ANY route (audit #24/#25 — the prior redaction was
// scoped to /ws/ and the "token" key only, leaking tts_token and OAuth code/state
// on other routes). Returns rawQuery unchanged when it contains no sensitive key
// or cannot be parsed.
func RedactQuery(rawQuery string) string {
	if rawQuery == "" {
		return rawQuery
	}
	values, err := url.ParseQuery(rawQuery)
	if err != nil {
		return rawQuery
	}
	changed := false
	for _, k := range sensitiveQueryKeys {
		if values.Has(k) {
			values.Set(k, "[REDACTED]")
			changed = true
		}
	}
	if !changed {
		return rawQuery
	}
	return values.Encode()
}

// Logging returns a gin middleware that logs HTTP requests
func Logging(logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		query := RedactQuery(c.Request.URL.RawQuery)

		// Process request
		c.Next()

		// Log after request
		latency := time.Since(start)
		status := c.Writer.Status()

		fields := []zap.Field{
			zap.String("method", c.Request.Method),
			zap.String("path", path),
			zap.String("query", query),
			zap.Int("status", status),
			zap.Duration("latency", latency),
			zap.String("client_ip", AnonymizeIP(c.ClientIP())),
			zap.String("user_agent", c.Request.UserAgent()),
		}

		// Add error if present
		if len(c.Errors) > 0 {
			fields = append(fields, zap.String("error", c.Errors.String()))
		}

		// Log at appropriate level
		if status >= 500 {
			logger.Error("Request failed", fields...)
		} else if status >= 400 {
			logger.Warn("Client error", fields...)
		} else {
			logger.Info("Request completed", fields...)
		}
	}
}
