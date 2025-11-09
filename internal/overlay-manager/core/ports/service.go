package ports

import (
	"context"

	"github.com/caesar/all-chat/internal/overlay-manager/core/domain"
)

type OverlayService interface {
	CreateOverlay(ctx context.Context, userID, name, description, twitchChannel string) (*domain.Overlay, *domain.OverlayConfig, error)
	GetOverlay(ctx context.Context, overlayID, userID string) (*domain.Overlay, error)
	GetUserOverlays(ctx context.Context, userID string) ([]*domain.Overlay, error)
	UpdateOverlay(ctx context.Context, overlayID, userID, name, description string, isActive bool) error
	DeleteOverlay(ctx context.Context, overlayID, userID string) error

	GetOverlayConfig(ctx context.Context, overlayID, userID string) (*domain.OverlayConfig, error)
	UpdateOverlayConfig(ctx context.Context, overlayID, userID string, config *domain.OverlayConfig) error
}
