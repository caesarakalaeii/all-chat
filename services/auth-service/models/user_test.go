package models

import (
	"testing"
	"time"
)

func TestUser_Validate(t *testing.T) {
	tests := []struct {
		name    string
		user    User
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid user",
			user: User{
				ID:              "550e8400-e29b-41d4-a716-446655440000",
				TwitchID:        "123456",
				Username:        "testuser",
				DisplayName:     "TestUser",
				ProfileImageURL: "https://example.com/avatar.png",
				Email:           "test@example.com",
				AccessToken:     "encrypted_access_token",
				RefreshToken:    "encrypted_refresh_token",
				TokenExpiresAt:  time.Now().Add(24 * time.Hour),
				CreatedAt:       time.Now(),
				UpdatedAt:       time.Now(),
			},
			wantErr: false,
		},
		{
			name: "missing twitch_id",
			user: User{
				ID:           "550e8400-e29b-41d4-a716-446655440000",
				Username:     "testuser",
				DisplayName:  "TestUser",
				AccessToken:  "encrypted_access_token",
				RefreshToken: "encrypted_refresh_token",
			},
			wantErr: true,
			errMsg:  "twitch_id is required",
		},
		{
			name: "missing username",
			user: User{
				ID:           "550e8400-e29b-41d4-a716-446655440000",
				TwitchID:     "123456",
				DisplayName:  "TestUser",
				AccessToken:  "encrypted_access_token",
				RefreshToken: "encrypted_refresh_token",
			},
			wantErr: true,
			errMsg:  "username is required",
		},
		{
			name: "missing display_name",
			user: User{
				ID:           "550e8400-e29b-41d4-a716-446655440000",
				TwitchID:     "123456",
				Username:     "testuser",
				AccessToken:  "encrypted_access_token",
				RefreshToken: "encrypted_refresh_token",
			},
			wantErr: true,
			errMsg:  "display_name is required",
		},
		{
			name: "missing access_token",
			user: User{
				ID:           "550e8400-e29b-41d4-a716-446655440000",
				TwitchID:     "123456",
				Username:     "testuser",
				DisplayName:  "TestUser",
				RefreshToken: "encrypted_refresh_token",
			},
			wantErr: true,
			errMsg:  "access_token is required",
		},
		{
			name: "missing refresh_token",
			user: User{
				ID:          "550e8400-e29b-41d4-a716-446655440000",
				TwitchID:    "123456",
				Username:    "testuser",
				DisplayName: "TestUser",
				AccessToken: "encrypted_access_token",
			},
			wantErr: true,
			errMsg:  "refresh_token is required",
		},
		{
			name: "username too short",
			user: User{
				ID:           "550e8400-e29b-41d4-a716-446655440000",
				TwitchID:     "123456",
				Username:     "ab",
				DisplayName:  "TestUser",
				AccessToken:  "encrypted_access_token",
				RefreshToken: "encrypted_refresh_token",
			},
			wantErr: true,
			errMsg:  "username must be between 3 and 50 characters",
		},
		{
			name: "invalid email format",
			user: User{
				ID:           "550e8400-e29b-41d4-a716-446655440000",
				TwitchID:     "123456",
				Username:     "testuser",
				DisplayName:  "TestUser",
				Email:        "invalid-email",
				AccessToken:  "encrypted_access_token",
				RefreshToken: "encrypted_refresh_token",
			},
			wantErr: true,
			errMsg:  "invalid email format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.user.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("User.Validate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil && tt.errMsg != "" {
				// Check if error message contains the expected message (wrapped errors include prefix)
				expected := "validation error: " + tt.errMsg
				if err.Error() != expected {
					t.Errorf("User.Validate() error message = %v, want %v", err.Error(), expected)
				}
			}
		})
	}
}

