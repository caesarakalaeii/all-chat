package ports

import (
	"context"
	"time"

	"github.com/caesar/all-chat/internal/auth-service/core/domain"
)

type UserRepository interface {
	GetByTwitchID(ctx context.Context, twitchID string) (*domain.User, error)
	GetByID(ctx context.Context, id string) (*domain.User, error)
	Create(ctx context.Context, user *domain.User) error
	Update(ctx context.Context, user *domain.User) error
	UpdateTokens(ctx context.Context, userID string, accessToken, refreshToken string, expiresAt time.Time) error
}
