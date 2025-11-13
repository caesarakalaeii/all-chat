package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/caesar/all-chat/services/auth-service/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// UserRepository handles user data persistence
type UserRepository struct {
	db *pgxpool.Pool
}

// NewUserRepository creates a new user repository
func NewUserRepository(db *pgxpool.Pool) *UserRepository {
	return &UserRepository{db: db}
}

// Create inserts a new user
func (r *UserRepository) Create(ctx context.Context, user *models.User) error {
	user.ID = uuid.New().String()
	user.CreatedAt = time.Now()
	user.UpdatedAt = time.Now()

	query := `
		INSERT INTO users (
			id, twitch_id, username, display_name, email, profile_image_url,
			access_token, refresh_token, token_expires_at, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`

	_, err := r.db.Exec(ctx, query,
		user.ID, user.TwitchID, user.Username, user.DisplayName, user.Email,
		user.ProfileImageURL, user.AccessToken, user.RefreshToken,
		user.TokenExpiresAt, user.CreatedAt, user.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}

	return nil
}

// GetByTwitchID retrieves a user by Twitch ID
func (r *UserRepository) GetByTwitchID(ctx context.Context, twitchID string) (*models.User, error) {
	user := &models.User{}

	query := `
		SELECT id, twitch_id, username, display_name, email, profile_image_url,
			   access_token, refresh_token, token_expires_at, created_at, updated_at
		FROM users
		WHERE twitch_id = $1
	`

	err := r.db.QueryRow(ctx, query, twitchID).Scan(
		&user.ID, &user.TwitchID, &user.Username, &user.DisplayName, &user.Email,
		&user.ProfileImageURL, &user.AccessToken, &user.RefreshToken,
		&user.TokenExpiresAt, &user.CreatedAt, &user.UpdatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}

	return user, nil
}

// GetByID retrieves a user by ID
func (r *UserRepository) GetByID(ctx context.Context, id string) (*models.User, error) {
	user := &models.User{}

	query := `
		SELECT id, twitch_id, username, display_name, email, profile_image_url,
			   access_token, refresh_token, token_expires_at, created_at, updated_at
		FROM users
		WHERE id = $1
	`

	err := r.db.QueryRow(ctx, query, id).Scan(
		&user.ID, &user.TwitchID, &user.Username, &user.DisplayName, &user.Email,
		&user.ProfileImageURL, &user.AccessToken, &user.RefreshToken,
		&user.TokenExpiresAt, &user.CreatedAt, &user.UpdatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}

	return user, nil
}

// Update updates an existing user
func (r *UserRepository) Update(ctx context.Context, user *models.User) error {
	user.UpdatedAt = time.Now()

	query := `
		UPDATE users
		SET username = $2, display_name = $3, email = $4, profile_image_url = $5,
			access_token = $6, refresh_token = $7, token_expires_at = $8, updated_at = $9
		WHERE id = $1
	`

	result, err := r.db.Exec(ctx, query,
		user.ID, user.Username, user.DisplayName, user.Email, user.ProfileImageURL,
		user.AccessToken, user.RefreshToken, user.TokenExpiresAt, user.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to update user: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("user not found")
	}

	return nil
}

// UpdateTokens updates only the OAuth tokens for a user
func (r *UserRepository) UpdateTokens(ctx context.Context, userID, accessToken, refreshToken string, expiresAt time.Time) error {
	query := `
		UPDATE users
		SET access_token = $2, refresh_token = $3, token_expires_at = $4, updated_at = $5
		WHERE id = $1
	`

	result, err := r.db.Exec(ctx, query, userID, accessToken, refreshToken, expiresAt, time.Now())
	if err != nil {
		return fmt.Errorf("failed to update tokens: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("user not found")
	}

	return nil
}
