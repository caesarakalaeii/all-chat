package coordination

import "time"

// MigrationEvent represents a channel migration event published to Redis Pub/Sub
// Implements MIGRATE-05: Full context for debugging and metrics
// Per CONTEXT.md user decision: Required fields are channel_id, platform, from_pod, to_pod, migration_id, timestamp, reason
type MigrationEvent struct {
	MigrationID string    `json:"migration_id"` // UUID for tracing
	ChannelID   string    `json:"channel_id"`   // Source UUID from database
	Platform    string    `json:"platform"`     // "twitch", "kick", "tiktok"
	FromPod     string    `json:"from_pod"`     // Kubernetes pod name (old pod)
	ToPod       string    `json:"to_pod"`       // Kubernetes pod name (new pod)
	Timestamp   time.Time `json:"timestamp"`    // Event creation time
	Reason      string    `json:"reason"`       // "scale_up", "rebalancing", "pod_failure"
	TraceParent string    `json:"traceparent,omitempty"` // W3C Trace Context propagation
	TraceState  string    `json:"tracestate,omitempty"`  // W3C Trace Context state
}

// MigrationConfirmation represents a confirmation message from a listener
// Used by listeners to confirm successful connection during overlap protocol
type MigrationConfirmation struct {
	MigrationID string    `json:"migration_id"`        // Matches MigrationEvent.MigrationID
	Status      string    `json:"status"`              // "connected", "failed"
	PodID       string    `json:"pod_id"`              // Pod publishing confirmation
	Timestamp   time.Time `json:"timestamp"`           // Confirmation time
	Error       string    `json:"error,omitempty"`     // Error message if status="failed"
}

// AssignmentResponse is the response from GET /assignments endpoint
// Matches source-manager handlers response structure
type AssignmentResponse struct {
	Assignments []*Assignment `json:"assignments"`
	Count       int           `json:"count"`
}
