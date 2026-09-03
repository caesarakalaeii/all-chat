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

import { useState, useEffect, useCallback, useId, useRef } from 'react'
import Link from 'next/link'
import { AppNav } from '@/components/AppNav'
import { Card } from '@/components/ui/card'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { buildGradientCSS } from '@/lib/utils/gradient'
import { cn } from '@/lib/utils'
import { safeExternalRedirect } from '@/lib/auth/redirect-allowlist'
import type { NameGradient } from '@/lib/types/message'
import { UserAvatar } from '@/components/UserAvatar'
import { useTranslations } from '@/lib/i18n'

// JWT claims from viewer token
interface ViewerJWTClaims {
  viewer_id?: string
  display_name?: string
  username?: string
  avatar_url?: string
  platform?: 'twitch' | 'youtube' | 'kick' | 'tiktok'
  name_color?: string
  name_gradient?: NameGradient
  is_premium?: boolean
  exp?: number
}

const PLATFORMS: { key: 'twitch' | 'youtube' | 'kick'; color: string }[] = [
  { key: 'twitch', color: 'bg-purple-500' },
  { key: 'youtube', color: 'bg-red-500' },
  { key: 'kick', color: 'bg-green-500' },
]

function decodeViewerJWT(token: string): ViewerJWTClaims | null {
  try {
    const payload = token.split('.')[1]
    if (!payload) return null
    const claims = JSON.parse(atob(payload)) as ViewerJWTClaims
    // Check expiry
    if (claims.exp && claims.exp * 1000 < Date.now()) return null
    return claims
  } catch {
    return null
  }
}

function PlatformBadge({ platform }: { platform: string }) {
  const t = useTranslations()
  const found = PLATFORMS.find((p) => p.key === platform)
  if (!found) return null
  return (
    <span
      className={cn(
        'inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-xs font-medium text-bg',
        found.color
      )}
    >
      {t(`common.platforms.${found.key}`)}
    </span>
  )
}

/**
 * Start a viewer OAuth login for initial sign-in (no existing viewer session).
 */
async function viewerLogin(platform: 'twitch' | 'youtube' | 'kick') {
  try {
    const params = new URLSearchParams({ redirect_to: '/settings/viewer' })
    const response = await fetch(`/api/v1/auth/viewer/${platform}/login?${params}`)
    const data = await response.json()
    if (data.auth_url) {
      safeExternalRedirect(data.auth_url)
    }
  } catch {
    // silently ignore — user stays on page
  }
}

/**
 * Start a viewer OAuth connect flow that links the new platform to the existing
 * viewer identity.  The current viewer_id from localStorage is passed as
 * link_viewer_id so the backend can associate the new platform session with
 * the same viewer record instead of creating a fresh one.
 */
async function viewerConnect(platform: 'twitch' | 'youtube' | 'kick', viewerID: string) {
  try {
    const params = new URLSearchParams({
      redirect_to: '/settings/viewer',
      link_viewer_id: viewerID,
    })
    const response = await fetch(`/api/v1/auth/viewer/${platform}/login?${params}`)
    const data = await response.json()
    if (data.auth_url) {
      safeExternalRedirect(data.auth_url)
    }
  } catch {
    // silently ignore — user stays on page
  }
}

