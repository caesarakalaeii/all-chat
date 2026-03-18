'use client'

import React from 'react'
import type { VisualSettings } from '@/lib/types/visual-settings'
import { CollapsibleSection } from './CollapsibleSection'
import { TypographyGroup } from './TypographyGroup'
import { ColorsGroup } from './ColorsGroup'
import { BackgroundGroup } from './BackgroundGroup'

export interface AppearancePanelProps {
  visualSettings: Partial<VisualSettings>
  onChange: (patch: Partial<VisualSettings>) => void
}

export function AppearancePanel({ visualSettings, onChange }: AppearancePanelProps): React.ReactElement {
  return (
    <div className="flex flex-col gap-0">
      <CollapsibleSection id="typography" title="Typography">
        <TypographyGroup visualSettings={visualSettings} onChange={onChange} />
      </CollapsibleSection>
      <CollapsibleSection id="colors" title="Colors">
        <ColorsGroup visualSettings={visualSettings} onChange={onChange} />
      </CollapsibleSection>
      <CollapsibleSection id="background" title="Background & Bubbles">
        <BackgroundGroup visualSettings={visualSettings} onChange={onChange} />
      </CollapsibleSection>
    </div>
  )
}
