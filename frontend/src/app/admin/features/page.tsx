/**
 * Admin Feature Gates Page
 *
 * Allows admins to manage feature gate premium status.
 * Each gate can be toggled between premium-only and free-for-all
 * via a confirmation dialog, with toast feedback.
 *
 * Route: /admin/features
 * Layout: inherits admin/layout.tsx (AdminNav, ToastProvider, ProtectedRoute)
 */

'use client';

import { useEffect, useState, useCallback } from 'react';
import { useRouter } from 'next/navigation';
import { useAuthStore } from '@/lib/stores/auth-store';
import { apiClient } from '@/lib/api/client';
import { Card } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import {
  DialogRoot,
  DialogContent,
  DialogDescription,
  DialogTitle,
} from '@/components/ui/dialog';
import { toastManager } from '@/lib/toast';
import { cn } from '@/lib/utils';

interface FeatureGate {
  feature_key: string;
  is_premium: boolean;
  description: string;
}

export default function AdminFeaturesPage() {
  const router = useRouter();
  const { user } = useAuthStore();

  const [gates, setGates] = useState<FeatureGate[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [toggling, setToggling] = useState<string | null>(null);
  const [confirmGate, setConfirmGate] = useState<FeatureGate | null>(null);

  const fetchGates = useCallback(async () => {
    try {
      const data = await apiClient.get<FeatureGate[]>('/api/v1/admin/feature-gates');
      setGates(data);
      setError(null);
    } catch {
      setError('Failed to load feature gates. Refresh the page to try again.');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    if (!user?.is_admin) {
      router.push('/dashboard');
      return;
    }
    fetchGates();
  }, [user, router, fetchGates]);

  const handleToggle = async () => {
    if (!confirmGate) return;
    const key = confirmGate.feature_key;
    const newIsPremium = !confirmGate.is_premium;
    setToggling(key);
    setConfirmGate(null);
    try {
      await apiClient.patch(`/api/v1/admin/feature-gates/${key}`, { is_premium: newIsPremium });
      setGates((prev) =>
        prev.map((g) => (g.feature_key === key ? { ...g, is_premium: newIsPremium } : g))
      );
      toastManager.add({
        title: newIsPremium
          ? `${key} is now premium-only`
          : `${key} is now free for all users`,
        type: 'success',
      });
    } catch {
      toastManager.add({
        title: `Failed to update ${key}. Please try again.`,
        type: 'error',
      });
    } finally {
      setToggling(null);
    }
  };

  return (
    <div className="max-w-3xl mx-auto px-4 py-8">
      {/* Page header */}
      <div className="mb-6">
        <h1 className="text-2xl font-bold text-text">Features</h1>
        <p className="mt-1 text-sm text-text-sub">
          Manage capability-level premium gates. Toggle a feature free to grant access to all users
          without a code deploy.
        </p>
      </div>

      {/* Gate list */}
      {loading ? (
        <Card className="p-6 space-y-3">
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
          <p className="text-base font-bold text-text mb-1">No feature gates configured</p>
          <p className="text-sm text-text-sub">
            Feature gates are added automatically when new features ship. Check back after the next
            deployment.
          </p>
        </Card>
      ) : (
        <Card className="overflow-hidden">
          <div className="px-4 py-3 border-b border-border">
            <h2 className="text-base font-bold text-text">Feature Gates ({gates.length})</h2>
          </div>
          <div className="divide-y divide-border">
            {gates.map((gate) => (
              <div key={gate.feature_key} className="flex items-center gap-4 px-4 min-h-[44px] py-3">
                {/* Feature key + description */}
                <div className="flex-1 min-w-0">
                  <span className="font-mono text-sm text-text block">{gate.feature_key}</span>
                  {gate.description && (
                    <span className="text-sm text-text-sub block mt-0.5">{gate.description}</span>
                  )}
                </div>

                {/* Status badge */}
                <span
                  className={cn(
                    'flex-shrink-0 inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-bold border',
                    gate.is_premium
                      ? 'bg-amber-500/10 text-amber-400 border-amber-500/20'
                      : 'bg-green-500/10 text-green-400 border-green-500/20'
                  )}
                >
                  {gate.is_premium ? 'Premium only' : 'Free for all'}
                </span>

                {/* Toggle switch */}
                <button
                  role="switch"
                  aria-checked={gate.is_premium}
                  aria-label={`Toggle ${gate.feature_key}`}
                  onClick={() => setConfirmGate(gate)}
                  disabled={toggling === gate.feature_key}
                  className={cn(
                    'relative flex-shrink-0 inline-flex h-[28px] w-[52px] min-h-[44px] items-center rounded-full transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-twitch',
                    gate.is_premium ? 'bg-amber-500/20' : 'bg-green-500/20',
                    toggling === gate.feature_key ? 'opacity-50 cursor-not-allowed' : 'cursor-pointer'
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
            ))}
          </div>
        </Card>
      )}

      {/* Confirmation dialog */}
      <DialogRoot open={!!confirmGate} onOpenChange={() => setConfirmGate(null)}>
        <DialogContent>
          <DialogTitle>
            {confirmGate?.is_premium
              ? `Make ${confirmGate?.feature_key} free for all users?`
              : `Restrict ${confirmGate?.feature_key} to premium users?`}
          </DialogTitle>
          <DialogDescription>
            {confirmGate?.is_premium
              ? 'All authenticated users will gain access immediately. No code deploy required.'
              : 'Only users with premium access will be able to use this feature.'}
          </DialogDescription>
          <div className="mt-4 flex justify-end gap-3">
            <Button variant="outline" onClick={() => setConfirmGate(null)}>
              No, keep as-is
            </Button>
            <Button onClick={handleToggle}>
              {confirmGate?.is_premium ? 'Make Free' : 'Make Premium'}
            </Button>
          </div>
        </DialogContent>
      </DialogRoot>
    </div>
  );
}
