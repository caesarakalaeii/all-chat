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
import { ChevronDown, ChevronLeft, ChevronRight, ChevronUp } from 'lucide-react'
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

/** Percentage step applied per click of the divider's step buttons (WCAG 2.5.7). */
const BUTTON_STEP = 10

/** Percentage step applied per arrow-key press on the divider. */
const KEY_STEP = 5

/** 24px (WCAG 2.5.8) chevron buttons floating on the divider. */
const STEP_BUTTON_CLASS =
  'flex h-6 w-6 items-center justify-center rounded-md border border-border bg-surface text-text-sub shadow-sm hover:text-text focus-visible:ring-2 focus-visible:ring-twitch focus-visible:outline-none'

/**
 * Two-pane resizable split: side-by-side (or stacked, via `orientation`) with a
 * draggable divider on desktop, stacked (no divider) on mobile. Keyboard-accessible
 * (Arrow keys move the divider ±5%), with single-click step buttons as the
 * no-drag pointer alternative (WCAG 2.5.7), and persists the ratio to
 * localStorage. Generalized from SplitView; no external dependency.
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
    [storageKey]
  )

  const setPct = useCallback(
    (pct: number) => {
      const clamped = clamp(pct)
      firstPctRef.current = clamped
      setFirstPct(clamped)
      return clamped
    },
    [clamp]
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
    [setPct, isVertical, reversed]
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
        e.preventDefault()
        persist(setPct(firstPctRef.current - KEY_STEP))
      } else if (e.key === increaseKey) {
        e.preventDefault()
        persist(setPct(firstPctRef.current + KEY_STEP))
      }
    },
    [persist, setPct, isVertical]
  )

  /** Single-pointer, no-drag resize alternative (WCAG 2.5.7). */
  const stepBy = useCallback(
    (delta: number) => {
      persist(setPct(firstPctRef.current + delta))
    },
    [persist, setPct]
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
        isVertical ? 'md:flex-col' : 'md:flex-row'
      )}
      onPointerMove={onPointerMove}
      onPointerUp={onPointerUp}
    >
      <div
        className={clsx(
          'min-h-0 flex-1 overflow-hidden',
          isVertical ? 'md:flex-none' : 'md:flex-none'
        )}
        style={firstStyle}
      >
        {firstPanel}
      </div>

      {/* Draggable divider — hidden on mobile (stacked layout). The handle is
          exposed as a value slider (aria-valuenow = first panel %): aria-query
          classifies `separator` as non-interactive structure, so the focusable
          window-splitter separator cannot satisfy the a11y gate; `slider`
          carries the same value/min/max/orientation + arrow-key semantics. */}
      <div
        className={clsx(
          'relative hidden flex-shrink-0 md:block',
          isVertical ? 'h-1 w-full' : 'w-1'
        )}
      >
        <div
          role="slider"
          aria-label="Resize panels"
          aria-orientation={isVertical ? 'vertical' : 'horizontal'}
          aria-valuemin={min}
          aria-valuemax={max}
          aria-valuenow={Math.round(firstPct)}
          tabIndex={0}
          onPointerDown={onPointerDown}
          onKeyDown={onKeyDown}
          className={clsx(
            // The ELEMENT box is >=24px (WCAG 2.5.8 — axe measures the target's
            // own bounding box, pseudo-elements do not count); the visible 4px
            // bar is drawn by the before: pseudo-element instead.
            "select-none before:absolute before:bg-border before:transition-colors before:content-[''] hover:before:bg-twitch/50 focus-visible:ring-2 focus-visible:ring-twitch focus-visible:outline-none",
            isVertical
              ? 'absolute inset-x-0 top-1/2 h-6 -translate-y-1/2 cursor-row-resize before:inset-x-0 before:top-1/2 before:h-1 before:-translate-y-1/2'
              : 'absolute inset-y-0 left-1/2 w-6 -translate-x-1/2 cursor-col-resize before:inset-y-0 before:left-1/2 before:w-1 before:-translate-x-1/2'
          )}
        />
        {/* Step buttons: single-pointer, no-drag resize alternative (WCAG
            2.5.7) — each click moves the split by 10%. Siblings of the slider
            (not children) because role=slider makes descendants presentational. */}
        <div
          className={clsx(
            'absolute top-1/2 left-1/2 z-10 flex -translate-x-1/2 -translate-y-1/2 gap-1',
            isVertical ? 'flex-row' : 'flex-col'
          )}
        >
          <button
            type="button"
            aria-label={isVertical ? 'Shrink top panel' : 'Shrink left panel'}
            onClick={() => stepBy(-BUTTON_STEP)}
            className={STEP_BUTTON_CLASS}
          >
            {isVertical ? (
              <ChevronUp className="h-4 w-4" aria-hidden="true" />
            ) : (
              <ChevronLeft className="h-4 w-4" aria-hidden="true" />
            )}
          </button>
          <button
            type="button"
            aria-label={isVertical ? 'Grow top panel' : 'Grow left panel'}
            onClick={() => stepBy(BUTTON_STEP)}
            className={STEP_BUTTON_CLASS}
          >
            {isVertical ? (
              <ChevronDown className="h-4 w-4" aria-hidden="true" />
            ) : (
              <ChevronRight className="h-4 w-4" aria-hidden="true" />
            )}
          </button>
        </div>
      </div>

      <div className="min-h-0 flex-1 overflow-hidden">{secondPanel}</div>
    </div>
  )
}
