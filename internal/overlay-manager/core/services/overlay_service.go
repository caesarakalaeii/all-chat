package services

import (
	"context"
	"errors"
	"time"

	"github.com/caesar/all-chat/internal/overlay-manager/core/domain"
	"github.com/caesar/all-chat/internal/overlay-manager/core/ports"
	"github.com/google/uuid"
)

var (
	ErrOverlayNotFound  = errors.New("overlay not found")
	ErrUnauthorized     = errors.New("unauthorized access to overlay")
	ErrInvalidInput     = errors.New("invalid input")
)

type overlayService struct {
	repo ports.OverlayRepository
}

func NewOverlayService(repo ports.OverlayRepository) ports.OverlayService {
	return &overlayService{repo: repo}
}

func (s *overlayService) CreateOverlay(ctx context.Context, userID, name, description, twitchChannel string) (*domain.Overlay, *domain.OverlayConfig, error) {
	if name == "" || twitchChannel == "" {
		return nil, nil, ErrInvalidInput
	}

	now := time.Now()

	// Create overlay
	overlay := &domain.Overlay{
		ID:          uuid.New().String(),
		UserID:      userID,
		Name:        name,
		Description: description,
		IsActive:    true,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := s.repo.Create(ctx, overlay); err != nil {
		return nil, nil, err
	}

	// Create default config
	config := &domain.OverlayConfig{
		ID:            uuid.New().String(),
		OverlayID:     overlay.ID,
		TwitchChannel: twitchChannel,
		Enable7TV:     true,
		EnableBTTV:    true,
		EnableFFZ:     false,
		DisplaySettings: domain.DisplaySettings{
			MaxMessages:     50,
			MessageDuration: 10,
			FontSize:        16,
			Animation:       "slide",
			Theme:           "dark",
		},
		FilterSettings: domain.FilterSettings{
			BlockedUsers:   []string{},
			BlockedWords:   []string{},
			SubscriberOnly: false,
			ModeratorOnly:  false,
			MinChatDelay:   0,
		},
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := s.repo.CreateConfig(ctx, config); err != nil {
		return nil, nil, err
	}

	return overlay, config, nil
}

func (s *overlayService) GetOverlay(ctx context.Context, overlayID, userID string) (*domain.Overlay, error) {
	overlay, err := s.repo.GetByID(ctx, overlayID)
	if err != nil {
		return nil, ErrOverlayNotFound
	}

	if overlay.UserID != userID {
		return nil, ErrUnauthorized
	}

	return overlay, nil
}

func (s *overlayService) GetUserOverlays(ctx context.Context, userID string) ([]*domain.Overlay, error) {
	return s.repo.GetByUserID(ctx, userID)
}

func (s *overlayService) UpdateOverlay(ctx context.Context, overlayID, userID, name, description string, isActive bool) error {
	overlay, err := s.repo.GetByID(ctx, overlayID)
	if err != nil {
		return ErrOverlayNotFound
	}

	if overlay.UserID != userID {
		return ErrUnauthorized
	}

	overlay.Name = name
	overlay.Description = description
	overlay.IsActive = isActive
	overlay.UpdatedAt = time.Now()

	return s.repo.Update(ctx, overlay)
}

func (s *overlayService) DeleteOverlay(ctx context.Context, overlayID, userID string) error {
	overlay, err := s.repo.GetByID(ctx, overlayID)
	if err != nil {
		return ErrOverlayNotFound
	}

	if overlay.UserID != userID {
		return ErrUnauthorized
	}

	return s.repo.Delete(ctx, overlayID)
}

func (s *overlayService) GetOverlayConfig(ctx context.Context, overlayID, userID string) (*domain.OverlayConfig, error) {
	// Verify ownership
	overlay, err := s.repo.GetByID(ctx, overlayID)
	if err != nil {
		return nil, ErrOverlayNotFound
	}

	if overlay.UserID != userID {
		return nil, ErrUnauthorized
	}

	return s.repo.GetConfig(ctx, overlayID)
}

func (s *overlayService) UpdateOverlayConfig(ctx context.Context, overlayID, userID string, config *domain.OverlayConfig) error {
	// Verify ownership
	overlay, err := s.repo.GetByID(ctx, overlayID)
	if err != nil {
		return ErrOverlayNotFound
	}

	if overlay.UserID != userID {
		return ErrUnauthorized
	}

	config.OverlayID = overlayID
	config.UpdatedAt = time.Now()

	return s.repo.UpdateConfig(ctx, config)
}
