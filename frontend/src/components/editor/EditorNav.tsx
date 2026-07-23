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

import React from 'react'
import { cn } from '@/lib/utils'
import {
  EDITOR_GROUPS,
  EDITOR_SECTIONS,
  type EditorSection,
  type EditorSectionId,
} from './sectionRegistry'

export interface EditorNavProps {
  activeId: EditorSectionId
  onSelect: (id: EditorSectionId) => void
  sections?: EditorSection[]
}

/**
 * Left-nav section list for the overlay editor (ADR-0042). Every section is
 * always visible; exactly one is active. Inside a `@container` parent the
 * nav is a vertical rail from @md upward and collapses to a horizontally
 * scrollable chip row below that (the config column is user-resizable, so
 * this responds to the column width, not the viewport).
 */
export function EditorNav({
  activeId,
  onSelect,
  sections = EDITOR_SECTIONS,
}: EditorNavProps): React.ReactElement {
  return (
    <nav
      aria-label="Overlay settings"
      className="flex shrink-0 gap-1 overflow-x-auto pb-2 @md:w-40 @md:flex-col @md:gap-0 @md:overflow-visible @md:border-r @md:border-border @md:pr-3 @md:pb-0"
    >
      {EDITOR_GROUPS.map((group) => {
        const groupSections = sections.filter((s) => s.group === group.id)
        if (groupSections.length === 0) return null
        return (
          <div key={group.id} className="contents @md:block">
            <p
              aria-hidden="true"
              className="hidden pt-4 pb-1 pl-2.5 text-[10px] font-medium tracking-widest text-text-sub uppercase select-none @md:block @md:first-of-type:pt-0"
            >
              {group.label}
            </p>
            {groupSections.map((section) => {
              const isActive = section.id === activeId
              const isDanger = section.id === 'danger-zone'
              return (
                <button
                  key={section.id}
                  type="button"
                  aria-current={isActive ? 'true' : undefined}
                  onClick={() => onSelect(section.id)}
                  className={cn(
                    'block w-max shrink-0 rounded-md px-2.5 py-1.5 text-left text-sm whitespace-nowrap transition-colors focus-visible:ring-2 focus-visible:ring-twitch focus-visible:outline-none @md:w-full',
                    isActive
                      ? isDanger
                        ? 'bg-destructive/10 text-destructive font-medium'
                        : 'bg-twitch/10 font-medium text-text'
                      : isDanger
                        ? 'text-destructive/70 hover:bg-destructive/10 hover:text-destructive'
                        : 'text-text-sub hover:bg-surface-2 hover:text-text'
                  )}
                >
                  {section.navLabel ?? section.title}
                </button>
              )
            })}
          </div>
        )
      })}
    </nav>
  )
}
