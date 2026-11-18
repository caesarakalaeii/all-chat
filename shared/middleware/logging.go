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
	"fmt"
	"net/url"
	"strings"
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

// GinAccessLog is gin's stock access log line with the client IP anonymised
// and sensitive query values redacted; health probes are skipped. Use it
// instead of gin.Logger(): a backend behind the api-gateway receives Envoy's
// X-Forwarded-For, gin trusts every proxy by default, so gin.Logger() prints
// the visitor's full IP (and raw OAuth codes or tokens from the query) to
// stdout, which Alloy ships to Loki.
func GinAccessLog() gin.HandlerFunc {
	return gin.LoggerWithConfig(gin.LoggerConfig{
		SkipPaths: []string{"/health/live", "/health/ready"},
		Formatter: func(p gin.LogFormatterParams) string {
			path, rawQuery, hasQuery := strings.Cut(p.Path, "?")
			if hasQuery {
				path += "?" + RedactQuery(rawQuery)
			}
			return fmt.Sprintf("[GIN] %v | %3d | %13v | %15s | %-7s %#v\n%s",
				p.TimeStamp.Format("2006/01/02 - 15:04:05"),
				p.StatusCode,
				p.Latency,
				AnonymizeIP(p.ClientIP),
				p.Method,
				path,
				p.ErrorMessage,
			)
		},
	})
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
