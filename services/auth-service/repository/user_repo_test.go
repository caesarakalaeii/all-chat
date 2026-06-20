// This file is part of All-Chat.
// Copyright (C) 2026 caesarakalaeii
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/caesar/all-chat/services/auth-service/models"
	"github.com/caesar/all-chat/shared/encryption"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

const testEncryptionKey = "0123456789abcdef0123456789abcdef"

// setupTestDB creates a PostgreSQL testcontainer and returns a connection pool
func setupTestDB(t *testing.T) (*pgxpool.Pool, func()) {
	ctx := context.Background()

	// Create PostgreSQL container
	req := testcontainers.ContainerRequest{
		Image:        "postgres:16-alpine",
		ExposedPorts: []string{"5432/tcp"},
		Env: map[string]string{
			"POSTGRES_USER":     "testuser",
			"POSTGRES_PASSWORD": "testpass",
			"POSTGRES_DB":       "testdb",
		},
		WaitingFor: wait.ForLog("database system is ready to accept connections").
			WithOccurrence(2).
			WithStartupTimeout(60 * time.Second),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		t.Fatalf("Failed to start container: %v", err)
	}

	// Get container host and port
	host, err := container.Host(ctx)
	if err != nil {
		t.Fatalf("Failed to get container host: %v", err)
	}

	port, err := container.MappedPort(ctx, "5432")
	if err != nil {
		t.Fatalf("Failed to get container port: %v", err)
	}

	// Create connection string
	connString := "postgres://testuser:testpass@" + host + ":" + port.Port() + "/testdb?sslmode=disable"

	// Create connection pool
	pool, err := pgxpool.New(ctx, connString)
	if err != nil {
		t.Fatalf("Failed to create connection pool: %v", err)
	}

	// Create users table (matches migration 005)
	schema := `
		CREATE TABLE IF NOT EXISTS users (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			twitch_id VARCHAR(50) UNIQUE,
			google_id VARCHAR(100) UNIQUE,
			kick_id VARCHAR(255) UNIQUE,
			auth_provider VARCHAR(20) NOT NULL DEFAULT 'twitch',
			username VARCHAR(50) UNIQUE NOT NULL,
			display_name VARCHAR(100) NOT NULL,
			profile_image_url TEXT,
			is_admin BOOLEAN NOT NULL DEFAULT FALSE,
			is_premium BOOLEAN NOT NULL DEFAULT FALSE,
			is_beta_tester BOOLEAN NOT NULL DEFAULT FALSE,
			is_banned BOOLEAN NOT NULL DEFAULT FALSE,
			banned_at TIMESTAMP,
			banned_reason TEXT,
			banned_by UUID,
			access_token TEXT NOT NULL,
			refresh_token TEXT NOT NULL,
			token_expires_at TIMESTAMP NOT NULL,
			granted_scopes TEXT[] NOT NULL DEFAULT '{}',
			created_at TIMESTAMP DEFAULT NOW(),
			updated_at TIMESTAMP DEFAULT NOW(),
			CONSTRAINT check_auth_ids CHECK (
				(auth_provider = 'twitch' AND twitch_id IS NOT NULL) OR
				(auth_provider = 'youtube' AND google_id IS NOT NULL) OR
				(auth_provider = 'kick' AND kick_id IS NOT NULL)
			)
		);

		CREATE INDEX idx_users_twitch_id ON users(twitch_id);
		CREATE INDEX idx_users_google_id ON users(google_id);
		CREATE INDEX idx_users_username ON users(username);

		CREATE TABLE IF NOT EXISTS banned_platform_ids (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			platform VARCHAR(50) NOT NULL,
			platform_id VARCHAR(100) NOT NULL,
			banned_by UUID,
			reason TEXT NOT NULL,
			banned_at TIMESTAMP NOT NULL DEFAULT NOW(),
			unbanned_at TIMESTAMP NULL,
			is_active BOOLEAN NOT NULL DEFAULT TRUE
		);
	`

	_, err = pool.Exec(ctx, schema)
	if err != nil {
		t.Fatalf("Failed to create schema: %v", err)
	}

	// Return cleanup function
	cleanup := func() {
		pool.Close()
		container.Terminate(ctx)
	}

	return pool, cleanup
}

