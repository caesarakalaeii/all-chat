'use client'

import { useRef, useState, useCallback } from 'react'

const MIN_LEFT = 25
const MAX_LEFT = 70

export function SplitView({
  overlayId,
  children,
}: {
  overlayId: string
  children: React.ReactNode
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

  const onKeyDown = useCallback(
    (e: React.KeyboardEvent) => {
      if (e.key === 'ArrowLeft') {
        setLeftPct(prev => Math.max(MIN_LEFT, prev - 5))
      } else if (e.key === 'ArrowRight') {
        setLeftPct(prev => Math.min(MAX_LEFT, prev + 5))
      }
    },
    []
  )

  return (
    <div
      ref={containerRef}
      style={{ '--split-left': `${leftPct}%` } as React.CSSProperties}
      className="flex flex-col md:flex-row h-[calc(100vh-60px)] overflow-hidden"
      onPointerMove={onPointerMove}
      onPointerUp={onPointerUp}
    >
      {/* Config panel: full width on mobile, leftPct% on desktop via .split-view-config */}
      <div className="split-view-config flex-shrink-0 overflow-y-auto">
        {children}
      </div>

      {/* Draggable divider — hidden on mobile */}
      <div
        className="hidden md:block w-1 flex-shrink-0 bg-border hover:bg-twitch/50 cursor-col-resize transition-colors select-none focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-twitch"
        onPointerDown={onPointerDown}
        onKeyDown={onKeyDown}
        role="separator"
        aria-label="Drag to resize panels"
        aria-orientation="vertical"
        tabIndex={0}
      />

      {/* Preview panel */}
      <div className="flex-1 overflow-hidden bg-bg min-h-[300px] md:min-h-0">
        <iframe
          src={`/overlays/${overlayId}/preview/embed`}
          className="w-full h-full border-0"
          title="Overlay live preview"
          sandbox="allow-scripts allow-same-origin"
        />
      </div>
    </div>
  )
}
