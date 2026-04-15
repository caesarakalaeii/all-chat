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

package demand

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// DemandHandler provides the HTTP handler for the GET /demand endpoint.
// Listeners use this as a fallback poll when Pub/Sub is unavailable.
type DemandHandler struct {
	subscriber *OverlayDemandSubscriber
}

// NewDemandHandler creates a new DemandHandler.
func NewDemandHandler(subscriber *OverlayDemandSubscriber) *DemandHandler {
	return &DemandHandler{subscriber: subscriber}
}

// GetDemand handles GET /demand[?platform=<platform>].
// When a platform query param is provided, only sources for that platform are returned.
// Otherwise all demanded sources are returned.
// Response format: {"sources": [...DemandSource]}
func (h *DemandHandler) GetDemand(c *gin.Context) {
	platform := c.Query("platform")

	var sources []DemandSource
	if platform != "" {
		sources = h.subscriber.GetDemandedSourcesByPlatform(platform)
	} else {
		sources = h.subscriber.GetDemandedSources()
	}

	c.JSON(http.StatusOK, gin.H{
		"sources": sources,
	})
}
