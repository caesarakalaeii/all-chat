/**
 * Create Overlay Page
 *
 * Form for creating a new overlay.
 *
 * Features:
 * - Name input (required)
 * - Description textarea (optional)
 * - Form validation
 * - Error handling
 * - Redirects to overlay editor on success
 *
 * Client Component for form handling and state management.
 */

'use client';

import { useState, useEffect } from 'react';
import { useRouter } from 'next/navigation';
import { useAuthStore } from '@/lib/stores/auth-store';
import { useOverlayStore } from '@/lib/stores/overlay-store';

export default function NewOverlayPage() {
  const router = useRouter();
  const { token } = useAuthStore();
  const { createOverlay } = useOverlayStore();

  const [name, setName] = useState('');
  const [description, setDescription] = useState('');
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');

  // Redirect if not logged in
  useEffect(() => {
    if (!token) {
      router.push('/');
    }
  }, [token, router]);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();

    if (!name.trim()) {
      setError('Overlay name is required');
      return;
    }

    setLoading(true);
    setError('');

    try {
      const overlay = await createOverlay({
        name,
        description: description || undefined
      });
      router.push(`/overlays/${overlay.id}`);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to create overlay');
      setLoading(false);
    }
  };

  return (
    <div className="min-h-screen bg-gray-900">
      {/* Navbar */}
      <nav className="bg-gray-800 border-b border-gray-700">
        <div className="container mx-auto px-4 py-4">
          <a href="/dashboard" className="text-2xl font-bold text-white">
            All-Chat
          </a>
        </div>
      </nav>

      {/* Main Content */}
      <div className="container mx-auto px-4 py-8 max-w-2xl">
        <div className="mb-8">
          <button
            onClick={() => router.push('/dashboard')}
            className="text-gray-400 hover:text-white transition-colors inline-flex items-center gap-2"
          >
            <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                strokeWidth={2}
                d="M15 19l-7-7 7-7"
              />
            </svg>
            Back to Dashboard
          </button>
        </div>

        <h1 className="text-3xl font-bold text-white mb-8">Create New Overlay</h1>

        <form onSubmit={handleSubmit} className="space-y-6">
          {/* Name Input */}
          <div>
            <label htmlFor="name" className="block text-sm font-medium text-gray-300 mb-2">
              Overlay Name <span className="text-red-500">*</span>
            </label>
            <input
              id="name"
              type="text"
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="My Awesome Overlay"
              required
              className="w-full px-4 py-2 bg-gray-800 text-white border border-gray-700 rounded-lg focus:outline-none focus:border-twitch transition-colors"
            />
          </div>

          {/* Description Input */}
          <div>
            <label htmlFor="description" className="block text-sm font-medium text-gray-300 mb-2">
              Description (Optional)
            </label>
            <textarea
              id="description"
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              placeholder="Describe your overlay..."
              rows={3}
              className="w-full px-4 py-2 bg-gray-800 text-white border border-gray-700 rounded-lg focus:outline-none focus:border-twitch transition-colors resize-none"
            ></textarea>
          </div>

          {/* Error Display */}
          {error && (
            <div className="bg-red-500/10 border border-red-500 rounded-lg p-4">
              <p className="text-red-500 text-sm">{error}</p>
            </div>
          )}

          {/* Action Buttons */}
          <div className="flex gap-4">
            <button
              type="submit"
              disabled={loading}
              className="flex-1 bg-twitch hover:bg-purple-700 disabled:bg-gray-700 text-white font-semibold py-3 px-6 rounded-lg transition-colors"
            >
              {loading ? 'Creating...' : 'Create Overlay'}
            </button>
            <button
              type="button"
              onClick={() => router.push('/dashboard')}
              className="px-6 py-3 bg-gray-700 hover:bg-gray-600 text-white rounded-lg transition-colors"
            >
              Cancel
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}
