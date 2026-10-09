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
 * along with this program. If not, <https://www.gnu.org/licenses/>.
 */

/**
 * PlatformSignInButton — the one "Sign in with <platform>" button of the
 * lanes design, shared by the homepage and the viewer identity page so the
 * two cannot drift apart again (the viewer page once painted Twitch white
 * with a Kick-green hover). Styling lives in `.lanes-signin` (globals.css)
 * and uses each platform's official brand color, not the desaturated lane
 * palette: these are brand marks, not chart fills.
 */

import type { ReactNode } from 'react'

export type SignInPlatform = 'twitch' | 'youtube' | 'kick'

export interface PlatformSignInButtonProps {
  platform: SignInPlatform
  onClick: () => void
  children: ReactNode
}

export function PlatformSignInButton({ platform, onClick, children }: PlatformSignInButtonProps) {
  return (
    <button type="button" className="lanes-signin" data-p={platform} onClick={onClick}>
      {ICONS[platform]}
      {children}
    </button>
  )
}

const ICONS: Record<SignInPlatform, ReactNode> = {
  twitch: (
    <svg
      className="h-5 w-5 shrink-0"
      viewBox="0 0 24 24"
      xmlns="http://www.w3.org/2000/svg"
      aria-hidden="true"
    >
      <path
        fill="currentColor"
        d="M11.571 4.714h1.715v5.143H11.57zm4.715 0H18v5.143h-1.714zM6 0L1.714 4.286v15.428h5.143V24l4.286-4.286h3.428L22.286 12V0zm14.571 11.143l-3.428 3.428h-3.429l-3 3v-3H6.857V1.714h13.714z"
      />
    </svg>
  ),
  // Official full-color YouTube icon: #FF0033 rect, white triangle
  // (brand.youtube: "The triangle in the full-color red icon must always be
  // white"). On hover the button fills with the brand red, so CSS swaps to
  // the official monochrome-white variant (knocked-out triangle).
  youtube: (
    <svg
      className="h-8 w-8 shrink-0"
      viewBox="0 0 24 24"
      xmlns="http://www.w3.org/2000/svg"
      aria-hidden="true"
    >
      <path
        className="yt-icon-rect"
        d="M23.498 6.186a3.016 3.016 0 0 0-2.122-2.136C19.505 3.545 12 3.545 12 3.545s-7.505 0-9.377.505A3.017 3.017 0 0 0 .502 6.186C0 8.07 0 12 0 12s0 3.93.502 5.814a3.016 3.016 0 0 0 2.122 2.136c1.871.505 9.376.505 9.376.505s7.505 0 9.377-.505a3.015 3.015 0 0 0 2.122-2.136C24 15.93 24 12 24 12s0-3.93-.502-5.814z"
      />
      <path className="yt-icon-tri" d="M9.545 15.568V8.432L15.818 12l-6.273 3.568z" />
    </svg>
  ),
  kick: (
    <svg
      className="h-5 w-5 shrink-0"
      viewBox="0 0 512 512"
      xmlns="http://www.w3.org/2000/svg"
      aria-hidden="true"
    >
      <path
        fill="currentColor"
        d="M37 .036h164.448v113.621h54.71v-56.82h54.731V.036h164.448v170.777h-54.73v56.82h-54.711v56.8h54.71v56.82h54.73V512.03H310.89v-56.82h-54.73v-56.8h-54.711v113.62H37V.036z"
      />
    </svg>
  ),
}