func newTestUserRepository(t *testing.T, pool *pgxpool.Pool) *UserRepository {
	key, err := encryption.ParseKey(testEncryptionKey)
	if err != nil {
		t.Fatalf("failed to parse test encryption key: %v", err)
	}
	cipher, err := encryption.NewAESEncryptor(key)
	if err != nil {
		t.Fatalf("failed to create cipher: %v", err)
	}
	return NewUserRepository(pool, cipher)
}

func TestUserRepository_Create(t *testing.T) {
	pool, cleanup := setupTestDB(t)
	defer cleanup()

	repo := newTestUserRepository(t, pool)
	ctx := context.Background()

	twitchID1 := "123456"
	twitchID2 := "789012"

	tests := []struct {
		name    string
		user    *models.User
		wantErr bool
	}{
		{
			name: "successful create",
			user: &models.User{
				TwitchID:        &twitchID1,
				AuthProvider:    "twitch",
				Username:        "testuser",
				DisplayName:     "TestUser",
				ProfileImageURL: "https://example.com/avatar.png",
				AccessToken:     "encrypted_access_token",
				RefreshToken:    "encrypted_refresh_token",
				TokenExpiresAt:  time.Now().Add(24 * time.Hour),
			},
			wantErr: false,
		},
		{
			name: "duplicate twitch_id",
			user: &models.User{
				TwitchID:       &twitchID1, // Same as above
				AuthProvider:   "twitch",
				Username:       "anotheruser",
				DisplayName:    "AnotherUser",
				AccessToken:    "encrypted_access_token",
				RefreshToken:   "encrypted_refresh_token",
				TokenExpiresAt: time.Now().Add(24 * time.Hour),
			},
			wantErr: true,
		},
		{
			name: "duplicate username",
			user: &models.User{
				TwitchID:       &twitchID2,
				AuthProvider:   "twitch",
				Username:       "testuser", // Same as first test
				DisplayName:    "DifferentUser",
				AccessToken:    "encrypted_access_token",
				RefreshToken:   "encrypted_refresh_token",
				TokenExpiresAt: time.Now().Add(24 * time.Hour),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := repo.Create(ctx, tt.user)
			if (err != nil) != tt.wantErr {
				t.Errorf("Create() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				// Verify user was created with an ID
				if tt.user.ID == "" {
					t.Error("Create() did not set user ID")
				}
				// Verify timestamps were set
				if tt.user.CreatedAt.IsZero() {
					t.Error("Create() did not set CreatedAt")
				}
				if tt.user.UpdatedAt.IsZero() {
					t.Error("Create() did not set UpdatedAt")
				}
			}
		})
	}
}

// TestUserRepository_NullProfileImageURL verifies that users whose
// profile_image_url column is NULL (the column is nullable, e.g. accounts
// created without an avatar) can still be scanned. Regression test for the
// admin "failed to load users" HTTP 500 caused by scanning NULL into a
// non-nullable string destination.
func TestUserRepository_NullProfileImageURL(t *testing.T) {
	pool, cleanup := setupTestDB(t)
	defer cleanup()

	repo := newTestUserRepository(t, pool)
	ctx := context.Background()

	// Insert a user with a NULL profile_image_url directly, bypassing Create
	// (which would supply a non-NULL value).
	var userID string
	err := pool.QueryRow(ctx, `
		INSERT INTO users (twitch_id, auth_provider, username, display_name, profile_image_url, access_token, refresh_token, token_expires_at)
		VALUES ($1, 'twitch', 'noavatar', 'NoAvatar', NULL, '', '', $2)
		RETURNING id
	`, "555000", time.Now().Add(24*time.Hour)).Scan(&userID)
	if err != nil {
		t.Fatalf("Failed to insert user with NULL profile_image_url: %v", err)
	}

	t.Run("GetAllUsers tolerates NULL profile_image_url", func(t *testing.T) {
		users, err := repo.GetAllUsers(ctx)
		if err != nil {
			t.Fatalf("GetAllUsers() error = %v, want nil", err)
		}
		if len(users) != 1 {
			t.Fatalf("GetAllUsers() returned %d users, want 1", len(users))
		}
		if users[0].ProfileImageURL != "" {
			t.Errorf("GetAllUsers() ProfileImageURL = %q, want empty string", users[0].ProfileImageURL)
		}
	})

	t.Run("GetByID tolerates NULL profile_image_url", func(t *testing.T) {
		user, err := repo.GetByID(ctx, userID)
		if err != nil {
			t.Fatalf("GetByID() error = %v, want nil", err)
		}
		if user.ProfileImageURL != "" {
			t.Errorf("GetByID() ProfileImageURL = %q, want empty string", user.ProfileImageURL)
		}
	})
}

// TestUserRepository_GetAllUsers_UndecryptableToken verifies that the admin
// user listing does not fail when an account has a corrupt or legacy-plaintext
// access/refresh token that cannot be decrypted. GetAllUsers is a metadata-only
// listing and must not depend on token decryption. Regression test for the
// admin "failed to load users" HTTP 500 ("illegal base64 data" decrypt error).
func TestUserRepository_GetAllUsers_UndecryptableToken(t *testing.T) {
	pool, cleanup := setupTestDB(t)
	defer cleanup()

	repo := newTestUserRepository(t, pool)
	ctx := context.Background()

	// Insert a user whose stored token is not valid encrypted/base64 data,
	// mimicking a legacy plaintext token in production.
	_, err := pool.Exec(ctx, `
		INSERT INTO users (twitch_id, auth_provider, username, display_name, profile_image_url, access_token, refresh_token, token_expires_at)
		VALUES ($1, 'twitch', 'legacytoken', 'LegacyToken', 'https://example.com/a.png', 'oauth:not-base64!', 'oauth:also-bad!', $2)
	`, "555111", time.Now().Add(24*time.Hour))
	if err != nil {
		t.Fatalf("Failed to insert user with corrupt token: %v", err)
	}

	users, err := repo.GetAllUsers(ctx)
	if err != nil {
		t.Fatalf("GetAllUsers() error = %v, want nil", err)
	}
	if len(users) != 1 {
		t.Fatalf("GetAllUsers() returned %d users, want 1", len(users))
	}
	if users[0].Username != "legacytoken" {
		t.Errorf("GetAllUsers() Username = %q, want %q", users[0].Username, "legacytoken")
	}
}

func TestUserRepository_GetByID(t *testing.T) {
	pool, cleanup := setupTestDB(t)
	defer cleanup()

	repo := newTestUserRepository(t, pool)
	ctx := context.Background()

	// Create a test user
	twitchID := "123456789"
	testUser := &models.User{
		TwitchID:        &twitchID,
		AuthProvider:    "twitch",
		Username:        "testuser",
		DisplayName:     "TestUser",
		ProfileImageURL: "https://example.com/avatar.png",
		AccessToken:     "encrypted_access_token",
		RefreshToken:    "encrypted_refresh_token",
		TokenExpiresAt:  time.Now().Add(24 * time.Hour),
	}

	err := repo.Create(ctx, testUser)
	if err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}

	tests := []struct {
		name    string
		id      string
		wantErr bool
		wantNil bool
	}{
		{
			name:    "existing user",
			id:      testUser.ID,
			wantErr: false,
			wantNil: false,
		},
		{
			name:    "non-existent user",
			id:      "550e8400-e29b-41d4-a716-446655440000",
			wantErr: true,
			wantNil: true,
		},
		{
			name:    "invalid UUID",
			id:      "invalid-uuid",
			wantErr: true,
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user, err := repo.GetByID(ctx, tt.id)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetByID() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantNil && user != nil {
				t.Error("GetByID() should return nil user")
			}

			if !tt.wantErr && !tt.wantNil {
				if user.ID != testUser.ID {
					t.Errorf("GetByID() ID = %v, want %v", user.ID, testUser.ID)
				}
				if (user.TwitchID == nil && testUser.TwitchID != nil) || (user.TwitchID != nil && testUser.TwitchID == nil) {
					t.Errorf("GetByID() TwitchID = %v, want %v", user.TwitchID, testUser.TwitchID)
				} else if user.TwitchID != nil && testUser.TwitchID != nil && *user.TwitchID != *testUser.TwitchID {
					t.Errorf("GetByID() TwitchID = %v, want %v", *user.TwitchID, *testUser.TwitchID)
				}
				if user.Username != testUser.Username {
					t.Errorf("GetByID() Username = %v, want %v", user.Username, testUser.Username)
				}
			}
		})
	}
}

