package models

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestOverlay_Validate(t *testing.T) {
	tests := []struct {
		name    string
		overlay *Overlay
		wantErr bool
	}{
		{
			name: "valid overlay",
			overlay: &Overlay{
				ID:          uuid.New().String(),
				UserID:      uuid.New().String(),
				Name:        "My Overlay",
				Description: "Test description",
				IsActive:    true,
				CreatedAt:   time.Now(),
				UpdatedAt:   time.Now(),
			},
			wantErr: false,
		},
		{
			name: "missing user_id",
			overlay: &Overlay{
				ID:       uuid.New().String(),
				UserID:   "",
				Name:     "My Overlay",
				IsActive: true,
			},
			wantErr: true,
		},
		{
			name: "missing name",
			overlay: &Overlay{
				ID:       uuid.New().String(),
				UserID:   uuid.New().String(),
				Name:     "",
				IsActive: true,
			},
			wantErr: true,
		},
		{
			name: "name too short",
			overlay: &Overlay{
				ID:       uuid.New().String(),
				UserID:   uuid.New().String(),
				Name:     "",
				IsActive: true,
			},
			wantErr: true,
		},
		{
			name: "name too long (over 100 chars)",
			overlay: &Overlay{
				ID:       uuid.New().String(),
				UserID:   uuid.New().String(),
				Name:     "a very long name that exceeds the maximum allowed length of 100 characters and should fail validation check",
				IsActive: true,
			},
			wantErr: true,
		},
		{
			name: "description too long (over 500 chars)",
			overlay: &Overlay{
				ID:          uuid.New().String(),
				UserID:      uuid.New().String(),
				Name:        "Valid Name",
				Description: "Lorem ipsum dolor sit amet, consectetur adipiscing elit. Sed do eiusmod tempor incididunt ut labore et dolore magna aliqua. Ut enim ad minim veniam, quis nostrud exercitation ullamco laboris nisi ut aliquip ex ea commodo consequat. Duis aute irure dolor in reprehenderit in voluptate velit esse cillum dolore eu fugiat nulla pariatur. Excepteur sint occaecat cupidatat non proident, sunt in culpa qui officia deserunt mollit anim id est laborum. Sed ut perspiciatis unde omnis iste natus error sit voluptatem accusantium doloremque laudantium, totam rem aperiam.",
				IsActive:    true,
			},
			wantErr: true,
		},
		{
			name: "valid overlay with empty description (optional)",
			overlay: &Overlay{
				ID:          uuid.New().String(),
				UserID:      uuid.New().String(),
				Name:        "My Overlay",
				Description: "",
				IsActive:    true,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.overlay.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Overlay.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
