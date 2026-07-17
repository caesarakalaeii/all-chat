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

/*
 * Attachments come from arbitrary remote media hosts (Discord CDN, Tenor, Giphy),
 * so they are rendered with plain <img>/<video> rather than next/image: image
 * optimization is disabled globally (next.config images.unoptimized) so <Image>
 * would add no value, and its host allowlist would reject these varied hosts.
 * The CSP permits https: for img-src and media-src.
 */
/* eslint-disable @next/next/no-img-element */

'use client'

import clsx from 'clsx'
import { useState } from 'react'

import { useReducedMotion } from '@/hooks/useReducedMotion'
import type { Attachment, ChatMessage } from '@/lib/types/message'

type Variant = 'overlay' | 'compact'

interface MessageAttachmentsProps {
  message: ChatMessage
  variant?: Variant
}

/**
 * Renders a chat message's image/GIF/video attachments as capped thumbnails on
 * the line(s) below the text. Event messages carry no attachments, so callers
 * only need to render this for regular chat.
 *
 * - `overlay` variant is the broadcast surface (out of the WCAG scope): videos
 *   autoplay muted+looping and GIFs animate freely — that is the intended look.
 * - `compact` variant is the in-app monitor/dashboard (in the WCAG scope): every
 *   auto-moving item gets a control per WCAG 2.2.2 — videos never autoplay and
 *   expose native controls, and animated GIFs get a hide/show toggle (defaulting
 *   to hidden when the viewer prefers reduced motion). Static images are inert
 *   and need no control.
 */
export function MessageAttachments({ message, variant = 'overlay' }: MessageAttachmentsProps) {
  // Called once here (not per item) so a message with several attachments makes a
  // single matchMedia subscription.
  const reducedMotion = useReducedMotion()
  const attachments = message.message?.attachments ?? []
  if (attachments.length === 0) {
    return null
  }

  return (
    <div className="mt-1 flex flex-wrap gap-2">
      {attachments.map((attachment, index) => (
        <AttachmentItem
          key={`${attachment.url}-${index}`}
          attachment={attachment}
          variant={variant}
          reducedMotion={reducedMotion}
        />
      ))}
    </div>
  )
}

function AttachmentItem({
  attachment,
  variant,
  reducedMotion,
}: {
  attachment: Attachment
  variant: Variant
  reducedMotion: boolean
}) {
  const [revealed, setRevealed] = useState(false)

  const blurred = Boolean(attachment.spoiler) && !revealed

  const mediaSizing = clsx(
    'block h-auto w-auto rounded-md',
    variant === 'compact' ? 'max-h-32 max-w-48' : 'max-h-64 max-w-full'
  )
  const mediaEffect = clsx(mediaSizing, blurred && 'scale-105 blur-xl')

  // Animated GIFs auto-loop with no native controls, so on the in-scope compact
  // surface they need a hide/show mechanism to satisfy WCAG 2.2.2. Spoilers are
  // handled by their own reveal gate below. The broadcast overlay is out of scope.
  if (variant === 'compact' && !attachment.spoiler && isAnimatedGif(attachment)) {
    return (
      <AnimatedGifControl
        attachment={attachment}
        sizing={mediaSizing}
        startHidden={reducedMotion}
      />
    )
  }

  let media: React.ReactNode
  if (attachment.type === 'video') {
    const autoplay = variant === 'overlay' && !reducedMotion
    media = (
      <video
        src={attachment.url}
        poster={attachment.thumb_url}
        aria-label={videoLabel(attachment)}
        className={mediaEffect}
        muted
        loop
        playsInline
        autoPlay={autoplay}
        controls={!autoplay}
        preload="metadata"
      />
    )
  } else {
    media = (
      <img
        src={attachment.url}
        alt={imageAlt(attachment)}
        width={attachment.width || undefined}
        height={attachment.height || undefined}
        className={mediaEffect}
        loading="lazy"
        decoding="async"
      />
    )
  }

  if (!attachment.spoiler) {
    return <div className="inline-flex overflow-hidden rounded-md">{media}</div>
  }

  return (
    <div className="relative inline-flex overflow-hidden rounded-md">
      {media}
      <button
        type="button"
        onClick={() => setRevealed((value) => !value)}
        aria-pressed={revealed}
        className={clsx(
          'absolute rounded focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-white',
          revealed
            ? 'right-1 top-1 min-h-6 min-w-6 bg-black/60 px-2 py-1 text-xs text-white'
            : 'inset-0 flex items-center justify-center bg-black/40 text-sm font-medium text-white'
        )}
      >
        {revealed ? 'Hide' : 'Spoiler — reveal'}
      </button>
    </div>
  )
}

/**
 * An animated GIF on the in-scope monitor with a WCAG 2.2.2 hide/show control.
 * "Hide" is an accepted mechanism for auto-moving content; defaulting to hidden
 * under reduced motion respects the viewer's preference. drawing a still frame
 * would require canvas readback of cross-origin media, so hiding is the robust
 * choice here.
 */
function AnimatedGifControl({
  attachment,
  sizing,
  startHidden,
}: {
  attachment: Attachment
  sizing: string
  startHidden: boolean
}) {
  const [hidden, setHidden] = useState(startHidden)

  return (
    <div className="relative inline-flex overflow-hidden rounded-md">
      {hidden ? (
        <span className="flex h-16 w-24 items-center justify-center rounded-md bg-surface-2 px-2 text-center text-xs text-text-sub">
          {imageAlt(attachment)}
        </span>
      ) : (
        <img
          src={attachment.url}
          alt={imageAlt(attachment)}
          width={attachment.width || undefined}
          height={attachment.height || undefined}
          className={sizing}
          loading="lazy"
          decoding="async"
        />
      )}
      <button
        type="button"
        onClick={() => setHidden((value) => !value)}
        aria-pressed={!hidden}
        className="absolute right-1 top-1 min-h-6 min-w-6 rounded bg-black/60 px-2 py-1 text-xs text-white focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-white"
      >
        {hidden ? 'Show GIF' : 'Hide GIF'}
      </button>
    </div>
  )
}

function isAnimatedGif(attachment: Attachment): boolean {
  return attachment.type === 'image' && Boolean(attachment.content_type?.includes('gif'))
}

function mediaKind(attachment: Attachment): string {
  if (attachment.type === 'video') {
    return 'video clip'
  }
  if (attachment.content_type?.includes('gif')) {
    return 'GIF'
  }
  return 'image'
}

function imageAlt(attachment: Attachment): string {
  return attachment.filename?.trim() || `Shared ${mediaKind(attachment)}`
}

function videoLabel(attachment: Attachment): string {
  return attachment.filename?.trim() || 'Shared video clip'
}