func TestUserRepository_GetByTwitchID(t *testing.T) {
	pool, cleanup := setupTestDB(t)
	defer cleanup()

	repo := newTestUserRepository(t, pool)
	ctx := context.Background()

	// Create a test user
	twitchID := "123456"
	testUser := &models.User{
		TwitchID:        &twitchID,
		AuthProvider:    "twitch",
		Username:        "testuser",
		DisplayName:     "TestUser",
		ProfileImageURL: "https://example.com/avatar.png",
		AccessToken:     "encrypted_access_token",
		RefreshToken:    "encrypted_refresh_token",
		TokenExpiresAt:  time.Now().Add(24 * time.Hour),
	}

	err := repo.Create(ctx, testUser)
	if err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}

	tests := []struct {
		name     string
		twitchID string
		wantErr  bool
		wantNil  bool
	}{
		{
			name:     "existing user",
			twitchID: "123456",
			wantErr:  false,
			wantNil:  false,
		},
		{
			name:     "non-existent user",
			twitchID: "999999",
			wantErr:  true,
			wantNil:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user, err := repo.GetByTwitchID(ctx, tt.twitchID)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetByTwitchID() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantNil && user != nil {
				t.Error("GetByTwitchID() should return nil user")
			}

			if !tt.wantErr && !tt.wantNil {
				if (user.TwitchID == nil && testUser.TwitchID != nil) || (user.TwitchID != nil && testUser.TwitchID == nil) {
					t.Errorf("GetByTwitchID() TwitchID = %v, want %v", user.TwitchID, testUser.TwitchID)
				} else if user.TwitchID != nil && testUser.TwitchID != nil && *user.TwitchID != *testUser.TwitchID {
					t.Errorf("GetByTwitchID() TwitchID = %v, want %v", *user.TwitchID, *testUser.TwitchID)
				}
			}
		})
	}
}

