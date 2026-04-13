'use client'

import React from 'react'
import type { VisualSettings } from '@/lib/types/visual-settings'
import type { FilterSettings, DisplaySettings } from '@/lib/types/overlay'
import { CollapsibleSection } from './CollapsibleSection'
import { TypographyGroup } from './TypographyGroup'
import { ColorsGroup } from './ColorsGroup'
import { BackgroundGroup } from './BackgroundGroup'
import { VisibilityGroup } from './VisibilityGroup'
import { SizingGroup } from './SizingGroup'
import { PlatformColorsGroup } from './PlatformColorsGroup'
import { EventsGroup } from './EventsGroup'
import { FilterGroup } from './FilterGroup'
import { SoundGroup } from './SoundGroup'

export interface AppearancePanelProps {
  visualSettings: Partial<VisualSettings>
  onChange: (patch: Partial<VisualSettings>) => void
  visibilityDefaults?: Partial<VisualSettings>
  filterSettings?: FilterSettings
  onFilterChange?: (patch: Partial<FilterSettings>) => void
  displaySettings?: Partial<DisplaySettings>
  onSoundChange?: (patch: Partial<DisplaySettings>) => void
  isPremium?: boolean
}

export function AppearancePanel({
  visualSettings,
  onChange,
  visibilityDefaults = {},
  filterSettings,
  onFilterChange,
  displaySettings,
  onSoundChange,
  isPremium,
}: AppearancePanelProps): React.ReactElement {
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
      {filterSettings && onFilterChange && (
        <CollapsibleSection id="filters" title="Filters">
          <FilterGroup filterSettings={filterSettings} onChange={onFilterChange} />
        </CollapsibleSection>
      )}
      {displaySettings && onSoundChange && (
        <CollapsibleSection id="sounds" title="Notification Sounds">
          <SoundGroup
            displaySettings={displaySettings}
            onChange={onSoundChange}
            isPremium={isPremium ?? false}
          />
        </CollapsibleSection>
      )}
    </div>
  )
}
