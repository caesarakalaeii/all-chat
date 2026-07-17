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

import React, { useState } from 'react'
import { Collapsible } from '@base-ui/react/collapsible'
import { ChevronDown } from 'lucide-react'

const STORAGE_KEY = 'appearance-panel-sections-v1'

function readStoredSections(key: string): Record<string, boolean> {
  try {
    const raw = localStorage.getItem(key)
    if (!raw) return {}
    return JSON.parse(raw) as Record<string, boolean>
  } catch {
    return {}
  }
}

export interface CollapsibleSectionProps {
  id: string
  title: string
  children: React.ReactNode
  storageKey?: string
  defaultOpen?: boolean
  /**
   * Controlled override used by the onboarding setup guide to spotlight a
   * section: true forces open, false forces closed; undefined (default)
   * keeps the normal user-toggled behavior. The forced state is NEVER
   * written to localStorage, so the user's own open/closed preferences
   * survive onboarding untouched.
   */
  forceOpen?: boolean
  /**
   * When set, wraps the trigger in an <h2>/<h3>/<h4> so screen-reader users
   * can jump between sections via heading navigation (WCAG 1.3.1 / 2.4.6).
   * The heading carries no styling of its own — the trigger keeps its
   * existing classes. Undefined (default) keeps the bare trigger markup.
   */
  headingLevel?: 2 | 3 | 4
}

export function CollapsibleSection({
  id,
  title,
  children,
  storageKey = STORAGE_KEY,
  defaultOpen = false,
  forceOpen,
  headingLevel,
}: CollapsibleSectionProps): React.ReactElement {
  const [open, setOpen] = useState<boolean>(() => {
    const stored = readStoredSections(storageKey)
    return stored[id] ?? defaultOpen
  })

  function handleOpenChange(nextOpen: boolean): void {
    if (forceOpen !== undefined) return
    setOpen(nextOpen)
    try {
      const stored = readStoredSections(storageKey)
      const next: Record<string, boolean> = { ...stored, [id]: nextOpen }
      localStorage.setItem(storageKey, JSON.stringify(next))
    } catch {
      // localStorage unavailable (e.g. SSR or private mode) — silently ignore
    }
  }

  const trigger = (
    <Collapsible.Trigger className="flex w-full items-center justify-between py-2 text-sm font-medium text-text transition-colors hover:text-text-sub">
      <span>{title}</span>
      <ChevronDown
        aria-hidden="true"
        className="h-4 w-4 text-text-dim transition-transform duration-200 data-[open]:rotate-180"
        data-open={(forceOpen ?? open) ? '' : undefined}
      />
    </Collapsible.Trigger>
  )

  // Tailwind preflight zeroes heading margins and inherits font styles, so
  // the heading wrapper adds structure without visual change.
  const HeadingTag = headingLevel !== undefined ? (`h${headingLevel}` as const) : null

  return (
    <Collapsible.Root open={forceOpen ?? open} onOpenChange={handleOpenChange}>
      {HeadingTag !== null ? <HeadingTag>{trigger}</HeadingTag> : trigger}
      <Collapsible.Panel
        keepMounted
        className="max-h-0 overflow-hidden transition-all duration-200 data-[open]:max-h-[2000px]"
      >
        <div className="pb-3">{children}</div>
      </Collapsible.Panel>
    </Collapsible.Root>
  )
}
