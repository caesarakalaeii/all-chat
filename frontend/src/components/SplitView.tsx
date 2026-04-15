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


import { useRef, useState, useCallback } from 'react'

const MIN_LEFT = 25
const MAX_LEFT = 70

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
    setLeftPct(Math.min(MAX_LEFT, Math.max(MIN_LEFT, pct)))
  }, [])

  const onPointerUp = useCallback(() => {
    isDragging.current = false
  }, [])

  const onKeyDown = useCallback((e: React.KeyboardEvent) => {
    if (e.key === 'ArrowLeft') {
      setLeftPct((prev) => Math.max(MIN_LEFT, prev - 5))
    } else if (e.key === 'ArrowRight') {
      setLeftPct((prev) => Math.min(MAX_LEFT, prev + 5))
    }
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

      {/* Draggable divider — hidden on mobile */}
      <div
        className="hidden w-1 flex-shrink-0 cursor-col-resize bg-border transition-colors select-none hover:bg-twitch/50 focus-visible:ring-2 focus-visible:ring-twitch focus-visible:outline-none md:block"
        onPointerDown={onPointerDown}
        onKeyDown={onKeyDown}
        role="separator"
        aria-label="Drag to resize panels"
        aria-orientation="vertical"
        tabIndex={0}
      />

      {/* Preview panel */}
      <div className="min-h-[300px] flex-1 overflow-hidden bg-bg md:min-h-0">
        <iframe
          ref={useCallback((el: HTMLIFrameElement | null) => { if (el) onIframeReady?.(el) }, [onIframeReady])}
          src={`/overlays/${overlayId}/preview/embed`}
          className="h-full w-full border-0"
          title="Overlay live preview"
          sandbox="allow-scripts allow-same-origin"
        />
      </div>
    </div>
  )
}