function UnauthenticatedState() {
  const t = useTranslations()
  return (
    <div className="min-h-screen bg-bg">
      <AppNav />
      <main id="main-content" tabIndex={-1} className="mx-auto max-w-2xl space-y-6 px-4 py-12">
        <h1 className="text-2xl font-bold text-text">{t('settings.viewer.heading')}</h1>
        <p className="text-sm text-text-sub">{t('settings.viewer.subheading')}</p>

        <Card className="p-6">
          <h2 className="mb-2 text-lg font-semibold text-text">
            {t('settings.viewer.signInHeading')}
          </h2>
          <p className="mb-6 text-sm text-text-sub">{t('settings.viewer.signInBody')}</p>
          <div className="flex flex-col gap-3 sm:flex-row">
            <Button onClick={() => viewerLogin('twitch')} size="lg" className="gap-2.5 px-6 py-3">
              <svg
                className="h-5 w-5 shrink-0"
                viewBox="0 0 24 24"
                xmlns="http://www.w3.org/2000/svg"
                aria-hidden="true"
              >
                <path
                  fill="currentColor"
                  d="M11.571 4.714h1.715v5.143H11.57zm4.715 0H18v5.143h-1.714zM6 0L1.714 4.286v15.428h5.143V24l4.286-4.286h3.428L22.286 12V0zm14.571 11.143l-3.428 3.428h-3.429l-3 3v-3H6.857V1.714h13.714z"
                />
              </svg>
              {t('settings.viewer.signInTwitch')}
            </Button>
            <Button
              onClick={() => viewerLogin('youtube')}
              size="lg"
              className="gap-2.5 px-6 py-3 text-bg"
              style={{ backgroundColor: '#FF0000', ['--tw-ring-color' as string]: '#FF0000' }}
            >
              <svg
                className="h-5 w-5 shrink-0"
                viewBox="0 0 24 24"
                xmlns="http://www.w3.org/2000/svg"
                aria-hidden="true"
              >
                <path
                  fill="#FFFFFF"
                  d="M23.498 6.186a3.016 3.016 0 0 0-2.122-2.136C19.505 3.545 12 3.545 12 3.545s-7.505 0-9.377.505A3.017 3.017 0 0 0 .502 6.186C0 8.07 0 12 0 12s0 3.93.502 5.814a3.016 3.016 0 0 0 2.122 2.136c1.871.505 9.376.505 9.376.505s7.505 0 9.377-.505a3.015 3.015 0 0 0 2.122-2.136C24 15.93 24 12 24 12s0-3.93-.502-5.814zM9.545 15.568V8.432L15.818 12l-6.273 3.568z"
                />
              </svg>
              {t('settings.viewer.signInYoutube')}
            </Button>
            <Button
              onClick={() => viewerLogin('kick')}
              size="lg"
              className="gap-2.5 px-6 py-3 text-bg"
              style={{ backgroundColor: 'var(--color-kick)' }}
            >
              <svg
                className="h-5 w-5 shrink-0"
                viewBox="0 0 512 512"
                xmlns="http://www.w3.org/2000/svg"
                aria-hidden="true"
              >
                <path
                  fill="currentColor"
                  d="M37 .036h164.448v113.621h54.71v-56.82h54.731V.036h164.448v170.777h-54.73v56.82h-54.711v56.8h54.71v56.82h54.73V512.03H310.89v-56.82h-54.73v-56.8h-54.711v113.62H37V.036z"
                />
              </svg>
              {t('settings.viewer.signInKick')}
            </Button>
          </div>
        </Card>
      </main>
    </div>
  )
}

