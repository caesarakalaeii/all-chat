/**
 * Credit Roll Display Page (Unauthenticated)
 *
 * Public endpoint for displaying end-of-stream credits in OBS.
 * Shows leaderboards for subs, bits, raids, donations, follows, etc.
 *
 * Features:
 * - Scrolling credits with theme support
 * - Leaderboard categories
 * - User avatars and platform badges
 * - Configurable styling
 * - Auto-loop or single-play based on config
 *
 * This is a Client Component for animation and dynamic rendering.
 */

'use client';

import Image from 'next/image';
import { useEffect, useState } from 'react';
import type { CreditRollResponse, CreditRollConfig, LeaderboardEntry } from '@/lib/types/overlay';

export default function CreditRollPage({ params }: { params: { id: string } }) {
  const [creditData, setCreditData] = useState<CreditRollResponse | null>(null);
  const [config, setConfig] = useState<CreditRollConfig | null>(null);
  const [customCss, setCustomCss] = useState('');
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    const loadCreditRoll = async () => {
      try {
        // Load config
        const configResponse = await fetch(`/api/v1/overlays/public/${params.id}/creditroll`);
        if (configResponse.ok) {
          const configData = await configResponse.json();
          setConfig(configData);
          setCustomCss(configData.custom_css || '');
        }

        // Load credit roll data
        const dataResponse = await fetch(`/api/v1/overlays/${params.id}/credit-roll`);
        if (!dataResponse.ok) {
          const errorData = await dataResponse.json();
          throw new Error(errorData.error || 'Failed to load credit roll');
        }

        const data = await dataResponse.json();
        setCreditData(data);
      } catch (err) {
        console.error('Failed to load credit roll:', err);
        setError(err instanceof Error ? err.message : 'Failed to load credit roll');
      } finally {
        setLoading(false);
      }
    };

    loadCreditRoll();
  }, [params.id]);

  const renderLeaderboard = (title: string, entries: LeaderboardEntry[] | undefined, emoji: string) => {
    if (!entries || entries.length === 0) return null;

    return (
      <div className="mb-12">
        <h2 className="text-4xl font-bold text-white mb-6 flex items-center gap-3">
          <span className="text-5xl">{emoji}</span>
          {title}
        </h2>
        <div className="space-y-4">
          {entries.map((entry, index) => (
            <div
              key={index}
              className={`flex items-center gap-4 p-4 rounded-lg ${
                index === 0 ? 'bg-yellow-500/20 border-2 border-yellow-500' :
                index === 1 ? 'bg-gray-400/20 border-2 border-gray-400' :
                index === 2 ? 'bg-orange-600/20 border-2 border-orange-600' :
                'bg-gray-800/50 border border-gray-700'
              }`}
            >
              <div className={`text-3xl font-bold w-12 text-center ${
                index === 0 ? 'text-yellow-400' :
                index === 1 ? 'text-gray-300' :
                index === 2 ? 'text-orange-500' :
                'text-gray-500'
              }`}>
                #{entry.rank}
              </div>
              {entry.avatar_url && (
                <div className="relative w-12 h-12 rounded-full overflow-hidden">
                  <Image
                    src={entry.avatar_url}
                    alt={entry.display_name}
                    fill
                    className="object-cover"
                  />
                </div>
              )}
              <div className="flex-1">
                <div className="text-xl font-semibold text-white">{entry.display_name}</div>
                <div className="text-sm text-gray-400 capitalize">{entry.platform}</div>
              </div>
              <div className="text-2xl font-bold text-white">
                {entry.total_value !== undefined && entry.total_value > 0
                  ? `$${entry.total_value.toFixed(2)}`
                  : entry.count || ''}
              </div>
            </div>
          ))}
        </div>
      </div>
    );
  };

  if (loading) {
    return (
      <div className="min-h-screen flex items-center justify-center bg-gradient-to-b from-gray-900 to-black">
        <div className="text-center">
          <div className="animate-spin rounded-full h-16 w-16 border-b-4 border-white mx-auto mb-4"></div>
          <p className="text-white text-xl">Loading Credits...</p>
        </div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="min-h-screen flex items-center justify-center bg-gradient-to-b from-gray-900 to-black">
        <div className="text-center">
          <div className="text-6xl mb-4">⚠️</div>
          <p className="text-white text-xl mb-2">Unable to Load Credit Roll</p>
          <p className="text-gray-400 text-sm">{error}</p>
          <p className="text-gray-500 text-xs mt-4">Make sure you have an active streaming session</p>
        </div>
      </div>
    );
  }

  if (!creditData) {
    return (
      <div className="min-h-screen flex items-center justify-center bg-gradient-to-b from-gray-900 to-black">
        <div className="text-center">
          <p className="text-white text-xl">No credit roll data available</p>
        </div>
      </div>
    );
  }

  const theme = config?.theme || 'cinematic';
  const bgOpacity = config?.background_opacity || 0.8;

  return (
    <>
      {/* Custom CSS Injection */}
      {customCss && customCss.trim() && (
        <style dangerouslySetInnerHTML={{ __html: customCss }} />
      )}

      <div
        className="min-h-screen overflow-hidden"
        style={{
          background: theme === 'cinematic'
            ? `linear-gradient(to bottom, rgba(17, 24, 39, ${bgOpacity}), rgba(0, 0, 0, ${bgOpacity}))`
            : theme === 'modern'
            ? `linear-gradient(135deg, rgba(99, 102, 241, ${bgOpacity * 0.3}), rgba(139, 92, 246, ${bgOpacity * 0.3}))`
            : `rgba(17, 24, 39, ${bgOpacity})`,
        }}
      >
        <div className="container mx-auto px-8 py-12">
          {/* Header */}
          <div className="text-center mb-16">
            <h1 className="text-6xl font-bold text-white mb-4 animate-fade-in">
              🎬 Stream Credits
            </h1>
            <p className="text-2xl text-gray-300">
              Thank you to everyone who supported the stream!
            </p>
            <div className="mt-4 text-gray-400">
              <p>Session: {new Date(creditData.session_started_at).toLocaleDateString()}</p>
              <p>Duration: {Math.floor(creditData.session_duration_seconds / 60)} minutes</p>
            </div>
          </div>

          {/* Leaderboards */}
          <div className="max-w-4xl mx-auto">
            {renderLeaderboard('Top Subscribers', creditData.leaderboards.subs, '⭐')}
            {renderLeaderboard('Top Gifters', creditData.leaderboards.gifts, '🎁')}
            {renderLeaderboard('Top Cheerers', creditData.leaderboards.bits, '💎')}
            {renderLeaderboard('Top Raiders', creditData.leaderboards.raids, '⚔️')}
            {renderLeaderboard('Top Super Chats', creditData.leaderboards.super_chats, '💰')}
            {renderLeaderboard('New Followers', creditData.leaderboards.follows, '❤️')}
          </div>

          {/* Clips Section */}
          {config?.clips_enabled && creditData.clips && creditData.clips.length > 0 && (
            <div className="max-w-4xl mx-auto mt-16">
              <h2 className="text-4xl font-bold text-white mb-6 flex items-center gap-3">
                <span className="text-5xl">🎥</span>
                Stream Highlights
              </h2>
              <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
                {creditData.clips.map((clip) => (
                  <div key={clip.id} className="bg-gray-800/50 rounded-lg overflow-hidden border border-gray-700">
                    <div className="relative h-48">
                      <Image
                        src={clip.thumbnail_url}
                        alt={clip.title}
                        fill
                        className="object-cover"
                      />
                    </div>
                    <div className="p-4">
                      <h3 className="text-lg font-semibold text-white mb-2">{clip.title}</h3>
                      <div className="flex justify-between text-sm text-gray-400">
                        <span>{clip.view_count.toLocaleString()} views</span>
                        <span>{clip.duration.toFixed(1)}s</span>
                      </div>
                    </div>
                  </div>
                ))}
              </div>
            </div>
          )}

          {/* Footer */}
          <div className="text-center mt-24 mb-12">
            <div className="text-4xl font-bold text-white mb-4">
              Thank you for watching! ❤️
            </div>
            <p className="text-xl text-gray-300">
              See you next stream!
            </p>
          </div>
        </div>
      </div>

      <style jsx global>{`
        @keyframes fade-in {
          from {
            opacity: 0;
            transform: translateY(-20px);
          }
          to {
            opacity: 1;
            transform: translateY(0);
          }
        }

        .animate-fade-in {
          animation: fade-in 1s ease-out;
        }
        `}</style>
      </>
    );
}
