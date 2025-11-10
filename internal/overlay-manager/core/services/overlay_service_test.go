package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/caesar/all-chat/internal/overlay-manager/core/domain"
)

// Mock repository for testing
type mockOverlayRepository struct {
	createFunc             func(ctx context.Context, overlay *domain.Overlay) error
	createConfigFunc       func(ctx context.Context, config *domain.OverlayConfig) error
	getByIDFunc            func(ctx context.Context, id string) (*domain.Overlay, error)
	getByUserIDFunc        func(ctx context.Context, userID string) ([]*domain.Overlay, error)
	updateFunc             func(ctx context.Context, overlay *domain.Overlay) error
	deleteFunc             func(ctx context.Context, id string) error
	getConfigFunc          func(ctx context.Context, overlayID string) (*domain.OverlayConfig, error)
	updateConfigFunc       func(ctx context.Context, config *domain.OverlayConfig) error
}

func (m *mockOverlayRepository) Create(ctx context.Context, overlay *domain.Overlay) error {
	if m.createFunc != nil {
		return m.createFunc(ctx, overlay)
	}
	return nil
}

func (m *mockOverlayRepository) CreateConfig(ctx context.Context, config *domain.OverlayConfig) error {
	if m.createConfigFunc != nil {
		return m.createConfigFunc(ctx, config)
	}
	return nil
}

func (m *mockOverlayRepository) GetByID(ctx context.Context, id string) (*domain.Overlay, error) {
	if m.getByIDFunc != nil {
		return m.getByIDFunc(ctx, id)
	}
	return nil, ErrOverlayNotFound
}

func (m *mockOverlayRepository) GetByUserID(ctx context.Context, userID string) ([]*domain.Overlay, error) {
	if m.getByUserIDFunc != nil {
		return m.getByUserIDFunc(ctx, userID)
	}
	return []*domain.Overlay{}, nil
}

func (m *mockOverlayRepository) Update(ctx context.Context, overlay *domain.Overlay) error {
	if m.updateFunc != nil {
		return m.updateFunc(ctx, overlay)
	}
	return nil
}

func (m *mockOverlayRepository) Delete(ctx context.Context, id string) error {
	if m.deleteFunc != nil {
		return m.deleteFunc(ctx, id)
	}
	return nil
}

func (m *mockOverlayRepository) GetConfig(ctx context.Context, overlayID string) (*domain.OverlayConfig, error) {
	if m.getConfigFunc != nil {
		return m.getConfigFunc(ctx, overlayID)
	}
	return nil, ErrOverlayNotFound
}

func (m *mockOverlayRepository) UpdateConfig(ctx context.Context, config *domain.OverlayConfig) error {
	if m.updateConfigFunc != nil {
		return m.updateConfigFunc(ctx, config)
	}
	return nil
}

