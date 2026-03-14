package handlers

import (
	"context"
	"fmt"
	"net/http"
	"regexp"

	"github.com/caesar/all-chat/services/auth-service/repository"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// hexColorRegex validates 7-character hex color strings (#rrggbb).
var hexColorRegex = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

// ViewerCosmeticsHandler handles viewer cosmetics updates.
type ViewerCosmeticsHandler struct {
	identityRepo *repository.ViewerIdentityRepository
	redis        *redis.Client
	logger       *zap.Logger
}

// NewViewerCosmeticsHandler creates a new ViewerCosmeticsHandler.
func NewViewerCosmeticsHandler(
	identityRepo *repository.ViewerIdentityRepository,
	redisClient *redis.Client,
	logger *zap.Logger,
) *ViewerCosmeticsHandler {
	return &ViewerCosmeticsHandler{
		identityRepo: identityRepo,
		redis:        redisClient,
		logger:       logger,
	}
}

// HandlePatchCosmetics handles PATCH /viewer/cosmetics.
//
// Requires a viewer JWT with a non-empty viewer_id claim (Phase 28+ tokens).
// Tokens issued before Phase 28 have an empty viewer_id — those callers must
// re-authenticate via the extension sign-in flow to obtain a new Phase 28 token.
//
// Request body: {"name_color": "#rrggbb"} or {"name_color": null}
// Response:     {"name_color": "#rrggbb"} or {"name_color": null}
func (h *ViewerCosmeticsHandler) HandlePatchCosmetics(c *gin.Context) {
	handlePatchCosmeticsLogic(c, h.identityRepo)

	// Invalidate Redis identity cache on success
	if c.Writer.Status() == http.StatusOK {
		platform, _ := c.Get("platform")
		platformUserID, _ := c.Get("platform_user_id")
		if platform != nil && platformUserID != nil {
			cacheKey := fmt.Sprintf("viewer:identity:%s:%s", platform, platformUserID)
			if err := h.redis.Del(context.Background(), cacheKey).Err(); err != nil {
				h.logger.Warn("Failed to invalidate viewer identity cache",
					zap.String("key", cacheKey),
					zap.Error(err),
				)
			}
		}
	}
}

// cosmeticsUpsertRepo is the minimal interface for the cosmetics handler's DB access.
// This enables unit testing with mock implementations.
type cosmeticsUpsertRepo interface {
	UpsertViewerCosmetics(ctx context.Context, viewerID uuid.UUID, nameColor *string) error
}

// patchCosmeticsRequest is the expected request body for PATCH /viewer/cosmetics.
// Using *string allows distinguishing between null and absent name_color.
type patchCosmeticsRequest struct {
	NameColor *string `json:"name_color"`
}

// handlePatchCosmeticsLogic contains the core business logic for PATCH cosmetics.
// Extracted to allow unit testing with a mock repository.
func handlePatchCosmeticsLogic(c *gin.Context, repo cosmeticsUpsertRepo) {
	// Step 1: Extract viewer_id from JWT claims (set by middleware)
	viewerIDVal, exists := c.Get("viewer_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "viewer identity not established — please sign in again"})
		return
	}

	viewerIDStr, ok := viewerIDVal.(string)
	if !ok || viewerIDStr == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "viewer identity not established — please sign in again"})
		return
	}

	// Step 2: Parse viewer_id string to uuid.UUID
	viewerID, err := uuid.Parse(viewerIDStr)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "viewer identity not established — please sign in again"})
		return
	}

	// Step 3: Parse request body
	var req patchCosmeticsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	// Step 4: Validate name_color if non-null
	if req.NameColor != nil {
		if !hexColorRegex.MatchString(*req.NameColor) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "name_color must be a 7-character hex color (#rrggbb)"})
			return
		}
	}

	// Step 5: Upsert cosmetics in DB
	if err := repo.UpsertViewerCosmetics(c.Request.Context(), viewerID, req.NameColor); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update cosmetics"})
		return
	}

	// Step 6: Return updated value
	c.JSON(http.StatusOK, gin.H{"name_color": req.NameColor})
}
