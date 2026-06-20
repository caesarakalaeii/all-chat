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

/**
 * Admin Feature Gates Page
 *
 * Allows admins to manage two orthogonal gate dimensions per feature:
 *  - is_premium (ADR-0008): premium-only vs free for all.
 *  - early_access (ADR-0020): beta-testers only vs graduated.
 * Each dimension toggles via a confirmation dialog, with toast feedback.
 *
 * Route: /admin/features
 * Layout: inherits admin/layout.tsx (AdminNav, ToastProvider, ProtectedRoute)
 */

'use client'

import { useEffect, useState } from 'react'
import { useRouter } from 'next/navigation'
import { useAuthStore } from '@/lib/stores/auth-store'
import { apiClient } from '@/lib/api/client'
import { Card } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { DialogRoot, DialogContent, DialogDescription, DialogTitle } from '@/components/ui/dialog'
import { toastManager } from '@/lib/toast'
import { cn } from '@/lib/utils'

interface FeatureGate {
  feature_key: string
  is_premium: boolean
  early_access: boolean
  description: string
}

type GateDimension = 'premium' | 'early_access'

interface PendingToggle {
  gate: FeatureGate
  dimension: GateDimension
}

export default function AdminFeaturesPage() {
  const router = useRouter()
  const { user } = useAuthStore()

  const [gates, setGates] = useState<FeatureGate[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [toggling, setToggling] = useState<string | null>(null)
  const [confirm, setConfirm] = useState<PendingToggle | null>(null)

  useEffect(() => {
    if (!user?.is_admin) {
      router.push('/dashboard')
      return
    }
    async function fetchGates() {
      try {
        const data = await apiClient.get<FeatureGate[]>('/api/v1/admin/feature-gates')
        setGates(data)
        setError(null)
      } catch {
        setError('Failed to load feature gates. Refresh the page to try again.')
      } finally {
        setLoading(false)
      }
    }
    fetchGates()
  }, [user, router])

  const handleToggle = async () => {
    if (!confirm) return
    const { gate, dimension } = confirm
    const key = gate.feature_key
    const togglingKey = `${key}:${dimension}`
    const field = dimension === 'premium' ? 'is_premium' : 'early_access'
    const newValue = dimension === 'premium' ? !gate.is_premium : !gate.early_access

    setToggling(togglingKey)
    setConfirm(null)
    try {
      await apiClient.patch(`/api/v1/admin/feature-gates/${key}`, { [field]: newValue })
      setGates((prev) => prev.map((g) => (g.feature_key === key ? { ...g, [field]: newValue } : g)))
      toastManager.add({
        title:
          dimension === 'premium'
            ? newValue
              ? `${key} is now premium-only`
              : `${key} is now free for all users`
            : newValue
              ? `${key} is now early-access (beta testers only)`
              : `${key} graduated from early access`,
        type: 'success',
      })
    } catch {
      toastManager.add({
        title: `Failed to update ${key}. Please try again.`,
        type: 'error',
      })
    } finally {
      setToggling(null)
    }
  }

  const dialogText = (): { title: string; description: string; confirmLabel: string } => {
    if (!confirm) return { title: '', description: '', confirmLabel: '' }
    const { gate, dimension } = confirm
    if (dimension === 'premium') {
      return gate.is_premium
        ? {
            title: `Make ${gate.feature_key} free for all users?`,
            description:
              'All authenticated users will gain access immediately. No code deploy required.',
            confirmLabel: 'Make Free',
          }
        : {
            title: `Restrict ${gate.feature_key} to premium users?`,
            description: 'Only users with premium access will be able to use this feature.',
            confirmLabel: 'Make Premium',
          }
    }
    return gate.early_access
      ? {
          title: `Graduate ${gate.feature_key} from early access?`,
          description: 'Beta-tester-only access is lifted; the feature defers to its premium gate.',
          confirmLabel: 'Graduate',
        }
      : {
          title: `Restrict ${gate.feature_key} to beta testers?`,
          description: 'Only beta testers will be able to use this early-access feature.',
          confirmLabel: 'Make Early Access',
        }
  }

  const text = dialogText()

  return (
    <div className="mx-auto max-w-3xl px-4 py-8">
      {/* Page header */}
      <div className="mb-6">
        <h1 className="text-2xl font-bold text-text">Features</h1>
        <p className="mt-1 text-sm text-text-sub">
          Manage capability-level gates. Premium controls paid access; early access restricts a
          feature to beta testers. Both toggle without a code deploy.
        </p>
      </div>

      {/* Gate list */}
      {loading ? (
        <Card className="space-y-3 p-6">
          {Array.from({ length: 3 }).map((_, i) => (
            <Skeleton key={i} className="h-16 w-full rounded-lg" />
          ))}
        </Card>
      ) : error ? (
        <Card className="p-6">
          <p className="text-sm text-text-sub">{error}</p>
        </Card>
      ) : gates.length === 0 ? (
        <Card className="p-6 text-center">
          <p className="mb-1 text-base font-bold text-text">No feature gates configured</p>
          <p className="text-sm text-text-sub">
            Feature gates are added automatically when new features ship. Check back after the next
            deployment.
          </p>
        </Card>
      ) : (
        <Card className="overflow-hidden">
          <div className="border-b border-border px-4 py-3">
            <h2 className="text-base font-bold text-text">Feature Gates ({gates.length})</h2>
          </div>
          <div className="divide-y divide-border">
            {gates.map((gate) => {
              const premiumBusy = toggling === `${gate.feature_key}:premium`
              const earlyAccessBusy = toggling === `${gate.feature_key}:early_access`
              return (
                <div
                  key={gate.feature_key}
                  className="flex min-h-[44px] items-center gap-4 px-4 py-3"
                >
                  {/* Feature key + description */}
                  <div className="min-w-0 flex-1">
                    <span className="block font-mono text-sm text-text">{gate.feature_key}</span>
                    {gate.description && (
                      <span className="mt-0.5 block text-sm text-text-sub">{gate.description}</span>
                    )}
                  </div>

                  {/* Premium dimension */}
                  <div className="flex flex-shrink-0 items-center gap-2">
                    <span
                      className={cn(
                        'inline-flex items-center rounded-full border px-2.5 py-0.5 text-xs font-bold',
                        gate.is_premium
                          ? 'border-amber-500/20 bg-amber-500/10 text-amber-400'
                          : 'border-green-500/20 bg-green-500/10 text-green-400'
                      )}
                    >
                      {gate.is_premium ? 'Premium only' : 'Free for all'}
                    </span>
                    <button
                      role="switch"
                      aria-checked={gate.is_premium}
                      aria-label={`Toggle premium for ${gate.feature_key}`}
                      onClick={() => setConfirm({ gate, dimension: 'premium' })}
                      disabled={premiumBusy}
                      className={cn(
                        'relative inline-flex h-[28px] min-h-[44px] w-[52px] items-center rounded-full transition-colors focus-visible:ring-2 focus-visible:ring-twitch focus-visible:outline-none',
                        gate.is_premium ? 'bg-amber-500/20' : 'bg-green-500/20',
                        premiumBusy ? 'cursor-not-allowed opacity-50' : 'cursor-pointer'
                      )}
                    >
                      <span
                        className={cn(
                          'inline-block h-5 w-5 rounded-full bg-white transition-transform',
                          gate.is_premium ? 'translate-x-[26px]' : 'translate-x-[4px]'
                        )}
                      />
                    </button>
                  </div>

                  {/* Early-access dimension */}
                  <div className="flex flex-shrink-0 items-center gap-2">
                    <span
                      className={cn(
                        'inline-flex items-center rounded-full border px-2.5 py-0.5 text-xs font-bold',
                        gate.early_access
                          ? 'border-violet-500/20 bg-violet-500/10 text-violet-400'
                          : 'border-border bg-badge-bg text-text-sub'
                      )}
                    >
                      {gate.early_access ? 'Early access' : 'Standard'}
                    </span>
                    <button
                      role="switch"
                      aria-checked={gate.early_access}
                      aria-label={`Toggle early access for ${gate.feature_key}`}
                      onClick={() => setConfirm({ gate, dimension: 'early_access' })}
                      disabled={earlyAccessBusy}
                      className={cn(
                        'relative inline-flex h-[28px] min-h-[44px] w-[52px] items-center rounded-full transition-colors focus-visible:ring-2 focus-visible:ring-twitch focus-visible:outline-none',
                        gate.early_access ? 'bg-violet-500/20' : 'bg-badge-bg',
                        earlyAccessBusy ? 'cursor-not-allowed opacity-50' : 'cursor-pointer'
                      )}
                    >
                      <span
                        className={cn(
                          'inline-block h-5 w-5 rounded-full bg-white transition-transform',
                          gate.early_access ? 'translate-x-[26px]' : 'translate-x-[4px]'
                        )}
                      />
                    </button>
                  </div>
                </div>
              )
            })}
          </div>
        </Card>
      )}

      {/* Confirmation dialog */}
      <DialogRoot open={!!confirm} onOpenChange={() => setConfirm(null)}>
        <DialogContent>
          <DialogTitle>{text.title}</DialogTitle>
          <DialogDescription>{text.description}</DialogDescription>
          <div className="mt-4 flex justify-end gap-3">
            <Button variant="outline" onClick={() => setConfirm(null)}>
              No, keep as-is
            </Button>
            <Button onClick={handleToggle}>{text.confirmLabel}</Button>
          </div>
        </DialogContent>
      </DialogRoot>
    </div>
  )
}
