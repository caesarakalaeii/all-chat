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
	"strings"

	"github.com/caesar/all-chat/services/engagement-service/models"
	mpmodels "github.com/caesar/all-chat/services/message-processor/models"
)

// EarnAward is the points award derived from a platform event.
type EarnAward struct {
	Delta  int64
	Reason string // ledger reason: earn_bits|earn_donation|earn_sub|earn_gift
}

// PointsForEvent maps a normalized platform event to a points award using the
// overlay's earn config. It handles the monetary/loyalty events that have a
// configured multiplier — bits, USD donations (super chat / super sticker / kick
// donation), subscriptions (by tier), and gift subscriptions (proportional to
// count). Events without a configured earn rule (raid, follow, channel points,
// likes) return ok=false and are ignored in v1 to avoid mis-attribution; chat and
// watch points are awarded on separate signals, not here.
func PointsForEvent(cfg models.EarnConfig, ev *mpmodels.EventInfo) (EarnAward, bool) {
	if ev == nil || !cfg.Enabled {
		return EarnAward{}, false
	}

	switch ev.Type {
	case "bits":
		amt := valueAmount(ev)
		return awardIfPositive(scalePoints(amt, cfg.BitsMultiplier), "earn_bits")

	case "super_chat", "super_sticker", "kick_donation":
		// USD-denominated donations.
		amt := valueAmount(ev)
		return awardIfPositive(scalePoints(amt, cfg.USDMultiplier), "earn_donation")

	case "subscription", "resubscription", "new_sponsor", "member_milestone":
		return awardIfPositive(subPoints(cfg, subTier(ev)), "earn_sub")

	case "gift_subscription", "mystery_gift", "membership_gift":
		// Award the gifter proportional to how many subs/memberships they gifted.
		count := valueAmount(ev)
		if count < 1 {
			count = 1
		}
		return awardIfPositive(mulPoints(roundToInt64(count), cfg.GiftPerSub), "earn_gift")
	}
	return EarnAward{}, false
}

// maxEventPoints caps a single event's award. It is far above any realistic event
// yet far below math.MaxInt64, so a misconfigured or pre-constraint multiplier/tier
// can neither overflow int64 (a bare float→int64 conversion of an out-of-range value
// is undefined in Go) nor push a balance toward overflow. Belt-and-suspenders with
// the handler-side earn-config validation (which now also clamps config at write).
const maxEventPoints int64 = 1_000_000_000_000 // 1e12

// roundToInt64 rounds a non-negative float to an int64 award, clamped to
// [0, maxEventPoints]. NaN/Inf/out-of-range clamp to 0 or the cap rather than
// producing an undefined conversion.
func roundToInt64(f float64) int64 {
	v := math.Round(f)
	if math.IsNaN(v) || v <= 0 {
		return 0
	}
	if v >= float64(maxEventPoints) {
		return maxEventPoints
	}
	return int64(v)
}

// scalePoints rounds amt*mult to a clamped int64 award (see roundToInt64).
func scalePoints(amt, mult float64) int64 {
	return roundToInt64(amt * mult)
}

// mulPoints multiplies two non-negative int64s, clamped to [0, maxEventPoints],
// detecting overflow so a huge count*perUnit can't wrap.
func mulPoints(a, b int64) int64 {
	if a <= 0 || b <= 0 {
		return 0
	}
	if a > maxEventPoints/b { // product would exceed the cap (also catches overflow)
		return maxEventPoints
	}
	return a * b
}

// subPoints maps a normalized tier ("high"/"medium"/"low"/"") to configured points.
func subPoints(cfg models.EarnConfig, tier string) int64 {
	switch tier {
	case "high":
		return cfg.SubHigh
	case "medium":
		return cfg.SubMedium
	default: // "low" or unknown → base tier
		return cfg.SubLow
	}
}

// subTier buckets a subscription event into the configured tier. ev.Tier can't
// be used here: the classifier assigns a DISPLAY-importance tier, and every
// Twitch sub classifies "high" regardless of plan — using it would pay the
// Tier-3 amount for a Tier-1 sub. The raw platform plan is preserved in the
// event metadata, so: Twitch 3000 → high (Tier 3), 2000 → medium (Tier 2),
// 1000/Prime → low (Tier 1). Events without a paid plan (Kick subs, YouTube
// memberships/milestones) earn the base tier.
func subTier(ev *mpmodels.EventInfo) string {
	if raw, ok := ev.Metadata["tier"].(string); ok {
		switch strings.ToLower(strings.TrimSpace(raw)) {
		case "3000":
			return "high"
		case "2000":
			return "medium"
		}
	}
	return "low"
}

func valueAmount(ev *mpmodels.EventInfo) float64 {
	if ev.Value == nil {
		return 0
	}
	return ev.Value.Amount
}

func awardIfPositive(delta int64, reason string) (EarnAward, bool) {
	if delta <= 0 {
		return EarnAward{}, false
	}
	return EarnAward{Delta: delta, Reason: reason}, true
}
