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

package quota

import "time"

// QuotaEventType identifies the kind of quota notification.
type QuotaEventType string

const (
	EventStateChanged     QuotaEventType = "state_changed"
	EventThresholdCrossed QuotaEventType = "threshold_crossed"
	EventQuotaExhausted   QuotaEventType = "quota_exhausted"
	EventQuotaDepleted    QuotaEventType = "quota_depleted"
	EventQuotaRecovered   QuotaEventType = "quota_recovered"
	EventChannelExceeded  QuotaEventType = "channel_quota_exceeded"
)

// QuotaEvent is the JSON payload published to the "quota:alerts" Redis Pub/Sub
// channel and consumed by the discord-bot (services/discord-bot/src/index.js).
//
// WIRE CONTRACT: the discord-bot destructures these exact json field names
// (type, global_state, usage_percentage, units_used, units_limit, units_remaining,
// message, severity, affected_channels). Do not rename or retag — see
// shared/quota/event_test.go which locks the contract.
type QuotaEvent struct {
	Type             QuotaEventType `json:"type"`
	Timestamp        time.Time      `json:"timestamp"`
	GlobalState      QuotaState     `json:"global_state"`
	UsagePercentage  float64        `json:"usage_percentage"`
	UnitsUsed        int            `json:"units_used"`
	UnitsLimit       int            `json:"units_limit"`
	UnitsRemaining   int            `json:"units_remaining"`
	PreviousState    *QuotaState    `json:"previous_state,omitempty"`
	AffectedChannels []string       `json:"affected_channels,omitempty"`
	Message          string         `json:"message"`
	Severity         string         `json:"severity"` // info, warning, error, critical
}

// Severity maps a quota state to the severity string the discord-bot uses to
// colour its embeds.
func Severity(state QuotaState) string {
	switch state {
	case QuotaStateDepleted:
		return "critical"
	case QuotaStateExhausted:
		return "error"
	case QuotaStateCritical:
		return "error"
	case QuotaStateDegraded:
		return "warning"
	case QuotaStateHealthy:
		return "info"
	default:
		return "info"
	}
}
