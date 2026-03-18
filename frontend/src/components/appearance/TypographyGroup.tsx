'use client'

import React from 'react'
import type { VisualSettings } from '@/lib/types/visual-settings'

export interface TypographyGroupProps {
  visualSettings: Partial<VisualSettings>
  onChange: (patch: Partial<VisualSettings>) => void
}

/**
 * Placeholder — full implementation in Task 2 (34-01 plan).
 * This file exists to satisfy TypeScript module resolution for test stubs.
 */
export function TypographyGroup({ visualSettings: _visualSettings, onChange: _onChange }: TypographyGroupProps): React.ReactElement {
  return <div>TypographyGroup placeholder</div>
}
