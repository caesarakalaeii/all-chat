'use client'

/**
 * This file is part of All-Chat.
 * Copyright (C) 2026 caesarakalaeii
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program. If not, see <https://www.gnu.org/licenses/>.
 */

/**
 * Differently-coloured chat bubbles, on two independent axes:
 *
 * - Per platform — Twitch rows one fill, YouTube another. Useful for
 *   multistreamers who want to tell sources apart at a glance.
 * - Palette — 2 to MAX_BUBBLE_PALETTE fills cycled down the feed, for rhythm.
 *
 * A platform fill wins over the palette on that platform's rows; the emitted CSS
 * encodes that by source order (see bubbleFillRules).
 *
 * Free to use. `locked` comes from the server-resolved `bubble_colors_locked`
 * flag on the overlay config rather than from `user.is_premium`, so flipping the
 * `bubble_colors` gate to premium in the admin UI locks these controls with no
 * deploy — and until someone does, nothing here mentions Premium.
 */

import React from 'react'
import { Plus, RotateCcw, X } from 'lucide-react'
import { PremiumBadge } from '@/components/PremiumBadge'
import { PremiumUpsellLink } from '@/components/PremiumUpsellLink'
import { useTranslations } from '@/lib/i18n'
import { emphasise } from '@/lib/i18n/emphasise'
import type { VisualSettings } from '@/lib/types/visual-settings'
import { MAX_BUBBLE_PALETTE } from '@/lib/utils/visual-settings-to-css'
import { ColorPickerControl } from './ColorPickerControl'

/** Starting colour for a newly added palette entry — a neutral dark bubble. */
const NEW_SWATCH = '#1e293b'

// `platform` keys the display name in common.platforms.*, shared with the
// moderator roster rather than duplicated here.
const PLATFORMS: ReadonlyArray<{
  field: keyof VisualSettings
  platform: 'twitch' | 'youtube' | 'kick' | 'tiktok' | 'discord'
  sample: string
}> = [
  { field: 'twitchBubbleBg', platform: 'twitch', sample: '#2a1b3d' },
  { field: 'youtubeBubbleBg', platform: 'youtube', sample: '#3d1b1b' },
  { field: 'kickBubbleBg', platform: 'kick', sample: '#1b3d22' },
  { field: 'tiktokBubbleBg', platform: 'tiktok', sample: '#1b333d' },
  { field: 'discordBubbleBg', platform: 'discord', sample: '#22253d' },
]

export interface BubbleColorsGroupProps {
  visualSettings: Partial<VisualSettings>
  onChange: (patch: Partial<VisualSettings>) => void
  /** Server-resolved `bubble_colors_locked`; the gate ships open, so normally false. */
  locked?: boolean
}

export function BubbleColorsGroup({
  visualSettings,
  onChange,
  locked = false,
}: BubbleColorsGroupProps): React.ReactElement {
  const t = useTranslations()
  const palette = visualSettings.bubblePalette ?? []

  // Always write the whole list back: the setting is one array, and an entry's
  // index is what decides which rows get it.
  const writePalette = (next: string[]): void => {
    onChange({ bubblePalette: next.length > 0 ? next : undefined })
  }

  return (
    <div className="space-y-5">
      {locked && (
        <div className="flex items-start gap-2 rounded-lg border border-border bg-surface p-3">
          <PremiumBadge />
          <p className="text-sm text-text-sub">
            {emphasise(
              t('overlayEditor.bubbleColors.lockedNotice', {
                emphasis: t('overlayEditor.bubbleColors.lockedNoticeEmphasis'),
              }),
              t('overlayEditor.bubbleColors.lockedNoticeEmphasis'),
              (run) => (
                <PremiumUpsellLink>{run}</PremiumUpsellLink>
              )
            )}
          </p>
        </div>
      )}

      <fieldset disabled={locked} className="space-y-5 disabled:opacity-50">
        <div className="space-y-3">
          <div>
            <h3 className="text-sm font-medium text-text">
              {t('overlayEditor.bubbleColors.perPlatformHeading')}
            </h3>
            <p className="text-xs text-text-dim">
              {t('overlayEditor.bubbleColors.perPlatformBody')}
            </p>
          </div>
          {PLATFORMS.map((platform) => {
            const current = visualSettings[platform.field]
            const platformName = t(`common.platforms.${platform.platform}`)
            return (
              <div key={String(platform.field)} className="flex items-center gap-1">
                <ColorPickerControl
                  label={platformName}
                  value={typeof current === 'string' ? current : platform.sample}
                  onChange={(hex) => onChange({ [platform.field]: hex })}
                />
                <button
                  type="button"
                  aria-label={t('overlayEditor.bubbleColors.resetPlatform', {
                    platform: platformName,
                  })}
                  disabled={locked}
                  onClick={() => onChange({ [platform.field]: undefined })}
                  className="rounded p-1 text-text-dim transition-colors hover:text-text disabled:cursor-not-allowed"
                >
                  <RotateCcw className="h-3 w-3" />
                </button>
              </div>
            )
          })}
        </div>

        <div className="space-y-3">
          <div>
            <h3 className="text-sm font-medium text-text">
              {t('overlayEditor.bubbleColors.paletteHeading')}
            </h3>
            <p className="text-xs text-text-dim">{t('overlayEditor.bubbleColors.paletteBody')}</p>
          </div>

          {palette.map((color, index) => (
            <div key={index} className="flex items-center gap-1">
              <ColorPickerControl
                label={t('overlayEditor.bubbleColors.swatchLabel', { index: index + 1 })}
                value={color}
                onChange={(hex) =>
                  writePalette(palette.map((entry, i) => (i === index ? hex : entry)))
                }
              />
              <button
                type="button"
                aria-label={t('overlayEditor.bubbleColors.removeSwatch', { index: index + 1 })}
                disabled={locked}
                onClick={() => writePalette(palette.filter((_, i) => i !== index))}
                className="rounded p-1 text-text-dim transition-colors hover:text-text disabled:cursor-not-allowed"
              >
                <X className="h-3 w-3" />
              </button>
            </div>
          ))}

          {palette.length < MAX_BUBBLE_PALETTE && (
            <button
              type="button"
              disabled={locked}
              onClick={() => writePalette([...palette, NEW_SWATCH])}
              className="hover:bg-surface-alt flex items-center gap-1.5 rounded-lg border border-border bg-surface px-3 py-1.5 text-sm text-text transition-colors disabled:cursor-not-allowed"
            >
              <Plus className="h-3.5 w-3.5" />
              {t('overlayEditor.bubbleColors.addSwatch')}
            </button>
          )}

          {palette.length === 1 && (
            <p className="text-xs text-text-dim">
              {t('overlayEditor.bubbleColors.singleSwatchNote')}
            </p>
          )}
        </div>
      </fieldset>
    </div>
  )
}
