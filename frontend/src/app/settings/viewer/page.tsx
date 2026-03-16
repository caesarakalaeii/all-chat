'use client'

import { useState, useEffect, useCallback, useRef } from 'react'
import { AppNav } from '@/components/AppNav'
import { Card } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { buildGradientCSS } from '@/lib/utils/gradient'
import type { NameGradient } from '@/lib/types/message'
import { UserAvatar } from '@/components/UserAvatar'

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

const PLATFORMS: { key: 'twitch' | 'youtube' | 'kick'; label: string; color: string }[] = [
  { key: 'twitch', label: 'Twitch', color: 'bg-purple-500' },
  { key: 'youtube', label: 'YouTube', color: 'bg-red-500' },
  { key: 'kick', label: 'Kick', color: 'bg-green-500' },
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
  const found = PLATFORMS.find(p => p.key === platform)
  if (!found) return null
  return (
    <span
      className={`inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-xs font-medium text-white ${found.color}`}
    >
      {found.label}
    </span>
  )
}

async function viewerLogin(platform: 'twitch' | 'youtube' | 'kick') {
  try {
    const params = new URLSearchParams({ redirect_to: '/settings/viewer' })
    const response = await fetch(`/api/v1/auth/viewer/${platform}/login?${params}`)
    const data = await response.json()
    if (data.auth_url) {
      window.location.href = data.auth_url
    }
  } catch {
    // silently ignore — user stays on page
  }
}

function UnauthenticatedState() {
  return (
    <div className="min-h-screen bg-bg">
      <AppNav />
      <main className="max-w-2xl mx-auto px-4 py-12 space-y-6">
        <h1 className="text-2xl font-bold text-text">Viewer Identity</h1>
        <p className="text-text-sub text-sm">
          Customize how your name appears across all overlays
        </p>

        <Card className="p-6">
          <h2 className="text-lg font-semibold text-text mb-2">Sign in to manage your viewer identity</h2>
          <p className="text-text-sub text-sm mb-6">
            Connect your streaming platform account to set a custom name color and manage your viewer identity.
          </p>
          <div className="flex flex-col gap-3 sm:flex-row">
            <button
              onClick={() => viewerLogin('twitch')}
              className="flex items-center gap-2.5 rounded-lg bg-twitch px-6 py-3 font-semibold text-white transition-opacity hover:opacity-90 focus-visible:ring-2 focus-visible:ring-twitch focus-visible:ring-offset-2 focus-visible:ring-offset-bg focus-visible:outline-none"
            >
              <svg className="h-5 w-5 shrink-0" viewBox="0 0 24 24" xmlns="http://www.w3.org/2000/svg" aria-hidden="true">
                <path fill="#FFFFFF" d="M11.571 4.714h1.715v5.143H11.57zm4.715 0H18v5.143h-1.714zM6 0L1.714 4.286v15.428h5.143V24l4.286-4.286h3.428L22.286 12V0zm14.571 11.143l-3.428 3.428h-3.429l-3 3v-3H6.857V1.714h13.714z" />
              </svg>
              Sign in with Twitch
            </button>
            <button
              onClick={() => viewerLogin('youtube')}
              className="flex items-center gap-2.5 rounded-lg px-6 py-3 font-semibold text-white transition-opacity hover:opacity-90 focus-visible:ring-2 focus-visible:ring-offset-2 focus-visible:ring-offset-bg focus-visible:outline-none"
              style={{ backgroundColor: '#FF0000', ['--tw-ring-color' as string]: '#FF0000' }}
            >
              <svg className="h-5 w-5 shrink-0" viewBox="0 0 24 24" xmlns="http://www.w3.org/2000/svg" aria-hidden="true">
                <path fill="#FFFFFF" d="M23.498 6.186a3.016 3.016 0 0 0-2.122-2.136C19.505 3.545 12 3.545 12 3.545s-7.505 0-9.377.505A3.017 3.017 0 0 0 .502 6.186C0 8.07 0 12 0 12s0 3.93.502 5.814a3.016 3.016 0 0 0 2.122 2.136c1.871.505 9.376.505 9.376.505s7.505 0 9.377-.505a3.015 3.015 0 0 0 2.122-2.136C24 15.93 24 12 24 12s0-3.93-.502-5.814zM9.545 15.568V8.432L15.818 12l-6.273 3.568z" />
              </svg>
              Sign in with YouTube
            </button>
            <button
              onClick={() => viewerLogin('kick')}
              className="flex items-center gap-2.5 rounded-lg px-6 py-3 font-semibold text-bg transition-opacity hover:opacity-90 focus-visible:ring-2 focus-visible:ring-kick focus-visible:ring-offset-2 focus-visible:ring-offset-bg focus-visible:outline-none"
              style={{ backgroundColor: 'var(--color-kick)' }}
            >
              <svg className="h-5 w-5 shrink-0" viewBox="0 0 512 512" xmlns="http://www.w3.org/2000/svg" aria-hidden="true">
                <path fill="currentColor" d="M37 .036h164.448v113.621h54.71v-56.82h54.731V.036h164.448v170.777h-54.73v56.82h-54.711v56.8h54.71v56.82h54.73V512.03H310.89v-56.82h-54.73v-56.8h-54.711v113.62H37V.036z" />
              </svg>
              Sign in with Kick
            </button>
          </div>
        </Card>
      </main>
    </div>
  )
}

