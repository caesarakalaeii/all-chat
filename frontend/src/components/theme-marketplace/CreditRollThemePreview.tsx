/**
 * Credit Roll Theme Preview Component
 *
 * Shows a preview of what the credit roll will look like with the theme applied.
 * Displays sample leaderboard entries instead of chat messages.
 */

'use client';

interface CreditRollThemePreviewProps {
  css: string;
}

const SAMPLE_LEADERBOARD_DATA = [
  {
    rank: 1,
    display_name: 'TopSupporter',
    platform: 'twitch',
    avatar_url: 'https://static-cdn.jtvnw.net/jtv_user_pictures/aaa-profile_image-70x70.png',
    total_value: 500.00,
  },
  {
    rank: 2,
    display_name: 'GenerousViewer',
    platform: 'youtube',
    avatar_url: 'https://yt3.ggpht.com/a/default-user=s88-c-k-c0x00ffffff-no-rj',
    total_value: 250.00,
  },
  {
    rank: 3,
    display_name: 'AwesomeFan',
    platform: 'kick',
    avatar_url: 'https://static-cdn.jtvnw.net/jtv_user_pictures/bbb-profile_image-70x70.png',
    total_value: 100.00,
  },
];

export default function CreditRollThemePreview({ css }: CreditRollThemePreviewProps) {
  // Scope CSS to this preview only
  const scopedCss = css
    .replace(/body\s*{/g, '.credit-roll-preview-body {')
    .replace(/\bbody\b/g, '.credit-roll-preview-body')
    .split('\n')
    .map(line => {
      // Skip @import, @keyframes, and already scoped selectors
      if (line.trim().startsWith('@import') ||
          line.trim().startsWith('@keyframes') ||
          line.includes('.credit-roll-preview')) {
        return line;
      }
      // Scope other selectors to .credit-roll-preview-root
      if (line.includes('{') && !line.trim().startsWith('@')) {
        const parts = line.split('{');
        if (parts[0].trim()) {
          return `.credit-roll-preview-root ${parts[0]} {${parts[1] || ''}`;
        }
      }
      return line;
    })
    .join('\n');

  return (
    <div className="credit-roll-preview-root relative w-full h-full overflow-hidden">
      {/* Inject scoped theme CSS */}
      <style dangerouslySetInnerHTML={{ __html: scopedCss }} />

      {/* Credit roll preview container */}
      <div className="credit-roll-preview-body min-h-full overflow-y-auto p-4 bg-gradient-to-b from-gray-900 to-black">
        {/* Header */}
        <div className="text-center mb-6">
          <h1 className="text-3xl font-bold text-white mb-2">
            🎬 Stream Credits
          </h1>
          <p className="text-sm text-gray-300">
            Thank you for your support!
          </p>
        </div>

        {/* Sample Leaderboard */}
        <div className="max-w-md mx-auto">
          <h2 className="text-2xl font-bold text-white mb-4 flex items-center gap-2">
            <span className="text-3xl">⭐</span>
            Top Subscribers
          </h2>
          <div className="space-y-4">
            {SAMPLE_LEADERBOARD_DATA.map((entry) => (
              <div
                key={entry.rank}
                className={`flex items-center gap-4 p-4 rounded-lg ${
                  entry.rank === 1 ? 'bg-yellow-500/20 border-2 border-yellow-500' :
                  entry.rank === 2 ? 'bg-gray-400/20 border-2 border-gray-400' :
                  entry.rank === 3 ? 'bg-orange-600/20 border-2 border-orange-600' :
                  'bg-gray-800/50 border border-gray-700'
                }`}
              >
                <div className={`text-3xl font-bold w-12 text-center ${
                  entry.rank === 1 ? 'text-yellow-400' :
                  entry.rank === 2 ? 'text-gray-300' :
                  entry.rank === 3 ? 'text-orange-500' :
                  'text-gray-500'
                }`}>
                  #{entry.rank}
                </div>
                <div className="relative w-12 h-12 rounded-full overflow-hidden bg-gray-700">
                  <img
                    src={entry.avatar_url}
                    alt={entry.display_name}
                    className="w-full h-full object-cover"
                    onError={(e) => {
                      (e.target as HTMLImageElement).src = 'data:image/svg+xml,%3Csvg xmlns="http://www.w3.org/2000/svg" width="48" height="48"%3E%3Crect fill="%23374151" width="48" height="48"/%3E%3C/svg%3E';
                    }}
                  />
                </div>
                <div className="flex-1">
                  <div className="text-xl font-semibold text-white">{entry.display_name}</div>
                  <div className="text-sm text-gray-400 capitalize">{entry.platform}</div>
                </div>
                <div className="text-2xl font-bold text-white">
                  ${entry.total_value.toFixed(2)}
                </div>
              </div>
            ))}
          </div>
        </div>

        {/* Footer */}
        <div className="text-center mt-8">
          <div className="text-2xl font-bold text-white mb-2">
            Thank you! ❤️
          </div>
          <p className="text-sm text-gray-300">
            See you next stream!
          </p>
        </div>
      </div>
    </div>
  );
}
