'use client'

import React, { useState } from 'react'
import { Collapsible } from '@base-ui/react/collapsible'
import { ChevronDown } from 'lucide-react'

const STORAGE_KEY = 'appearance-panel-sections-v1'

function readStoredSections(): Record<string, boolean> {
  try {
    const raw = localStorage.getItem(STORAGE_KEY)
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
}

export function CollapsibleSection({ id, title, children }: CollapsibleSectionProps): React.ReactElement {
  const [open, setOpen] = useState<boolean>(() => {
    const stored = readStoredSections()
    return stored[id] ?? false
  })

  function handleOpenChange(nextOpen: boolean): void {
    setOpen(nextOpen)
    try {
      const stored = readStoredSections()
      const next: Record<string, boolean> = { ...stored, [id]: nextOpen }
      localStorage.setItem(STORAGE_KEY, JSON.stringify(next))
    } catch {
      // localStorage unavailable (e.g. SSR or private mode) — silently ignore
    }
  }

  return (
    <Collapsible.Root open={open} onOpenChange={handleOpenChange}>
      <Collapsible.Trigger className="flex w-full items-center justify-between py-2 text-sm font-medium text-text hover:text-text-sub transition-colors">
        <span>{title}</span>
        <ChevronDown
          className="h-4 w-4 text-text-dim transition-transform duration-200 data-[open]:rotate-180"
          data-open={open ? '' : undefined}
        />
      </Collapsible.Trigger>
      <Collapsible.Panel
        keepMounted
        className="overflow-hidden max-h-0 data-[open]:max-h-[2000px] transition-all duration-200"
      >
        <div className="pb-3">{children}</div>
      </Collapsible.Panel>
    </Collapsible.Root>
  )
}
