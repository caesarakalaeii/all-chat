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

package streams

import (
	"context"
	"time"

	"github.com/caesar/all-chat/services/youtube-listener/models"
	"github.com/caesar/all-chat/services/youtube-listener/quota"
	"go.uber.org/zap"
)

// claimRefreshInterval must stay well under youtubeclaim.DefaultClaimTTL so a healthy leader
// never lets a claim lapse between rounds.
const claimRefreshInterval = 60 * time.Second

type eligibleSourceLister interface {
	GetEligibleSources(ctx context.Context, gateFree bool) ([]*models.StreamSource, error)
}

// claimsLoop runs refreshClaims on its own ticker rather than inside syncStreams, whose early
// returns and debounced call sites would otherwise decide how often claims are refreshed.
func (m *Manager) claimsLoop(ctx context.Context) {
	defer close(m.claimsDone)

	m.refreshClaims(ctx)
	ticker := time.NewTicker(claimRefreshInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			m.refreshClaims(ctx)
		case <-ctx.Done():
			return
		case <-m.stopChan:
			return
		}
	}
}

// quotaBlocksClaims is true from CRITICAL upwards, where the coordinator already refuses
// discovery: holding claims there would keep channels away from innertube while this listener
// cannot serve them.
func quotaBlocksClaims(state quota.QuotaState) bool {
	switch state {
	case quota.QuotaStateCritical, quota.QuotaStateExhausted, quota.QuotaStateDepleted:
		return true
	}
	return false
}

// refreshClaims claims every eligible channel with a verified owner and releases the channels
// this replica claimed last round that no longer qualify. Only the global-sync lease holder acts,
// the same replica that runs syncStreams, so the claimed set it keeps is the one syncStreams reads.
func (m *Manager) refreshClaims(ctx context.Context) {
	if m.syncLeader != nil {
		isLeader, err := m.syncLeader.EnsureLeadership(ctx, m.syncLeaderStreamID, nil)
		if err != nil {
			m.logger.Error("Failed to check global sync leadership for claims", zap.Error(err))
			return
		}
		if !isLeader {
			// syncStreams also runs on this replica (demand and disconnect paths) and reads this
			// set, so a stale one would serve channels the leader may have released. The Redis
			// keys are the leader's to refresh or release now.
			m.claimMu.Lock()
			m.claimed = make(map[string]string)
			m.claimMu.Unlock()
			return
		}
	}

	m.claimMu.RLock()
	roundStart := m.dropSeq
	m.claimMu.RUnlock()

	next := make(map[string]string)
	if state := m.quotaState(); quotaBlocksClaims(state) {
		m.logger.Warn("Quota under pressure, releasing all official-API claims to innertube",
			zap.String("quota_state", string(state)),
		)
	} else {
		sources, err := m.eligibility.GetEligibleSources(ctx, m.gateFree())
		if err != nil {
			// Leave the previous round in place; the claim TTL hands channels back if this persists.
			m.logger.Error("Failed to load eligible sources for claims", zap.Error(err))
			return
		}
		connected := m.onAirOverlays(sources)

		for _, source := range sources {
			if _, done := next[source.ChannelID]; done {
				continue
			}
			// A claim takes the channel away from innertube for every overlay on it, so it is
			// only worth taking while the opted-in owner's own overlay is on air.
			if !connected[source.OverlayID] {
				continue
			}
			verified, err := m.verifier.Verified(ctx, source.OwnerUserID, source.ChannelID)
			if err != nil {
				m.logger.Warn("Could not verify official-API channel owner",
					zap.String("channel_id", source.ChannelID),
					zap.String("user_id", source.OwnerUserID),
					zap.Error(err),
				)
				continue
			}
			if verified {
				next[source.ChannelID] = source.OwnerUserID
			}
		}
	}

	for channelID, ownerUserID := range next {
		if err := m.claims.Claim(ctx, channelID, ownerUserID); err != nil {
			// An unwritten claim means innertube keeps serving the channel, so this listener must not.
			m.logger.Error("Failed to claim channel for official API",
				zap.String("channel_id", channelID),
				zap.Error(err),
			)
			delete(next, channelID)
		}
	}

	m.claimMu.Lock()
	stale := make(map[string]struct{}, len(m.claimed))
	for channelID := range m.claimed {
		stale[channelID] = struct{}{}
	}
	for channelID, ownerUserID := range next {
		if m.droppedAt[ownerUserID+"|"+channelID] > roundStart {
			// dropOwner ran after this round read the verdict, so the claim just written rests
			// on a token that has since been rejected.
			delete(next, channelID)
			stale[channelID] = struct{}{}
			continue
		}
		delete(stale, channelID)
	}
	m.claimed = next
	m.claimMu.Unlock()

	for channelID := range stale {
		if err := m.claims.Release(ctx, channelID); err != nil {
			m.logger.Warn("Failed to release official-API claim, TTL will expire it",
				zap.String("channel_id", channelID),
				zap.Error(err),
			)
			continue
		}
		m.logger.Info("Released official-API claim", zap.String("channel_id", channelID))
	}
}