function ColorGradientCard({ claims }: { claims: ViewerJWTClaims }) {
  const t = useTranslations()
  const [activeTab, setActiveTab] = useState<'solid' | 'gradient'>('solid')
  const angleId = useId()

  const [nameColor, setNameColor] = useState<string>(claims.name_color ?? '#9146ff')
  const [savedFeedback, setSavedFeedback] = useState(false)
  const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null)

  const [gradientStops, setGradientStops] = useState<string[]>(['#9146ff', '#00b5ad'])
  const [gradientAngle, setGradientAngle] = useState<number>(90)

  // Fetch saved cosmetics from the backend on mount so values persist across
  // page loads (the JWT does not carry cosmetic data).
  useEffect(() => {
    const token = typeof window !== 'undefined' ? localStorage.getItem('viewer_jwt_token') : null
    if (!token || !claims.viewer_id) return

    fetch('/api/v1/auth/viewer/cosmetics', {
      headers: { Authorization: `Bearer ${token}` },
    })
      .then((res) => (res.ok ? res.json() : null))
      .then((data: { name_color?: string | null; name_gradient?: NameGradient | null } | null) => {
        if (!data) return
        if (data.name_color) {
          setNameColor(data.name_color)
        }
        if (data.name_gradient) {
          setActiveTab('gradient')
          if (data.name_gradient.colors && data.name_gradient.colors.length >= 2) {
            setGradientStops(data.name_gradient.colors)
          }
          if (data.name_gradient.angle != null) {
            setGradientAngle(data.name_gradient.angle)
          }
        }
      })
      .catch(() => {
        // Best-effort — fall back to JWT defaults / hardcoded defaults
      })
  }, [claims.viewer_id])

  const saveColor = useCallback(async (color: string) => {
    setNameColor(color)
    try {
      const token = typeof window !== 'undefined' ? localStorage.getItem('viewer_jwt_token') : null
      const headers: Record<string, string> = { 'Content-Type': 'application/json' }
      if (token) headers['Authorization'] = `Bearer ${token}`

      const res = await fetch('/api/v1/auth/viewer/cosmetics', {
        method: 'PATCH',
        headers,
        body: JSON.stringify({ name_color: color, name_gradient: null }),
      })

      if (res.ok) {
        setSavedFeedback(true)
        setTimeout(() => setSavedFeedback(false), 2000)
      }
    } catch {
      // Silently fail — cosmetics are best-effort
    }
  }, [])

  const debouncedSaveColor = useCallback(
    (color: string) => {
      if (timerRef.current) clearTimeout(timerRef.current)
      timerRef.current = setTimeout(() => {
        if (/^#[0-9a-fA-F]{6}$/.test(color)) saveColor(color)
      }, 400)
    },
    [saveColor]
  )
  const [gradientSaving, setGradientSaving] = useState(false)
  const [gradientError, setGradientError] = useState<string | null>(null)

  const handleSaveGradient = useCallback(async () => {
    // Re-validate premium from localStorage JWT before sending PATCH
    const token = typeof window !== 'undefined' ? localStorage.getItem('viewer_jwt_token') : null
    if (token) {
      const latestClaims = decodeViewerJWT(token)
      if (!latestClaims?.is_premium) {
        setGradientError(t('settings.viewer.premiumRequired'))
        return
      }
    } else {
      setGradientError(t('settings.viewer.premiumRequired'))
      return
    }

    setGradientSaving(true)
    setGradientError(null)
    try {
      const headers: Record<string, string> = { 'Content-Type': 'application/json' }
      if (token) headers['Authorization'] = `Bearer ${token}`

      const res = await fetch('/api/v1/auth/viewer/cosmetics', {
        method: 'PATCH',
        headers,
        body: JSON.stringify({
          name_color: null,
          name_gradient: { type: 'linear', colors: gradientStops, angle: gradientAngle },
        }),
      })

      if (!res.ok) {
        if (res.status === 403) {
          setGradientError(t('settings.viewer.premiumRequired'))
        } else {
          setGradientError(t('settings.viewer.saveFailed'))
        }
      }
    } catch {
      setGradientError(t('settings.viewer.saveFailed'))
    } finally {
      setGradientSaving(false)
    }
  }, [gradientStops, gradientAngle, t])

  return (
    <Card className="p-6">
      <h2 className="mb-1 text-lg font-semibold text-text">
        {t('settings.viewer.nameColorHeading')}
      </h2>
      <p className="mb-4 text-sm text-text-sub">{t('settings.viewer.nameColorBody')}</p>

      {/* Tab bar */}
      <Tabs
        value={activeTab}
        onValueChange={(value) => setActiveTab(value as 'solid' | 'gradient')}
        className="mb-4"
      >
        <TabsList variant="line" className="w-full justify-start border-b border-border">
          <TabsTrigger value="solid">{t('settings.viewer.solidTab')}</TabsTrigger>
          {/* Gating stays on `disabled` rather than on an onClick guard: the
              trigger then reports aria-disabled and drops out of the tab list's
              arrow-key order, instead of looking focusable and silently
              refusing. */}
          <TabsTrigger value="gradient" disabled={!claims.is_premium}>
            {t('settings.viewer.gradientTab')}
            {!claims.is_premium && (
              <span className="rounded bg-premium/20 px-1.5 py-0.5 text-xs text-premium">
                {t('settings.viewer.premiumPill')}
              </span>
            )}
          </TabsTrigger>
        </TabsList>
      </Tabs>

      {!claims.is_premium && (
        <p className="mb-4 text-xs text-text-dim">
          {t('settings.viewer.gradientUpsell')}{' '}
          <Link href="/settings/viewer/premium" className="font-medium text-twitch hover:underline">
            {t('settings.viewer.unlockPremium')}
          </Link>
        </p>
      )}

      {/* Solid Color tab panel */}
      {activeTab === 'solid' && (
        <div>
          <div className="flex items-center gap-3">
            <input
              type="color"
              value={nameColor}
              onChange={(e) => saveColor(e.target.value)}
              className="h-10 w-10 cursor-pointer rounded border border-border bg-transparent"
              aria-label={t('settings.viewer.colorPickerLabel')}
            />
            <Input
              type="text"
              value={nameColor}
              onChange={(e) => debouncedSaveColor(e.target.value)}
              className="w-28 font-mono"
              aria-label={t('settings.viewer.colorHexLabel')}
              maxLength={7}
            />
            {savedFeedback && (
              <span className="ml-2 text-xs text-green-400">
                {t('settings.viewer.savedFeedback')}
              </span>
            )}
          </div>
          <p className="mt-2 text-xs text-text-sub">{t('settings.viewer.autoSaveNote')}</p>

          {/* Live preview */}
          <div className="mt-4 border-t border-border pt-4">
            <p className="mb-2 text-xs text-text-sub">{t('settings.viewer.previewLabel')}</p>
            <div className="flex items-center gap-2">
              <div className="flex h-6 w-6 items-center justify-center rounded-full bg-surface-2 text-xs font-medium text-text-sub">
                {(claims.display_name ?? claims.username ?? 'V').charAt(0).toUpperCase()}
              </div>
              <span className="text-sm font-semibold" style={{ color: nameColor }}>
                {claims.display_name ?? claims.username ?? t('settings.viewer.viewerFallbackName')}
              </span>
              <span className="text-sm text-text-sub">{t('settings.viewer.previewMessage')}</span>
            </div>
          </div>
        </div>
      )}

      {/* Gradient tab panel */}
      {activeTab === 'gradient' && (
        <div>
          {/* Color stop list */}
          <div>
            {gradientStops.map((stop, i) => (
              <div key={i} className="mb-2 flex items-center gap-2">
                <input
                  type="color"
                  value={stop}
                  onChange={(e) => {
                    const s = [...gradientStops]
                    s[i] = e.target.value
                    setGradientStops(s)
                  }}
                  className="h-8 w-8 cursor-pointer rounded border border-border"
                />
                <Input
                  type="text"
                  value={stop}
                  onChange={(e) => {
                    if (/^#[0-9a-fA-F]{6}$/.test(e.target.value)) {
                      const s = [...gradientStops]
                      s[i] = e.target.value
                      setGradientStops(s)
                    }
                  }}
                  className="w-24 font-mono"
                />
                {gradientStops.length > 2 && (
                  <Button
                    onClick={() => setGradientStops(gradientStops.filter((_, j) => j !== i))}
                    variant="ghost"
                    size="icon-xs"
                    className="text-lg leading-none"
                    aria-label={t('settings.viewer.removeStopLabel', { index: i + 1 })}
                  >
                    ×
                  </Button>
                )}
              </div>
            ))}
          </div>

          {/* Add stop button */}
          <Button
            variant="outline"
            size="sm"
            disabled={gradientStops.length >= 4}
            onClick={() => setGradientStops([...gradientStops, '#ffffff'])}
          >
            {t('settings.viewer.addStop')}
          </Button>

          {/* Angle controls */}
          <div className="mt-3 flex items-center gap-3">
            <label htmlFor={angleId} className="w-10 text-xs text-text-sub">
              {t('settings.viewer.angleLabel')}
            </label>
            <input
              id={angleId}
              type="range"
              min={0}
              max={360}
              value={gradientAngle}
              onChange={(e) => setGradientAngle(Number(e.target.value))}
              className="flex-1"
            />
            <Input
              type="number"
              min={0}
              max={360}
              value={gradientAngle}
              onChange={(e) => setGradientAngle(Math.min(360, Math.max(0, Number(e.target.value))))}
              aria-label={t('settings.viewer.angleDegreesLabel')}
              className="w-16 text-right"
            />
            <span className="text-xs text-text-sub">°</span>
          </div>

          {/* Save gradient button + error */}
          <div className="mt-4 flex items-center gap-3">
            <Button onClick={handleSaveGradient} disabled={gradientSaving}>
              {gradientSaving
                ? t('settings.viewer.savingGradient')
                : t('settings.viewer.saveGradient')}
            </Button>
            {gradientError && <span className="text-xs text-red-400">{gradientError}</span>}
          </div>

          {/* Live preview */}
          <div className="mt-4 border-t border-border pt-4">
            <p className="mb-2 text-xs text-text-sub">{t('settings.viewer.previewLabel')}</p>
            <div className="flex items-center gap-2">
              <div className="flex h-6 w-6 items-center justify-center rounded-full bg-surface-2 text-xs font-medium text-text-sub">
                {(claims.display_name ?? 'V').charAt(0).toUpperCase()}
              </div>
              <span
                className="bg-clip-text text-sm font-semibold text-transparent"
                style={{
                  backgroundImage: buildGradientCSS({
                    type: 'linear',
                    colors: gradientStops,
                    angle: gradientAngle,
                  }),
                }}
              >
                {claims.display_name ?? claims.username ?? t('settings.viewer.viewerFallbackName')}
              </span>
              <span className="text-sm text-text-sub">{t('settings.viewer.previewMessage')}</span>
            </div>
          </div>
        </div>
      )}
    </Card>
  )
}

