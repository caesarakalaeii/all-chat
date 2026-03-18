'use client'

import React from 'react'
import type { VisualSettings } from '@/lib/types/visual-settings'
import { CollapsibleSection } from './CollapsibleSection'
import { TypographyGroup } from './TypographyGroup'
import { ColorsGroup } from './ColorsGroup'
import { BackgroundGroup } from './BackgroundGroup'
import { VisibilityGroup } from './VisibilityGroup'
import { SizingGroup } from './SizingGroup'
import { PlatformColorsGroup } from './PlatformColorsGroup'
import { EventsGroup } from './EventsGroup'

export interface AppearancePanelProps {
  visualSettings: Partial<VisualSettings>
  onChange: (patch: Partial<VisualSettings>) => void
  visibilityDefaults?: Partial<VisualSettings>
}

export function AppearancePanel({ visualSettings, onChange, visibilityDefaults = {} }: AppearancePanelProps): React.ReactElement {
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
      <CollapsibleSection id="visibility" title="Visibility">
        <VisibilityGroup
          visualSettings={visualSettings}
          onChange={onChange}
          visibilityDefaults={visibilityDefaults}
        />
      </CollapsibleSection>
      <CollapsibleSection id="sizing" title="Sizing">
        <SizingGroup visualSettings={visualSettings} onChange={onChange} />
      </CollapsibleSection>
      <CollapsibleSection id="platform-colors" title="Platform Colors">
        <PlatformColorsGroup visualSettings={visualSettings} onChange={onChange} />
      </CollapsibleSection>
      <CollapsibleSection id="events" title="Events">
        <EventsGroup visualSettings={visualSettings} onChange={onChange} />
      </CollapsibleSection>
    </div>
  )
}
