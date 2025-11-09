package ports

import (
	"context"

	"github.com/caesar/all-chat/internal/auth-service/core/domain"
	"github.com/caesar/all-chat/pkg/auth"
)

type AuthService interface {
	GetAuthURL(state string) string
	ExchangeCode(ctx context.Context, code string) (*domain.User, *auth.TokenPair, error)
	RefreshToken(ctx context.Context, refreshToken string) (*auth.TokenPair, error)
	GetUserInfo(ctx context.Context, userID string) (*domain.User, error)
	Logout(ctx context.Context, userID string) error
}
