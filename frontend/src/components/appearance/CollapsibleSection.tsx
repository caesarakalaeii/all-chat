'use client'

import React from 'react'

export interface CollapsibleSectionProps {
  id: string
  title: string
  children: React.ReactNode
}

/**
 * Placeholder — full implementation in Task 2 (34-01 plan).
 * This file exists to satisfy TypeScript module resolution for test stubs.
 */
export function CollapsibleSection({ title, children }: CollapsibleSectionProps): React.ReactElement {
  return (
    <div>
      <button>{title}</button>
      <div>{children}</div>
    </div>
  )
}
