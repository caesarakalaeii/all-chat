package oauth

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
	"golang.org/x/oauth2"
)

// TokenStore defines the interface for storing and retrieving OAuth tokens
type TokenStore interface {
	SaveToken(ctx context.Context, userID, channelID string, token *oauth2.Token) error
	GetToken(ctx context.Context, userID, channelID string) (*oauth2.Token, error)
	DeleteToken(ctx context.Context, userID, channelID string) error
}

// PostgresTokenStore implements TokenStore using PostgreSQL
type PostgresTokenStore struct {
	db     *pgxpool.Pool
	logger *zap.Logger
}

// NewPostgresTokenStore creates a new PostgreSQL token store
func NewPostgresTokenStore(db *pgxpool.Pool, logger *zap.Logger) *PostgresTokenStore {
	return &PostgresTokenStore{
		db:     db,
		logger: logger,
	}
}

// SaveToken saves an OAuth token to the database
func (s *PostgresTokenStore) SaveToken(ctx context.Context, userID, channelID string, token *oauth2.Token) error {
	query := `
		INSERT INTO youtube_oauth_tokens (user_id, channel_id, access_token, refresh_token, token_type, expiry, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, NOW())
		ON CONFLICT (user_id, channel_id)
		DO UPDATE SET
			access_token = EXCLUDED.access_token,
			refresh_token = EXCLUDED.refresh_token,
			token_type = EXCLUDED.token_type,
			expiry = EXCLUDED.expiry,
			updated_at = NOW()
	`

	_, err := s.db.Exec(ctx, query,
		userID,
		channelID,
		token.AccessToken,
		token.RefreshToken,
		token.TokenType,
		token.Expiry,
	)

	if err != nil {
		s.logger.Error("Failed to save OAuth token",
			zap.String("user_id", userID),
			zap.String("channel_id", channelID),
			zap.Error(err),
		)
		return fmt.Errorf("failed to save token: %w", err)
	}

	s.logger.Info("Saved OAuth token",
		zap.String("user_id", userID),
		zap.String("channel_id", channelID),
	)

	return nil
}

// GetToken retrieves an OAuth token from the database
func (s *PostgresTokenStore) GetToken(ctx context.Context, userID, channelID string) (*oauth2.Token, error) {
	query := `
		SELECT access_token, refresh_token, token_type, expiry
		FROM youtube_oauth_tokens
		WHERE user_id = $1 AND channel_id = $2
	`

	var token oauth2.Token
	var expiry time.Time

	err := s.db.QueryRow(ctx, query, userID, channelID).Scan(
		&token.AccessToken,
		&token.RefreshToken,
		&token.TokenType,
		&expiry,
	)

	if err != nil {
		s.logger.Error("Failed to get OAuth token",
			zap.String("user_id", userID),
			zap.String("channel_id", channelID),
			zap.Error(err),
		)
		return nil, fmt.Errorf("failed to get token: %w", err)
	}

	token.Expiry = expiry

	return &token, nil
}

// DeleteToken removes an OAuth token from the database
func (s *PostgresTokenStore) DeleteToken(ctx context.Context, userID, channelID string) error {
	query := `
		DELETE FROM youtube_oauth_tokens
		WHERE user_id = $1 AND channel_id = $2
	`

	_, err := s.db.Exec(ctx, query, userID, channelID)
	if err != nil {
		s.logger.Error("Failed to delete OAuth token",
			zap.String("user_id", userID),
			zap.String("channel_id", channelID),
			zap.Error(err),
		)
		return fmt.Errorf("failed to delete token: %w", err)
	}

	s.logger.Info("Deleted OAuth token",
		zap.String("user_id", userID),
		zap.String("channel_id", channelID),
	)

	return nil
}