func TestUser_EncryptTokens(t *testing.T) {
	tests := []struct {
		name         string
		accessToken  string
		refreshToken string
		secret       string
		wantErr      bool
	}{
		{
			name:         "successful encryption",
			accessToken:  "test_access_token_12345",
			refreshToken: "test_refresh_token_67890",
			secret:       "test-secret-key-32-chars-long!",
			wantErr:      false,
		},
		{
			name:         "empty access token",
			accessToken:  "",
			refreshToken: "test_refresh_token_67890",
			secret:       "test-secret-key-32-chars-long!",
			wantErr:      true,
		},
		{
			name:         "empty refresh token",
			accessToken:  "test_access_token_12345",
			refreshToken: "",
			secret:       "test-secret-key-32-chars-long!",
			wantErr:      true,
		},
		{
			name:         "empty secret",
			accessToken:  "test_access_token_12345",
			refreshToken: "test_refresh_token_67890",
			secret:       "",
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user := &User{}
			err := user.EncryptTokens(tt.accessToken, tt.refreshToken, tt.secret)
			if (err != nil) != tt.wantErr {
				t.Errorf("User.EncryptTokens() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				// Verify tokens were encrypted (should not match original)
				if user.AccessToken == tt.accessToken {
					t.Error("User.EncryptTokens() did not encrypt access token")
				}
				if user.RefreshToken == tt.refreshToken {
					t.Error("User.EncryptTokens() did not encrypt refresh token")
				}
				// Verify tokens are not empty
				if user.AccessToken == "" || user.RefreshToken == "" {
					t.Error("User.EncryptTokens() resulted in empty tokens")
				}
			}
		})
	}
}

func TestUser_DecryptTokens(t *testing.T) {
	secret := "test-secret-key-32-chars-long!"
	originalAccess := "test_access_token_12345"
	originalRefresh := "test_refresh_token_67890"

	tests := []struct {
		name         string
		setupUser    func() *User
		secret       string
		wantAccess   string
		wantRefresh  string
		wantErr      bool
	}{
		{
			name: "successful decryption",
			setupUser: func() *User {
				user := &User{}
				err := user.EncryptTokens(originalAccess, originalRefresh, secret)
				if err != nil {
					t.Fatalf("Failed to setup test: %v", err)
				}
				return user
			},
			secret:      secret,
			wantAccess:  originalAccess,
			wantRefresh: originalRefresh,
			wantErr:     false,
		},
		{
			name: "wrong secret",
			setupUser: func() *User {
				user := &User{}
				err := user.EncryptTokens(originalAccess, originalRefresh, secret)
				if err != nil {
					t.Fatalf("Failed to setup test: %v", err)
				}
				return user
			},
			secret:  "wrong-secret-key-32-chars-long",
			wantErr: true,
		},
		{
			name: "corrupted access token",
			setupUser: func() *User {
				return &User{
					AccessToken:  "corrupted_token",
					RefreshToken: "valid_token",
				}
			},
			secret:  secret,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user := tt.setupUser()
			access, refresh, err := user.DecryptTokens(tt.secret)
			if (err != nil) != tt.wantErr {
				t.Errorf("User.DecryptTokens() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if access != tt.wantAccess {
					t.Errorf("User.DecryptTokens() access = %v, want %v", access, tt.wantAccess)
				}
				if refresh != tt.wantRefresh {
					t.Errorf("User.DecryptTokens() refresh = %v, want %v", refresh, tt.wantRefresh)
				}
			}
		})
	}
}

func TestUser_EncryptDecrypt_RoundTrip(t *testing.T) {
	secret := "test-secret-key-32-chars-long!"
	originalAccess := "my_access_token_abc123"
	originalRefresh := "my_refresh_token_xyz789"

	user := &User{}

	// Encrypt
	err := user.EncryptTokens(originalAccess, originalRefresh, secret)
	if err != nil {
		t.Fatalf("EncryptTokens() failed: %v", err)
	}

	// Decrypt
	access, refresh, err := user.DecryptTokens(secret)
	if err != nil {
		t.Fatalf("DecryptTokens() failed: %v", err)
	}

	// Verify round-trip
	if access != originalAccess {
		t.Errorf("Round-trip failed: access token = %v, want %v", access, originalAccess)
	}
	if refresh != originalRefresh {
		t.Errorf("Round-trip failed: refresh token = %v, want %v", refresh, originalRefresh)
	}
}
