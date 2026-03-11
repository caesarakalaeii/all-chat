package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

type PremiumRepository struct {
	db     *pgxpool.Pool
	logger *zap.Logger
}

func NewPremiumRepository(db *pgxpool.Pool, logger *zap.Logger) *PremiumRepository {
	return &PremiumRepository{db: db, logger: logger}
}

func (r *PremiumRepository) UpdateUserPremium(ctx context.Context, userID string, isPremium bool) error {
	result, err := r.db.Exec(ctx,
		"UPDATE users SET is_premium = $1 WHERE id = $2",
		isPremium, userID)

	if err != nil {
		r.logger.Error("Failed to update premium status",
			zap.String("user_id", userID),
			zap.Bool("is_premium", isPremium),
			zap.Error(err))
		return fmt.Errorf("failed to update premium status: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("user not found: %s", userID)
	}

	r.logger.Info("Premium status updated",
		zap.String("user_id", userID),
		zap.Bool("is_premium", isPremium))

	return nil
}

func (r *PremiumRepository) IsPremium(ctx context.Context, userID string) (bool, error) {
	var isPremium bool
	err := r.db.QueryRow(ctx,
		"SELECT is_premium FROM users WHERE id = $1", userID).Scan(&isPremium)

	if err != nil {
		return false, fmt.Errorf("failed to check premium status: %w", err)
	}

	return isPremium, nil
}
