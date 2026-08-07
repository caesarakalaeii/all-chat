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

package handler

import (
	"context"
	"net/http"

	"github.com/caesar/all-chat/services/moderation-service/models"
	"github.com/caesar/all-chat/services/moderation-service/repository"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// HandleListDelegations reports the channels the caller may moderate for someone else.
//
// This is the moderator's only route into an overlay they do not own: GET /api/v1/overlays is
// owner-filtered and has no shared-with-me listing, so without this endpoint an accepted grant is
// unreachable (ADR-0048).
//
// Deliberately ungated. A moderator must be able to see — and therefore understand — a delegation
// even when the streamer's plan has lapsed; the entitlement shows up as `available: false` on the
// row, with the streamer's plan named as the cause, never as an upgrade prompt aimed at a
// volunteer who cannot buy it.
func (h *GrantHandler) HandleListDelegations(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing user context"})
		return
	}

	ctx := c.Request.Context()
	delegations, err := h.grants.ListDelegationsFor(ctx, userID)
	if err != nil {
		h.internalError(c, "list delegations failed", err)
		return
	}

	// One gate lookup per distinct owner, not per row: a volunteer often moderates several
	// overlays for the same streamer, and the answer cannot differ between them.
	availability := make(map[string]bool, len(delegations))
	out := models.DelegationList{Delegations: make([]models.Delegation, 0, len(delegations))}
	for _, d := range delegations {
		available, cached := availability[d.OwnerUserID]
		if !cached {
			available = h.delegationAvailable(ctx, d.OwnerUserID)
			availability[d.OwnerUserID] = available
		}
		out.Delegations = append(out.Delegations, toDelegation(d, available))
	}
	c.JSON(http.StatusOK, out)
}

// delegationAvailable reports whether the owner's plan currently has delegated moderation open.
//
// Fails closed: a gate-lookup error reports unavailable rather than promising a capability the
// action path would then refuse. The action path asks the same question, so the two agree.
func (h *GrantHandler) delegationAvailable(ctx context.Context, ownerUserID string) bool {
	enabled, err := h.gate.DelegationEnabled(ctx, ownerUserID)
	if err != nil {
		h.logger.Warn("delegation gate check failed; reporting unavailable",
			zap.String("owner_user_id", ownerUserID), zap.Error(err))
		return false
	}
	return enabled
}

// toDelegation maps a repository row to the moderator-facing DTO.
//
// The owner's user id is dropped on the way out: the moderator needs the streamer's name, not an
// identifier that would let them address the streamer's account elsewhere in the API.
func toDelegation(d repository.Delegation, available bool) models.Delegation {
	out := models.Delegation{
		GrantID:          d.GrantID,
		OverlayID:        d.OverlayID,
		OverlayName:      d.OverlayName,
		OwnerDisplayName: d.OwnerDisplayName,
		Status:           d.Status,
		Actions:          d.Actions,
		Platforms:        make([]models.GrantPlatformLeg, 0, len(d.Platforms)),
		Available:        available,
		AcceptedAt:       d.AcceptedAt,
		LastActionAt:     d.LastActionAt,
	}
	if out.Actions == nil {
		out.Actions = []string{}
	}
	for _, leg := range d.Platforms {
		out.Platforms = append(out.Platforms, models.GrantPlatformLeg{
			Platform:     leg.Platform,
			Enabled:      leg.Enabled,
			Verification: leg.Verification,
			VerifiedAt:   leg.VerifiedAt,
		})
	}
	return out
}
