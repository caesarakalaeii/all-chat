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

import React, { useEffect, useRef, useState } from 'react'
import toast from 'react-hot-toast'
import { trackEvent } from '@/lib/analytics'
import { ToggleSwitch } from './ToggleSwitch'
import { SliderControl } from './SliderControl'
import { PremiumBadge } from '@/components/PremiumBadge'
import { PremiumUpsellLink } from '@/components/PremiumUpsellLink'
import { AlertDialog } from '@/components/ui/alert-dialog'
import { useBrowserVoices } from '@/lib/hooks/useBrowserVoices'
import type { DisplaySettings } from '@/lib/types/overlay'

/**
 * TTSGroup — the Text-to-Speech settings group under the AppearancePanel.
 * Mirrors SoundGroup/FilterGroup in shape. Sub-sections: Voice, Throttling,
 * Content, Priority, and Advanced (ElevenLabs premium).
 *
 * Plan 03 replaced Plan 01's Advanced stub with the full ElevenLabs UX:
 * API-key input (Save / Remove), Test-key button + character-quota display,
 * ElevenLabs voice picker (lazy-loaded on focus), read-only OBS URL input
 * with Copy / Regenerate buttons + confirmation modal.
 *
 * See 13-UI-SPEC.md for the authoritative copy and interaction contract.
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
  errorMessage?: string
}

export interface TTSGroupProps {
  displaySettings: Partial<DisplaySettings>
  onChange: (patch: Partial<DisplaySettings>) => void
  isPremium: boolean
  overlayId: string
  hasElevenLabsConfig: boolean
  obsUrl?: string
  // Persisted voice_id for the overlay, read from the GET /tts-config response
  // (Issue #276). Used to initialise the picker so the saved selection is
  // shown on load, and to drive the "Save voice" button visibility.
  savedVoiceId?: string
  onPreview?: () => void
  onPreviewStop?: () => void
  // ElevenLabs async callbacks — Plan 03 wires these in the editor page.
  onSaveKey?: (key: string, voiceId: string) => Promise<void>
  // Voice-only PATCH used after a key is saved (Issue #276) — switches voice
  // without re-submitting the api_key (which the UI doesn't have anymore).
  onSaveVoice?: (voiceId: string) => Promise<void>
  onTestKey?: () => Promise<TestKeyResult>
  onRotateToken?: () => Promise<{ obsUrl: string }>
  onRemoveKey?: () => Promise<void>
  onFetchVoices?: () => Promise<ElevenLabsVoice[]>
  // Lists voices using the typed (unsaved) key — used before the very first
  // save so the picker can populate. See services/overlay-manager
  // /handlers/tts.go HandleGetVoicesPreview.
  onPreviewVoices?: (apiKey: string) => Promise<ElevenLabsVoice[]>
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
      <span className="text-xs font-semibold tracking-wide text-text-dim uppercase">{label}</span>
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
                : 'bg-surface-alt rounded-full border border-border px-3 py-1 text-xs text-text-sub'
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

// ==========================================================================
// Advanced (ElevenLabs) sub-components — Plan 03
// ==========================================================================

interface ApiKeyInputProps {
  hasSavedKey: boolean
  onSave: (key: string, voiceId: string) => Promise<void>
  onRemove: () => Promise<void>
  onTest: () => Promise<TestKeyResult>
  disabled: boolean
  isPremium: boolean
  voiceId: string
  // Controlled value lifted to TTSGroup so the voice picker can react to the
  // typed (unsaved) key in real time.
  apiKey: string
  onApiKeyChange: (next: string) => void
}

function ApiKeyInput({
  hasSavedKey,
  onSave,
  onRemove,
  onTest,
  disabled,
  isPremium,
  voiceId,
  apiKey,
  onApiKeyChange,
}: ApiKeyInputProps): React.ReactElement {
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [testing, setTesting] = useState(false)
  const [quota, setQuota] = useState<{ remaining: number; limit: number } | null>(null)
  const [removeArmed, setRemoveArmed] = useState(false)
  const removeTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  const [removing, setRemoving] = useState(false)

  useEffect(
    () => () => {
      if (removeTimerRef.current) {
        clearTimeout(removeTimerRef.current)
        removeTimerRef.current = null
      }
    },
    []
  )

  async function handleSave(): Promise<void> {
    if (apiKey.trim() === '') {
      setError('API key cannot be empty.')
      return
    }
    if (voiceId.trim() === '') {
      setError('Pick a voice before saving.')
      return
    }
    setSaving(true)
    setError(null)
    try {
      await onSave(apiKey, voiceId)
      // T-13-07 mitigation: clear key from state immediately after POST resolves.
      onApiKeyChange('')
      toast.success('API key saved.')
    } catch (e) {
      setError('Could not save. Try again.')
      toast.error(`Could not save key: ${e instanceof Error ? e.message : 'network error'}`)
    } finally {
      setSaving(false)
    }
  }

  async function handleTest(): Promise<void> {
    setTesting(true)
    try {
      const r = await onTest()
      if (r.ok) {
        if (typeof r.charactersRemaining === 'number' && typeof r.charactersLimit === 'number') {
          setQuota({ remaining: r.charactersRemaining, limit: r.charactersLimit })
        }
      } else {
        // Backend returns 422 for ElevenLabs-rejected keys (missing scope,
        // truly invalid, etc.) with a structured `error` field. Prefer that
        // specific copy when available; fall back to generic per-code toasts.
        if (r.errorCode === 422 && r.errorMessage) {
          toast.error(r.errorMessage)
        } else {
          switch (r.errorCode) {
            case 401:
              toast.error('Invalid API key')
              break
            case 422:
              toast.error('Invalid API key')
              break
            case 429:
              toast.error('Rate-limited — try again in a minute')
              break
            case 0:
              toast.error('Could not reach ElevenLabs. Check your connection.')
              break
            default:
              toast.error('ElevenLabs service unavailable')
          }
        }
      }
    } finally {
      setTesting(false)
    }
  }

  async function handleRemoveClick(): Promise<void> {
    if (!removeArmed) {
      setRemoveArmed(true)
      if (removeTimerRef.current) clearTimeout(removeTimerRef.current)
      removeTimerRef.current = setTimeout(() => {
        setRemoveArmed(false)
        removeTimerRef.current = null
      }, 3000)
      return
    }
    if (removeTimerRef.current) {
      clearTimeout(removeTimerRef.current)
      removeTimerRef.current = null
    }
    setRemoving(true)
    try {
      await onRemove()
      toast.success('API key removed.')
      setQuota(null)
    } catch {
      toast.error('Could not remove key. Try again.')
    } finally {
      setRemoving(false)
      setRemoveArmed(false)
    }
  }

  const quotaPct = quota ? Math.round((quota.remaining / quota.limit) * 100) : null

  return (
    <div className="space-y-3">
      {!hasSavedKey && (
        <div>
          <p className="mb-1 text-xs text-text-dim">
            {isPremium ? (
              'Your key is encrypted server-side and never returned.'
            ) : (
              <>
                <PremiumUpsellLink /> to use ElevenLabs voices.
              </>
            )}
          </p>
          <div className="flex gap-2">
            <input
              type="password"
              value={apiKey}
              onChange={(e) => onApiKeyChange(e.target.value)}
              placeholder="sk-..."
              autoComplete="off"
              spellCheck={false}
              aria-label="ElevenLabs API key"
              disabled={disabled || saving}
              className="flex-1 rounded-lg border border-border bg-surface px-3 py-1.5 font-mono text-sm text-text placeholder:text-text-dim disabled:cursor-not-allowed disabled:opacity-50"
            />
            <button
              type="button"
              onClick={() => {
                void handleSave()
              }}
              disabled={disabled || saving}
              className="hover:bg-surface-alt rounded-lg border border-border bg-surface px-3 py-1.5 text-sm text-text disabled:cursor-not-allowed disabled:opacity-50"
            >
              {saving ? 'Saving…' : 'Save key'}
            </button>
          </div>
          {error && (
            <p role="alert" className="mt-1 text-xs font-medium text-red-400">
              {error}
            </p>
          )}
        </div>
      )}

      {hasSavedKey && (
        <>
          <p className="text-xs text-text-dim">
            Key saved and encrypted. Click Test key to verify.
          </p>
          <button
            type="button"
            onClick={() => {
              void handleTest()
            }}
            disabled={disabled || testing}
            className="hover:bg-surface-alt rounded-lg border border-border bg-surface px-3 py-1.5 text-sm text-text disabled:cursor-not-allowed disabled:opacity-50"
          >
            {testing ? 'Testing…' : 'Test key'}
          </button>

          {quota ? (
            <p className="text-xs text-text-dim">
              {quota.remaining.toLocaleString()} / {quota.limit.toLocaleString()} characters this
              month ({quotaPct}%)
            </p>
          ) : (
            <p className="text-xs text-text-dim">Click Test key to see your remaining quota.</p>
          )}

          <button
            type="button"
            onClick={() => {
              void handleRemoveClick()
            }}
            disabled={disabled || removing}
            className={`hover:bg-surface-alt rounded-lg border px-3 py-1.5 text-sm disabled:cursor-not-allowed disabled:opacity-50 ${
              removeArmed
                ? 'border-red-500 bg-red-500/10 text-red-400'
                : 'border-border bg-surface text-text-sub'
            }`}
          >
            {removing ? 'Removing…' : removeArmed ? 'Confirm remove' : 'Remove key'}
          </button>
        </>
      )}
    </div>
  )
}

interface ObsUrlPanelProps {
  obsUrl: string
  onCopy: () => Promise<void>
  onRegenerate: () => Promise<void>
}

function ObsUrlPanel({ obsUrl, onCopy, onRegenerate }: ObsUrlPanelProps): React.ReactElement {
  const [confirmOpen, setConfirmOpen] = useState(false)
  const [rotating, setRotating] = useState(false)

  async function handleConfirm(): Promise<void> {
    setRotating(true)
    try {
      await onRegenerate()
    } finally {
      setRotating(false)
      setConfirmOpen(false)
    }
  }

  return (
    <div className="space-y-2">
      <p className="text-xs text-text-dim">
        Paste this URL into OBS as your browser source to enable ElevenLabs TTS.
      </p>
      <input
        type="text"
        readOnly
        value={obsUrl}
        onFocus={(e) => e.target.select()}
        aria-label="OBS URL — copy and paste into OBS browser source"
        className="w-full rounded-lg border border-border bg-surface px-3 py-1.5 text-sm text-text select-all"
      />
      <div className="flex gap-2">
        <button
          type="button"
          onClick={() => {
            void onCopy()
          }}
          className="hover:bg-surface-alt rounded-lg border border-border bg-surface px-3 py-1.5 text-sm text-text"
        >
          Copy OBS URL
        </button>
        <button
          type="button"
          onClick={() => setConfirmOpen(true)}
          className="hover:bg-surface-alt rounded-lg border border-border bg-surface px-3 py-1.5 text-sm text-text"
        >
          Regenerate URL
        </button>
      </div>
      <AlertDialog.Root open={confirmOpen} onOpenChange={setConfirmOpen}>
        <AlertDialog.Content size="sm">
          <AlertDialog.Title className="text-sm font-medium">Regenerate OBS URL?</AlertDialog.Title>
          <AlertDialog.Description className="text-xs">
            This invalidates the current OBS URL. You&apos;ll need to paste the new URL into OBS.
          </AlertDialog.Description>
          <div className="mt-4 flex justify-end gap-2">
            <AlertDialog.Close className="hover:bg-surface-alt rounded-lg border border-border bg-surface px-3 py-1.5 text-sm text-text-sub focus-visible:ring-2 focus-visible:ring-twitch focus-visible:outline-none">
              Cancel
            </AlertDialog.Close>
            <button
              type="button"
              onClick={() => {
                void handleConfirm()
              }}
              disabled={rotating}
              className="rounded-lg border border-red-500 bg-red-500/10 px-3 py-1.5 text-sm text-red-400 hover:bg-red-500/20 focus-visible:ring-2 focus-visible:ring-red-500 focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-50"
            >
              {rotating ? 'Regenerating…' : 'Regenerate URL'}
            </button>
          </div>
        </AlertDialog.Content>
      </AlertDialog.Root>
    </div>
  )
}

interface ElevenLabsVoicePickerProps {
  selected: string
  onChange: (voiceId: string) => void
  // Used when the overlay already has a saved key — proxies via the
  // saved-key-aware GET /:id/tts-voices endpoint.
  onFetchVoices?: () => Promise<ElevenLabsVoice[]>
  // Used when the user has typed a key but not yet saved — proxies via the
  // unsaved-key POST /:id/tts-voices/preview endpoint.
  onPreviewVoices?: (apiKey: string) => Promise<ElevenLabsVoice[]>
  // True when the overlay has a persisted ElevenLabs config. Decides which
  // loader to call.
  hasSavedKey: boolean
  // Live (typed) value of the API key input. Drives the preview path. Empty
  // string means the user hasn't started typing.
  typedApiKey: string
  disabled: boolean
}

function ElevenLabsVoicePicker({
  selected,
  onChange,
  onFetchVoices,
  onPreviewVoices,
  hasSavedKey,
  typedApiKey,
  disabled,
}: ElevenLabsVoicePickerProps): React.ReactElement {
  const [voices, setVoices] = useState<ElevenLabsVoice[] | null>(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState(false)
  // Track which key fingerprint produced the current voices list so re-renders
  // with the same value skip the network call.
  const lastKeyRef = useRef<string>('')

  // Auto-load voices whenever the input that drives the loader changes:
  //   - hasSavedKey=true → load once via GET /tts-voices
  //   - hasSavedKey=false → debounce on typedApiKey, load via POST preview
  useEffect(() => {
    let cancelled = false
    let timer: ReturnType<typeof setTimeout> | null = null

    async function run(
      loader: () => Promise<ElevenLabsVoice[]>,
      fingerprint: string
    ): Promise<void> {
      if (lastKeyRef.current === fingerprint) return
      lastKeyRef.current = fingerprint
      setLoading(true)
      setError(false)
      try {
        const list = await loader()
        if (!cancelled) setVoices(list)
      } catch (e) {
        if (!cancelled) {
          setError(true)
          // Surface the backend's specific message (e.g. ElevenLabs
          // missing_permissions text naming the missing scope) instead of a
          // generic "Could not load voices." Falls back to the generic copy
          // when the error has no readable message.
          const msg =
            e instanceof Error && e.message && e.message !== 'Unknown error'
              ? e.message
              : 'Could not load voices.'
          toast.error(msg)
        }
      } finally {
        if (!cancelled) setLoading(false)
      }
    }

    if (hasSavedKey && onFetchVoices) {
      void run(onFetchVoices, '__saved__')
    } else if (!hasSavedKey && onPreviewVoices && typedApiKey.trim().length >= 8) {
      // Debounce so we don't hammer ElevenLabs while the user is still typing.
      timer = setTimeout(() => {
        void run(() => onPreviewVoices(typedApiKey.trim()), `typed:${typedApiKey.trim()}`)
      }, 500)
    } else if (!hasSavedKey) {
      // Not enough input yet — keep the picker empty + reset cache so a future
      // edit re-triggers the loader.
      lastKeyRef.current = ''
      setVoices(null)
      setError(false)
    }
    return () => {
      cancelled = true
      if (timer) clearTimeout(timer)
    }
  }, [hasSavedKey, onFetchVoices, onPreviewVoices, typedApiKey])

  // Auto-select the first voice when the list loads and the user hasn't
  // already chosen one. Without this, Save fails with "Pick a voice before
  // saving." for users who type a key, see voices appear, and click Save
  // without first opening the picker.
  useEffect(() => {
    if (selected === '' && voices && voices.length > 0) {
      onChange(voices[0].voice_id)
    }
  }, [voices, selected, onChange])

  return (
    <div>
      <label htmlFor="tts-elevenlabs-voice" className="mb-1 block text-sm text-text-sub">
        ElevenLabs voice
      </label>
      <select
        id="tts-elevenlabs-voice"
        aria-label="ElevenLabs voice"
        value={selected}
        onChange={(e) => onChange(e.target.value)}
        disabled={disabled}
        className="w-full rounded-lg border border-border bg-surface px-3 py-1.5 text-sm text-text disabled:cursor-not-allowed disabled:opacity-50"
      >
        {loading && <option value="">Loading voices…</option>}
        {error && <option value="">Could not load voices</option>}
        {!loading && !error && voices === null && (
          <option value="">
            {hasSavedKey ? 'Voices will load shortly…' : 'Enter your API key above to load voices.'}
          </option>
        )}
        {!loading && !error && voices !== null && voices.length === 0 && (
          <option value="">No voices available</option>
        )}
        {voices?.map((v) => (
          <option key={v.voice_id} value={v.voice_id}>
            {v.name}
          </option>
        ))}
      </select>
    </div>
  )
}

// ==========================================================================
// TTSGroup — main component
// ==========================================================================

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

  // Advanced block — ElevenLabs voice selection state. Held locally because
  // the chosen voice is sent with the Save-key call, NOT persisted to
  // display_settings (ElevenLabs voice_id lives in overlay_tts_configs).
  // Initialised from savedVoiceId (Issue #276) so the persisted selection is
  // shown on load instead of the auto-pick-first-voice fallback.
  const [pickedVoiceId, setPickedVoiceId] = useState<string>(() => props.savedVoiceId ?? '')
  // Lifted out of ApiKeyInput so the voice picker can react to the typed
  // (unsaved) key in real time and call the preview endpoint.
  const [advancedApiKey, setAdvancedApiKey] = useState('')
  // Saving state for the voice-only PATCH (Issue #276).
  const [savingVoice, setSavingVoice] = useState(false)

  // Sync the picker when the parent provides a fresh savedVoiceId — for
  // example, after the very first key save resolves and the page reads back
  // meta.voice_id from getTTSConfig.
  useEffect(() => {
    if (props.savedVoiceId && pickedVoiceId === '') {
      setPickedVoiceId(props.savedVoiceId)
    }
    // Intentionally only react to savedVoiceId changes; pickedVoiceId is the
    // user's live selection and must not be clobbered after they pick a new
    // voice but before they save it.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [props.savedVoiceId])

  function handlePlatformToggle(platform: string): void {
    const current = d.tts_enabled_platforms ?? [...ALL_PLATFORMS]
    const next = current.includes(platform)
      ? current.filter((p) => p !== platform)
      : [...current, platform]
    onChange({ tts_enabled_platforms: next })
  }

  // ADVANCED (ELEVENLABS) — extracted so it can render directly under the
  // provider radio (where the user expects it after picking ElevenLabs)
  // instead of at the very bottom of the panel after Throttling/Content/
  // Priority. See bug report: "place it directly under the selector so it's
  // immediately visible, not after scrolling a bunch".
  const advancedBlock =
    provider === 'elevenlabs' ? (
      <>
        <SubSectionHeader label="ADVANCED (ELEVENLABS)" />
        <div className={`space-y-3 ${!isPremium ? 'relative' : ''}`}>
          {!isPremium && (
            <div className="absolute inset-0 z-10 flex items-center justify-center rounded-lg bg-surface/80">
              <div className="flex flex-col items-center gap-2 text-center">
                <PremiumBadge />
                <span className="text-xs text-text-dim">
                  <PremiumUpsellLink /> to use ElevenLabs voices.
                </span>
              </div>
            </div>
          )}
          <ApiKeyInput
            hasSavedKey={props.hasElevenLabsConfig}
            isPremium={isPremium}
            disabled={!isPremium}
            voiceId={pickedVoiceId}
            apiKey={advancedApiKey}
            onApiKeyChange={setAdvancedApiKey}
            onSave={props.onSaveKey ?? (async (): Promise<void> => {})}
            onRemove={props.onRemoveKey ?? (async (): Promise<void> => {})}
            onTest={
              props.onTestKey ?? (async (): Promise<TestKeyResult> => ({ ok: false, errorCode: 0 }))
            }
          />
          <ElevenLabsVoicePicker
            selected={pickedVoiceId}
            onChange={setPickedVoiceId}
            onFetchVoices={props.onFetchVoices}
            onPreviewVoices={props.onPreviewVoices}
            hasSavedKey={props.hasElevenLabsConfig}
            typedApiKey={advancedApiKey}
            disabled={!isPremium}
          />
          {/* Save voice (Issue #276) — visible only when the user has changed
            the picker after a key is already saved. The pre-save flow uses the
            "Save key" button inside ApiKeyInput, which carries voice_id. */}
          {props.hasElevenLabsConfig &&
            props.onSaveVoice &&
            pickedVoiceId !== '' &&
            pickedVoiceId !== (props.savedVoiceId ?? '') && (
              <button
                type="button"
                disabled={!isPremium || savingVoice}
                onClick={() => {
                  if (!props.onSaveVoice) return
                  setSavingVoice(true)
                  void (async (): Promise<void> => {
                    try {
                      await props.onSaveVoice!(pickedVoiceId)
                      toast.success('Voice updated.')
                    } catch (e) {
                      toast.error(
                        `Could not save voice: ${e instanceof Error ? e.message : 'network error'}`
                      )
                    } finally {
                      setSavingVoice(false)
                    }
                  })()
                }}
                className="hover:bg-surface-alt rounded-lg border border-border bg-surface px-3 py-1.5 text-sm text-text disabled:cursor-not-allowed disabled:opacity-50"
              >
                {savingVoice ? 'Saving voice…' : 'Save voice'}
              </button>
            )}
          {props.hasElevenLabsConfig && props.obsUrl && (
            <ObsUrlPanel
              obsUrl={props.obsUrl}
              onCopy={async (): Promise<void> => {
                if (!props.obsUrl) return
                try {
                  await navigator.clipboard.writeText(props.obsUrl)
                  toast.success('OBS URL copied.')
                } catch {
                  toast.error('Could not copy URL.')
                }
              }}
              onRegenerate={async (): Promise<void> => {
                if (!props.onRotateToken) return
                try {
                  const result = await props.onRotateToken()
                  try {
                    await navigator.clipboard.writeText(result.obsUrl)
                  } catch {
                    // Clipboard permission missing — toast still surfaces success,
                    // but on failure we fall through to the catch below.
                  }
                  toast.success('New OBS URL copied to clipboard.')
                } catch {
                  toast.error('Could not regenerate URL. Try again.')
                }
              }}
            />
          )}
        </div>
      </>
    ) : null

  return (
    <div className="space-y-4">
      <ToggleSwitch
        label="Enable text-to-speech"
        checked={enabled && supportsSpeech}
        onChange={(checked) => {
          if (!supportsSpeech) return
          if (checked) trackEvent('tts_enabled', { engine: d.tts_provider ?? 'browser' })
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
                <PremiumUpsellLink /> to use ElevenLabs voices.
              </p>
            )}
          </fieldset>

          {/* Render the ElevenLabs config (API key + voice picker + OBS URL)
              directly under the provider radio so it is visible without
              scrolling past Throttling/Content/Priority. */}
          {advancedBlock}

          <SliderControl
            label="Volume"
            value={d.tts_volume ?? 0.8}
            min={0}
            max={1}
            step={0.05}
            onChange={(v) => onChange({ tts_volume: v })}
          />

          {provider !== 'elevenlabs' && (
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
          )}

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
              className="hover:bg-surface-alt rounded-lg border border-border bg-surface px-3 py-1.5 text-sm text-text"
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
              <p className="text-xs text-text-dim">Chance a non-priority message is spoken.</p>
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

          {/* The ADVANCED (ELEVENLABS) block is rendered higher up — directly
              under the provider radio — so the API key field is visible
              without scrolling past Throttling/Content/Priority. */}
        </>
      )}
    </div>
  )
}
