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

import { ChevronLeft, ChevronRight } from 'lucide-react'
import { useRef, useState, useCallback } from 'react'

const MIN_LEFT = 25
const MAX_LEFT = 70

/** Percentage step applied per click of the divider's step buttons (WCAG 2.5.7). */
const BUTTON_STEP = 10

/** Percentage step applied per arrow-key press on the divider. */
const KEY_STEP = 5

/** 24px (WCAG 2.5.8) chevron buttons floating on the divider (mirrors ResizableSplit). */
const STEP_BUTTON_CLASS =
  'flex h-6 w-6 items-center justify-center rounded-md border border-border bg-surface text-text-sub shadow-sm hover:text-text focus-visible:ring-2 focus-visible:ring-twitch focus-visible:outline-none'

const clampLeft = (pct: number) => Math.min(MAX_LEFT, Math.max(MIN_LEFT, pct))

export function SplitView({
  overlayId,
  children,
  onIframeReady,
}: {
  overlayId: string
  children: React.ReactNode
  onIframeReady?: (iframe: HTMLIFrameElement) => void
}) {
  const containerRef = useRef<HTMLDivElement>(null)
  const [leftPct, setLeftPct] = useState(40)
  const isDragging = useRef(false)

  const onPointerDown = useCallback((e: React.PointerEvent) => {
    e.currentTarget.setPointerCapture(e.pointerId)
    isDragging.current = true
  }, [])

  const onPointerMove = useCallback((e: React.PointerEvent) => {
    if (!isDragging.current || !containerRef.current) return
    const rect = containerRef.current.getBoundingClientRect()
    const pct = ((e.clientX - rect.left) / rect.width) * 100
    setLeftPct(clampLeft(pct))
  }, [])

  const onPointerUp = useCallback(() => {
    isDragging.current = false
  }, [])

  const onKeyDown = useCallback((e: React.KeyboardEvent) => {
    if (e.key === 'ArrowLeft') {
      e.preventDefault()
      setLeftPct((prev) => clampLeft(prev - KEY_STEP))
    } else if (e.key === 'ArrowRight') {
      e.preventDefault()
      setLeftPct((prev) => clampLeft(prev + KEY_STEP))
    }
  }, [])

  /** Single-pointer, no-drag resize alternative (WCAG 2.5.7). */
  const stepBy = useCallback((delta: number) => {
    setLeftPct((prev) => clampLeft(prev + delta))
  }, [])

  return (
    <div
      ref={containerRef}
      style={{ '--split-left': `${leftPct}%` } as React.CSSProperties}
      className="flex h-[calc(100vh-60px)] flex-col overflow-hidden md:flex-row"
      onPointerMove={onPointerMove}
      onPointerUp={onPointerUp}
    >
      {/* Config panel: full width on mobile, leftPct% on desktop via .split-view-config */}
      <div className="split-view-config flex-shrink-0 overflow-y-auto">{children}</div>

      {/* Draggable divider — hidden on mobile. Exposed as a value slider
          (aria-valuenow = config panel %): aria-query classifies `separator`
          as non-interactive structure, so the focusable window-splitter
          separator cannot satisfy the a11y gate; `slider` carries the same
          value/min/max/orientation + arrow-key semantics (mirrors
          ResizableSplit). */}
      <div className="relative hidden w-1 flex-shrink-0 md:block">
        <div
          role="slider"
          aria-label="Resize panels"
          aria-orientation="horizontal"
          aria-valuemin={MIN_LEFT}
          aria-valuemax={MAX_LEFT}
          aria-valuenow={Math.round(leftPct)}
          tabIndex={0}
          onPointerDown={onPointerDown}
          onKeyDown={onKeyDown}
          // The ELEMENT box is >=24px wide (WCAG 2.5.8 — axe measures the
          // target's own bounding box, pseudo-elements do not count); the
          // visible 4px bar is drawn by the before: pseudo-element instead.
          className="absolute inset-y-0 left-1/2 w-6 -translate-x-1/2 cursor-col-resize select-none before:absolute before:inset-y-0 before:left-1/2 before:w-1 before:-translate-x-1/2 before:bg-border before:transition-colors before:content-[''] hover:before:bg-twitch/50 focus-visible:ring-2 focus-visible:ring-twitch focus-visible:outline-none"
        />
        {/* Step buttons: single-pointer, no-drag resize alternative (WCAG
            2.5.7) — each click moves the split by 10%. Siblings of the slider
            (not children) because role=slider makes descendants presentational. */}
        <div className="absolute top-1/2 left-1/2 z-10 flex -translate-x-1/2 -translate-y-1/2 flex-col gap-1">
          <button
            type="button"
            aria-label="Shrink config panel"
            onClick={() => stepBy(-BUTTON_STEP)}
            className={STEP_BUTTON_CLASS}
          >
            <ChevronLeft className="h-4 w-4" aria-hidden="true" />
          </button>
          <button
            type="button"
            aria-label="Grow config panel"
            onClick={() => stepBy(BUTTON_STEP)}
            className={STEP_BUTTON_CLASS}
          >
            <ChevronRight className="h-4 w-4" aria-hidden="true" />
          </button>
        </div>
      </div>

      {/* Preview panel */}
      <div className="min-h-[300px] flex-1 overflow-hidden bg-bg md:min-h-0">
        <iframe
          ref={useCallback(
            (el: HTMLIFrameElement | null) => {
              if (el) onIframeReady?.(el)
            },
            [onIframeReady]
          )}
          src={`/overlays/${overlayId}/preview/embed`}
          className="h-full w-full border-0"
          title="Overlay live preview"
          sandbox="allow-scripts allow-same-origin"
        />
      </div>
    </div>
  )
}
