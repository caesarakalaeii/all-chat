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

package handlers

import (
	"net/http"
	"time"

	"github.com/caesar/all-chat/services/youtube-quota-monitor/monitor"
	"github.com/caesar/all-chat/shared/quota"
	"github.com/gin-gonic/gin"
)

// globalStatus mirrors the shape the discord-bot's poll (fetchQuotaStatus →
// createQuotaEmbed) expects under "global". Keep these json names in sync with
// services/discord-bot/src/index.js.
type globalStatus struct {
	State             string  `json:"state"`
	Used              int     `json:"used"`
	Limit             int     `json:"limit"`
	Remaining         int     `json:"remaining"`
	Percentage        float64 `json:"percentage"`
	ResetsAt          string  `json:"resets_at"`
	PollingMultiplier float64 `json:"polling_multiplier"`
}

// statusResponse is the /quota/status envelope. Channels is always an empty array (the
// monitor has no per-channel data); it must serialize as [] not null.
type statusResponse struct {
	Global   globalStatus  `json:"global"`
	Channels []interface{} `json:"channels"`
}

// StatusHandler serves GET /quota/status from the monitor's last snapshot.
type StatusHandler struct {
	mon *monitor.Monitor
	loc *time.Location
}

// NewStatusHandler builds a status handler.
func NewStatusHandler(mon *monitor.Monitor) *StatusHandler {
	return &StatusHandler{mon: mon, loc: quota.Pacific()}
}

// GetQuotaStatus returns the current global quota status. used = confirmed + reserved
// so used + remaining = limit and used/limit ≈ percentage.
func (h *StatusHandler) GetQuotaStatus(c *gin.Context) {
	snap, state, _ := h.mon.Snapshot()
	c.JSON(http.StatusOK, statusResponse{
		Global: globalStatus{
			State:             string(state),
			Used:              snap.Used + snap.Reserved,
			Limit:             snap.Limit,
			Remaining:         snap.Available,
			Percentage:        snap.Percentage,
			ResetsAt:          nextMidnight(h.loc).Format(time.RFC3339),
			PollingMultiplier: 1.0,
		},
		Channels: []interface{}{},
	})
}

// nextMidnight is the next midnight in the given (Pacific) location — when YouTube
// resets the daily quota.
func nextMidnight(loc *time.Location) time.Time {
	now := time.Now().In(loc)
	y, m, d := now.Date()
	return time.Date(y, m, d+1, 0, 0, 0, 0, loc)
}
