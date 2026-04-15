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
 * Admin Cosmetics Catalog Page
 *
 * Allows admins to manage the avatar frame and flair catalog.
 *
 * Features:
 * - Tabs: Frames | Flairs
 * - Entry list: thumbnail, name, Premium badge, delete button
 * - Add form: name, image URL (with blur preview), is_premium toggle, submit
 *
 * Route: /admin/cosmetics
 */

'use client';

import { useEffect, useState, useCallback } from 'react';
import { useRouter } from 'next/navigation';
import { useAuthStore } from '@/lib/stores/auth-store';
import { apiClient } from '@/lib/api/client';
import { Card } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Badge } from '@/components/ui/badge';
import { Skeleton } from '@/components/ui/skeleton';
import { toastManager } from '@/lib/toast';
import { AdminNav } from '@/components/AdminNav';

interface CatalogEntry {
  id: string;
  name: string;
  image_url: string;
  is_premium: boolean;
}

interface CatalogListResponse {
  frames?: CatalogEntry[];
  flairs?: CatalogEntry[];
}

export default function AdminCosmeticsPage() {
  const router = useRouter();
  const { user } = useAuthStore();

  const [activeTab, setActiveTab] = useState<'frames' | 'flairs'>('frames');
  const [frames, setFrames] = useState<CatalogEntry[]>([]);
  const [flairs, setFlairs] = useState<CatalogEntry[]>([]);
  const [loading, setLoading] = useState(true);

  // Add form state
  const [addName, setAddName] = useState('');
  const [addImageUrl, setAddImageUrl] = useState('');
  const [addPreviewUrl, setAddPreviewUrl] = useState('');
  const [addIsPremium, setAddIsPremium] = useState(false);
  const [submitting, setSubmitting] = useState(false);

  useEffect(() => {
    if (!user?.is_admin) {
      router.push('/dashboard');
      return;
    }
    fetchAll();
  }, [user, router]);

  const fetchFrames = useCallback(async () => {
    try {
      const response = await apiClient.get<CatalogListResponse>('/api/v1/admin/cosmetics/frames');
      setFrames(response.frames ?? []);
    } catch {
      toastManager.add({ title: 'Failed to load frames', type: 'error' });
    }
  }, []);

  const fetchFlairs = useCallback(async () => {
    try {
      const response = await apiClient.get<CatalogListResponse>('/api/v1/admin/cosmetics/flairs');
      setFlairs(response.flairs ?? []);
    } catch {
      toastManager.add({ title: 'Failed to load flairs', type: 'error' });
    }
  }, []);

  const fetchAll = async () => {
    setLoading(true);
    try {
      await Promise.all([fetchFrames(), fetchFlairs()]);
    } finally {
      setLoading(false);
    }
  };

  const handleDelete = async (id: string) => {
    try {
      if (activeTab === 'frames') {
        await apiClient.delete(`/api/v1/admin/cosmetics/frames/${id}`);
        toastManager.add({ title: 'Frame deleted', type: 'success' });
        await fetchFrames();
      } else {
        await apiClient.delete(`/api/v1/admin/cosmetics/flairs/${id}`);
        toastManager.add({ title: 'Flair deleted', type: 'success' });
        await fetchFlairs();
      }
    } catch {
      toastManager.add({ title: 'Delete failed', type: 'error' });
    }
  };

  const handleAdd = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!addName.trim() || !addImageUrl.trim()) return;

    setSubmitting(true);
    try {
      if (activeTab === 'frames') {
        await apiClient.post('/api/v1/admin/cosmetics/frames', {
          name: addName.trim(),
          image_url: addImageUrl.trim(),
          is_premium: addIsPremium,
        });
        toastManager.add({ title: 'Frame added', type: 'success' });
        await fetchFrames();
      } else {
        await apiClient.post('/api/v1/admin/cosmetics/flairs', {
          name: addName.trim(),
          image_url: addImageUrl.trim(),
          is_premium: addIsPremium,
        });
        toastManager.add({ title: 'Flair added', type: 'success' });
        await fetchFlairs();
      }
      // Clear form
      setAddName('');
      setAddImageUrl('');
      setAddPreviewUrl('');
      setAddIsPremium(false);
    } catch {
      toastManager.add({ title: 'Add failed', type: 'error' });
    } finally {
      setSubmitting(false);
    }
  };

  const currentEntries = activeTab === 'frames' ? frames : flairs;
  const itemLabel = activeTab === 'frames' ? 'Frame' : 'Flair';

  return (
    <div className="min-h-screen bg-bg">
      <AdminNav />
      <div className="max-w-4xl mx-auto px-4 py-8">
        <div className="mb-6">
          <h1 className="text-2xl font-bold text-text">Cosmetics Catalog</h1>
          <p className="mt-1 text-sm text-text-sub">Manage avatar frames and flairs</p>
        </div>

        {/* Tab bar */}
        <div className="flex border-b border-border mb-6">
          {(['frames', 'flairs'] as const).map((tab) => (
            <button
              key={tab}
              onClick={() => setActiveTab(tab)}
              className={`px-4 py-2 text-sm font-medium capitalize transition-colors ${
                activeTab === tab
                  ? 'border-b-2 border-primary text-text'
                  : 'text-text-sub hover:text-text'
              }`}
            >
              {tab}
            </button>
          ))}
        </div>

        {/* Entry list */}
        {loading ? (
          <Card className="p-6 space-y-3 mb-6">
            {Array.from({ length: 4 }).map((_, i) => (
              <Skeleton key={i} className="h-16 w-full rounded-lg" />
            ))}
          </Card>
        ) : (
          <Card className="overflow-hidden mb-6">
            {currentEntries.length === 0 ? (
              <div className="px-4 py-8 text-center text-text-sub text-sm">
                No {itemLabel.toLowerCase()}s in catalog yet
              </div>
            ) : (
              <div className="divide-y divide-border">
                {currentEntries.map((entry) => (
                  <div key={entry.id} className="flex items-center gap-3 px-4 py-3">
                    {/* Thumbnail */}
                    <div className="w-16 h-16 flex-shrink-0 rounded bg-surface-2 flex items-center justify-center overflow-hidden">
                      {/* eslint-disable-next-line @next/next/no-img-element */}
                      <img
                        src={entry.image_url}
                        alt={entry.name}
                        className="w-16 h-16 object-contain"
                      />
                    </div>

                    {/* Name and premium badge */}
                    <div className="flex-1 min-w-0">
                      <div className="flex items-center gap-2">
                        <span className="text-sm font-medium text-text truncate">{entry.name}</span>
                        {entry.is_premium && (
                          <Badge className="text-xs">Premium</Badge>
                        )}
                      </div>
                    </div>

                    {/* Delete button */}
                    <Button
                      variant="ghost"
                      size="sm"
                      onClick={() => handleDelete(entry.id)}
                      className="text-text-sub hover:text-destructive"
                      aria-label={`Delete ${entry.name}`}
                    >
                      ×
                    </Button>
                  </div>
                ))}
              </div>
            )}
          </Card>
        )}

        {/* Add form */}
        <Card className="p-6">
          <h2 className="text-base font-semibold text-text mb-4">Add {itemLabel}</h2>
          <form onSubmit={handleAdd} className="space-y-4">
            <div>
              <label className="block text-xs text-text-sub mb-1" htmlFor="add-name">
                Name
              </label>
              <Input
                id="add-name"
                value={addName}
                onChange={(e) => setAddName(e.target.value)}
                placeholder={`${itemLabel} name`}
                required
              />
            </div>

            <div>
              <label className="block text-xs text-text-sub mb-1" htmlFor="add-image-url">
                Image URL
              </label>
              <div className="flex items-center gap-3">
                <Input
                  id="add-image-url"
                  value={addImageUrl}
                  onChange={(e) => setAddImageUrl(e.target.value)}
                  onBlur={() => setAddPreviewUrl(addImageUrl)}
                  placeholder="https://example.com/frame.png"
                  required
                  className="flex-1"
                />
                {addPreviewUrl && (
                  <div className="w-16 h-16 flex-shrink-0 rounded bg-surface-2 flex items-center justify-center overflow-hidden">
                    {/* eslint-disable-next-line @next/next/no-img-element */}
                    <img
                      src={addPreviewUrl}
                      alt="Preview"
                      className="w-16 h-16 object-contain rounded"
                    />
                  </div>
                )}
              </div>
            </div>

            <div className="flex items-center gap-2">
              <input
                id="add-is-premium"
                type="checkbox"
                checked={addIsPremium}
                onChange={(e) => setAddIsPremium(e.target.checked)}
                className="rounded border-border"
              />
              <label htmlFor="add-is-premium" className="text-sm text-text">
                Premium only
              </label>
            </div>

            <Button type="submit" disabled={submitting || !addName.trim() || !addImageUrl.trim()}>
              {submitting ? 'Adding…' : `Add ${itemLabel}`}
            </Button>
          </form>
        </Card>
      </div>
    </div>
  );
}
