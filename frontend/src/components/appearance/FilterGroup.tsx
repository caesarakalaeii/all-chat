'use client'

import React, { useState } from 'react'
import { X } from 'lucide-react'
import { ToggleSwitch } from './ToggleSwitch'
import { SliderControl } from './SliderControl'
import type { FilterSettings } from '@/lib/types/overlay'

export interface FilterGroupProps {
  filterSettings: FilterSettings
  onChange: (patch: Partial<FilterSettings>) => void
}

const COMMON_BOTS = [
  'nightbot', 'streamelements', 'moobot', 'fossabot', 'soundalerts',
  'streamlabs', 'stay_hydrated_bot', 'serybot', 'wizebot', 'botisimo',
]

function TagInput({ tags, onAdd, onRemove, placeholder }: {
  tags: string[]
  onAdd: (tag: string) => void
  onRemove: (tag: string) => void
  placeholder: string
}) {
  const [draft, setDraft] = useState('')

  function handleKeyDown(e: React.KeyboardEvent<HTMLInputElement>) {
    if ((e.key === 'Enter' || e.key === ',') && draft.trim()) {
      e.preventDefault()
      const value = draft.trim().toLowerCase()
      if (!tags.includes(value)) {
        onAdd(value)
      }
      setDraft('')
    }
    if (e.key === 'Backspace' && draft === '' && tags.length > 0) {
      onRemove(tags[tags.length - 1])
    }
  }

  return (
    <div className="flex flex-wrap gap-1 rounded-lg border border-border bg-surface p-2">
      {tags.map(tag => (
        <span key={tag} className="flex items-center gap-1 rounded bg-surface-alt px-2 py-0.5 text-xs text-text">
          {tag}
          <button type="button" onClick={() => onRemove(tag)} aria-label={`Remove ${tag}`}>
            <X className="h-3 w-3" />
          </button>
        </span>
      ))}
      <input
        className="min-w-[120px] flex-1 bg-transparent text-xs text-text outline-none placeholder:text-text-dim"
        value={draft}
        onChange={e => setDraft(e.target.value)}
        onKeyDown={handleKeyDown}
        placeholder={placeholder}
      />
    </div>
  )
}

export function FilterGroup({ filterSettings, onChange }: FilterGroupProps): React.ReactElement {
  function handleAddUser(username: string) {
    onChange({ banned_users: [...(filterSettings.banned_users ?? []), username] })
  }
  function handleRemoveUser(username: string) {
    onChange({ banned_users: (filterSettings.banned_users ?? []).filter(u => u !== username) })
  }
  function handleAddWord(word: string) {
    onChange({ banned_words: [...(filterSettings.banned_words ?? []), word] })
  }
  function handleRemoveWord(word: string) {
    onChange({ banned_words: (filterSettings.banned_words ?? []).filter(w => w !== word) })
  }

  function handleAddCommonBots() {
    const existing = new Set((filterSettings.banned_users ?? []).map(u => u.toLowerCase()))
    const toAdd = COMMON_BOTS.filter(bot => !existing.has(bot))
    if (toAdd.length > 0) {
      onChange({ banned_users: [...(filterSettings.banned_users ?? []), ...toAdd] })
    }
  }

  return (
    <div className="space-y-4">
      <div>
        <p className="mb-1 text-sm text-text-sub">Blocked usernames</p>
        <TagInput
          tags={filterSettings.banned_users ?? []}
          onAdd={handleAddUser}
          onRemove={handleRemoveUser}
          placeholder="Type username, press Enter"
        />
        <button
          type="button"
          className="mt-1 text-xs text-twitch hover:underline"
          onClick={handleAddCommonBots}
        >
          Add common bots
        </button>
      </div>
      <div>
        <p className="mb-1 text-sm text-text-sub">Blocked keywords</p>
        <TagInput
          tags={filterSettings.banned_words ?? []}
          onAdd={handleAddWord}
          onRemove={handleRemoveWord}
          placeholder="Type keyword or regex, press Enter"
        />
      </div>
      <ToggleSwitch
        label="Hide bot commands (!)"
        checked={filterSettings.hide_commands ?? false}
        onChange={(checked) => onChange({ hide_commands: checked })}
      />
      <SliderControl
        label="Min message length"
        value={filterSettings.min_message_length ?? 0}
        min={0}
        max={200}
        step={1}
        unit=" chars"
        onChange={(v) => onChange({ min_message_length: v })}
      />
    </div>
  )
}
