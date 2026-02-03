package repository

import (
	"context"
	"testing"
	"time"

	"github.com/caesar/all-chat/services/overlay-manager/models"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

// setupTestDatabase starts a PostgreSQL container and returns the repository
func setupTestDatabase(t *testing.T) (*OverlayRepository, func()) {
	ctx := context.Background()

	// Start PostgreSQL container
	pgContainer, err := postgres.Run(ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("testdb"),
		postgres.WithUsername("testuser"),
		postgres.WithPassword("testpass"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second)),
	)
	require.NoError(t, err)

	// Get connection string
	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	// Create repository
	repo, err := NewOverlayRepository(connStr)
	require.NoError(t, err)

	// Create tables (simplified schema for tests)
	_, err = repo.pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS overlays (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			user_id UUID NOT NULL,
			name VARCHAR(100) NOT NULL,
			description TEXT,
			is_active BOOLEAN DEFAULT TRUE,
			is_public_for_viewers BOOLEAN NOT NULL DEFAULT false,
			created_at TIMESTAMP DEFAULT NOW(),
			updated_at TIMESTAMP DEFAULT NOW()
		);

		CREATE TABLE IF NOT EXISTS overlay_chat_sources (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			overlay_id UUID NOT NULL REFERENCES overlays(id) ON DELETE CASCADE,
			platform VARCHAR(50) NOT NULL,
			channel_id VARCHAR(100) NOT NULL,
			channel_name VARCHAR(100) NOT NULL,
			auth_required BOOLEAN DEFAULT FALSE,
			config JSONB DEFAULT '{}'::jsonb,
			is_active BOOLEAN DEFAULT TRUE,
			created_at TIMESTAMP DEFAULT NOW(),
			updated_at TIMESTAMP DEFAULT NOW(),
			UNIQUE(overlay_id, platform, channel_id)
		);
	`)
	require.NoError(t, err)

	// Cleanup function
	cleanup := func() {
		repo.pool.Close()
		if err := pgContainer.Terminate(ctx); err != nil {
			t.Logf("Failed to terminate container: %v", err)
		}
	}

	return repo, cleanup
}

func TestOverlayRepository_Create(t *testing.T) {
	repo, cleanup := setupTestDatabase(t)
	defer cleanup()

	ctx := context.Background()
	userID := uuid.New().String()

	tests := []struct {
		name    string
		overlay *models.Overlay
		wantErr bool
	}{
		{
			name: "successful creation",
			overlay: &models.Overlay{
				UserID:      userID,
				Name:        "Test Overlay",
				Description: "Test description",
				IsActive:    true,
			},
			wantErr: false,
		},
		{
			name: "missing user_id",
			overlay: &models.Overlay{
				UserID:      "",
				Name:        "Test Overlay",
				Description: "Test description",
			},
			wantErr: true,
		},
		{
			name: "missing name",
			overlay: &models.Overlay{
				UserID:      userID,
				Name:        "",
				Description: "Test description",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := repo.Create(ctx, tt.overlay)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotEmpty(t, tt.overlay.ID, "ID should be generated")
				assert.False(t, tt.overlay.CreatedAt.IsZero(), "CreatedAt should be set")
			}
		})
	}
}

func TestOverlayRepository_GetByID(t *testing.T) {
	repo, cleanup := setupTestDatabase(t)
	defer cleanup()

	ctx := context.Background()
	userID := uuid.New().String()

	// Create a test overlay
	overlay := &models.Overlay{
		UserID:      userID,
		Name:        "Test Overlay",
		Description: "Test description",
		IsActive:    true,
	}
	err := repo.Create(ctx, overlay)
	require.NoError(t, err)

	tests := []struct {
		name    string
		id      string
		wantErr bool
	}{
		{
			name:    "existing overlay",
			id:      overlay.ID,
			wantErr: false,
		},
		{
			name:    "non-existent overlay",
			id:      uuid.New().String(),
			wantErr: true,
		},
		{
			name:    "invalid uuid format",
			id:      "invalid-uuid",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := repo.GetByID(ctx, tt.id)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.Equal(t, tt.id, result.ID)
				assert.Equal(t, overlay.Name, result.Name)
			}
		})
	}
}

func TestOverlayRepository_ListByUserID(t *testing.T) {
	repo, cleanup := setupTestDatabase(t)
	defer cleanup()

	ctx := context.Background()
	userID := uuid.New().String()
	otherUserID := uuid.New().String()

	// Create overlays for test user
	overlay1 := &models.Overlay{
		UserID:   userID,
		Name:     "Overlay 1",
		IsActive: true,
	}
	overlay2 := &models.Overlay{
		UserID:   userID,
		Name:     "Overlay 2",
		IsActive: false,
	}

	// Create overlay for other user
	overlay3 := &models.Overlay{
		UserID:   otherUserID,
		Name:     "Other User Overlay",
		IsActive: true,
	}

	require.NoError(t, repo.Create(ctx, overlay1))
	require.NoError(t, repo.Create(ctx, overlay2))
	require.NoError(t, repo.Create(ctx, overlay3))

	tests := []struct {
		name      string
		userID    string
		wantCount int
	}{
		{
			name:      "user with 2 overlays",
			userID:    userID,
			wantCount: 2,
		},
		{
			name:      "user with 1 overlay",
			userID:    otherUserID,
			wantCount: 1,
		},
		{
			name:      "user with no overlays",
			userID:    uuid.New().String(),
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := repo.ListByUserID(ctx, tt.userID)
			assert.NoError(t, err)
			assert.Len(t, result, tt.wantCount)

			// Verify all returned overlays belong to the user
			for _, overlay := range result {
				assert.Equal(t, tt.userID, overlay.UserID)
			}
		})
	}
}

func TestOverlayRepository_Update(t *testing.T) {
	repo, cleanup := setupTestDatabase(t)
	defer cleanup()

	ctx := context.Background()
	userID := uuid.New().String()

	// Create initial overlay
	overlay := &models.Overlay{
		UserID:      userID,
		Name:        "Original Name",
		Description: "Original description",
		IsActive:    true,
	}
	err := repo.Create(ctx, overlay)
	require.NoError(t, err)

	tests := []struct {
		name        string
		updateFn    func(*models.Overlay)
		wantErr     bool
		checkResult func(*testing.T, *models.Overlay)
	}{
		{
			name: "update name",
			updateFn: func(o *models.Overlay) {
				o.Name = "Updated Name"
			},
			wantErr: false,
			checkResult: func(t *testing.T, o *models.Overlay) {
				assert.Equal(t, "Updated Name", o.Name)
			},
		},
		{
			name: "update description",
			updateFn: func(o *models.Overlay) {
				o.Description = "Updated description"
			},
			wantErr: false,
			checkResult: func(t *testing.T, o *models.Overlay) {
				assert.Equal(t, "Updated description", o.Description)
			},
		},
		{
			name: "deactivate overlay",
			updateFn: func(o *models.Overlay) {
				o.IsActive = false
			},
			wantErr: false,
			checkResult: func(t *testing.T, o *models.Overlay) {
				assert.False(t, o.IsActive)
			},
		},
		{
			name: "update to empty name (should fail)",
			updateFn: func(o *models.Overlay) {
				o.Name = ""
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Get fresh copy
			current, err := repo.GetByID(ctx, overlay.ID)
			require.NoError(t, err)

			// Apply update
			tt.updateFn(current)

			// Try to update
			err = repo.Update(ctx, current)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)

				// Verify update by fetching again
				updated, err := repo.GetByID(ctx, overlay.ID)
				require.NoError(t, err)

				if tt.checkResult != nil {
					tt.checkResult(t, updated)
				}
			}
		})
	}
}

func TestOverlayRepository_Delete(t *testing.T) {
	repo, cleanup := setupTestDatabase(t)
	defer cleanup()

	ctx := context.Background()
	userID := uuid.New().String()

	// Create overlay to delete
	overlay := &models.Overlay{
		UserID:   userID,
		Name:     "To Delete",
		IsActive: true,
	}
	err := repo.Create(ctx, overlay)
	require.NoError(t, err)

	tests := []struct {
		name    string
		id      string
		wantErr bool
	}{
		{
			name:    "delete existing overlay",
			id:      overlay.ID,
			wantErr: false,
		},
		{
			name:    "delete non-existent overlay",
			id:      uuid.New().String(),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := repo.Delete(ctx, tt.id)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)

				// Verify deletion
				_, err := repo.GetByID(ctx, tt.id)
				assert.Error(t, err, "Should not be able to fetch deleted overlay")
			}
		})
	}
}

func TestOverlayRepository_GetByIDAndUserID(t *testing.T) {
	repo, cleanup := setupTestDatabase(t)
	defer cleanup()

	ctx := context.Background()
	userID := uuid.New().String()
	otherUserID := uuid.New().String()

	// Create overlay for user
	overlay := &models.Overlay{
		UserID:   userID,
		Name:     "User Overlay",
		IsActive: true,
	}
	err := repo.Create(ctx, overlay)
	require.NoError(t, err)

	tests := []struct {
		name    string
		id      string
		userID  string
		wantErr bool
	}{
		{
			name:    "owner can fetch",
			id:      overlay.ID,
			userID:  userID,
			wantErr: false,
		},
		{
			name:    "non-owner cannot fetch (authorization check)",
			id:      overlay.ID,
			userID:  otherUserID,
			wantErr: true,
		},
		{
			name:    "non-existent overlay",
			id:      uuid.New().String(),
			userID:  userID,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := repo.GetByIDAndUserID(ctx, tt.id, tt.userID)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.Equal(t, tt.id, result.ID)
				assert.Equal(t, tt.userID, result.UserID)
			}
		})
	}
}
