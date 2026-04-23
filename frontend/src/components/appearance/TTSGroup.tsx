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
import { ToggleSwitch } from './ToggleSwitch'
import { SliderControl } from './SliderControl'
import { PremiumBadge } from '@/components/PremiumBadge'
import { useBrowserVoices } from '@/lib/hooks/useBrowserVoices'
import type { DisplaySettings } from '@/lib/types/overlay'

/**
 * TTSGroup — the Text-to-Speech settings group under the AppearancePanel.
 * Mirrors SoundGroup/FilterGroup in shape. Sub-sections: Voice, Throttling,
 * Content, Priority, plus an Advanced (ElevenLabs) block that is a stub in
 * Plan 01 — Plan 03 will replace it with the full ElevenLabs UX.
 *
 * See 13-UI-SPEC.md "Component tree" for the authoritative structure.
 */

export interface ElevenLabsVoice {
  voice_id: string
  name: string
  category?: string
}

export interface TestKeyResult {
  ok: boolean
  charactersRemaining?: number
  charactersLimit?: number
  errorCode?: number
}

export interface TTSGroupProps {
  displaySettings: Partial<DisplaySettings>
  onChange: (patch: Partial<DisplaySettings>) => void
  isPremium: boolean
  overlayId: string
  hasElevenLabsConfig: boolean
  obsUrl?: string
  onPreview?: () => void
  onPreviewStop?: () => void
  // ElevenLabs async callbacks populated in Plan 03 — optional in Plan 01
  onSaveKey?: (key: string, voiceId: string) => Promise<void>
  onTestKey?: () => Promise<TestKeyResult>
  onRotateToken?: () => Promise<{ obsUrl: string }>
  onRemoveKey?: () => Promise<void>
  onFetchVoices?: () => Promise<ElevenLabsVoice[]>
}

const ALL_PLATFORMS: readonly string[] = ['twitch', 'youtube', 'kick', 'tiktok', 'discord'] as const
const PLATFORM_LABELS: Record<string, string> = {
  twitch: 'Twitch',
  youtube: 'YouTube',
  kick: 'Kick',
  tiktok: 'TikTok',
  discord: 'Discord',
}

interface SubHeaderProps {
  label: string
  first?: boolean
}

function SubSectionHeader({ label, first }: SubHeaderProps): React.ReactElement {
  const border = first ? '' : 'border-t border-border pt-4 mt-4'
  return (
    <div className={`flex items-center gap-2 ${border}`}>
      <span className="text-xs font-semibold uppercase tracking-wide text-text-dim">{label}</span>
    </div>
  )
}

interface NumberControlProps {
  label: string
  value: number
  min: number
  max: number
  step?: number
  unit?: string
  onChange: (v: number) => void
}

function NumberControl({
  label,
  value,
  min,
  max,
  step = 1,
  unit,
  onChange,
}: NumberControlProps): React.ReactElement {
  return (
    <div className="flex items-center gap-2">
      <span className="w-40 shrink-0 text-sm text-text-sub">{label}</span>
      <input
        type="number"
        aria-label={label}
        value={value}
        min={min}
        max={max}
        step={step}
        onChange={(e) => {
          const parsed = parseFloat(e.target.value)
          if (Number.isFinite(parsed)) onChange(parsed)
        }}
        className="w-24 rounded-lg border border-border bg-surface px-2 py-1 text-sm text-text"
      />
      {unit && <span className="text-xs text-text-dim">{unit}</span>}
    </div>
  )
}

interface PlatformChipRowProps {
  platforms: string[]
  onToggle: (platform: string) => void
}

function PlatformChipRow({ platforms, onToggle }: PlatformChipRowProps): React.ReactElement {
  return (
    <div className="flex flex-wrap gap-2">
      {ALL_PLATFORMS.map((p) => {
        const active = platforms.includes(p)
        return (
          <button
            key={p}
            type="button"
            onClick={() => onToggle(p)}
            className={
              active
                ? 'rounded-full border border-twitch bg-twitch/15 px-3 py-1 text-xs text-text'
                : 'rounded-full border border-border bg-surface-alt px-3 py-1 text-xs text-text-sub'
            }
            aria-pressed={active}
          >
            {PLATFORM_LABELS[p] ?? p}
          </button>
        )
      })}
    </div>
  )
}