interface CatalogItem {
  id: string | null
  name: string
  image_url: string
  is_premium: boolean
}

// The "no frame" / "no flair" row prepended to each catalog. Its name is copy,
// so it is resolved per render rather than baked into a module constant.
const NONE_ITEM: Omit<CatalogItem, 'name'> = { id: null, image_url: '', is_premium: false }

function AvatarCosmeticsCard({ claims }: { claims: ViewerJWTClaims }) {
  const t = useTranslations()
  const isPremium = claims.is_premium ?? false
  const [frames, setFrames] = useState<CatalogItem[]>([])
  const [flairs, setFlairs] = useState<CatalogItem[]>([])
  const [selectedFrameId, setSelectedFrameId] = useState<string | null>(null)
  const [selectedFlairId, setSelectedFlairId] = useState<string | null>(null)
  const [previewFrameUrl, setPreviewFrameUrl] = useState<string>('')
  const [previewFlairUrl, setPreviewFlairUrl] = useState<string>('')
  const [saving, setSaving] = useState(false)
  const [saveError, setSaveError] = useState<string | null>(null)
  const [savedFeedback, setSavedFeedback] = useState(false)

  useEffect(() => {
    const fetchCatalogsAndSelection = async () => {
      try {
        const token =
          typeof window !== 'undefined' ? localStorage.getItem('viewer_jwt_token') : null
        const cosmeticsHeaders: Record<string, string> = {}
        if (token) cosmeticsHeaders['Authorization'] = `Bearer ${token}`

        const [framesRes, flairsRes, cosmeticsRes] = await Promise.all([
          fetch('/api/v1/auth/viewer/catalog/frames'),
          fetch('/api/v1/auth/viewer/catalog/flairs'),
          token && claims.viewer_id
            ? fetch('/api/v1/auth/viewer/cosmetics', { headers: cosmeticsHeaders })
            : Promise.resolve(null),
        ])

        const noneItem: CatalogItem = { ...NONE_ITEM, name: t('settings.viewer.noneItem') }
        let frameList: CatalogItem[] = [noneItem]
        let flairList: CatalogItem[] = [noneItem]

        if (framesRes.ok) {
          const data = (await framesRes.json()) as { frames: CatalogItem[] }
          frameList = [noneItem, ...(data.frames ?? [])]
          setFrames(frameList)
        }
        if (flairsRes.ok) {
          const data = (await flairsRes.json()) as { flairs: CatalogItem[] }
          flairList = [noneItem, ...(data.flairs ?? [])]
          setFlairs(flairList)
        }

        // Restore saved selection so the correct item is highlighted after page reload.
        if (cosmeticsRes?.ok) {
          const cosmetics = (await cosmeticsRes.json()) as {
            avatar_frame_id?: string | null
            avatar_flair_id?: string | null
          }
          if (cosmetics.avatar_frame_id) {
            setSelectedFrameId(cosmetics.avatar_frame_id)
            const found = frameList.find((f) => f.id === cosmetics.avatar_frame_id)
            if (found) setPreviewFrameUrl(found.image_url)
          }
          if (cosmetics.avatar_flair_id) {
            setSelectedFlairId(cosmetics.avatar_flair_id)
            const found = flairList.find((f) => f.id === cosmetics.avatar_flair_id)
            if (found) setPreviewFlairUrl(found.image_url)
          }
        }
      } catch {
        // Silently fail — cosmetics catalog is best-effort
      }
    }
    fetchCatalogsAndSelection()
  }, [claims.viewer_id, t])

  const handleSelectFrame = (item: CatalogItem) => {
    if (item.is_premium && !isPremium) return
    setSelectedFrameId(item.id)
    setPreviewFrameUrl(item.image_url)
  }

  const handleSelectFlair = (item: CatalogItem) => {
    if (item.is_premium && !isPremium) return
    setSelectedFlairId(item.id)
    setPreviewFlairUrl(item.image_url)
  }

  const handleSave = async () => {
    // Re-validate premium from localStorage JWT before PATCH
    const token = typeof window !== 'undefined' ? localStorage.getItem('viewer_jwt_token') : null
    setSaving(true)
    setSaveError(null)
    try {
      const headers: Record<string, string> = { 'Content-Type': 'application/json' }
      if (token) headers['Authorization'] = `Bearer ${token}`

      const res = await fetch('/api/v1/auth/viewer/cosmetics', {
        method: 'PATCH',
        headers,
        body: JSON.stringify({
          avatar_frame_id: selectedFrameId,
          avatar_flair_id: selectedFlairId,
        }),
      })

      if (!res.ok) {
        if (res.status === 403) {
          setSaveError(t('settings.viewer.premiumRequired'))
        } else {
          setSaveError(t('settings.viewer.saveFailed'))
        }
      } else {
        setSavedFeedback(true)
        setTimeout(() => setSavedFeedback(false), 2000)
      }
    } catch {
      setSaveError(t('settings.viewer.saveFailed'))
    } finally {
      setSaving(false)
    }
  }

  const lockIcon = (
    <div className="pointer-events-none absolute inset-0 flex items-center justify-center">
      <svg
        className="h-5 w-5 text-text-sub"
        fill="currentColor"
        viewBox="0 0 20 20"
        aria-hidden="true"
      >
        <path
          fillRule="evenodd"
          d="M10 1a4.5 4.5 0 00-4.5 4.5V9H5a2 2 0 00-2 2v6a2 2 0 002 2h10a2 2 0 002-2v-6a2 2 0 00-2-2h-.5V5.5A4.5 4.5 0 0010 1zm3 8V5.5a3 3 0 10-6 0V9h6z"
          clipRule="evenodd"
        />
      </svg>
    </div>
  )

  return (
    <Card className="p-6">
      <h2 className="mb-1 text-lg font-semibold text-text">
        {t('settings.viewer.cosmeticsHeading')}
      </h2>
      <p className="mb-4 text-sm text-text-sub">{t('settings.viewer.cosmeticsBody')}</p>

      {!isPremium && (
        <p className="mb-4 text-xs text-text-dim">
          {t('settings.viewer.cosmeticsUpsell')}{' '}
          <Link href="/settings/viewer/premium" className="font-medium text-twitch hover:underline">
            {t('settings.viewer.unlockPremium')}
          </Link>
        </p>
      )}

      {/* Avatar Frame section */}
      <div className="mb-6">
        <h3 className="mb-3 text-sm font-medium text-text">{t('settings.viewer.frameHeading')}</h3>
        <div className="grid grid-cols-4 gap-2">
          {frames.map((item) => (
            <button
              key={item.id ?? 'none-frame'}
              onClick={() => handleSelectFrame(item)}
              disabled={item.is_premium && !isPremium}
              className={cn(
                // A picker tile, deliberately not <Button>: the primitive is a
                // fixed-height centred inline-flex row, so a two-line image+label
                // tile would have to override h-auto/flex-col/border-2 and keep
                // almost nothing. It still owes the user a focus ring.
                'relative rounded-lg border-2 p-1 transition-colors',
                'focus-visible:ring-3 focus-visible:ring-ring/50 focus-visible:outline-none',
                selectedFrameId === item.id
                  ? 'border-twitch bg-surface-2'
                  : 'border-border hover:border-text-sub',
                item.is_premium && !isPremium && 'cursor-not-allowed opacity-50'
              )}
            >
              {item.image_url ? (
                // eslint-disable-next-line @next/next/no-img-element
                <img
                  src={item.image_url}
                  alt={item.name}
                  className="h-16 w-16 rounded object-contain"
                />
              ) : (
                <div className="flex h-16 w-16 items-center justify-center rounded bg-surface-2 text-xs text-text-sub">
                  {t('settings.viewer.noneItem')}
                </div>
              )}
              <span className="mt-1 block truncate text-center text-xs text-text-sub">
                {item.name}
              </span>
              {item.is_premium && !isPremium && lockIcon}
            </button>
          ))}
        </div>
      </div>

      {/* Avatar Flair section */}
      <div className="mb-6">
        <h3 className="mb-3 text-sm font-medium text-text">{t('settings.viewer.flairHeading')}</h3>
        <div className="grid grid-cols-4 gap-2">
          {flairs.map((item) => (
            <button
              key={item.id ?? 'none-flair'}
              onClick={() => handleSelectFlair(item)}
              disabled={item.is_premium && !isPremium}
              className={cn(
                // A picker tile, deliberately not <Button>: the primitive is a
                // fixed-height centred inline-flex row, so a two-line image+label
                // tile would have to override h-auto/flex-col/border-2 and keep
                // almost nothing. It still owes the user a focus ring.
                'relative rounded-lg border-2 p-1 transition-colors',
                'focus-visible:ring-3 focus-visible:ring-ring/50 focus-visible:outline-none',
                selectedFlairId === item.id
                  ? 'border-twitch bg-surface-2'
                  : 'border-border hover:border-text-sub',
                item.is_premium && !isPremium && 'cursor-not-allowed opacity-50'
              )}
            >
              {item.image_url ? (
                // eslint-disable-next-line @next/next/no-img-element
                <img
                  src={item.image_url}
                  alt={item.name}
                  className="h-16 w-16 rounded object-contain"
                />
              ) : (
                <div className="flex h-16 w-16 items-center justify-center rounded bg-surface-2 text-xs text-text-sub">
                  {t('settings.viewer.noneItem')}
                </div>
              )}
              <span className="mt-1 block truncate text-center text-xs text-text-sub">
                {item.name}
              </span>
              {item.is_premium && !isPremium && lockIcon}
            </button>
          ))}
        </div>
      </div>

      {/* Live preview */}
      <div className="mt-4 mb-6">
        <p className="mb-2 text-xs text-text-sub">{t('settings.viewer.previewLabel')}</p>
        <UserAvatar
          avatarUrl={claims.avatar_url}
          frameUrl={previewFrameUrl}
          flairUrl={previewFlairUrl}
          size={48}
          displayName={claims.display_name}
        />
      </div>

      {/* Save button */}
      <div className="flex items-center gap-3">
        <Button onClick={handleSave} disabled={saving}>
          {saving ? t('settings.viewer.saving') : t('settings.viewer.save')}
        </Button>
        {savedFeedback && (
          <span className="text-xs text-green-400">{t('settings.viewer.savedFeedback')}</span>
        )}
        {saveError && <span className="text-xs text-red-400">{saveError}</span>}
      </div>
    </Card>
  )
}

