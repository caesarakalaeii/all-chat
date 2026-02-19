package models

import "time"

// Assignment represents a channel-to-pod assignment in the sharding system
type Assignment struct {
	SourceID  string    `json:"source_id"`  // overlay_chat_source.id (UUID)
	PodID     string    `json:"pod_id"`     // Kubernetes pod ID
	Timestamp time.Time `json:"timestamp"`  // When assignment was created
	Version   int64     `json:"version"`    // Global version counter for fencing
}