func TestCreateOverlay(t *testing.T) {
	tests := []struct {
		name          string
		userID        string
		overlayName   string
		description   string
		twitchChannel string
		mockRepo      *mockOverlayRepository
		wantErr       bool
		expectedErr   error
	}{
		{
			name:          "successful overlay creation",
			userID:        "user-123",
			overlayName:   "Test Overlay",
			description:   "Test Description",
			twitchChannel: "testchannel",
			mockRepo:      &mockOverlayRepository{},
			wantErr:       false,
		},
		{
			name:          "missing overlay name",
			userID:        "user-123",
			overlayName:   "",
			description:   "Test Description",
			twitchChannel: "testchannel",
			mockRepo:      &mockOverlayRepository{},
			wantErr:       true,
			expectedErr:   ErrInvalidInput,
		},
		{
			name:          "missing twitch channel",
			userID:        "user-123",
			overlayName:   "Test Overlay",
			description:   "Test Description",
			twitchChannel: "",
			mockRepo:      &mockOverlayRepository{},
			wantErr:       true,
			expectedErr:   ErrInvalidInput,
		},
		{
			name:          "repository error on create",
			userID:        "user-123",
			overlayName:   "Test Overlay",
			description:   "Test Description",
			twitchChannel: "testchannel",
			mockRepo: &mockOverlayRepository{
				createFunc: func(ctx context.Context, overlay *domain.Overlay) error {
					return errors.New("database error")
				},
			},
			wantErr: true,
		},
		{
			name:          "repository error on create config",
			userID:        "user-123",
			overlayName:   "Test Overlay",
			description:   "Test Description",
			twitchChannel: "testchannel",
			mockRepo: &mockOverlayRepository{
				createConfigFunc: func(ctx context.Context, config *domain.OverlayConfig) error {
					return errors.New("config creation error")
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := NewOverlayService(tt.mockRepo)
			overlay, config, err := service.CreateOverlay(
				context.Background(),
				tt.userID,
				tt.overlayName,
				tt.description,
				tt.twitchChannel,
			)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error but got nil")
				}
				if tt.expectedErr != nil && !errors.Is(err, tt.expectedErr) {
					t.Errorf("expected error %v, got %v", tt.expectedErr, err)
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if overlay == nil {
				t.Error("expected overlay but got nil")
				return
			}

			if overlay.UserID != tt.userID {
				t.Errorf("expected userID %s, got %s", tt.userID, overlay.UserID)
			}

			if overlay.Name != tt.overlayName {
				t.Errorf("expected name %s, got %s", tt.overlayName, overlay.Name)
			}

			if !overlay.IsActive {
				t.Error("expected overlay to be active by default")
			}

			if config == nil {
				t.Error("expected config but got nil")
				return
			}

			if config.TwitchChannel != tt.twitchChannel {
				t.Errorf("expected twitch channel %s, got %s", tt.twitchChannel, config.TwitchChannel)
			}
		})
	}
}

func TestGetOverlay(t *testing.T) {
	mockOverlay := &domain.Overlay{
		ID:          "overlay-123",
		UserID:      "user-123",
		Name:        "Test Overlay",
		Description: "Test Description",
		IsActive:    true,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	tests := []struct {
		name        string
		overlayID   string
		userID      string
		mockRepo    *mockOverlayRepository
		wantErr     bool
		expectedErr error
	}{
		{
			name:      "successful get overlay",
			overlayID: "overlay-123",
			userID:    "user-123",
			mockRepo: &mockOverlayRepository{
				getByIDFunc: func(ctx context.Context, id string) (*domain.Overlay, error) {
					return mockOverlay, nil
				},
			},
			wantErr: false,
		},
		{
			name:      "overlay not found",
			overlayID: "nonexistent",
			userID:    "user-123",
			mockRepo: &mockOverlayRepository{
				getByIDFunc: func(ctx context.Context, id string) (*domain.Overlay, error) {
					return nil, ErrOverlayNotFound
				},
			},
			wantErr:     true,
			expectedErr: ErrOverlayNotFound,
		},
		{
			name:      "unauthorized access",
			overlayID: "overlay-123",
			userID:    "different-user",
			mockRepo: &mockOverlayRepository{
				getByIDFunc: func(ctx context.Context, id string) (*domain.Overlay, error) {
					return mockOverlay, nil
				},
			},
			wantErr:     true,
			expectedErr: ErrUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := NewOverlayService(tt.mockRepo)
			overlay, err := service.GetOverlay(context.Background(), tt.overlayID, tt.userID)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error but got nil")
				}
				if tt.expectedErr != nil && !errors.Is(err, tt.expectedErr) {
					t.Errorf("expected error %v, got %v", tt.expectedErr, err)
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if overlay == nil {
				t.Error("expected overlay but got nil")
			}
		})
	}
}

func TestGetUserOverlays(t *testing.T) {
	mockOverlays := []*domain.Overlay{
		{
			ID:          "overlay-1",
			UserID:      "user-123",
			Name:        "Overlay 1",
			Description: "First overlay",
			IsActive:    true,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
		{
			ID:          "overlay-2",
			UserID:      "user-123",
			Name:        "Overlay 2",
			Description: "Second overlay",
			IsActive:    false,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
	}

	tests := []struct {
		name          string
		userID        string
		mockRepo      *mockOverlayRepository
		wantErr       bool
		expectedCount int
	}{
		{
			name:   "successful get user overlays",
			userID: "user-123",
			mockRepo: &mockOverlayRepository{
				getByUserIDFunc: func(ctx context.Context, userID string) ([]*domain.Overlay, error) {
					return mockOverlays, nil
				},
			},
			wantErr:       false,
			expectedCount: 2,
		},
		{
			name:   "empty list",
			userID: "user-456",
			mockRepo: &mockOverlayRepository{
				getByUserIDFunc: func(ctx context.Context, userID string) ([]*domain.Overlay, error) {
					return []*domain.Overlay{}, nil
				},
			},
			wantErr:       false,
			expectedCount: 0,
		},
		{
			name:   "repository error",
			userID: "user-123",
			mockRepo: &mockOverlayRepository{
				getByUserIDFunc: func(ctx context.Context, userID string) ([]*domain.Overlay, error) {
					return nil, errors.New("database error")
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := NewOverlayService(tt.mockRepo)
			overlays, err := service.GetUserOverlays(context.Background(), tt.userID)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error but got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if len(overlays) != tt.expectedCount {
				t.Errorf("expected %d overlays, got %d", tt.expectedCount, len(overlays))
			}
		})
	}
}

func TestUpdateOverlay(t *testing.T) {
	existingOverlay := &domain.Overlay{
		ID:          "overlay-123",
		UserID:      "user-123",
		Name:        "Original Name",
		Description: "Original Description",
		IsActive:    true,
		CreatedAt:   time.Now().Add(-24 * time.Hour),
		UpdatedAt:   time.Now().Add(-24 * time.Hour),
	}

	tests := []struct {
		name        string
		overlayID   string
		userID      string
		newName     string
		newDesc     string
		isActive    bool
		mockRepo    *mockOverlayRepository
		wantErr     bool
		expectedErr error
	}{
		{
			name:      "successful update name",
			overlayID: "overlay-123",
			userID:    "user-123",
			newName:   "Updated Name",
			mockRepo: &mockOverlayRepository{
				getByIDFunc: func(ctx context.Context, id string) (*domain.Overlay, error) {
					return existingOverlay, nil
				},
			},
			wantErr: false,
		},
		{
			name:      "overlay not found",
			overlayID: "nonexistent",
			userID:    "user-123",
			newName:   "Updated Name",
			mockRepo: &mockOverlayRepository{
				getByIDFunc: func(ctx context.Context, id string) (*domain.Overlay, error) {
					return nil, ErrOverlayNotFound
				},
			},
			wantErr:     true,
			expectedErr: ErrOverlayNotFound,
		},
		{
			name:      "unauthorized update",
			overlayID: "overlay-123",
			userID:    "different-user",
			newName:   "Updated Name",
			mockRepo: &mockOverlayRepository{
				getByIDFunc: func(ctx context.Context, id string) (*domain.Overlay, error) {
					return existingOverlay, nil
				},
			},
			wantErr:     true,
			expectedErr: ErrUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := NewOverlayService(tt.mockRepo)
			err := service.UpdateOverlay(
				context.Background(),
				tt.overlayID,
				tt.userID,
				tt.newName,
				tt.newDesc,
				tt.isActive,
			)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error but got nil")
				}
				if tt.expectedErr != nil && !errors.Is(err, tt.expectedErr) {
					t.Errorf("expected error %v, got %v", tt.expectedErr, err)
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestDeleteOverlay(t *testing.T) {
	existingOverlay := &domain.Overlay{
		ID:          "overlay-123",
		UserID:      "user-123",
		Name:        "Test Overlay",
		Description: "Test Description",
		IsActive:    true,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	tests := []struct {
		name        string
		overlayID   string
		userID      string
		mockRepo    *mockOverlayRepository
		wantErr     bool
		expectedErr error
	}{
		{
			name:      "successful delete",
			overlayID: "overlay-123",
			userID:    "user-123",
			mockRepo: &mockOverlayRepository{
				getByIDFunc: func(ctx context.Context, id string) (*domain.Overlay, error) {
					return existingOverlay, nil
				},
			},
			wantErr: false,
		},
		{
			name:      "overlay not found",
			overlayID: "nonexistent",
			userID:    "user-123",
			mockRepo: &mockOverlayRepository{
				getByIDFunc: func(ctx context.Context, id string) (*domain.Overlay, error) {
					return nil, ErrOverlayNotFound
				},
			},
			wantErr:     true,
			expectedErr: ErrOverlayNotFound,
		},
		{
			name:      "unauthorized delete",
			overlayID: "overlay-123",
			userID:    "different-user",
			mockRepo: &mockOverlayRepository{
				getByIDFunc: func(ctx context.Context, id string) (*domain.Overlay, error) {
					return existingOverlay, nil
				},
			},
			wantErr:     true,
			expectedErr: ErrUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := NewOverlayService(tt.mockRepo)
			err := service.DeleteOverlay(context.Background(), tt.overlayID, tt.userID)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error but got nil")
				}
				if tt.expectedErr != nil && !errors.Is(err, tt.expectedErr) {
					t.Errorf("expected error %v, got %v", tt.expectedErr, err)
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}
