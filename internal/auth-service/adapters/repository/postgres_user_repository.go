package repository

import (
	"context"
	"errors"
	"time"

	"github.com/caesar/all-chat/internal/auth-service/core/domain"
	"github.com/caesar/all-chat/internal/auth-service/core/ports"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type postgresUserRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresUserRepository(pool *pgxpool.Pool) ports.UserRepository {
	return &postgresUserRepository{pool: pool}
}

func (r *postgresUserRepository) GetByTwitchID(ctx context.Context, twitchID string) (*domain.User, error) {
	query := `
		SELECT id, twitch_id, username, display_name, avatar_url,
		       access_token_encrypted, refresh_token_encrypted, token_expires_at,
		       created_at, updated_at, last_login_at
		FROM users
		WHERE twitch_id = $1
	`

	var user domain.User
	err := r.pool.QueryRow(ctx, query, twitchID).Scan(
		&user.ID,
		&user.TwitchID,
		&user.Username,
		&user.DisplayName,
		&user.AvatarURL,
		&user.AccessTokenEncrypted,
		&user.RefreshTokenEncrypted,
		&user.TokenExpiresAt,
		&user.CreatedAt,
		&user.UpdatedAt,
		&user.LastLoginAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("user not found")
		}
		return nil, err
	}

	return &user, nil
}

func (r *postgresUserRepository) GetByID(ctx context.Context, id string) (*domain.User, error) {
	query := `
		SELECT id, twitch_id, username, display_name, avatar_url,
		       access_token_encrypted, refresh_token_encrypted, token_expires_at,
		       created_at, updated_at, last_login_at
		FROM users
		WHERE id = $1
	`

	var user domain.User
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&user.ID,
		&user.TwitchID,
		&user.Username,
		&user.DisplayName,
		&user.AvatarURL,
		&user.AccessTokenEncrypted,
		&user.RefreshTokenEncrypted,
		&user.TokenExpiresAt,
		&user.CreatedAt,
		&user.UpdatedAt,
		&user.LastLoginAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("user not found")
		}
		return nil, err
	}

	return &user, nil
}

func (r *postgresUserRepository) Create(ctx context.Context, user *domain.User) error {
	query := `
		INSERT INTO users (id, twitch_id, username, display_name, avatar_url,
		                   access_token_encrypted, refresh_token_encrypted, token_expires_at,
		                   created_at, updated_at, last_login_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`

	_, err := r.pool.Exec(ctx, query,
		user.ID,
		user.TwitchID,
		user.Username,
		user.DisplayName,
		user.AvatarURL,
		user.AccessTokenEncrypted,
		user.RefreshTokenEncrypted,
		user.TokenExpiresAt,
		user.CreatedAt,
		user.UpdatedAt,
		user.LastLoginAt,
	)

	return err
}

func (r *postgresUserRepository) Update(ctx context.Context, user *domain.User) error {
	query := `
		UPDATE users
		SET username = $2,
		    display_name = $3,
		    avatar_url = $4,
		    access_token_encrypted = $5,
		    refresh_token_encrypted = $6,
		    token_expires_at = $7,
		    updated_at = $8,
		    last_login_at = $9
		WHERE id = $1
	`

	result, err := r.pool.Exec(ctx, query,
		user.ID,
		user.Username,
		user.DisplayName,
		user.AvatarURL,
		user.AccessTokenEncrypted,
		user.RefreshTokenEncrypted,
		user.TokenExpiresAt,
		time.Now(),
		user.LastLoginAt,
	)

	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return errors.New("user not found")
	}

	return nil
}

func (r *postgresUserRepository) UpdateTokens(ctx context.Context, userID string, accessToken, refreshToken string, expiresAt time.Time) error {
	query := `
		UPDATE users
		SET access_token_encrypted = $2,
		    refresh_token_encrypted = $3,
		    token_expires_at = $4,
		    updated_at = $5
		WHERE id = $1
	`

	result, err := r.pool.Exec(ctx, query, userID, accessToken, refreshToken, expiresAt, time.Now())
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return errors.New("user not found")
	}

	return nil
}
