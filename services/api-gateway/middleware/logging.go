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
	"time"

	sharedmw "github.com/caesar/all-chat/shared/middleware"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Logging returns a middleware that logs HTTP requests.
// Client IPs are anonymised (last octet zeroed) per DSGVO data-minimisation.
func Logging(log *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		// Redact credentials from the query string on ALL routes (audit H5/#24/#25):
		// JWTs (?token=/?tts_token=/?access_token=/?refresh_token=) and OAuth
		// ?code=/?state= must never reach access logs. Previously this was scoped to
		// /ws/ + the "token" key only, leaking tts_token and OAuth code/state.
		query := sharedmw.RedactQuery(c.Request.URL.RawQuery)

		// Process request
		c.Next()

		// Calculate latency
		latency := time.Since(start)

		// Get response status
		status := c.Writer.Status()

		// Log request details
		log.Info("HTTP request",
			zap.String("method", c.Request.Method),
			zap.String("path", path),
			zap.String("query", query),
			zap.Int("status", status),
			zap.Duration("latency", latency),
			zap.String("client_ip", sharedmw.AnonymizeIP(c.ClientIP())),
			zap.String("user_agent", c.Request.UserAgent()),
		)

		// Log errors if any
		if len(c.Errors) > 0 {
			for _, err := range c.Errors {
				log.Error("Request error",
					zap.String("path", path),
					zap.Error(err),
				)
			}
		}
	}
}
