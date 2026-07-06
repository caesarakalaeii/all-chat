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

package engine

import (
	"testing"

	"github.com/caesar/all-chat/services/engagement-service/models"
	mpmodels "github.com/caesar/all-chat/services/message-processor/models"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func defCfg() models.EarnConfig { return models.DefaultEarnConfig(uuid.New()) }

func TestPointsForEvent_Bits(t *testing.T) {
	cfg := defCfg() // BitsMultiplier=1
	award, ok := PointsForEvent(cfg, &mpmodels.EventInfo{Type: "bits", Value: &mpmodels.EventValue{Amount: 100, Currency: "bits"}})
	assert.True(t, ok)
	assert.Equal(t, int64(100), award.Delta)
	assert.Equal(t, "earn_bits", award.Reason)
}

func TestPointsForEvent_DonationUSD(t *testing.T) {
	cfg := defCfg() // USDMultiplier=100
	award, ok := PointsForEvent(cfg, &mpmodels.EventInfo{Type: "super_chat", Value: &mpmodels.EventValue{Amount: 5, Currency: "USD"}})
	assert.True(t, ok)
	assert.Equal(t, int64(500), award.Delta)
	assert.Equal(t, "earn_donation", award.Reason)
}

func TestPointsForEvent_SubTiers(t *testing.T) {
	cfg := defCfg() // high=500 medium=300 low=150

	// The classifier's display tier is "high" for EVERY Twitch sub — the raw
	// plan in the event metadata decides the earn tier, not ev.Tier.
	sub := func(plan string) *mpmodels.EventInfo {
		return &mpmodels.EventInfo{
			Type: "subscription", Tier: "high",
			Metadata: map[string]interface{}{"tier": plan},
		}
	}

	t3, ok := PointsForEvent(cfg, sub("3000"))
	assert.True(t, ok)
	assert.Equal(t, int64(500), t3.Delta, "Twitch Tier 3 pays sub_high")

	t2, ok := PointsForEvent(cfg, sub("2000"))
	assert.True(t, ok)
	assert.Equal(t, int64(300), t2.Delta, "Twitch Tier 2 pays sub_medium")

	t1, ok := PointsForEvent(cfg, sub("1000"))
	assert.True(t, ok)
	assert.Equal(t, int64(150), t1.Delta, "Twitch Tier 1 pays the base tier despite display tier 'high'")

	prime, ok := PointsForEvent(cfg, sub("Prime"))
	assert.True(t, ok)
	assert.Equal(t, int64(150), prime.Delta, "Prime subs pay the base tier")

	// No plan metadata at all (Kick subs, YouTube memberships) → base tier.
	low, ok := PointsForEvent(cfg, &mpmodels.EventInfo{Type: "subscription", Tier: "high"})
	assert.True(t, ok)
	assert.Equal(t, int64(150), low.Delta, "events without a paid plan fall back to base")
}

func TestPointsForEvent_GiftProportional(t *testing.T) {
	cfg := defCfg() // GiftPerSub=150
	award, ok := PointsForEvent(cfg, &mpmodels.EventInfo{Type: "gift_subscription", Value: &mpmodels.EventValue{Amount: 3, Currency: "gifts"}})
	assert.True(t, ok)
	assert.Equal(t, int64(450), award.Delta)
	assert.Equal(t, "earn_gift", award.Reason)
}

func TestPointsForEvent_UnhandledAndDisabled(t *testing.T) {
	cfg := defCfg()
	_, ok := PointsForEvent(cfg, &mpmodels.EventInfo{Type: "raid", Value: &mpmodels.EventValue{Amount: 10, Currency: "viewers"}})
	assert.False(t, ok, "raid has no configured earn rule in v1")

	_, ok = PointsForEvent(cfg, nil)
	assert.False(t, ok)

	disabled := defCfg()
	disabled.Enabled = false
	_, ok = PointsForEvent(disabled, &mpmodels.EventInfo{Type: "bits", Value: &mpmodels.EventValue{Amount: 100, Currency: "bits"}})
	assert.False(t, ok, "disabled config awards nothing")
}