function LinkedPlatformsCard({ claims }: { claims: ViewerJWTClaims }) {
  const t = useTranslations()
  const [connecting, setConnecting] = useState<string | null>(null)
  const [disconnecting, setDisconnecting] = useState<string | null>(null)
  // linkedPlatforms: null = loading, string[] = fetched set
  const [linkedPlatforms, setLinkedPlatforms] = useState<Set<string> | null>(null)
  const [fetchError, setFetchError] = useState<string | null>(null)

  // Fetch all linked platforms from the backend on mount.
  useEffect(() => {
    if (!claims.viewer_id) return

    const token = localStorage.getItem('viewer_jwt_token')
    if (!token) return

    fetch('/api/v1/auth/viewer/linked-platforms', {
      headers: { Authorization: `Bearer ${token}` },
    })
      .then((res) => res.json())
      .then((data: { platforms?: string[]; error?: string }) => {
        if (data.platforms) {
          setLinkedPlatforms(new Set(data.platforms))
        } else {
          // Fallback: show at least the current JWT platform
          setLinkedPlatforms(new Set(claims.platform ? [claims.platform] : []))
          if (data.error) setFetchError(data.error)
        }
      })
      .catch(() => {
        // On network error fall back to JWT platform so UI still shows something
        setLinkedPlatforms(new Set(claims.platform ? [claims.platform] : []))
        setFetchError(t('settings.viewer.loadLinkedFailed'))
      })
  }, [claims.viewer_id, claims.platform, t])

  const handleConnect = async (key: 'twitch' | 'youtube' | 'kick') => {
    if (!claims.viewer_id) return
    setConnecting(key)
    await viewerConnect(key, claims.viewer_id)
    // If viewerConnect redirects the page, this line is never reached.
    setConnecting(null)
  }

  const handleDisconnect = async (key: 'twitch' | 'youtube' | 'kick') => {
    if (!claims.viewer_id) return
    const token = localStorage.getItem('viewer_jwt_token')
    if (!token) return

    setDisconnecting(key)
    try {
      const res = await fetch(`/api/v1/auth/viewer/linked-platforms/${key}`, {
        method: 'DELETE',
        headers: { Authorization: `Bearer ${token}` },
      })
      if (res.ok) {
        setLinkedPlatforms((prev) => {
          const next = new Set(prev)
          next.delete(key)
          return next
        })
      } else {
        const data = (await res.json()) as { error?: string }
        setFetchError(data.error ?? t('settings.viewer.disconnectFailed'))
      }
    } catch {
      setFetchError(t('settings.viewer.disconnectFailed'))
    } finally {
      setDisconnecting(null)
    }
  }

  // While loading, fall back to showing the JWT platform as connected so there's
  // no jarring flash of all "Connect" buttons.
  const effectiveLinked = linkedPlatforms ?? new Set(claims.platform ? [claims.platform] : [])

  return (
    <Card className="p-6">
      <h2 className="mb-1 text-lg font-semibold text-text">{t('settings.viewer.linkedHeading')}</h2>
      <p className="mb-4 text-sm text-text-sub">{t('settings.viewer.linkedBody')}</p>
      {fetchError && <p className="mb-3 text-xs text-red-400">{fetchError}</p>}
      <div className="flex flex-col gap-3">
        {PLATFORMS.map(({ key, color }) => {
          const isConnected = effectiveLinked.has(key)
          const isCurrentPlatform = claims.platform === key
          return (
            <div
              key={key}
              className="flex items-center justify-between rounded-lg border border-border px-4 py-3"
            >
              <div className="flex items-center gap-3">
                <span className={cn('h-3 w-3 rounded-full', color)} aria-hidden="true" />
                <span className="text-sm text-text">{t(`common.platforms.${key}`)}</span>
              </div>
              {isConnected ? (
                <div className="flex items-center gap-2">
                  <span className="rounded-full bg-green-500/15 px-2 py-0.5 text-xs font-medium text-green-400">
                    {t('settings.viewer.connected')}
                  </span>
                  {/* Cannot disconnect the platform the viewer is currently signed in with */}
                  {!isCurrentPlatform && (
                    <Button
                      onClick={() => handleDisconnect(key)}
                      disabled={disconnecting === key}
                      variant="destructive"
                      size="xs"
                    >
                      {disconnecting === key
                        ? t('settings.viewer.disconnecting')
                        : t('settings.viewer.disconnect')}
                    </Button>
                  )}
                </div>
              ) : (
                <Button
                  onClick={() => handleConnect(key)}
                  disabled={connecting === key || !claims.viewer_id}
                  variant="outline"
                  size="xs"
                >
                  {connecting === key
                    ? t('settings.viewer.connecting')
                    : t('settings.viewer.connect')}
                </Button>
              )}
            </div>
          )
        })}
      </div>
    </Card>
  )
}

