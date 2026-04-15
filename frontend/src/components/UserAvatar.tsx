'use client'

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
