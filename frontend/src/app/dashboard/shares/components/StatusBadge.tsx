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
      className: 'bg-amber-500/10 text-amber-400 border border-amber-500/20',
      icon: '⏳',
    },
    accepted: {
      label: 'Active',
      className: 'bg-green-500/10 text-green-400 border border-green-500/20',
      icon: '✓',
    },
    expired: {
      label: 'Expired',
      className: 'bg-slate-700/40 text-slate-400 border border-slate-600/20',
      icon: '⏱',
    },
    revoked: {
      label: 'Revoked',
      className: 'bg-red-500/10 text-red-400 border border-red-500/20',
      icon: '✗',
    },
    rejected: {
      label: 'Rejected',
      className: 'bg-red-500/10 text-red-400 border border-red-500/20',
      icon: '✗',
    },
  }[status];

  const sizeClasses = size === 'sm' ? 'text-xs px-2 py-0.5' : 'text-xs px-2.5 py-0.5';

  return (
    <span
      className={`inline-flex items-center gap-1 rounded-full font-medium ${config.className} ${sizeClasses}`}
    >
      <span>{config.icon}</span>
      <span>{config.label}</span>
    </span>
  );
}
