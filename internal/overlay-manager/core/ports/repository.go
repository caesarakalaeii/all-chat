package ports

import (
	"context"

	"github.com/caesar/all-chat/internal/overlay-manager/core/domain"
)

type OverlayRepository interface {
	// Overlay operations
	GetByID(ctx context.Context, id string) (*domain.Overlay, error)
	GetByUserID(ctx context.Context, userID string) ([]*domain.Overlay, error)
	Create(ctx context.Context, overlay *domain.Overlay) error
	Update(ctx context.Context, overlay *domain.Overlay) error
	Delete(ctx context.Context, id string) error

	// Config operations
	GetConfig(ctx context.Context, overlayID string) (*domain.OverlayConfig, error)
	CreateConfig(ctx context.Context, config *domain.OverlayConfig) error
	UpdateConfig(ctx context.Context, config *domain.OverlayConfig) error
}
