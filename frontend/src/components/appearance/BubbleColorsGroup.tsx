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
 * Differently-coloured chat bubbles (premium), on two independent axes:
 *
 * - Per platform — Twitch rows one fill, YouTube another. Useful for
 *   multistreamers who want to tell sources apart at a glance.
 * - Palette — 2 to MAX_BUBBLE_PALETTE fills cycled down the feed, for rhythm.
 *
 * A platform fill wins over the palette on that platform's rows; the emitted CSS
 * encodes that by source order (see bubbleFillRules).
 */

import React from 'react'
import { Plus, RotateCcw, X } from 'lucide-react'
import { PremiumBadge } from '@/components/PremiumBadge'
import { PremiumUpsellLink } from '@/components/PremiumUpsellLink'
import type { VisualSettings } from '@/lib/types/visual-settings'
import { MAX_BUBBLE_PALETTE } from '@/lib/utils/visual-settings-to-css'
import { ColorPickerControl } from './ColorPickerControl'

/** Starting colour for a newly added palette entry — a neutral dark bubble. */
const NEW_SWATCH = '#1e293b'

const PLATFORMS: ReadonlyArray<{ field: keyof VisualSettings; label: string; sample: string }> = [
  { field: 'twitchBubbleBg', label: 'Twitch', sample: '#2a1b3d' },
  { field: 'youtubeBubbleBg', label: 'YouTube', sample: '#3d1b1b' },
  { field: 'kickBubbleBg', label: 'Kick', sample: '#1b3d22' },
  { field: 'tiktokBubbleBg', label: 'TikTok', sample: '#1b333d' },
  { field: 'discordBubbleBg', label: 'Discord', sample: '#22253d' },
]

export interface BubbleColorsGroupProps {
  visualSettings: Partial<VisualSettings>
  onChange: (patch: Partial<VisualSettings>) => void
  isPremium: boolean
}

export function BubbleColorsGroup({
  visualSettings,
  onChange,
  isPremium,
}: BubbleColorsGroupProps): React.ReactElement {
  const palette = visualSettings.bubblePalette ?? []

  // Always write the whole list back: the setting is one array, and an entry's
  // index is what decides which rows get it.
  const writePalette = (next: string[]): void => {
    onChange({ bubblePalette: next.length > 0 ? next : undefined })
  }

  return (
    <div className="space-y-5">
      {!isPremium && (
        <div className="flex items-start gap-2 rounded-lg border border-border bg-surface p-3">
          <PremiumBadge />
          <p className="text-sm text-text-sub">
            Different bubble colours per platform, or a palette cycled down the feed, are a{' '}
            <PremiumUpsellLink>Premium</PremiumUpsellLink> feature.
          </p>
        </div>
      )}

      <fieldset disabled={!isPremium} className="space-y-5 disabled:opacity-50">
        <div className="space-y-3">
          <div>
            <h3 className="text-sm font-medium text-text">Per platform</h3>
            <p className="text-xs text-text-dim">
              Tell sources apart at a glance. Unset platforms keep the normal bubble background.
            </p>
          </div>
          {PLATFORMS.map((platform) => {
            const current = visualSettings[platform.field]
            return (
              <div key={String(platform.field)} className="flex items-center gap-1">
                <ColorPickerControl
                  label={platform.label}
                  value={typeof current === 'string' ? current : platform.sample}
                  onChange={(hex) => onChange({ [platform.field]: hex })}
                />
                <button
                  type="button"
                  aria-label={`Reset ${platform.label} bubble colour`}
                  disabled={!isPremium}
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
            <h3 className="text-sm font-medium text-text">Palette</h3>
            <p className="text-xs text-text-dim">
              Two or more colours, cycled down the feed. A row keeps its colour while it is on
              screen. Needs at least two to take effect.
            </p>
          </div>

          {palette.map((color, index) => (
            <div key={index} className="flex items-center gap-1">
              <ColorPickerControl
                label={`Colour ${index + 1}`}
                value={color}
                onChange={(hex) =>
                  writePalette(palette.map((entry, i) => (i === index ? hex : entry)))
                }
              />
              <button
                type="button"
                aria-label={`Remove colour ${index + 1}`}
                disabled={!isPremium}
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
              disabled={!isPremium}
              onClick={() => writePalette([...palette, NEW_SWATCH])}
              className="hover:bg-surface-alt flex items-center gap-1.5 rounded-lg border border-border bg-surface px-3 py-1.5 text-sm text-text transition-colors disabled:cursor-not-allowed"
            >
              <Plus className="h-3.5 w-3.5" />
              Add colour
            </button>
          )}

          {palette.length === 1 && (
            <p className="text-xs text-text-dim">
              One colour behaves the same as Bubble background. Add a second to start cycling.
            </p>
          )}
        </div>
      </fieldset>
    </div>
  )
}