// ---------------------------------------------------------------------------
// ColorGradientCard — full two-tab card replacing Phase 28 stub
// ---------------------------------------------------------------------------

function ColorGradientCard({ claims }: { claims: ViewerJWTClaims }) {
  const [activeTab, setActiveTab] = useState<'solid' | 'gradient'>('solid')

  // ------ Solid Color state ------
  const [nameColor, setNameColor] = useState<string>(claims.name_color ?? '#9146ff')
  const [savedFeedback, setSavedFeedback] = useState(false)
  const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null)

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

  const debouncedSaveColor = useCallback((color: string) => {
    if (timerRef.current) clearTimeout(timerRef.current)
    timerRef.current = setTimeout(() => {
      if (/^#[0-9a-fA-F]{6}$/.test(color)) saveColor(color)
    }, 400)
  }, [saveColor])

  // ------ Gradient state ------
  const initialStops = claims.name_gradient?.colors ?? ['#9146ff', '#00b5ad']
  const [gradientStops, setGradientStops] = useState<string[]>(
    initialStops.length >= 2 ? initialStops : ['#9146ff', '#00b5ad']
  )
  const [gradientAngle, setGradientAngle] = useState<number>(claims.name_gradient?.angle ?? 90)
  const [gradientSaving, setGradientSaving] = useState(false)
  const [gradientError, setGradientError] = useState<string | null>(null)

  const handleSaveGradient = useCallback(async () => {
    // Re-validate premium from localStorage JWT before sending PATCH
    const token = typeof window !== 'undefined' ? localStorage.getItem('viewer_jwt_token') : null
    if (token) {
      const latestClaims = decodeViewerJWT(token)
      if (!latestClaims?.is_premium) {
        setGradientError('Premium required')
        return
      }
    } else {
      setGradientError('Premium required')
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
          setGradientError('Premium required')
        } else {
          setGradientError('Save failed')
        }
      }
    } catch {
      setGradientError('Save failed')
    } finally {
      setGradientSaving(false)
    }
  }, [gradientStops, gradientAngle])

  return (
    <Card className="p-6">
      <h2 className="text-lg font-semibold text-text mb-1">Name Color</h2>
      <p className="text-text-sub text-sm mb-4">
        Set a custom color or gradient for your name on overlays
      </p>

      {/* Tab bar */}
      <div className="flex border-b border-border mb-4">
        <button
          onClick={() => setActiveTab('solid')}
          className={`px-4 py-2 text-sm font-medium transition-colors ${
            activeTab === 'solid'
              ? 'border-b-2 border-primary text-text'
              : 'text-text-sub hover:text-text'
          }`}
        >
          Solid Color
        </button>
        <button
          disabled={!claims.is_premium}
          onClick={() => { if (claims.is_premium) setActiveTab('gradient') }}
          className={`px-4 py-2 text-sm font-medium transition-colors flex items-center gap-1.5 ${
            activeTab === 'gradient'
              ? 'border-b-2 border-primary text-text'
              : 'text-text-sub hover:text-text'
          } ${!claims.is_premium ? 'opacity-50 cursor-not-allowed' : ''}`}
        >
          Gradient
          {!claims.is_premium && (
            <span className="text-xs px-1.5 py-0.5 rounded bg-amber-500/20 text-amber-400">
              Premium
            </span>
          )}
        </button>
      </div>

      {/* Solid Color tab panel */}
      {activeTab === 'solid' && (
        <div>
          <div className="flex items-center gap-3">
            <input
              type="color"
              value={nameColor}
              onChange={e => saveColor(e.target.value)}
              className="w-10 h-10 rounded cursor-pointer border border-border bg-transparent"
              aria-label="Name color picker"
            />
            <input
              type="text"
              value={nameColor}
              onChange={e => debouncedSaveColor(e.target.value)}
              className="w-28 text-sm font-mono bg-surface-2 border border-border rounded px-2 py-1 text-text"
              aria-label="Name color hex value"
              maxLength={7}
            />
            {savedFeedback && (
              <span className="text-xs text-green-400 ml-2">Saved ✓</span>
            )}
          </div>

          {/* Live preview */}
          <div className="mt-4 pt-4 border-t border-border">
            <p className="text-xs text-text-sub mb-2">Preview</p>
            <div className="flex items-center gap-2">
              <div className="w-6 h-6 rounded-full bg-surface-2 flex items-center justify-center text-xs text-text-sub font-medium">
                {(claims.display_name ?? claims.username ?? 'V').charAt(0).toUpperCase()}
              </div>
              <span
                className="text-sm font-semibold"
                style={{ color: nameColor }}
              >
                {claims.display_name ?? claims.username ?? 'Viewer'}
              </span>
              <span className="text-sm text-text-sub">Hello world!</span>
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
              <div key={i} className="flex items-center gap-2 mb-2">
                <input
                  type="color"
                  value={stop}
                  onChange={e => {
                    const s = [...gradientStops]
                    s[i] = e.target.value
                    setGradientStops(s)
                  }}
                  className="w-8 h-8 rounded cursor-pointer border border-border"
                />
                <input
                  type="text"
                  value={stop}
                  onChange={e => {
                    if (/^#[0-9a-fA-F]{6}$/.test(e.target.value)) {
                      const s = [...gradientStops]
                      s[i] = e.target.value
                      setGradientStops(s)
                    }
                  }}
                  className="w-24 text-sm font-mono bg-surface-2 border border-border rounded px-2 py-1 text-text"
                />
                {gradientStops.length > 2 && (
                  <button
                    onClick={() => setGradientStops(gradientStops.filter((_, j) => j !== i))}
                    className="text-text-sub hover:text-text text-lg leading-none"
                    aria-label={`Remove stop ${i + 1}`}
                  >
                    ×
                  </button>
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
            + Add stop
          </Button>

          {/* Angle controls */}
          <div className="flex items-center gap-3 mt-3">
            <label className="text-xs text-text-sub w-10">Angle</label>
            <input
              type="range"
              min={0}
              max={360}
              value={gradientAngle}
              onChange={e => setGradientAngle(Number(e.target.value))}
              className="flex-1"
            />
            <input
              type="number"
              min={0}
              max={360}
              value={gradientAngle}
              onChange={e =>
                setGradientAngle(Math.min(360, Math.max(0, Number(e.target.value))))
              }
              className="w-16 text-sm bg-surface-2 border border-border rounded px-2 py-1 text-text text-right"
            />
            <span className="text-xs text-text-sub">°</span>
          </div>

          {/* Save gradient button + error */}
          <div className="flex items-center gap-3 mt-4">
            <Button onClick={handleSaveGradient} disabled={gradientSaving}>
              {gradientSaving ? 'Saving…' : 'Save gradient'}
            </Button>
            {gradientError && (
              <span className="text-xs text-red-400">{gradientError}</span>
            )}
          </div>

          {/* Live preview */}
          <div className="mt-4 pt-4 border-t border-border">
            <p className="text-xs text-text-sub mb-2">Preview</p>
            <div className="flex items-center gap-2">
              <div className="w-6 h-6 rounded-full bg-surface-2 flex items-center justify-center text-xs text-text-sub font-medium">
                {(claims.display_name ?? 'V').charAt(0).toUpperCase()}
              </div>
              <span
                className="text-sm font-semibold bg-clip-text text-transparent"
                style={{
                  backgroundImage: buildGradientCSS({
                    type: 'linear',
                    colors: gradientStops,
                    angle: gradientAngle,
                  }),
                }}
              >
                {claims.display_name ?? claims.username ?? 'Viewer'}
              </span>
              <span className="text-sm text-text-sub">Hello world!</span>
            </div>
          </div>
        </div>
      )}
    </Card>
  )
}

// ---------------------------------------------------------------------------
// AvatarCosmeticsCard — frame and flair picker
// ---------------------------------------------------------------------------

interface CatalogItem {
  id: string | null
  name: string
  image_url: string
  is_premium: boolean
}

const NONE_ITEM: CatalogItem = { id: null, name: 'None', image_url: '', is_premium: false }

function AvatarCosmeticsCard({ claims }: { claims: ViewerJWTClaims }) {
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
    const fetchCatalogs = async () => {
      try {
        const [framesRes, flairsRes] = await Promise.all([
          fetch('/api/v1/auth/viewer/catalog/frames'),
          fetch('/api/v1/auth/viewer/catalog/flairs'),
        ])
        if (framesRes.ok) {
          const data = await framesRes.json() as { frames: CatalogItem[] }
          setFrames([NONE_ITEM, ...(data.frames ?? [])])
        }
        if (flairsRes.ok) {
          const data = await flairsRes.json() as { flairs: CatalogItem[] }
          setFlairs([NONE_ITEM, ...(data.flairs ?? [])])
        }
      } catch {
        // Silently fail — cosmetics catalog is best-effort
      }
    }
    fetchCatalogs()
  }, [])

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
          setSaveError('Premium required')
        } else {
          setSaveError('Save failed')
        }
      } else {
        setSavedFeedback(true)
        setTimeout(() => setSavedFeedback(false), 2000)
      }
    } catch {
      setSaveError('Save failed')
    } finally {
      setSaving(false)
    }
  }

  const lockIcon = (
    <div className="absolute inset-0 flex items-center justify-center pointer-events-none">
      <svg className="w-5 h-5 text-text-sub" fill="currentColor" viewBox="0 0 20 20" aria-hidden="true">
        <path fillRule="evenodd" d="M10 1a4.5 4.5 0 00-4.5 4.5V9H5a2 2 0 00-2 2v6a2 2 0 002 2h10a2 2 0 002-2v-6a2 2 0 00-2-2h-.5V5.5A4.5 4.5 0 0010 1zm3 8V5.5a3 3 0 10-6 0V9h6z" clipRule="evenodd" />
      </svg>
    </div>
  )

  return (
    <Card className="p-6">
      <h2 className="text-lg font-semibold text-text mb-1">Avatar Cosmetics</h2>
      <p className="text-text-sub text-sm mb-4">
        Choose a frame and flair for your avatar
      </p>

      {/* Avatar Frame section */}
      <div className="mb-6">
        <h3 className="text-sm font-medium text-text mb-3">Avatar Frame</h3>
        <div className="grid grid-cols-4 gap-2">
          {frames.map((item) => (
            <button
              key={item.id ?? 'none-frame'}
              onClick={() => handleSelectFrame(item)}
              disabled={item.is_premium && !isPremium}
              className={`relative p-1 rounded-lg border-2 transition-colors ${
                selectedFrameId === item.id
                  ? 'border-twitch bg-surface-2'
                  : 'border-border hover:border-text-sub'
              } ${item.is_premium && !isPremium ? 'opacity-50 cursor-not-allowed' : ''}`}
            >
              {item.image_url ? (
                // eslint-disable-next-line @next/next/no-img-element
                <img src={item.image_url} alt={item.name} className="w-16 h-16 rounded object-contain" />
              ) : (
                <div className="w-16 h-16 rounded bg-surface-2 flex items-center justify-center text-text-sub text-xs">None</div>
              )}
              <span className="block text-xs text-center text-text-sub mt-1 truncate">{item.name}</span>
              {item.is_premium && !isPremium && lockIcon}
            </button>
          ))}
        </div>
      </div>

      {/* Avatar Flair section */}
      <div className="mb-6">
        <h3 className="text-sm font-medium text-text mb-3">Avatar Flair</h3>
        <div className="grid grid-cols-4 gap-2">
          {flairs.map((item) => (
            <button
              key={item.id ?? 'none-flair'}
              onClick={() => handleSelectFlair(item)}
              disabled={item.is_premium && !isPremium}
              className={`relative p-1 rounded-lg border-2 transition-colors ${
                selectedFlairId === item.id
                  ? 'border-twitch bg-surface-2'
                  : 'border-border hover:border-text-sub'
              } ${item.is_premium && !isPremium ? 'opacity-50 cursor-not-allowed' : ''}`}
            >
              {item.image_url ? (
                // eslint-disable-next-line @next/next/no-img-element
                <img src={item.image_url} alt={item.name} className="w-16 h-16 rounded object-contain" />
              ) : (
                <div className="w-16 h-16 rounded bg-surface-2 flex items-center justify-center text-text-sub text-xs">None</div>
              )}
              <span className="block text-xs text-center text-text-sub mt-1 truncate">{item.name}</span>
              {item.is_premium && !isPremium && lockIcon}
            </button>
          ))}
        </div>
      </div>

      {/* Live preview */}
      <div className="mt-4 mb-6">
        <p className="text-xs text-text-sub mb-2">Preview</p>
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
          {saving ? 'Saving…' : 'Save'}
        </Button>
        {savedFeedback && (
          <span className="text-xs text-green-400">Saved ✓</span>
        )}
        {saveError && (
          <span className="text-xs text-red-400">{saveError}</span>
        )}
      </div>
    </Card>
  )
}

