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

package models

import "time"

// Assignment represents a channel-to-pod assignment in the sharding system
type Assignment struct {
	SourceID  string    `json:"source_id"`  // overlay_chat_source.id (UUID)
	PodID     string    `json:"pod_id"`     // Kubernetes pod ID
	Timestamp time.Time `json:"timestamp"`  // When assignment was created
	Version   int64     `json:"version"`    // Global version counter for fencing
}

// AssignmentResponse is the response format for assignment queries
type AssignmentResponse struct {
	Assignments []Assignment `json:"assignments"`
	Count       int          `json:"count"`
}

// HeartbeatRequest is the request format for heartbeat publishing
type HeartbeatRequest struct {
	PodID string `json:"pod_id" binding:"required"`
}