export function TTSGroup(props: TTSGroupProps): React.ReactElement {
  const { displaySettings: d, onChange, isPremium, onPreview } = props

  const voices = useBrowserVoices()

  // Detect Web Speech API availability. In jsdom the global may or may not
  // be present; guard for both cases.
  const supportsSpeech =
    typeof window !== 'undefined' && typeof window.speechSynthesis !== 'undefined'

  const enabled = d.tts_enabled ?? false
  const provider = (d.tts_provider ?? 'browser') as 'browser' | 'elevenlabs'
  const filterMode = d.tts_filter_mode ?? 'sample'
  const platforms = d.tts_enabled_platforms ?? [...ALL_PLATFORMS]

  function handlePlatformToggle(platform: string): void {
    const current = d.tts_enabled_platforms ?? [...ALL_PLATFORMS]
    const next = current.includes(platform)
      ? current.filter((p) => p !== platform)
      : [...current, platform]
    onChange({ tts_enabled_platforms: next })
  }

  return (
    <div className="space-y-4">
      <ToggleSwitch
        label="Enable text-to-speech"
        checked={enabled && supportsSpeech}
        onChange={(checked) => {
          if (!supportsSpeech) return
          onChange({ tts_enabled: checked })
        }}
      />
      {!supportsSpeech && (
        <p className="text-xs text-text-dim">This browser does not support text-to-speech.</p>
      )}

      {enabled && supportsSpeech && (
        <>
          {/* ---------- VOICE ---------- */}
          <SubSectionHeader label="VOICE" first />

          <fieldset className="space-y-2">
            <legend className="text-sm text-text-sub">Voice provider</legend>
            <label className="flex items-center gap-2 text-sm">
              <input
                type="radio"
                name="tts_provider"
                value="browser"
                checked={provider === 'browser'}
                onChange={() => onChange({ tts_provider: 'browser' })}
              />
              <span>Browser (free)</span>
            </label>
            <label className="flex items-center gap-2 text-sm">
              <input
                type="radio"
                name="tts_provider"
                value="elevenlabs"
                checked={provider === 'elevenlabs'}
                onChange={() => onChange({ tts_provider: 'elevenlabs' })}
                disabled={!isPremium}
              />
              <span>ElevenLabs (premium)</span>
              {!isPremium && <PremiumBadge />}
            </label>
            {!isPremium && (
              <p className="text-xs text-text-dim">
                Upgrade to Premium to use ElevenLabs voices.
              </p>
            )}
          </fieldset>

          <SliderControl
            label="Volume"
            value={d.tts_volume ?? 0.8}
            min={0}
            max={1}
            step={0.05}
            onChange={(v) => onChange({ tts_volume: v })}
          />

          <div>
            <label className="mb-1 block text-sm text-text-sub">
              Voice
              <select
                aria-label="Voice"
                value={d.tts_voice_uri ?? ''}
                onChange={(e) => onChange({ tts_voice_uri: e.target.value })}
                className="mt-1 block w-full rounded-lg border border-border bg-surface px-3 py-1.5 text-sm text-text"
              >
                <option value="">Default</option>
                {voices.map((v) => (
                  <option key={v.voiceURI} value={v.voiceURI}>
                    {v.name} ({v.lang})
                  </option>
                ))}
              </select>
            </label>
            <p className="text-xs text-text-dim">
              Browser voice — list depends on your OS/browser.
            </p>
          </div>

          <SliderControl
            label="Speech rate"
            value={d.tts_rate ?? 1.0}
            min={0.5}
            max={2}
            step={0.05}
            onChange={(v) => onChange({ tts_rate: v })}
          />

          {provider !== 'elevenlabs' && (
            <SliderControl
              label="Pitch"
              value={d.tts_pitch ?? 1.0}
              min={0}
              max={2}
              step={0.05}
              onChange={(v) => onChange({ tts_pitch: v })}
            />
          )}

          {onPreview && (
            <button
              type="button"
              onClick={onPreview}
              className="rounded-lg border border-border bg-surface px-3 py-1.5 text-sm text-text hover:bg-surface-alt"
            >
              Test voice
            </button>
          )}

          {/* ---------- THROTTLING ---------- */}
          <SubSectionHeader label="THROTTLING" />

          <fieldset className="space-y-2">
            <legend className="text-sm text-text-sub">Which messages are spoken</legend>
            {(['all', 'sample', 'priority_only'] as const).map((mode) => (
              <label key={mode} className="flex items-center gap-2 text-sm">
                <input
                  type="radio"
                  name="tts_filter_mode"
                  value={mode}
                  checked={filterMode === mode}
                  onChange={() => onChange({ tts_filter_mode: mode })}
                />
                <span>
                  {mode === 'all' && 'All'}
                  {mode === 'sample' && 'Sample'}
                  {mode === 'priority_only' && 'Priority-only'}
                </span>
              </label>
            ))}
          </fieldset>

          {filterMode === 'sample' && (
            <div>
              <SliderControl
                label="Sample rate"
                value={d.tts_sample_rate ?? 0.25}
                min={0}
                max={1}
                step={0.05}
                onChange={(v) => onChange({ tts_sample_rate: v })}
              />
              <p className="text-xs text-text-dim">
                Chance a non-priority message is spoken.
              </p>
            </div>
          )}

          <NumberControl
            label="Max queue length"
            value={d.tts_max_queue ?? 5}
            min={1}
            max={50}
            onChange={(v) => onChange({ tts_max_queue: v })}
          />
          <NumberControl
            label="Messages per minute"
            value={d.tts_messages_per_minute ?? 8}
            min={1}
            max={120}
            onChange={(v) => onChange({ tts_messages_per_minute: v })}
          />
          <NumberControl
            label="Per-user cooldown"
            value={d.tts_user_cooldown_seconds ?? 30}
            min={0}
            max={600}
            unit=" s"
            onChange={(v) => onChange({ tts_user_cooldown_seconds: v })}
          />
          <NumberControl
            label="Drop messages older than"
            value={d.tts_staleness_seconds ?? 15}
            min={1}
            max={300}
            unit=" s"
            onChange={(v) => onChange({ tts_staleness_seconds: v })}
          />

          {/* ---------- CONTENT ---------- */}
          <SubSectionHeader label="CONTENT" />

          <ToggleSwitch
            label="Read username"
            checked={d.tts_read_username ?? true}
            onChange={(v) => onChange({ tts_read_username: v })}
          />
          <ToggleSwitch
            label="Read platform name"
            checked={d.tts_read_platform ?? false}
            onChange={(v) => onChange({ tts_read_platform: v })}
          />
          <NumberControl
            label="Max message length"
            value={d.tts_max_message_chars ?? 200}
            min={20}
            max={1000}
            unit=" chars"
            onChange={(v) => onChange({ tts_max_message_chars: v })}
          />
          <ToggleSwitch
            label="Skip emote-only messages"
            checked={d.tts_skip_emote_only ?? true}
            onChange={(v) => onChange({ tts_skip_emote_only: v })}
          />
          <ToggleSwitch
            label="Skip link-only messages"
            checked={d.tts_skip_links ?? true}
            onChange={(v) => onChange({ tts_skip_links: v })}
          />

          <div>
            <p className="mb-2 text-sm text-text-sub">Platforms</p>
            <PlatformChipRow platforms={platforms} onToggle={handlePlatformToggle} />
          </div>

          {/* ---------- PRIORITY ---------- */}
          <SubSectionHeader label="PRIORITY" />

          <ToggleSwitch
            label="Announce priority events"
            checked={d.tts_priority_events ?? true}
            onChange={(v) => onChange({ tts_priority_events: v })}
          />
          {(d.tts_priority_events ?? true) && (
            <NumberControl
              label="Minimum bits to announce"
              value={d.tts_priority_bits_min ?? 0}
              min={0}
              max={100000}
              onChange={(v) => onChange({ tts_priority_bits_min: v })}
            />
          )}

          {/* ---------- ADVANCED (ELEVENLABS) — STUB FOR PLAN 03 ---------- */}
          {provider === 'elevenlabs' && (
            <>
              <SubSectionHeader label="ADVANCED (ELEVENLABS)" />
              <div className="rounded-lg border border-dashed border-border bg-surface-alt p-4 text-sm text-text-sub">
                <p>
                  ElevenLabs controls (API key, voice picker, test, OBS URL) ship in Plan 03.
                </p>
                {!isPremium && (
                  <div className="mt-2 flex items-center gap-2">
                    <PremiumBadge />
                    <span className="text-xs text-text-dim">
                      Upgrade to Premium to use ElevenLabs voices.
                    </span>
                  </div>
                )}
              </div>
            </>
          )}
        </>
      )}
    </div>
  )
}
