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

import { useEffect, useState } from 'react'

/**
 * Returns the list of SpeechSynthesisVoice objects available in the current
 * browser. Handles the Chromium quirk where getVoices() returns [] on first
 * call and later fires 'voiceschanged' when voices populate (Pitfall 1 in
 * 13-RESEARCH.md).
 */
export function useBrowserVoices(): SpeechSynthesisVoice[] {
  const [voices, setVoices] = useState<SpeechSynthesisVoice[]>([])

  useEffect(() => {
    if (typeof window === 'undefined' || !window.speechSynthesis) return

    const update = (): void => {
      setVoices(window.speechSynthesis.getVoices())
    }

    update() // synchronous first call — works in Firefox/Safari
    window.speechSynthesis.addEventListener('voiceschanged', update)
    return () => {
      window.speechSynthesis.removeEventListener('voiceschanged', update)
    }
  }, [])

  return voices
}
