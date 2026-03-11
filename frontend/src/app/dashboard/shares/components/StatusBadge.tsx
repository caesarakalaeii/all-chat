/**
 * StatusBadge Component
 *
 * Color-coded status indicator for share requests.
 */

interface StatusBadgeProps {
  status: 'pending' | 'accepted' | 'rejected' | 'expired' | 'revoked';
  size?: 'sm' | 'md';
}

export function StatusBadge({ status, size = 'md' }: StatusBadgeProps) {
  const config = {
    pending: {
      label: 'Pending',
      bgColor: 'bg-yellow-100',
      textColor: 'text-yellow-800',
      icon: '⏳',
    },
    accepted: {
      label: 'Active',
      bgColor: 'bg-green-100',
      textColor: 'text-green-800',
      icon: '✓',
    },
    expired: {
      label: 'Expired',
      bgColor: 'bg-gray-100',
      textColor: 'text-gray-600',
      icon: '⏱',
    },
    revoked: {
      label: 'Revoked',
      bgColor: 'bg-red-100',
      textColor: 'text-red-800',
      icon: '✗',
    },
    rejected: {
      label: 'Rejected',
      bgColor: 'bg-red-100',
      textColor: 'text-red-800',
      icon: '✗',
    },
  }[status];

  const sizeClasses = size === 'sm' ? 'text-xs px-2 py-0.5' : 'text-sm px-3 py-1';

  return (
    <span
      className={`inline-flex items-center gap-1 rounded-full ${config.bgColor} ${config.textColor} ${sizeClasses} font-medium`}
    >
      <span>{config.icon}</span>
      <span>{config.label}</span>
    </span>
  );
}
