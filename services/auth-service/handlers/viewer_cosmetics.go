package handlers

import (
	"context"
	"encoding/json"
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

// HandleGetCosmetics handles GET /viewer/cosmetics.
// Returns the current cosmetics for the authenticated viewer.
func (h *ViewerCosmeticsHandler) HandleGetCosmetics(c *gin.Context) {
	viewerIDStr, exists := c.Get("viewer_id")
	if !exists || viewerIDStr == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	viewerID, err := uuid.Parse(viewerIDStr.(string))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid viewer ID"})
		return
	}

	cosmetics, err := h.identityRepo.GetFullCosmetics(c.Request.Context(), viewerID)
	if err != nil {
		h.logger.Error("Failed to get cosmetics", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	if cosmetics == nil {
		c.JSON(http.StatusOK, gin.H{
			"name_color":      nil,
			"name_gradient":   nil,
			"avatar_frame_id": nil,
			"avatar_flair_id": nil,
		})
		return
	}

	// Parse name_gradient from raw JSON for the response
	var gradientResponse interface{}
	if len(cosmetics.NameGradient) > 0 && string(cosmetics.NameGradient) != "null" {
		var g interface{}
		if json.Unmarshal(cosmetics.NameGradient, &g) == nil {
			gradientResponse = g
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"name_color":      cosmetics.NameColor,
		"name_gradient":   gradientResponse,
		"avatar_frame_id": cosmetics.AvatarFrameID,
		"avatar_flair_id": cosmetics.AvatarFlairID,
	})
}

// HandlePatchCosmetics handles PATCH /viewer/cosmetics.
//
// Requires a viewer JWT with a non-empty viewer_id claim (Phase 28+ tokens).
// Tokens issued before Phase 28 have an empty viewer_id — those callers must
// re-authenticate via the extension sign-in flow to obtain a new Phase 28 token.
//
// Request body: {"name_color": "#rrggbb"} or {"name_gradient": {...}} (mutually exclusive)
// Response:     {"name_color": ..., "name_gradient": ...}
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
	UpsertViewerCosmetics(ctx context.Context, viewerID uuid.UUID, nameColor *string, nameGradient []byte, avatarFrameID *uuid.UUID, avatarFlairID *uuid.UUID) error
	// UpsertAvatarCosmetics updates only avatar_frame_id and avatar_flair_id, leaving
	// name_color and name_gradient untouched. Used for avatar-only PATCH requests.
	UpsertAvatarCosmetics(ctx context.Context, viewerID uuid.UUID, avatarFrameID *uuid.UUID, avatarFlairID *uuid.UUID) error
}

// NameGradientReq is the JSON shape for a gradient in the PATCH request body.
type NameGradientReq struct {
	Type   string   `json:"type"`
	Colors []string `json:"colors"`
	Angle  int      `json:"angle"`
}

// patchCosmeticsRaw is used as an intermediate type for JSON parsing.
// Using json.RawMessage for each field lets us distinguish absent (nil RawMessage)
// from explicitly null (RawMessage("null")) from a real value.
type patchCosmeticsRaw struct {
	NameColor     json.RawMessage `json:"name_color"`
	NameGradient  json.RawMessage `json:"name_gradient"`
	AvatarFrameID json.RawMessage `json:"avatar_frame_id"`
	AvatarFlairID json.RawMessage `json:"avatar_flair_id"`
}

// patchCosmeticsRequest is the expected request body for PATCH /viewer/cosmetics.
// Parsed from patchCosmeticsRaw to separate field-presence from field-value.
type patchCosmeticsRequest struct {
	NameColor     *string          // nil means "absent or explicit null"
	NameGradient  *NameGradientReq // nil means "absent or explicit null"
	AvatarFrameID *uuid.UUID       // nil means "absent or explicit null"
	AvatarFlairID *uuid.UUID       // nil means "absent or explicit null"

	// Presence flags — true when the field was present in the JSON body
	// (regardless of whether the value was null or non-null).
	nameColorPresent     bool
	nameGradientPresent  bool
	avatarFrameIDPresent bool
	avatarFlairIDPresent bool
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

	// Step 3: Parse request body using raw JSON to distinguish absent fields from null.
	var raw patchCosmeticsRaw
	if err := c.ShouldBindJSON(&raw); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	var req patchCosmeticsRequest

	// name_color: absent raw → nil / not present; "null" raw → nil + present; string → value + present
	if len(raw.NameColor) > 0 {
		req.nameColorPresent = true
		if string(raw.NameColor) != "null" {
			var nc string
			if err := json.Unmarshal(raw.NameColor, &nc); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
				return
			}
			req.NameColor = &nc
		}
	}

	// name_gradient: absent → nil / not present; "null" → nil + present; object → value + present
	if len(raw.NameGradient) > 0 {
		req.nameGradientPresent = true
		if string(raw.NameGradient) != "null" {
			var ng NameGradientReq
			if err := json.Unmarshal(raw.NameGradient, &ng); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
				return
			}
			req.NameGradient = &ng
		}
	}

	// avatar_frame_id: absent → nil / not present; "null" → nil + present; UUID string → value + present
	if len(raw.AvatarFrameID) > 0 {
		req.avatarFrameIDPresent = true
		if string(raw.AvatarFrameID) != "null" {
			var frameID uuid.UUID
			if err := json.Unmarshal(raw.AvatarFrameID, &frameID); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid avatar_frame_id"})
				return
			}
			req.AvatarFrameID = &frameID
		}
	}

	// avatar_flair_id: absent → nil / not present; "null" → nil + present; UUID string → value + present
	if len(raw.AvatarFlairID) > 0 {
		req.avatarFlairIDPresent = true
		if string(raw.AvatarFlairID) != "null" {
			var flairID uuid.UUID
			if err := json.Unmarshal(raw.AvatarFlairID, &flairID); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid avatar_flair_id"})
				return
			}
			req.AvatarFlairID = &flairID
		}
	}

	// Step 4: Validate name_color if non-null
	if req.NameColor != nil {
		if !hexColorRegex.MatchString(*req.NameColor) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "name_color must be a 7-character hex color (#rrggbb)"})
			return
		}
	}

	// Step 5: Validate gradient if present
	var nameGradientBytes []byte
	if req.NameGradient != nil {
		g := req.NameGradient

		// Premium gate: only premium viewers (or admins) may set gradients
		isPremiumVal, _ := c.Get("is_premium")
		isAdminVal, _ := c.Get("is_admin")
		if (isPremiumVal == nil || !isPremiumVal.(bool)) && (isAdminVal == nil || !isAdminVal.(bool)) {
			c.JSON(http.StatusForbidden, gin.H{"error": "gradient is a premium feature"})
			return
		}

		// Validate gradient type
		if g.Type != "linear" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "name_gradient.type must be \"linear\""})
			return
		}

		// Validate colors: 2-4 valid hex colors
		if len(g.Colors) < 2 || len(g.Colors) > 4 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "name_gradient.colors must have 2 to 4 entries"})
			return
		}
		for _, color := range g.Colors {
			if !hexColorRegex.MatchString(color) {
				c.JSON(http.StatusBadRequest, gin.H{"error": "each color in name_gradient.colors must be a 7-character hex color (#rrggbb)"})
				return
			}
		}

		// Validate angle: 0-360
		if g.Angle < 0 || g.Angle > 360 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "name_gradient.angle must be between 0 and 360"})
			return
		}

		// Mutual exclusion: gradient nullifies name_color
		req.NameColor = nil

		var marshalErr error
		nameGradientBytes, marshalErr = json.Marshal(g)
		if marshalErr != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process gradient"})
			return
		}
	} else if req.NameColor != nil {
		// Mutual exclusion: name_color nullifies gradient
		nameGradientBytes = nil
	}

	// Step 5b: Validate avatar frame/flair (premium gate + downgrade enforcement)
	isPremium := false
	if v, ok := c.Get("is_premium"); ok && v != nil {
		if b, ok := v.(bool); ok {
			isPremium = b
		}
	}
	isAdmin := false
	if v, ok := c.Get("is_admin"); ok && v != nil {
		if b, ok := v.(bool); ok {
			isAdmin = b
		}
	}
	hasAccess := isPremium || isAdmin

	var avatarFrameID *uuid.UUID
	var avatarFlairID *uuid.UUID

	if !hasAccess {
		// Non-premium gate: reject if viewer is trying to set a real (non-zero) frame or flair.
		if req.AvatarFrameID != nil && *req.AvatarFrameID != uuid.Nil {
			c.JSON(http.StatusForbidden, gin.H{"error": "avatar frames are a premium feature"})
			return
		}
		if req.AvatarFlairID != nil && *req.AvatarFlairID != uuid.Nil {
			c.JSON(http.StatusForbidden, gin.H{"error": "avatar flairs are a premium feature"})
			return
		}
		// Downgrade enforcement: always clear frame/flair for non-premium viewers,
		// regardless of whether they sent these fields. Pass &uuid.Nil as the
		// "clear" sentinel so the UPSERT writes NULL to the DB.
		nilUUID := uuid.Nil
		avatarFrameID = &nilUUID
		avatarFlairID = &nilUUID
	} else {
		// Premium viewer: pass through whatever was sent.
		// nil pointer = field absent in request body = UPSERT overwrites with NULL (v1.4 behavior).
		// &uuid.Nil = explicit clear sent as JSON null.
		// &<real UUID> = set to that frame/flair.
		avatarFrameID = req.AvatarFrameID
		avatarFlairID = req.AvatarFlairID
	}

	// Step 6: Upsert cosmetics in DB.
	// When the request explicitly includes name_color or name_gradient fields (even as null),
	// use the full upsert that can overwrite all four columns.
	// When only avatar fields are present (name_color and name_gradient both absent from the
	// JSON body), use a targeted UPDATE that leaves name_color and name_gradient untouched.
	// This prevents avatar-only saves from NULLing out a previously saved name color.
	nameFieldsProvided := req.nameColorPresent || req.nameGradientPresent
	if nameFieldsProvided {
		if err := repo.UpsertViewerCosmetics(c.Request.Context(), viewerID, req.NameColor, nameGradientBytes, avatarFrameID, avatarFlairID); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update cosmetics"})
			return
		}
	} else {
		if err := repo.UpsertAvatarCosmetics(c.Request.Context(), viewerID, avatarFrameID, avatarFlairID); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update cosmetics"})
			return
		}
	}

	// Step 7: Return updated values
	var gradientResponse interface{}
	if req.NameGradient != nil {
		gradientResponse = req.NameGradient
	}
	c.JSON(http.StatusOK, gin.H{
		"name_color":     req.NameColor,
		"name_gradient":  gradientResponse,
		"avatar_frame_id": req.AvatarFrameID,
		"avatar_flair_id": req.AvatarFlairID,
	})
}
