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
	"math"
	"testing"

	"github.com/caesar/all-chat/services/engagement-service/models"
	mpmodels "github.com/caesar/all-chat/services/message-processor/models"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

// defCfg returns the built-in defaults with earning ENABLED. DefaultEarnConfig is
// opt-in (Enabled=false, see TestDefaultEarnConfig_OptInDisabled), so the award tests
// must turn it on explicitly.
func defCfg() models.EarnConfig {
	c := models.DefaultEarnConfig(uuid.New())
	c.Enabled = true
	return c
}

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

// TestDefaultEarnConfig_OptInDisabled locks in the U3 opt-in default: a never-
// configured overlay accrues nothing until the streamer turns earning on.
func TestDefaultEarnConfig_OptInDisabled(t *testing.T) {
	def := models.DefaultEarnConfig(uuid.New())
	assert.False(t, def.Enabled, "DefaultEarnConfig must default disabled (opt-in)")
	_, ok := PointsForEvent(def, &mpmodels.EventInfo{Type: "bits", Value: &mpmodels.EventValue{Amount: 100, Currency: "bits"}})
	assert.False(t, ok, "an un-configured overlay awards nothing")
}

// TestPointsForEvent_OverflowClamped guards M2: a hand-set / pre-constraint config
// with absurd values must clamp a single event's award to maxEventPoints (a bounded
// positive), never producing a wrapped/negative int64 or an undefined float→int
// conversion.
func TestPointsForEvent_OverflowClamped(t *testing.T) {
	giftCfg := defCfg()
	giftCfg.GiftPerSub = math.MaxInt64 / 2
	award, ok := PointsForEvent(giftCfg, &mpmodels.EventInfo{Type: "gift_subscription", Value: &mpmodels.EventValue{Amount: 10, Currency: "gifts"}})
	assert.True(t, ok)
	assert.Equal(t, maxEventPoints, award.Delta, "overflowing gift multiply clamps to the cap")
	assert.Positive(t, award.Delta)

	bitsCfg := defCfg()
	bitsCfg.BitsMultiplier = math.MaxFloat64
	award, ok = PointsForEvent(bitsCfg, &mpmodels.EventInfo{Type: "bits", Value: &mpmodels.EventValue{Amount: 1e300, Currency: "bits"}})
	assert.True(t, ok)
	assert.Equal(t, maxEventPoints, award.Delta, "out-of-range float product clamps to the cap")

	// Normal ranges are unaffected (the clamp helpers are no-ops here).
	ok2 := defCfg()
	normal, ok := PointsForEvent(ok2, &mpmodels.EventInfo{Type: "bits", Value: &mpmodels.EventValue{Amount: 250, Currency: "bits"}})
	assert.True(t, ok)
	assert.Equal(t, int64(250), normal.Delta)
}
