// This file is part of All-Chat.
// Copyright (C) 2026 caesarakalaeii
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

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
	identityRepo    *repository.ViewerIdentityRepository
	linkedPlatforms linkedPlatformsGetter // always identityRepo in production; overridable in tests
	redis           *redis.Client
	logger          *zap.Logger
}

// NewViewerCosmeticsHandler creates a new ViewerCosmeticsHandler.
func NewViewerCosmeticsHandler(
	identityRepo *repository.ViewerIdentityRepository,
	redisClient *redis.Client,
	logger *zap.Logger,
) *ViewerCosmeticsHandler {
	return &ViewerCosmeticsHandler{
		identityRepo:    identityRepo,
		linkedPlatforms: identityRepo, // satisfies linkedPlatformsGetter
		redis:           redisClient,
		logger:          logger,
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

	c.JSON(http.StatusOK, cosmeticsResponse(cosmetics))
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
	handlePatchCosmeticsLogic(c, h.identityRepo, h.logger)

	// Invalidate Redis identity cache on success.
	// The cache is keyed per platform (viewer:identity:{platform}:{platform_user_id}), so
	// we must delete cache entries for ALL linked platforms — not just the current session's
	// platform — to ensure cosmetic changes (name_color, name_gradient, avatar) are reflected
	// immediately across every platform the viewer is connected to.
	if c.Writer.Status() == http.StatusOK {
		viewerIDVal, _ := c.Get("viewer_id")
		viewerIDStr, ok := viewerIDVal.(string)
		if ok && viewerIDStr != "" {
			if viewerID, parseErr := uuid.Parse(viewerIDStr); parseErr == nil {
				linked, listErr := h.linkedPlatforms.GetLinkedPlatforms(context.Background(), viewerID)
				if listErr != nil {
					h.logger.Warn("Failed to list linked platforms for cache invalidation",
						zap.String("viewer_id", viewerIDStr),
						zap.Error(listErr),
					)
				} else {
					// Delete cache key for every linked platform identity.
					for _, lp := range linked {
						cacheKey := fmt.Sprintf("viewer:identity:%s:%s", lp.Platform, lp.PlatformUserID)
						if err := h.redis.Del(context.Background(), cacheKey).Err(); err != nil {
							h.logger.Warn("Failed to invalidate viewer identity cache",
								zap.String("key", cacheKey),
								zap.Error(err),
							)
						}
					}
				}
			}
		}
	}
}

// cosmeticsUpsertRepo is the minimal interface for the cosmetics handler's DB access.
// This enables unit testing with mock implementations.
type cosmeticsUpsertRepo interface {
	// UpsertViewerCosmetics applies a per-column partial update and returns the full
	// persisted row (so the response reflects stored state without a separate read).
	UpsertViewerCosmetics(ctx context.Context, viewerID uuid.UUID, update repository.CosmeticsUpdate) (*repository.ViewerCosmetics, error)
	// GetFullCosmetics reads the current persisted cosmetics — used only for the
	// no-op case (an empty PATCH that writes nothing).
	GetFullCosmetics(ctx context.Context, viewerID uuid.UUID) (*repository.ViewerCosmetics, error)
}

// cosmeticsResponse builds the JSON body describing a viewer's persisted cosmetics.
// Shared by GET and PATCH so both report identical, accurate state. name_gradient is
// parsed from its raw JSON; a nil row yields all-null fields.
func cosmeticsResponse(c *repository.ViewerCosmetics) gin.H {
	if c == nil {
		return gin.H{"name_color": nil, "name_gradient": nil, "avatar_frame_id": nil, "avatar_flair_id": nil}
	}
	var gradient interface{}
	if len(c.NameGradient) > 0 && string(c.NameGradient) != "null" {
		var g interface{}
		if json.Unmarshal(c.NameGradient, &g) == nil {
			gradient = g
		}
	}
	return gin.H{
		"name_color":      c.NameColor,
		"name_gradient":   gradient,
		"avatar_frame_id": c.AvatarFrameID,
		"avatar_flair_id": c.AvatarFlairID,
	}
}

// linkedPlatformsGetter abstracts the GetLinkedPlatforms DB call for testability.
type linkedPlatformsGetter interface {
	GetLinkedPlatforms(ctx context.Context, viewerID uuid.UUID) ([]repository.LinkedPlatform, error)
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

// normalizeClearUUID maps a pointer to the zero UUID to a nil pointer.
//
// A nil *uuid.UUID is encoded by pgx as SQL NULL, which clears the column and
// satisfies the avatar_frame_id / avatar_flair_id foreign keys. A non-nil
// pointer to uuid.Nil is instead encoded as the literal '00000000-...-000000000000'
// value, which has no matching cosmetic_frames / cosmetic_flairs row and therefore
// violates the foreign key (SQLSTATE 23503) — surfacing as a 500. The zero UUID is
// never a real catalog id (PKs default to gen_random_uuid), so treating it as
// "clear the selection" is always correct.
func normalizeClearUUID(id *uuid.UUID) *uuid.UUID {
	if id != nil && *id == uuid.Nil {
		return nil
	}
	return id
}

// handlePatchCosmeticsLogic contains the core business logic for PATCH cosmetics.
// Extracted to allow unit testing with a mock repository.
func handlePatchCosmeticsLogic(c *gin.Context, repo cosmeticsUpsertRepo, logger *zap.Logger) {
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
		// regardless of whether they sent these fields. Leave both as nil pointers
		// so the UPSERT writes SQL NULL. A pointer to uuid.Nil would be encoded as
		// the literal zero UUID and violate the avatar FK constraints (see
		// normalizeClearUUID) — that bug 500'd every non-premium cosmetics save.
		avatarFrameID = nil
		avatarFlairID = nil
	} else {
		// Premium viewer: pass through whatever was sent, but map a zero-UUID
		// selection to nil (clear) so it writes SQL NULL instead of tripping the
		// avatar FK. nil pointer = field absent/null = UPSERT overwrites with NULL.
		avatarFrameID = normalizeClearUUID(req.AvatarFrameID)
		avatarFlairID = normalizeClearUUID(req.AvatarFlairID)
	}

	// Step 6: Persist only the column groups the request actually addressed (PATCH is
	// a partial update) and report the persisted result. Writing untouched columns
	// would clobber cosmetics the request never referenced:
	//   - setName:  a name_color or name_gradient field was present (the two move
	//     together — mutual exclusion is enforced above).
	//   - setFrame / setFlair: that avatar field was present, OR the viewer is
	//     non-premium — non-premium saves always re-clear the avatar (downgrade
	//     enforcement), with avatarFrameID/avatarFlairID already forced to nil above.
	ctx := c.Request.Context()
	setName := req.nameColorPresent || req.nameGradientPresent
	setFrame := req.avatarFrameIDPresent || !hasAccess
	setFlair := req.avatarFlairIDPresent || !hasAccess

	// No-op PATCH (nothing addressed): report current state without a pointless write.
	if !setName && !setFrame && !setFlair {
		full, err := repo.GetFullCosmetics(ctx, viewerID)
		if err != nil {
			if logger != nil {
				logger.Error("Failed to read viewer cosmetics", zap.String("viewer_id", viewerID.String()), zap.Error(err))
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update cosmetics"})
			return
		}
		c.JSON(http.StatusOK, cosmeticsResponse(full))
		return
	}

	persisted, err := repo.UpsertViewerCosmetics(ctx, viewerID, repository.CosmeticsUpdate{
		SetName:       setName,
		NameColor:     req.NameColor,
		NameGradient:  nameGradientBytes,
		SetFrame:      setFrame,
		AvatarFrameID: avatarFrameID,
		SetFlair:      setFlair,
		AvatarFlairID: avatarFlairID,
	})
	if err != nil {
		if logger != nil {
			logger.Error("Failed to upsert viewer cosmetics",
				zap.String("viewer_id", viewerID.String()),
				zap.Error(err),
			)
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update cosmetics"})
		return
	}

	// Step 7: Report the actual persisted state returned by the upsert, so the
	// response never diverges from what was stored (untouched columns, or a zero-UUID
	// normalized to NULL).
	c.JSON(http.StatusOK, cosmeticsResponse(persisted))
}
