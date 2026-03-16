'use client'

export interface UserAvatarProps {
  avatarUrl?: string
  frameUrl?: string
  flairUrl?: string
  size: number
  displayName?: string
}

export function UserAvatar({ avatarUrl, frameUrl, flairUrl, size, displayName }: UserAvatarProps) {
  const frameSize = Math.round(size * 1.4)
  const flairSize = Math.round(size * 0.4)
  const initials = displayName ? displayName.charAt(0).toUpperCase() : '?'

  return (
    <div className="relative" style={{ width: size, height: size, overflow: 'visible' }}>
      {/* Base avatar */}
      {avatarUrl ? (
        // eslint-disable-next-line @next/next/no-img-element
        <img
          src={avatarUrl}
          alt={displayName ?? 'Avatar'}
          className="w-full h-full rounded-full object-cover"
        />
      ) : (
        <div
          className="w-full h-full rounded-full bg-surface-2 flex items-center justify-center text-text-sub font-medium"
          style={{ fontSize: Math.round(size * 0.4) }}
          aria-label={displayName ?? 'Avatar'}
        >
          {initials}
        </div>
      )}

      {/* Frame — centered, 1.4× size, overflows container */}
      {frameUrl && (
        // eslint-disable-next-line @next/next/no-img-element
        <img
          src={frameUrl}
          alt=""
          aria-hidden="true"
          className="absolute pointer-events-none"
          style={{
            width: frameSize,
            height: frameSize,
            top: '50%',
            left: '50%',
            transform: 'translate(-50%, -50%)',
            zIndex: 10,
          }}
        />
      )}

      {/* Flair — bottom-right corner, 0.4× size */}
      {flairUrl && (
        // eslint-disable-next-line @next/next/no-img-element
        <img
          src={flairUrl}
          alt=""
          aria-hidden="true"
          className="absolute bottom-0 right-0 pointer-events-none"
          style={{ width: flairSize, height: flairSize, zIndex: 10 }}
        />
      )}
    </div>
  )
}
