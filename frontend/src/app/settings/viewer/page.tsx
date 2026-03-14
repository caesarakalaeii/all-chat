'use client'

import { useState, useEffect, useCallback } from 'react'
import { AppNav } from '@/components/AppNav'
import { Card } from '@/components/ui/card'
import { Button } from '@/components/ui/button'

// JWT claims from viewer token
interface ViewerJWTClaims {
  viewer_id?: string
  display_name?: string
  username?: string
  avatar_url?: string
  platform?: 'twitch' | 'youtube' | 'kick' | 'tiktok'
  name_color?: string
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

function ViewerSettingsContent({ claims }: { claims: ViewerJWTClaims }) {
  const [nameColor, setNameColor] = useState<string>(claims.name_color ?? '#9146ff')
  const [saving, setSaving] = useState(false)
  const [saved, setSaved] = useState(false)

  // Also try localStorage for persisted name_color
  useEffect(() => {
    if (typeof window !== 'undefined') {
      const stored = localStorage.getItem('name_color')
      if (stored) setNameColor(stored)
    }
  }, [])

  const handleColorChange = useCallback(async (color: string) => {
    setNameColor(color)
    if (typeof window !== 'undefined') {
      localStorage.setItem('name_color', color)
    }
  }, [])

  const handleSaveColor = useCallback(async () => {
    setSaving(true)
    setSaved(false)
    try {
      const token = typeof window !== 'undefined' ? localStorage.getItem('viewer_jwt_token') : null
      const headers: Record<string, string> = { 'Content-Type': 'application/json' }
      if (token) headers['Authorization'] = `Bearer ${token}`

      const res = await fetch('/api/v1/auth/viewer/cosmetics', {
        method: 'PATCH',
        headers,
        body: JSON.stringify({ name_color: nameColor }),
      })

      if (res.ok) {
        setSaved(true)
        setTimeout(() => setSaved(false), 2000)
      }
    } catch {
      // Silently fail — cosmetics are best-effort
    } finally {
      setSaving(false)
    }
  }, [nameColor])

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

        {/* Name Color section (Phase 28 stub) */}
        <Card className="p-6">
          <h2 className="text-lg font-semibold text-text mb-1">Name Color</h2>
          <p className="text-text-sub text-sm mb-4">
            Set a fallback color for when your platform does not provide one (YouTube, Kick, TikTok)
          </p>
          <div className="flex items-center gap-4">
            <input
              type="color"
              value={nameColor}
              onChange={e => handleColorChange(e.target.value)}
              className="w-10 h-10 rounded cursor-pointer border border-border bg-transparent"
              aria-label="Name color picker"
            />
            <span className="text-text-sub text-sm font-mono">{nameColor}</span>
            <Button
              onClick={handleSaveColor}
              disabled={saving}
              variant="outline"
              className="ml-auto"
            >
              {saving ? 'Saving…' : saved ? 'Saved' : 'Save color'}
            </Button>
          </div>
          <p className="text-text-sub text-xs mt-3">
            The full gradient editor will be available in a future update.
          </p>
        </Card>

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
