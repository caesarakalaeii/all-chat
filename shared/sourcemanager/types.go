package sourcemanager

import "time"

// ActiveSource matches the Source Manager API payload for active sources.
type ActiveSource struct {
	ID        string    `json:"id"`
	OverlayID string    `json:"overlay_id"`
	Platform  string    `json:"platform"`
	ChannelID string    `json:"channel_id"`
	StreamID  string    `json:"stream_id"`
	IsActive  bool      `json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ClaimResponse models the JSON response returned for leadership claims.
type ClaimResponse struct {
	Acquired   bool   `json:"acquired"`
	InstanceID string `json:"instance_id"`
	Platform   string `json:"platform"`
	StreamID   string `json:"stream_id"`
}

// RenewResponse models the JSON response returned when renewing leadership.
type RenewResponse struct {
	Renewed    bool   `json:"renewed"`
	InstanceID string `json:"instance_id"`
	Platform   string `json:"platform"`
	StreamID   string `json:"stream_id"`
}
