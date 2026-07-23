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

import React, { useId, useRef, useState } from 'react'
import { Search, X } from 'lucide-react'
import { cn } from '@/lib/utils'
import {
  EDITOR_GROUPS,
  searchSettings,
  type EditorSectionId,
  type SearchHit,
} from './sectionRegistry'

export interface SettingsSearchProps {
  /**
   * Called when the user picks a result. `anchorId` is set when the entry
   * declares a data-setting-anchor target inside its section; undefined
   * means "just open the section".
   */
  onNavigate: (sectionId: EditorSectionId, anchorId?: string) => void
}

function crumbFor(hit: SearchHit): string {
  const group = EDITOR_GROUPS.find((g) => g.id === hit.section.group)
  return hit.entry !== undefined
    ? `${group?.label ?? ''} › ${hit.section.title}`
    : (group?.label ?? '')
}

/**
 * Search across every editor setting (ADR-0042). Matches control labels and
 * synonym keywords from the section registry, so users find settings they
 * can name ("badge", "fade", "banned words") without knowing our grouping.
 */
export function SettingsSearch({ onNavigate }: SettingsSearchProps): React.ReactElement {
  const [query, setQuery] = useState('')
  const [selectedIndex, setSelectedIndex] = useState(0)
  const inputRef = useRef<HTMLInputElement>(null)
  const listboxId = useId()

  const hits = searchSettings(query)
  const open = query.trim().length >= 2

  function reset(): void {
    setQuery('')
    setSelectedIndex(0)
  }

  function pick(hit: SearchHit): void {
    reset()
    onNavigate(hit.section.id, hit.entry?.anchorId)
  }

  function handleKeyDown(e: React.KeyboardEvent<HTMLInputElement>): void {
    if (e.key === 'Escape') {
      reset()
      return
    }
    if (!open || hits.length === 0) return
    if (e.key === 'ArrowDown') {
      e.preventDefault()
      setSelectedIndex((i) => (i + 1) % hits.length)
    } else if (e.key === 'ArrowUp') {
      e.preventDefault()
      setSelectedIndex((i) => (i - 1 + hits.length) % hits.length)
    } else if (e.key === 'Enter') {
      e.preventDefault()
      const hit = hits[Math.min(selectedIndex, hits.length - 1)]
      if (hit !== undefined) pick(hit)
    }
  }

  return (
    <div className="relative">
      <Search
        aria-hidden="true"
        className="pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2 text-text-sub"
      />
      <input
        ref={inputRef}
        type="text"
        role="combobox"
        aria-label="Search settings"
        aria-expanded={open}
        // Only reference the listbox while it is actually mounted — a dangling
        // aria-controls id is an axe violation (aria-valid-attr-value)
        aria-controls={open ? listboxId : undefined}
        aria-activedescendant={
          open && hits.length > 0 ? `${listboxId}-option-${selectedIndex}` : undefined
        }
        aria-autocomplete="list"
        placeholder="Search settings… (e.g. badge, fade, banned words)"
        autoComplete="off"
        value={query}
        onChange={(e) => {
          setQuery(e.target.value)
          setSelectedIndex(0)
        }}
        onKeyDown={handleKeyDown}
        className="w-full rounded-lg border border-border bg-surface-2 py-2 pr-8 pl-9 text-sm text-text placeholder:text-text-sub focus-visible:ring-2 focus-visible:ring-twitch/50 focus-visible:outline-none"
      />
      {query !== '' && (
        <button
          type="button"
          aria-label="Clear search"
          onClick={() => {
            reset()
            inputRef.current?.focus()
          }}
          className="absolute top-1/2 right-2 -translate-y-1/2 rounded p-1 text-text-sub hover:text-text focus-visible:ring-2 focus-visible:ring-twitch focus-visible:outline-none"
        >
          <X aria-hidden="true" className="size-3.5" />
        </button>
      )}
      {open && (
        <div
          id={listboxId}
          role="listbox"
          aria-label="Matching settings"
          className="absolute top-full right-0 left-0 z-30 mt-1 overflow-hidden rounded-lg border border-border bg-surface p-1 shadow-lg"
        >
          {hits.length === 0 ? (
            <p className="px-3 py-2 text-sm text-text-sub">
              No settings match &ldquo;{query.trim()}&rdquo;
            </p>
          ) : (
            hits.map((hit, index) => (
              <button
                key={`${hit.section.id}/${hit.entry?.label ?? ''}`}
                type="button"
                role="option"
                id={`${listboxId}-option-${index}`}
                aria-selected={index === selectedIndex}
                // Fire before the input's blur so the row isn't unmounted mid-click
                onMouseDown={(e) => e.preventDefault()}
                onClick={() => pick(hit)}
                onMouseEnter={() => setSelectedIndex(index)}
                className={cn(
                  'flex w-full items-baseline justify-between gap-3 rounded-md px-3 py-2 text-left text-sm',
                  index === selectedIndex ? 'bg-twitch/10 text-text' : 'text-text-sub'
                )}
              >
                <span className="truncate">{hit.entry?.label ?? hit.section.title}</span>
                <span className="shrink-0 text-xs text-text-sub">{crumbFor(hit)}</span>
              </button>
            ))
          )}
        </div>
      )}
    </div>
  )
}
