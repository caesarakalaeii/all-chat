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
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/caesar/all-chat/services/overlay-manager/models"
)

// The frontend decides which text keys and roll elements exist; these bounds
// only keep a public, unauthenticated config response small and well-formed.
const (
	maxCustomTexts  = 80
	maxTextRunes    = 300
	maxFontScales   = 20
	minFontScalePct = 50
	maxFontScalePct = 300
	defaultScalePct = 100
)

var customizationKey = regexp.MustCompile(`^[A-Za-z]{1,40}$`)

// normalizeCustomization bounds a config's custom texts and font scales, drops
// entries that mean "use the default" (blank text, 100%), and replaces nil maps
// with empty ones for the NOT NULL JSONB columns.
func normalizeCustomization(cfg *models.CreditRollConfig) error {
	texts := make(map[string]string, len(cfg.Texts))
	for key, text := range cfg.Texts {
		if !customizationKey.MatchString(key) {
			return fmt.Errorf("invalid text key %q", key)
		}
		if strings.TrimSpace(text) == "" {
			continue
		}
		if utf8.RuneCountInString(text) > maxTextRunes {
			return fmt.Errorf("text %q is longer than %d characters", key, maxTextRunes)
		}
		texts[key] = text
	}
	if len(texts) > maxCustomTexts {
		return fmt.Errorf("at most %d custom texts", maxCustomTexts)
	}

	scales := make(map[string]int, len(cfg.FontScales))
	for key, pct := range cfg.FontScales {
		if !customizationKey.MatchString(key) {
			return fmt.Errorf("invalid font scale key %q", key)
		}
		if pct < minFontScalePct || pct > maxFontScalePct {
			return fmt.Errorf("font scale %q must be between %d and %d percent", key, minFontScalePct, maxFontScalePct)
		}
		if pct == defaultScalePct {
			continue
		}
		scales[key] = pct
	}
	if len(scales) > maxFontScales {
		return fmt.Errorf("at most %d font scales", maxFontScales)
	}

	cfg.Texts = texts
	cfg.FontScales = scales
	return nil
}
