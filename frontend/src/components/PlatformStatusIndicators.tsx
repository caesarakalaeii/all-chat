/**
 * Platform Status Indicators Component
 *
 * Displays small icons for each platform (Twitch, YouTube, Kick, TikTok).
 * Active platforms (recently sent messages) appear in color.
 * Inactive platforms appear in grayscale.
 * Reconnecting platforms show a countdown timer.
 *
 * Can be hidden via CSS by targeting `.platform-status-indicators`
 */

'use client';

import { useEffect, useState } from 'react';
import type { PlatformStatus } from '@/lib/types/message';

interface PlatformStatusIndicatorsProps {
  activePlatforms: Set<string>;
  platformStatuses: Map<string, PlatformStatus>;
}

// Platform SVG Icons - Using official brand colors per platform guidelines
const TwitchIcon = () => (
  <svg viewBox="0 0 24 24" className="w-5 h-5">
    {/* Twitch official purple: #9146FF - Per Twitch brand guidelines */}
    <path fill="#9146FF" d="M11.571 4.714h1.715v5.143H11.57zm4.715 0H18v5.143h-1.714zM6 0L1.714 4.286v15.428h5.143V24l4.286-4.286h3.428L22.286 12V0zm14.571 11.143l-3.428 3.428h-3.429l-3 3v-3H6.857V1.714h13.714Z"/>
  </svg>
);

const YouTubeIcon = () => (
  <svg viewBox="0 0 24 24" className="w-5 h-5">
    {/* YouTube official red: #FF0000 - Never modify this color per branding guidelines */}
    <path fill="#FF0000" d="M23.498 6.186a3.016 3.016 0 0 0-2.122-2.136C19.505 3.545 12 3.545 12 3.545s-7.505 0-9.377.505A3.017 3.017 0 0 0 .502 6.186C0 8.07 0 12 0 12s0 3.93.502 5.814a3.016 3.016 0 0 0 2.122 2.136c1.871.505 9.376.505 9.376.505s7.505 0 9.377-.505a3.015 3.015 0 0 0 2.122-2.136C24 15.93 24 12 24 12s0-3.93-.502-5.814zM9.545 15.568V8.432L15.818 12l-6.273 3.568z"/>
  </svg>
);

const KickIcon = () => (
  <svg viewBox="0 0 24 24" className="w-5 h-5" style={{ imageRendering: 'pixelated' }}>
    {/* Kick - Simple green K */}
    <text x="12" y="18" fontSize="20" fontWeight="bold" fill="#00E701" textAnchor="middle" fontFamily="monospace">K</text>
  </svg>
);

const TikTokIcon = () => (
  <svg viewBox="0 0 24 24" className="w-5 h-5">
    {/* TikTok official black: #000000 - Primary logo color per TikTok brand guidelines */}
    <path fill="#000000" d="M19.59 6.69a4.83 4.83 0 0 1-3.77-4.25V2h-3.45v13.67a2.89 2.89 0 0 1-5.2 1.74 2.89 2.89 0 0 1 2.31-4.64 2.93 2.93 0 0 1 .88.13V9.4a6.84 6.84 0 0 0-1-.05A6.33 6.33 0 0 0 5 20.1a6.34 6.34 0 0 0 10.86-4.43v-7a8.16 8.16 0 0 0 4.77 1.52v-3.4a4.85 4.85 0 0 1-1-.1z"/>
  </svg>
);

export default function PlatformStatusIndicators({ activePlatforms, platformStatuses }: PlatformStatusIndicatorsProps) {
  const [countdowns, setCountdowns] = useState<Map<string, number>>(new Map());

  const platforms = [
    {
      name: 'twitch',
      label: 'Twitch',
      icon: TwitchIcon,
      colorClass: '', // Twitch icon color is fixed to #9146FF per branding guidelines
    },
    {
      name: 'youtube',
      label: 'YouTube',
      icon: YouTubeIcon,
      colorClass: '', // YouTube icon color is fixed to #FF0000 per branding guidelines
    },
    {
      name: 'kick',
      label: 'Kick',
      icon: KickIcon,
      colorClass: '', // Kick icon color is fixed to #00E701 per branding guidelines
    },
    {
      name: 'tiktok',
      label: 'TikTok',
      icon: TikTokIcon,
      colorClass: '', // TikTok icon color is fixed to #000000 per branding guidelines
    },
  ];

  // Update countdown timers every second
  useEffect(() => {
    const interval = setInterval(() => {
      const newCountdowns = new Map<string, number>();

      platformStatuses.forEach((status, platform) => {
        if (status.status === 'reconnecting' && status.next_retry_at) {
          const nextRetry = new Date(status.next_retry_at).getTime();
          const now = Date.now();
          const secondsRemaining = Math.max(0, Math.ceil((nextRetry - now) / 1000));
          newCountdowns.set(platform, secondsRemaining);
        }
      });

      setCountdowns(newCountdowns);
    }, 1000);

    return () => clearInterval(interval);
  }, [platformStatuses]);

  return (
    <div className="platform-status-indicators fixed top-4 right-4 flex gap-2 bg-gray-900/80 backdrop-blur-sm rounded-lg px-3 py-2 shadow-lg z-50">
      {platforms.map((platform) => {
        const isActive = activePlatforms.has(platform.name);
        const status = platformStatuses.get(platform.name);
        const countdown = countdowns.get(platform.name);
        const Icon = platform.icon;

        // Determine status class
        let statusClass = isActive ? 'bg-white/10' : 'opacity-40 bg-gray-800/50';
        let tooltipText = `${platform.label} ${isActive ? '(Active)' : '(Inactive)'}`;

        if (status) {
          if (status.status === 'reconnecting' && countdown !== undefined) {
            statusClass = 'bg-yellow-500/20 opacity-100';
            tooltipText = `${platform.label} - Reconnecting in ${countdown}s`;
          } else if (status.status === 'quota_exceeded') {
            statusClass = 'bg-red-500/20 opacity-100';
            tooltipText = `${platform.label} - Quota exceeded`;
          } else if (status.status === 'offline') {
            // Check if offline is due to auth error
            const isAuthError = status.error_message?.toLowerCase().includes('oauth') ||
                               status.error_message?.toLowerCase().includes('token');

            if (isAuthError) {
              statusClass = 'bg-red-500/20 opacity-100 border border-red-500/50';
              tooltipText = `${platform.label} - Auth Required`;
            } else {
              statusClass = 'opacity-20 bg-gray-800/50';
              tooltipText = status.error_message
                ? `${platform.label} - ${status.error_message}`
                : `${platform.label} - Offline`;
            }
          } else if (status.status === 'connected') {
            statusClass = 'bg-green-500/20 opacity-100';
            tooltipText = `${platform.label} - Connected`;
          }
        }

        return (
          <div
            key={platform.name}
            className={`platform-indicator platform-indicator-${platform.name} relative flex items-center justify-center w-8 h-8 rounded-md transition-all duration-300 ${statusClass}`}
            title={tooltipText}
          >
            <Icon />
            {status?.status === 'reconnecting' && countdown !== undefined && (
              <div className="absolute -bottom-1 -right-1 bg-yellow-500 text-white text-xs px-1 rounded font-mono">
                {countdown}s
              </div>
            )}
          </div>
        );
      })}
    </div>
  );
}
