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

import { useCallback, useEffect, useRef, useState } from 'react'

import { useHydrated } from '@/hooks/useHydrated'

interface ResizableSplitProps {
  /** localStorage key used to persist the split ratio across reloads. */
  storageKey: string
  left: React.ReactNode
  right: React.ReactNode
  /** Minimum / maximum percentage width of the left panel (desktop only). */
  min?: number
  max?: number
  /** Initial left-panel width percentage before any stored value is restored. */
  initial?: number
}

/**
 * Two-pane resizable split: side-by-side with a draggable divider on desktop,
 * stacked (no divider) on mobile. Keyboard-accessible (Arrow keys move the
 * divider ±5%) and persists the ratio to localStorage. Generalized from
 * SplitView; no external dependency.
 */
export function ResizableSplit({
  storageKey,
  left,
  right,
  min = 25,
  max = 70,
  initial = 45,
}: ResizableSplitProps) {
  const hydrated = useHydrated()
  const containerRef = useRef<HTMLDivElement>(null)
  const [leftPct, setLeftPct] = useState(initial)
  const [isDesktop, setIsDesktop] = useState(true)
  const isDragging = useRef(false)
  const leftPctRef = useRef(initial)

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
      leftPctRef.current = clamped
      setLeftPct(clamped)
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

  // Track the md breakpoint so the inline width only applies on desktop.
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
      setPct(((e.clientX - rect.left) / rect.width) * 100)
    },
    [setPct],
  )

  const onPointerUp = useCallback(() => {
    if (!isDragging.current) return
    isDragging.current = false
    persist(leftPctRef.current)
  }, [persist])

  const onKeyDown = useCallback(
    (e: React.KeyboardEvent) => {
      if (e.key === 'ArrowLeft') {
        persist(setPct(leftPctRef.current - 5))
      } else if (e.key === 'ArrowRight') {
        persist(setPct(leftPctRef.current + 5))
      }
    },
    [persist, setPct],
  )

  return (
    <div
      ref={containerRef}
      className="flex min-h-0 flex-1 flex-col overflow-hidden md:flex-row"
      onPointerMove={onPointerMove}
      onPointerUp={onPointerUp}
    >
      <div
        className="min-h-0 flex-1 overflow-hidden md:flex-none"
        style={isDesktop ? { width: `${leftPct}%` } : undefined}
      >
        {left}
      </div>

      {/* Draggable divider — hidden on mobile (stacked layout) */}
      <div
        className="hidden w-1 flex-shrink-0 cursor-col-resize bg-border transition-colors select-none hover:bg-twitch/50 focus-visible:ring-2 focus-visible:ring-twitch focus-visible:outline-none md:block"
        onPointerDown={onPointerDown}
        onKeyDown={onKeyDown}
        role="separator"
        aria-label="Drag to resize panels"
        aria-orientation="vertical"
        tabIndex={0}
      />

      <div className="min-h-0 flex-1 overflow-hidden">{right}</div>
    </div>
  )
}
