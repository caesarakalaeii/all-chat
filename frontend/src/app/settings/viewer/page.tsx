'use client'

import { useState, useEffect, useCallback, useRef } from 'react'
import { AppNav } from '@/components/AppNav'
import { Card } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { buildGradientCSS } from '@/lib/utils/gradient'
import type { NameGradient } from '@/lib/types/message'

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
          <div className="flex flex-col gap-3">
            {PLATFORMS.map(({ key, label, color }) => (
              <a
                key={key}
                href={`/api/v1/auth/viewer/${key}/login`}
                className="inline-flex items-center gap-3 rounded-lg border border-border px-4 py-3 text-sm text-text hover:bg-surface-2 transition-colors"
              >
                <span className={`w-3 h-3 rounded-full ${color}`} aria-hidden="true" />
                <span>Sign in with {label}</span>
              </a>
            ))}
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
                    <a
                      href={`/api/v1/auth/viewer/${key}/login`}
                      className="text-xs font-medium px-3 py-1 rounded-md border border-border text-text hover:bg-surface-2 transition-colors"
                    >
                      Connect
                    </a>
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
