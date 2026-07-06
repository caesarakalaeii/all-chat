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
	"math"
	"strings"
	"testing"

	"github.com/caesar/all-chat/services/engagement-service/models"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestValidateEarnConfig covers M2: negative/non-finite values are rejected, huge
// values are clamped (not overflowed), the currency name is trimmed to the column
// width (rune-safe), and normal values pass through unchanged.
func TestValidateEarnConfig(t *testing.T) {
	t.Run("rejects negative int", func(t *testing.T) {
		c := models.DefaultEarnConfig(uuid.New())
		c.SubHigh = -1
		assert.Error(t, validateEarnConfig(&c))
	})
	t.Run("rejects negative multiplier", func(t *testing.T) {
		c := models.DefaultEarnConfig(uuid.New())
		c.BitsMultiplier = -0.5
		assert.Error(t, validateEarnConfig(&c))
	})
	t.Run("rejects NaN/Inf multiplier", func(t *testing.T) {
		c := models.DefaultEarnConfig(uuid.New())
		c.USDMultiplier = math.NaN()
		assert.Error(t, validateEarnConfig(&c))
		c.USDMultiplier = math.Inf(1)
		assert.Error(t, validateEarnConfig(&c))
	})
	t.Run("clamps huge values", func(t *testing.T) {
		c := models.DefaultEarnConfig(uuid.New())
		c.GiftPerSub = 9_000_000_000_000_000_000
		c.BitsMultiplier = 1e9
		require.NoError(t, validateEarnConfig(&c))
		assert.Equal(t, maxEarnPoints, c.GiftPerSub, "int values clamp to maxEarnPoints")
		assert.Equal(t, maxMultiplier, c.BitsMultiplier, "multipliers clamp to maxMultiplier")
	})
	t.Run("truncates long points name (rune-safe)", func(t *testing.T) {
		c := models.DefaultEarnConfig(uuid.New())
		c.PointsName = strings.Repeat("é", 200) // multi-byte runes
		require.NoError(t, validateEarnConfig(&c))
		assert.LessOrEqual(t, len([]rune(c.PointsName)), maxPointsNameLen)
	})
	t.Run("normal values unchanged", func(t *testing.T) {
		c := models.DefaultEarnConfig(uuid.New())
		c.Enabled = true
		require.NoError(t, validateEarnConfig(&c))
		assert.EqualValues(t, 150, c.SubLow)
		assert.EqualValues(t, 1, c.BitsMultiplier)
		assert.True(t, c.Enabled)
	})
}
