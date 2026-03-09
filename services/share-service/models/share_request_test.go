package models

import (
	"testing"
	"time"
)

func TestShareRequest_Validate(t *testing.T) {
	tests := []struct {
		name    string
		req     ShareRequest
		wantErr error
	}{
		{
			name: "valid share request",
			req: ShareRequest{
				SenderUserID:    "user1",
				SenderOverlayID: "overlay1",
				RecipientUserID: "user2",
				Status:          StatusPending,
			},
			wantErr: nil,
		},
		{
			name: "missing sender user id",
			req: ShareRequest{
				SenderOverlayID: "overlay1",
				RecipientUserID: "user2",
				Status:          StatusPending,
			},
			wantErr: ErrSenderUserIDRequired,
		},
		{
			name: "missing sender overlay id",
			req: ShareRequest{
				SenderUserID:    "user1",
				RecipientUserID: "user2",
				Status:          StatusPending,
			},
			wantErr: ErrSenderOverlayIDRequired,
		},
		{
			name: "missing recipient user id",
			req: ShareRequest{
				SenderUserID:    "user1",
				SenderOverlayID: "overlay1",
				Status:          StatusPending,
			},
			wantErr: ErrRecipientUserIDRequired,
		},
		{
			name: "self share",
			req: ShareRequest{
				SenderUserID:    "user1",
				SenderOverlayID: "overlay1",
				RecipientUserID: "user1",
				Status:          StatusPending,
			},
			wantErr: ErrCannotShareWithSelf,
		},
		{
			name: "invalid status",
			req: ShareRequest{
				SenderUserID:    "user1",
				SenderOverlayID: "overlay1",
				RecipientUserID: "user2",
				Status:          "invalid",
			},
			wantErr: ErrInvalidStatus,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.req.Validate()
			if err != tt.wantErr && (err == nil || tt.wantErr == nil || err.Error() != tt.wantErr.Error()) {
				if tt.wantErr == ErrInvalidStatus {
					// For invalid status, we wrap the error, so just check if it contains the error
					if err == nil || err.Error()[:len(tt.wantErr.Error())] != tt.wantErr.Error() {
						t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
					}
				} else {
					t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
				}
			}
		})
	}
}

func TestShareRequest_IsPending(t *testing.T) {
	req := ShareRequest{Status: StatusPending}
	if !req.IsPending() {
		t.Error("IsPending() should return true for pending status")
	}

	req.Status = StatusAccepted
	if req.IsPending() {
		t.Error("IsPending() should return false for accepted status")
	}
}

func TestShareRequest_IsExpired(t *testing.T) {
	now := time.Now()

	// Status is expired
	req := ShareRequest{Status: StatusExpired, ExpiresAt: now.Add(-time.Hour)}
	if !req.IsExpired() {
		t.Error("IsExpired() should return true for expired status")
	}

	// Pending but past expiry time
	req = ShareRequest{Status: StatusPending, ExpiresAt: now.Add(-time.Hour)}
	if !req.IsExpired() {
		t.Error("IsExpired() should return true for pending past expiry")
	}

	// Pending but not expired yet
	req = ShareRequest{Status: StatusPending, ExpiresAt: now.Add(time.Hour)}
	if req.IsExpired() {
		t.Error("IsExpired() should return false for pending before expiry")
	}

	// Accepted (not expired)
	req = ShareRequest{Status: StatusAccepted, ExpiresAt: now.Add(-time.Hour)}
	if req.IsExpired() {
		t.Error("IsExpired() should return false for accepted status")
	}
}

func TestShareRequest_IsActive(t *testing.T) {
	req := ShareRequest{Status: StatusAccepted}
	if !req.IsActive() {
		t.Error("IsActive() should return true for accepted status")
	}

	req.Status = StatusPending
	if req.IsActive() {
		t.Error("IsActive() should return false for pending status")
	}

	req.Status = StatusRejected
	if req.IsActive() {
		t.Error("IsActive() should return false for rejected status")
	}
}
