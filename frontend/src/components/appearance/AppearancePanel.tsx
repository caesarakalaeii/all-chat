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
import { TTSGroup } from './TTSGroup'
import type { ElevenLabsVoice, TestKeyResult } from './TTSGroup'

export interface AppearancePanelProps {
  visualSettings: Partial<VisualSettings>
  onChange: (patch: Partial<VisualSettings>) => void
  visibilityDefaults?: Partial<VisualSettings>
  filterSettings?: FilterSettings
  onFilterChange?: (patch: Partial<FilterSettings>) => void
  displaySettings?: Partial<DisplaySettings>
  onSoundChange?: (patch: Partial<DisplaySettings>) => void
  isPremium?: boolean
  // Phase 13: Text-to-Speech wiring (Plan 01 passes optionally; Plan 03 fills
  // the async callbacks for the Advanced ElevenLabs block)
  onTTSChange?: (patch: Partial<DisplaySettings>) => void
  overlayId?: string
  hasElevenLabsConfig?: boolean
  obsUrl?: string
  onTTSPreview?: () => void
  onTTSPreviewStop?: () => void
  onSaveTTSKey?: (key: string, voiceId: string) => Promise<void>
  onTestTTSKey?: () => Promise<TestKeyResult>
  onRotateTTSToken?: () => Promise<{ obsUrl: string }>
  onRemoveTTSKey?: () => Promise<void>
  onFetchTTSVoices?: () => Promise<ElevenLabsVoice[]>
  onPreviewTTSVoices?: (apiKey: string) => Promise<ElevenLabsVoice[]>
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
  onTTSChange,
  overlayId,
  hasElevenLabsConfig,
  obsUrl,
  onTTSPreview,
  onTTSPreviewStop,
  onSaveTTSKey,
  onTestTTSKey,
  onRotateTTSToken,
  onRemoveTTSKey,
  onFetchTTSVoices,
  onPreviewTTSVoices,
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
      {displaySettings && onTTSChange && overlayId && (
        <CollapsibleSection id="tts" title="Text-to-Speech">
          <TTSGroup
            displaySettings={displaySettings}
            onChange={onTTSChange}
            isPremium={isPremium ?? false}
            overlayId={overlayId}
            hasElevenLabsConfig={hasElevenLabsConfig ?? false}
            obsUrl={obsUrl}
            onPreview={onTTSPreview}
            onPreviewStop={onTTSPreviewStop}
            onSaveKey={onSaveTTSKey}
            onTestKey={onTestTTSKey}
            onRotateToken={onRotateTTSToken}
            onRemoveKey={onRemoveTTSKey}
            onFetchVoices={onFetchTTSVoices}
            onPreviewVoices={onPreviewTTSVoices}
          />
        </CollapsibleSection>
      )}
    </div>
  )
}
