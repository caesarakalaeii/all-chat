/**
 * Coordination Models
 *
 * TypeScript interfaces matching Go shared/coordination models for TikTok listener
 * coordinator integration (Phase 6).
 */

/**
 * Assignment represents a channel assignment from the coordinator.
 * Matches Go shared/coordination/models.go Assignment structure.
 */
export interface Assignment {
  source_id: string;
  pod_id: string;
  timestamp: string; // ISO 8601 timestamp
  version: number;
}

/**
 * AssignmentResponse is the response from GET /assignments endpoint.
 * Matches Go shared/coordination/models.go AssignmentResponse structure.
 */
export interface AssignmentResponse {
  assignments: Assignment[];
  count: number;
}

/**
 * HeartbeatRequest is the payload for heartbeat publishing.
 * Matches Go shared/coordination/client.go HeartbeatRequest structure.
 */
export interface HeartbeatRequest {
  pod_id: string;
}

/**
 * MigrationEvent represents a channel migration event published to Redis Pub/Sub.
 * Matches Go shared/coordination/models.go MigrationEvent structure.
 *
 * Implements MIGRATE-05: Full context for debugging and metrics.
 * Per CONTEXT.md user decision: Required fields are channel_id, platform,
 * from_pod, to_pod, migration_id, timestamp, reason.
 */
export interface MigrationEvent {
  migration_id: string;  // UUID for tracing
  channel_id: string;    // Source UUID from database
  platform: string;      // "twitch", "kick", "tiktok"
  from_pod: string;      // Kubernetes pod name (old pod)
  to_pod: string;        // Kubernetes pod name (new pod)
  timestamp: string;     // ISO 8601 timestamp
  reason: string;        // "scale_up", "rebalancing", "pod_failure"
}

/**
 * MigrationConfirmation represents a confirmation message from a listener.
 * Used by listeners to confirm successful connection during overlap protocol.
 * Matches Go shared/coordination/models.go MigrationConfirmation structure.
 */
export interface MigrationConfirmation {
  migration_id: string;
  status: 'connected' | 'failed';
  pod_id: string;
  timestamp: string; // ISO 8601 timestamp
  error?: string;    // Error message if status="failed"
}