// ---------------------------------------------------------------------------
// ViewerSettingsContent — main authenticated view
// ---------------------------------------------------------------------------

function ViewerSettingsContent({ claims }: { claims: ViewerJWTClaims }) {
  return (
    <div className="min-h-screen bg-bg">
      <AppNav />
      <main className="max-w-2xl mx-auto px-4 py-12 space-y-6">
        {/* Header */}
        <div>
          <h1 className="text-2xl font-bold text-text">Viewer Identity</h1>
          <p className="text-text-sub text-sm mt-1">
            Customize how your name appears across all overlays
          </p>
        </div>

        {/* Profile summary */}
        <Card className="p-6">
          <h2 className="text-lg font-semibold text-text mb-4">Profile</h2>
          <div className="flex items-center gap-3">
            {claims.avatar_url && (
              // eslint-disable-next-line @next/next/no-img-element
              <img
                src={claims.avatar_url}
                alt={claims.display_name ?? claims.username ?? 'Viewer'}
                width={48}
                height={48}
                className="rounded-full object-cover"
              />
            )}
            <div className="flex flex-col gap-1">
              <span className="text-text font-medium text-lg">
                {claims.display_name ?? claims.username ?? 'Viewer'}
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
        <Card className="p-6">
          <h2 className="text-lg font-semibold text-text mb-1">Linked Platforms</h2>
          <p className="text-text-sub text-sm mb-4">
            Sign in with each platform to enable cosmetics across all your chats
          </p>
          <div className="flex flex-col gap-3">
            {PLATFORMS.map(({ key, label, color }) => {
              const isConnected = claims.platform === key
              return (
                <div
                  key={key}
                  className="flex items-center justify-between rounded-lg border border-border px-4 py-3"
                >
                  <div className="flex items-center gap-3">
                    <span className={`w-3 h-3 rounded-full ${color}`} aria-hidden="true" />
                    <span className="text-sm text-text">{label}</span>
                  </div>
                  {isConnected ? (
                    <span className="text-xs font-medium px-2 py-0.5 rounded-full bg-green-500/15 text-green-400">
                      Connected
                    </span>
                  ) : (
                    <button
                      onClick={() => viewerLogin(key)}
                      className="text-xs font-medium px-3 py-1 rounded-md border border-border text-text hover:bg-surface-2 transition-colors"
                    >
                      Connect
                    </button>
                  )}
                </div>
              )
            })}
          </div>
          <p className="text-text-sub text-xs mt-3">
            Multi-platform account linking will be available in a future update.
          </p>
        </Card>
      </main>
    </div>
  )
}

// ---------------------------------------------------------------------------
// Page root — three-state hydration guard
// ---------------------------------------------------------------------------

export default function ViewerSettingsPage() {
  const [claims, setClaims] = useState<ViewerJWTClaims | null | undefined>(undefined)

  useEffect(() => {
    if (typeof window === 'undefined') return
    const token = localStorage.getItem('viewer_jwt_token')
    if (!token) {
      setClaims(null)
      return
    }
    const decoded = decodeViewerJWT(token)
    setClaims(decoded)
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
