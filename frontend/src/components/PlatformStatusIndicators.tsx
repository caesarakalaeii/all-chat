/**
 * Platform Status Indicators Component
 *
 * Displays small icons for each platform (Twitch, YouTube, Kick, TikTok).
 * Active platforms (recently sent messages) appear in color.
 * Inactive platforms appear in grayscale.
 *
 * Can be hidden via CSS by targeting `.platform-status-indicators`
 */

'use client';

interface PlatformStatusIndicatorsProps {
  activePlatforms: Set<string>;
}

// Platform SVG Icons
const TwitchIcon = () => (
  <svg viewBox="0 0 24 24" fill="currentColor" className="w-5 h-5">
    <path d="M11.571 4.714h1.715v5.143H11.57zm4.715 0H18v5.143h-1.714zM6 0L1.714 4.286v15.428h5.143V24l4.286-4.286h3.428L22.286 12V0zm14.571 11.143l-3.428 3.428h-3.429l-3 3v-3H6.857V1.714h13.714Z"/>
  </svg>
);

const YouTubeIcon = () => (
  <svg viewBox="0 0 24 24" fill="currentColor" className="w-5 h-5">
    <path d="M23.498 6.186a3.016 3.016 0 0 0-2.122-2.136C19.505 3.545 12 3.545 12 3.545s-7.505 0-9.377.505A3.017 3.017 0 0 0 .502 6.186C0 8.07 0 12 0 12s0 3.93.502 5.814a3.016 3.016 0 0 0 2.122 2.136c1.871.505 9.376.505 9.376.505s7.505 0 9.377-.505a3.015 3.015 0 0 0 2.122-2.136C24 15.93 24 12 24 12s0-3.93-.502-5.814zM9.545 15.568V8.432L15.818 12l-6.273 3.568z"/>
  </svg>
);

const KickIcon = () => (
  <svg viewBox="0 0 24 24" fill="currentColor" className="w-5 h-5">
    <path d="M6 3v18h4v-7l1.5 1.5L15 19l4.5-4.5L15 10l-3.5 3.5L10 12V3H6zm8 8.5l4.5 4.5-2.5 2.5-4.5-4.5L14 11.5z"/>
  </svg>
);

const TikTokIcon = () => (
  <svg viewBox="0 0 24 24" fill="currentColor" className="w-5 h-5">
    <path d="M19.59 6.69a4.83 4.83 0 0 1-3.77-4.25V2h-3.45v13.67a2.89 2.89 0 0 1-5.2 1.74 2.89 2.89 0 0 1 2.31-4.64 2.93 2.93 0 0 1 .88.13V9.4a6.84 6.84 0 0 0-1-.05A6.33 6.33 0 0 0 5 20.1a6.34 6.34 0 0 0 10.86-4.43v-7a8.16 8.16 0 0 0 4.77 1.52v-3.4a4.85 4.85 0 0 1-1-.1z"/>
  </svg>
);

export default function PlatformStatusIndicators({ activePlatforms }: PlatformStatusIndicatorsProps) {
  const platforms = [
    {
      name: 'twitch',
      label: 'Twitch',
      icon: TwitchIcon,
      colorClass: 'text-purple-500',
    },
    {
      name: 'youtube',
      label: 'YouTube',
      icon: YouTubeIcon,
      colorClass: 'text-red-500',
    },
    {
      name: 'kick',
      label: 'Kick',
      icon: KickIcon,
      colorClass: 'text-green-500',
    },
    {
      name: 'tiktok',
      label: 'TikTok',
      icon: TikTokIcon,
      colorClass: 'text-cyan-400',
    },
  ];

  return (
    <div className="platform-status-indicators fixed top-4 right-4 flex gap-2 bg-gray-900/80 backdrop-blur-sm rounded-lg px-3 py-2 shadow-lg z-50">
      {platforms.map((platform) => {
        const isActive = activePlatforms.has(platform.name);
        const Icon = platform.icon;
        return (
          <div
            key={platform.name}
            className={`platform-indicator platform-indicator-${platform.name} flex items-center justify-center w-8 h-8 rounded-md transition-all duration-300 ${
              isActive
                ? `${platform.colorClass} bg-white/10`
                : 'grayscale opacity-40 bg-gray-800/50'
            }`}
            title={`${platform.label} ${isActive ? '(Active)' : '(Inactive)'}`}
          >
            <Icon />
          </div>
        );
      })}
    </div>
  );
}
