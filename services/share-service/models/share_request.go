package models

import (
	"errors"
	"fmt"
	"time"
)

// OverlaySource represents a chat source platform entry attached to an overlay
type OverlaySource struct {
	Platform    string `json:"platform"`
	ChannelName string `json:"channel_name"`
}

// ShareRequest represents a request to share an overlay between two users
type ShareRequest struct {
	ID                 string          `json:"id" db:"id"`
	SenderUserID       string          `json:"sender_user_id" db:"sender_user_id"`
	SenderOverlayID    string          `json:"sender_overlay_id" db:"sender_overlay_id"`
	RecipientUserID    string          `json:"recipient_user_id" db:"recipient_user_id"`
	Status             string          `json:"status" db:"status"` // pending, accepted, rejected, expired, revoked
	CreatedAt          time.Time       `json:"created_at" db:"created_at"`
	RespondedAt        *time.Time      `json:"responded_at,omitempty" db:"responded_at"`
	ExpiresAt          time.Time       `json:"expires_at" db:"expires_at"`
	HasSeenAcceptance  bool            `json:"has_seen_acceptance" db:"has_seen_acceptance"`
	SenderDisplayName  string          `json:"sender_display_name,omitempty" db:"sender_display_name"` // Join with users table
	ExpiryOption       string          `json:"expiry_option,omitempty" db:"expiry_option"`
	ShareExpiresAt     *time.Time      `json:"share_expires_at,omitempty" db:"share_expires_at"`
	OverlaySources     []OverlaySource `json:"overlay_sources,omitempty"`
}

// Valid status constants
const (
	StatusPending  = "pending"
	StatusAccepted = "accepted"
	StatusRejected = "rejected"
	StatusExpired  = "expired"
	StatusRevoked  = "revoked"
)

// Common validation errors
var (
	ErrSenderUserIDRequired    = errors.New("sender_user_id is required")
	ErrSenderOverlayIDRequired = errors.New("sender_overlay_id is required")
	ErrRecipientUserIDRequired = errors.New("recipient_user_id is required")
	ErrCannotShareWithSelf     = errors.New("cannot share with yourself")
	ErrInvalidStatus           = errors.New("invalid status")
)

// Validate checks if the share request is valid
func (s *ShareRequest) Validate() error {
	if s.SenderUserID == "" {
		return ErrSenderUserIDRequired
	}
	if s.SenderOverlayID == "" {
		return ErrSenderOverlayIDRequired
	}
	if s.RecipientUserID == "" {
		return ErrRecipientUserIDRequired
	}
	if s.SenderUserID == s.RecipientUserID {
		return ErrCannotShareWithSelf
	}

	validStatuses := map[string]bool{
		StatusPending:  true,
		StatusAccepted: true,
		StatusRejected: true,
		StatusExpired:  true,
		StatusRevoked:  true,
	}
	if !validStatuses[s.Status] {
		return fmt.Errorf("%w: %s", ErrInvalidStatus, s.Status)
	}

	return nil
}

// IsPending returns true if the share request is in pending status
func (s *ShareRequest) IsPending() bool {
	return s.Status == StatusPending
}

// IsExpired returns true if the share request has expired
func (s *ShareRequest) IsExpired() bool {
	return s.Status == StatusExpired || (s.IsPending() && time.Now().After(s.ExpiresAt))
}

// IsActive returns true if the share request is accepted and active
func (s *ShareRequest) IsActive() bool {
	return s.Status == StatusAccepted
}
