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

package creditroll

import (
	"strings"
	"testing"

	"github.com/caesar/all-chat/services/overlay-manager/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeCustomization_KeepsOverridesDropsDefaults(t *testing.T) {
	cfg := &models.CreditRollConfig{
		Texts: map[string]string{
			"heading":    "Générique de fin",
			"session":    "Session du {day}/{month}/{year}",
			"seeYou":     "   ",
			"subheading": "",
		},
		FontScales: map[string]int{"session": 200, "heading": 100},
	}
	require.NoError(t, normalizeCustomization(cfg))
	assert.Equal(t, map[string]string{
		"heading": "Générique de fin",
		"session": "Session du {day}/{month}/{year}",
	}, cfg.Texts)
	assert.Equal(t, map[string]int{"session": 200}, cfg.FontScales)
}

func TestNormalizeCustomization_NilMapsBecomeEmpty(t *testing.T) {
	cfg := &models.CreditRollConfig{}
	require.NoError(t, normalizeCustomization(cfg))
	assert.NotNil(t, cfg.Texts)
	assert.NotNil(t, cfg.FontScales)
}

func TestNormalizeCustomization_RejectsOutOfBounds(t *testing.T) {
	tooMany := map[string]string{}
	for i := 0; i <= maxCustomTexts; i++ {
		tooMany["k"+string(rune('a'+i/26))+string(rune('a'+i%26))] = "x"
	}
	cases := map[string]*models.CreditRollConfig{
		"key with punctuation": {Texts: map[string]string{"head.ing": "x"}},
		"text too long":        {Texts: map[string]string{"heading": strings.Repeat("é", maxTextRunes+1)}},
		"too many texts":       {Texts: tooMany},
		"scale too small":      {FontScales: map[string]int{"session": minFontScalePct - 1}},
		"scale too large":      {FontScales: map[string]int{"session": maxFontScalePct + 1}},
		"scale key invalid":    {FontScales: map[string]int{"<b>": 120}},
	}
	for name, cfg := range cases {
		t.Run(name, func(t *testing.T) {
			assert.Error(t, normalizeCustomization(cfg))
		})
	}

	atLimit := &models.CreditRollConfig{
		Texts:      map[string]string{"heading": strings.Repeat("é", maxTextRunes)},
		FontScales: map[string]int{"a": minFontScalePct, "b": maxFontScalePct},
	}
	assert.NoError(t, normalizeCustomization(atLimit))
}