function ViewerSettingsContent({ claims }: { claims: ViewerJWTClaims }) {
  const t = useTranslations()
  return (
    <div className="min-h-screen bg-bg">
      <AppNav />
      <main id="main-content" tabIndex={-1} className="mx-auto max-w-2xl space-y-6 px-4 py-12">
        {/* Header */}
        <div>
          <h1 className="text-2xl font-bold text-text">{t('settings.viewer.heading')}</h1>
          <p className="mt-1 text-sm text-text-sub">{t('settings.viewer.subheading')}</p>
        </div>

        {/* Profile summary */}
        <Card className="p-6">
          <h2 className="mb-4 text-lg font-semibold text-text">
            {t('settings.viewer.profileHeading')}
          </h2>
          <div className="flex items-center gap-3">
            {claims.avatar_url && (
              // eslint-disable-next-line @next/next/no-img-element
              <img
                src={claims.avatar_url}
                alt={
                  claims.display_name ?? claims.username ?? t('settings.viewer.viewerFallbackName')
                }
                width={48}
                height={48}
                className="rounded-full object-cover"
              />
            )}
            <div className="flex flex-col gap-1">
              <span className="text-lg font-medium text-text">
                {claims.display_name ?? claims.username ?? t('settings.viewer.viewerFallbackName')}
              </span>
              {claims.platform && <PlatformBadge platform={claims.platform} />}
            </div>
          </div>
        </Card>

        {/* Color + Gradient tabbed editor */}
        <ColorGradientCard claims={claims} />

        {/* Avatar Cosmetics — frame and flair picker */}
        <AvatarCosmeticsCard claims={claims} />

        {/* Linked Platforms section */}
        <LinkedPlatformsCard claims={claims} />
      </main>
    </div>
  )
}

export default function ViewerSettingsPage() {
  const [claims, setClaims] = useState<ViewerJWTClaims | null | undefined>(undefined)

  // The `undefined` third state is the whole point of this guard, so none of the usual escapes
  // from `react-hooks/set-state-in-effect` apply: deriving during render or a lazy useState
  // initialiser would read localStorage on the server render too, and the mismatch with the first
  // client render is exactly what the guard exists to avoid. There is no promise to move the
  // setState into either — localStorage is synchronous. Same shape, same reason, as the one-time
  // restore at src/app/overlay/[id]/view/page.tsx.
  useEffect(() => {
    if (typeof window === 'undefined') return
    const token = localStorage.getItem('viewer_jwt_token')
    // eslint-disable-next-line react-hooks/set-state-in-effect -- one-time restore from localStorage; a render-time read would break hydration
    setClaims(token ? decodeViewerJWT(token) : null)
  }, [])

  // Still hydrating
  if (claims === undefined) {
    return (
      <div className="min-h-screen bg-bg">
        <AppNav />
      </div>
    )
  }

  if (!claims) {
    return <UnauthenticatedState />
  }

  return <ViewerSettingsContent claims={claims} />
}