func TestUserRepository_Update(t *testing.T) {
	pool, cleanup := setupTestDB(t)
	defer cleanup()

	repo := newTestUserRepository(t, pool)
	ctx := context.Background()

	// Create a test user
	twitchID := "123456"
	testUser := &models.User{
		TwitchID:        &twitchID,
		AuthProvider:    "twitch",
		Username:        "testuser",
		DisplayName:     "TestUser",
		ProfileImageURL: "https://example.com/avatar.png",
		AccessToken:     "encrypted_access_token",
		RefreshToken:    "encrypted_refresh_token",
		TokenExpiresAt:  time.Now().Add(24 * time.Hour),
	}

	err := repo.Create(ctx, testUser)
	if err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}

	tests := []struct {
		name    string
		setup   func() *models.User
		wantErr bool
	}{
		{
			name: "successful update",
			setup: func() *models.User {
				user := *testUser
				user.DisplayName = "UpdatedDisplayName"
				user.ProfileImageURL = "https://example.com/new-avatar.png"
				return &user
			},
			wantErr: false,
		},
		{
			name: "non-existent user",
			setup: func() *models.User {
				user := *testUser
				user.ID = "550e8400-e29b-41d4-a716-446655440000"
				return &user
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user := tt.setup()
			err := repo.Update(ctx, user)
			if (err != nil) != tt.wantErr {
				t.Errorf("Update() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				// Verify update
				updated, err := repo.GetByID(ctx, user.ID)
				if err != nil {
					t.Fatalf("Failed to fetch updated user: %v", err)
				}
				if updated.DisplayName != user.DisplayName {
					t.Errorf("Update() DisplayName = %v, want %v", updated.DisplayName, user.DisplayName)
				}
			}
		})
	}
}

