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
