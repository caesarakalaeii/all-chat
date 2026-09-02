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

'use client'

import clsx from 'clsx'

import type { DockTab } from '@/app/overlay/[id]/view/dockMode'
import { useTranslations } from '@/lib/i18n'

/**
 * Chat | Activity switcher for the monitor in dock mode.
 *
 * Replaces `ResizableSplit` at dock width, where two side-by-side columns are
 * roughly 150px each and neither is readable. Styled from the same token
 * vocabulary as `LayoutPicker` (the control it stands in for) rather than the
 * shadcn `Tabs` primitive: the monitor scopes its own light mode by overriding
 * `--color-*` in `.overlay-view.light`, which the primitive's
 * `muted`/`foreground` tokens do not follow.
 */
export function DockTabPicker({
  tab,
  onChange,
}: {
  tab: DockTab
  onChange: (tab: DockTab) => void
}) {
  const t = useTranslations()
  const tabs: ReadonlyArray<{ value: DockTab; label: string }> = [
    { value: 'chat', label: t('viewerOverlay.dock.chatTab') },
    { value: 'activity', label: t('viewerOverlay.dock.activityTab') },
  ]

  return (
    <div role="tablist" className="flex items-center gap-1 border-b border-border bg-surface px-2">
      {tabs.map(({ value, label }) => {
        const selected = tab === value
        return (
          <button
            key={value}
            type="button"
            role="tab"
            aria-selected={selected}
            // Re-selecting the current tab is not a change; reporting it would
            // make the page persist the same value on every stray click.
            onClick={() => !selected && onChange(value)}
            className={clsx(
              'border-b-2 px-3 py-2 text-xs font-medium transition-colors focus-visible:ring-2 focus-visible:ring-twitch focus-visible:outline-none',
              selected
                ? 'border-twitch text-text'
                : 'border-transparent text-text-sub hover:text-text'
            )}
          >
            {label}
          </button>
        )
      })}
    </div>
  )
}