func TestUserRepository_Delete(t *testing.T) {
	pool, cleanup := setupTestDB(t)
	defer cleanup()

	repo := newTestUserRepository(t, pool)
	ctx := context.Background()

	tests := []struct {
		name    string
		setup   func() string
		wantErr bool
	}{
		{
			name: "successful delete",
			setup: func() string {
				twitchID := "123456"
				user := &models.User{
					TwitchID:       &twitchID,
					AuthProvider:   "twitch",
					Username:       "deleteuser",
					DisplayName:    "DeleteUser",
					AccessToken:    "encrypted_access_token",
					RefreshToken:   "encrypted_refresh_token",
					TokenExpiresAt: time.Now().Add(24 * time.Hour),
				}
				err := repo.Create(ctx, user)
				if err != nil {
					t.Fatalf("Failed to create test user: %v", err)
				}
				return user.ID
			},
			wantErr: false,
		},
		{
			name: "non-existent user",
			setup: func() string {
				return "550e8400-e29b-41d4-a716-446655440000"
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id := tt.setup()
			err := repo.Delete(ctx, id)
			if (err != nil) != tt.wantErr {
				t.Errorf("Delete() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				// Verify deletion
				_, err := repo.GetByID(ctx, id)
				if err == nil {
					t.Error("Delete() did not delete user")
				}
			}
		})
	}
}

func TestUserRepository_UpdateTokens(t *testing.T) {
	pool, cleanup := setupTestDB(t)
	defer cleanup()

	repo := newTestUserRepository(t, pool)
	ctx := context.Background()

	// Create a test user
	twitchID := "123456"
	testUser := &models.User{
		TwitchID:       &twitchID,
		AuthProvider:   "twitch",
		Username:       "testuser",
		DisplayName:    "TestUser",
		AccessToken:    "old_encrypted_access_token",
		RefreshToken:   "old_encrypted_refresh_token",
		TokenExpiresAt: time.Now().Add(1 * time.Hour),
	}

	err := repo.Create(ctx, testUser)
	if err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}

	tests := []struct {
		name           string
		userID         string
		accessToken    string
		refreshToken   string
		tokenExpiresAt time.Time
		wantErr        bool
	}{
		{
			name:           "successful token update",
			userID:         testUser.ID,
			accessToken:    "new_encrypted_access_token",
			refreshToken:   "new_encrypted_refresh_token",
			tokenExpiresAt: time.Now().Add(24 * time.Hour),
			wantErr:        false,
		},
		{
			name:           "non-existent user",
			userID:         "550e8400-e29b-41d4-a716-446655440000",
			accessToken:    "new_access_token",
			refreshToken:   "new_refresh_token",
			tokenExpiresAt: time.Now().Add(24 * time.Hour),
			wantErr:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := repo.UpdateTokens(ctx, tt.userID, tt.accessToken, tt.refreshToken, tt.tokenExpiresAt)
			if (err != nil) != tt.wantErr {
				t.Errorf("UpdateTokens() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				// Verify tokens were updated
				user, err := repo.GetByID(ctx, tt.userID)
				if err != nil {
					t.Fatalf("Failed to fetch user: %v", err)
				}
				if user.AccessToken != tt.accessToken {
					t.Errorf("UpdateTokens() AccessToken = %v, want %v", user.AccessToken, tt.accessToken)
				}
				if user.RefreshToken != tt.refreshToken {
					t.Errorf("UpdateTokens() RefreshToken = %v, want %v", user.RefreshToken, tt.refreshToken)
				}
			}
		})
	}
}
