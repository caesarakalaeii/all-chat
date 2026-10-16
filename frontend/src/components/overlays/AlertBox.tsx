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

'use client'

/**
 * The alert-box renderer for alerts overlays (ADR-0068). `useAlertQueue` owns
 * the queue the overlay page feeds from useOverlayStream's onAlert;
 * `AlertPlayer` shows the current alert — text, image/GIF, sound — and reports
 * it done once BOTH its duration and its sound have finished.
 *
 * On-stream art inside an OBS browser source: no background, no app chrome,
 * and nothing at all while the queue is empty. The text sits in a polite live
 * region so it is in the accessibility tree; the sound is decorative (alerts
 * are visual events, docs/ACCESSIBILITY.md).
 */

import { useCallback, useEffect, useRef, useState } from 'react'

import { AlertQueue, alertDurationMs, alertText, type AlertFrame } from './alertQueue'

// A sound that never reports its end (stalled stream) must not freeze the queue.
const MAX_SOUND_OVERRUN_MS = 60_000

export function useAlertQueue() {
  const queueRef = useRef<AlertQueue | null>(null)
  queueRef.current ??= new AlertQueue()
  const [current, setCurrent] = useState<AlertFrame | null>(null)
  const currentRef = useRef<AlertFrame | null>(null)

  const show = useCallback((next: AlertFrame | null) => {
    currentRef.current = next
    setCurrent(next)
  }, [])

  const enqueue = useCallback(
    (alert: AlertFrame) => {
      if (!queueRef.current!.push(alert)) return
      if (!currentRef.current) show(queueRef.current!.shift() ?? null)
    },
    [show]
  )

  const finish = useCallback(
    (alertId: string) => {
      if (currentRef.current?.alert_id !== alertId) return
      show(queueRef.current!.shift() ?? null)
    },
    [show]
  )

  return { current, enqueue, finish }
}

export function AlertPlayer({
  alert,
  onDone,
}: {
  alert: AlertFrame | null
  onDone: (alertId: string) => void
}) {
  const onDoneRef = useRef(onDone)
  useEffect(() => {
    onDoneRef.current = onDone
  })

  useEffect(() => {
    if (!alert) return
    let timerDone = false
    let soundDone = !alert.profile?.sound_url
    let finished = false
    const finish = () => {
      if (finished || !timerDone || !soundDone) return
      finished = true
      onDoneRef.current(alert.alert_id)
    }
    const timers = [
      setTimeout(() => {
        timerDone = true
        finish()
      }, alertDurationMs(alert)),
    ]
    let audio: HTMLAudioElement | null = null
    if (alert.profile?.sound_url) {
      audio = new Audio(alert.profile.sound_url)
      audio.volume = Math.min(1, Math.max(0, alert.profile.volume / 100))
      const soundEnded = () => {
        soundDone = true
        finish()
      }
      audio.addEventListener('ended', soundEnded)
      audio.addEventListener('error', soundEnded)
      // Autoplay blocked (a normal browser tab, not OBS) counts as "ended".
      audio.play().catch(soundEnded)
      timers.push(setTimeout(soundEnded, alertDurationMs(alert) + MAX_SOUND_OVERRUN_MS))
    }
    return () => {
      timers.forEach(clearTimeout)
      audio?.pause()
    }
  }, [alert])

  if (!alert) return null

  // The profile image (GIF) wins; the sender's avatar fills in without one.
  const image = alert.profile?.image_url
  const avatar = !image && alert.user.avatar_url
  return (
    <div
      key={alert.alert_id}
      className="flex min-h-screen w-full items-center justify-center bg-transparent p-4"
      data-alert-id={alert.alert_id}
      data-event-type={alert.event_type}
    >
      {/* Hide scrollbars and keep the OBS canvas transparent, like the chat feed. */}
      <style>{`body { overflow: hidden !important; background: transparent !important; }`}</style>
      <div className="flex max-w-full animate-in flex-col items-center gap-3 text-center zoom-in-95 fade-in">
        {image && (
          // Decorative: the text below carries the alert. eslint: next/image
          // cannot load arbitrary streamer-hosted GIFs.
          // eslint-disable-next-line @next/next/no-img-element
          <img src={image} alt="" className="max-h-[60vh] max-w-full object-contain" />
        )}
        {avatar && (
          // eslint-disable-next-line @next/next/no-img-element
          <img src={avatar} alt="" className="size-20 rounded-full object-cover" />
        )}
        {/* On-stream text: white with a shadow reads on any stream background. */}
        <p
          role="status"
          aria-live="polite"
          className="text-3xl font-bold text-white drop-shadow-lg"
        >
          {alertText(alert)}
        </p>
      </div>
    </div>
  )
}
