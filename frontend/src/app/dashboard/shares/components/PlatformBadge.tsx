interface PlatformBadgeProps {
  source: {
    platform: string;
    channel_name: string;
  };
}

const PLATFORM_COLORS = {
  twitch: 'bg-purple-100 text-purple-800',
  youtube: 'bg-red-100 text-red-800',
  kick: 'bg-green-100 text-green-800',
  tiktok: 'bg-gray-100 text-gray-800',
};

export function PlatformBadge({ source }: PlatformBadgeProps) {
  const colorClass = PLATFORM_COLORS[source.platform as keyof typeof PLATFORM_COLORS] || 'bg-gray-100 text-gray-800';

  return (
    <div
      className={`inline-flex items-center px-2 py-1 rounded text-xs font-medium ${colorClass}`}
      title={`${source.channel_name} on ${source.platform}`}
    >
      <span className="capitalize">{source.platform}</span>
    </div>
  );
}
