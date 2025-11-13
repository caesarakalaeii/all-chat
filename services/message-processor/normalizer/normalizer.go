package normalizer

import "github.com/caesar/all-chat/services/message-processor/models"

// Normalizer defines the interface for platform-specific normalizers
type Normalizer interface {
	Normalize(raw *models.RawChatMessage, overlayID string) (*models.UnifiedChatMessage, error)
}
