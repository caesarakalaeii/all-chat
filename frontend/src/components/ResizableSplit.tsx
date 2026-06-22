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

import clsx from 'clsx'
import { useCallback, useEffect, useRef, useState } from 'react'

import { useHydrated } from '@/hooks/useHydrated'

/**
 * Layout orientation for the split on desktop.
 * - `horizontal`: panels side-by-side with a vertical divider (the persisted
 *   ratio drives the first panel's WIDTH). This is the original behavior.
 * - `vertical`: panels stacked top/bottom with a horizontal divider (the
 *   persisted ratio drives the first panel's HEIGHT).
 */
export type SplitOrientation = 'horizontal' | 'vertical'

interface ResizableSplitProps {
  /** localStorage key used to persist the split ratio across reloads. */
  storageKey: string
  left: React.ReactNode
  right: React.ReactNode
  /** Minimum / maximum percentage size of the first panel (desktop only). */
  min?: number
  max?: number
  /** Initial first-panel size percentage before any stored value is restored. */
  initial?: number
  /**
   * Desktop layout direction (default `horizontal`). `vertical` stacks the
   * panels top/bottom with a horizontal (row-resize) divider.
   */
  orientation?: SplitOrientation
  /**
   * Swap which panel comes first: false (default) keeps `left` first
   * (left / top); true puts `right` first (right / bottom).
   */
  reversed?: boolean
}

/**
 * Two-pane resizable split: side-by-side (or stacked, via `orientation`) with a
 * draggable divider on desktop, stacked (no divider) on mobile. Keyboard-accessible
 * (Arrow keys move the divider ±5%) and persists the ratio to localStorage.
 * Generalized from SplitView; no external dependency.
 */
export function ResizableSplit({
  storageKey,
  left,
  right,
  min = 25,
  max = 70,
  initial = 45,
  orientation = 'horizontal',
  reversed = false,
}: ResizableSplitProps) {
  const hydrated = useHydrated()
  const containerRef = useRef<HTMLDivElement>(null)
  const [firstPct, setFirstPct] = useState(initial)
  const [isDesktop, setIsDesktop] = useState(true)
  const isDragging = useRef(false)
  const firstPctRef = useRef(initial)
  const isVertical = orientation === 'vertical'

  const clamp = useCallback((pct: number) => Math.min(max, Math.max(min, pct)), [min, max])

  const persist = useCallback(
    (pct: number) => {
      try {
        localStorage.setItem(storageKey, String(pct))
      } catch {
        /* storage unavailable — resize still works in-session */
      }
    },
    [storageKey],
  )

  const setPct = useCallback(
    (pct: number) => {
      const clamped = clamp(pct)
      firstPctRef.current = clamped
      setFirstPct(clamped)
      return clamped
    },
    [clamp],
  )

  // Restore the persisted ratio after hydration (avoids SSR mismatch). Syncing
  // from localStorage on mount is the canonical external-data effect (same
  // disable as useHydrated.ts).
  useEffect(() => {
    if (!hydrated) return
    const stored = localStorage.getItem(storageKey)
    if (stored) {
      const n = parseFloat(stored)
      // eslint-disable-next-line react-hooks/set-state-in-effect -- one-time restore from localStorage
      if (Number.isFinite(n)) setPct(n)
    }
  }, [hydrated, storageKey, setPct])

  // Track the md breakpoint so the inline size only applies on desktop.
  useEffect(() => {
    if (typeof window.matchMedia !== 'function') return
    const mq = window.matchMedia('(min-width: 768px)')
    const update = () => setIsDesktop(mq.matches)
    update()
    mq.addEventListener('change', update)
    return () => mq.removeEventListener('change', update)
  }, [])

  const onPointerDown = useCallback((e: React.PointerEvent) => {
    e.currentTarget.setPointerCapture(e.pointerId)
    isDragging.current = true
  }, [])

  const onPointerMove = useCallback(
    (e: React.PointerEvent) => {
      if (!isDragging.current || !containerRef.current) return
      const rect = containerRef.current.getBoundingClientRect()
      // Compute the pointer position along the resize axis as a percentage of
      // the container. When reversed, the first (sized) panel is on the far
      // side, so invert the measurement.
      const raw = isVertical
        ? ((e.clientY - rect.top) / rect.height) * 100
        : ((e.clientX - rect.left) / rect.width) * 100
      setPct(reversed ? 100 - raw : raw)
    },
    [setPct, isVertical, reversed],
  )

  const onPointerUp = useCallback(() => {
    if (!isDragging.current) return
    isDragging.current = false
    persist(firstPctRef.current)
  }, [persist])

  const onKeyDown = useCallback(
    (e: React.KeyboardEvent) => {
      // Use the keys that match the divider's axis. Decrease/increase the first
      // panel's size by 5% per press.
      const decreaseKey = isVertical ? 'ArrowUp' : 'ArrowLeft'
      const increaseKey = isVertical ? 'ArrowDown' : 'ArrowRight'
      if (e.key === decreaseKey) {
        persist(setPct(firstPctRef.current - 5))
      } else if (e.key === increaseKey) {
        persist(setPct(firstPctRef.current + 5))
      }
    },
    [persist, setPct, isVertical],
  )

  // The first panel renders `right` when reversed, else `left`.
  const firstPanel = reversed ? right : left
  const secondPanel = reversed ? left : right

  // Inline size only applies on desktop; vertical sizes height, horizontal width.
  const firstStyle = isDesktop
    ? isVertical
      ? { height: `${firstPct}%` }
      : { width: `${firstPct}%` }
    : undefined

  return (
    <div
      ref={containerRef}
      className={clsx(
        'flex min-h-0 flex-1 flex-col overflow-hidden',
        isVertical ? 'md:flex-col' : 'md:flex-row',
      )}
      onPointerMove={onPointerMove}
      onPointerUp={onPointerUp}
    >
      <div
        className={clsx(
          'min-h-0 flex-1 overflow-hidden',
          isVertical ? 'md:flex-none' : 'md:flex-none',
        )}
        style={firstStyle}
      >
        {firstPanel}
      </div>

      {/* Draggable divider — hidden on mobile (stacked layout) */}
      <div
        className={clsx(
          'hidden flex-shrink-0 bg-border transition-colors select-none hover:bg-twitch/50 focus-visible:ring-2 focus-visible:ring-twitch focus-visible:outline-none md:block',
          isVertical ? 'h-1 w-full cursor-row-resize' : 'w-1 cursor-col-resize',
        )}
        onPointerDown={onPointerDown}
        onKeyDown={onKeyDown}
        role="separator"
        aria-label="Drag to resize panels"
        aria-orientation={isVertical ? 'horizontal' : 'vertical'}
        tabIndex={0}
      />

      <div className="min-h-0 flex-1 overflow-hidden">{secondPanel}</div>
    </div>
  )
}