// claimedOwner returns the verified owner of a channel claimed in the latest round.
func (m *Manager) claimedOwner(channelID string) (string, bool) {
	m.claimMu.RLock()
	defer m.claimMu.RUnlock()
	owner, ok := m.claimed[channelID]
	return owner, ok
}

// claimedChannelSources keeps the rows syncStreams may serve: the claimed channel's verified owner
// (whose token syncChannel uses) on overlays that are on air. It returns them per channel along
// with each channel's on-air overlay set.
func (m *Manager) claimedChannelSources(sources []*models.StreamSource) (map[string][]*models.StreamSource, map[string]map[string]struct{}) {
	connected := m.onAirOverlays(sources)

	channelSources := make(map[string][]*models.StreamSource)
	channelOverlays := make(map[string]map[string]struct{})
	for _, source := range sources {
		if owner, claimed := m.claimedOwner(source.ChannelID); !claimed || owner != source.OwnerUserID {
			continue
		}
		if !connected[source.OverlayID] {
			continue
		}
		channelSources[source.ChannelID] = append(channelSources[source.ChannelID], source)
		if _, exists := channelOverlays[source.ChannelID]; !exists {
			channelOverlays[source.ChannelID] = make(map[string]struct{})
		}
		channelOverlays[source.ChannelID][source.OverlayID] = struct{}{}
	}
	return channelSources, channelOverlays
}

// dropOwner hands a channel back to innertube once its owner token stops working. Without it the
// cached verdict and the claim keep the channel away from innertube for up to ownerVerifyTTL
// while every syncChannel fails, and its chat stays dark. Only a rejected token counts: a
// token-store or token-endpoint failure would otherwise bounce the channel on every blip.
func (m *Manager) dropOwner(ctx context.Context, channelID, userID string, cause error) {
	if !isOwnerAuthError(cause) {
		return
	}
	m.verifier.Forget(userID, channelID)

	m.claimMu.Lock()
	m.dropSeq++
	if m.droppedAt == nil {
		m.droppedAt = make(map[string]uint64)
	}
	m.droppedAt[userID+"|"+channelID] = m.dropSeq
	owner, held := m.claimed[channelID]
	ours := held && owner == userID
	if ours {
		delete(m.claimed, channelID)
	}
	m.claimMu.Unlock()
	if !ours {
		return
	}

	m.logger.Warn("Official-API owner token rejected, releasing claim to innertube",
		zap.String("channel_id", channelID),
		zap.String("user_id", userID),
		zap.Error(cause),
	)
	if err := m.claims.Release(ctx, channelID); err != nil {
		m.logger.Warn("Failed to release official-API claim, TTL will expire it",
			zap.String("channel_id", channelID),
			zap.Error(err),
		)
	}
}

// releaseClaims hands every claim of this replica back on shutdown. Left to the TTL, a rollout
// would keep those channels dark until the new leader's first claim round or the TTL expiry;
// released, innertube serves them meanwhile and the next leader re-claims what still qualifies.
func (m *Manager) releaseClaims() {
	m.claimMu.Lock()
	claimed := m.claimed
	m.claimed = make(map[string]string)
	m.claimMu.Unlock()
	if len(claimed) == 0 {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for channelID := range claimed {
		if err := m.claims.Release(ctx, channelID); err != nil {
			m.logger.Warn("Failed to release official-API claim on shutdown, TTL will expire it",
				zap.String("channel_id", channelID),
				zap.Error(err),
			)
		}
	}
}

// onAirOverlays returns which of the sources' overlays count as connected: those connected now and
// those inside the disconnect debounce. handleOverlayDisconnected drops an overlay from
// connectedOverlays at once, so without the debounce entries a page refresh would release the
// claim and stop the poller that the debounce exists to keep.
func (m *Manager) onAirOverlays(sources []*models.StreamSource) map[string]bool {
	onAir := make(map[string]bool, len(sources))
	m.connMu.RLock()
	for _, source := range sources {
		if _, ok := m.connectedOverlays[source.OverlayID]; ok {
			onAir[source.OverlayID] = true
		}
	}
	m.connMu.RUnlock()

	m.disconnectDebounceMu.Lock()
	for _, source := range sources {
		if _, pending := m.disconnectDebounceTimers[source.OverlayID]; pending {
			onAir[source.OverlayID] = true
		}
	}
	m.disconnectDebounceMu.Unlock()
	return onAir
}
