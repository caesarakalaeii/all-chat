/**
 * This file is part of All-Chat.
 * Copyright (C) 2026 caesarakalaeii
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published
 * by the Free Software Foundation, either version 3 of the License, or
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

import { useEffect } from 'react'

/**
 * Scroll-activated section reveals (feedback round 1, replacing the old
 * scroll-snap anchors). Observes every [data-reveal] element inside the
 * scope element and sets data-revealed='true' once when it enters the
 * viewport; the visual transition itself is pure CSS in globals.css, so
 * prefers-reduced-motion is already covered by the global transition gate.
 *
 * One-way by design: the observer unregisters each element after it fires,
 * so scrolling back up never hides a section the visitor has already seen.
 */
export function useReveal(scopeRef: React.RefObject<HTMLElement | null>): void {
  useEffect(() => {
    const scope = scopeRef.current
    if (scope === null) return

    const targets = scope.querySelectorAll<HTMLElement>('[data-reveal]')
    if (targets.length === 0) return

    if (typeof IntersectionObserver === 'undefined') {
      // Very old browser: reveal everything rather than shipping an
      // invisible page. The transition gate makes this a no-op visually.
      targets.forEach((el) => el.setAttribute('data-revealed', 'true'))
      return
    }

    const observer = new IntersectionObserver(
      (entries) => {
        for (const entry of entries) {
          if (!entry.isIntersecting) continue
          entry.target.setAttribute('data-revealed', 'true')
          observer.unobserve(entry.target)
        }
      },
      // A section is "entering" once a sixth of it is on screen: earlier
      // (smaller thresholds) fires while the section is still mostly
      // off-screen and the visitor misses the entrance entirely.
      { threshold: 0.15 }
    )

    targets.forEach((el) => observer.observe(el))
    return () => observer.disconnect()
  }, [scopeRef])
}
