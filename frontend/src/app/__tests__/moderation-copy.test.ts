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

/**
 * Moderation-capability copy honesty gate.
 *
 * The moderation-service (ADR-0017) does NOT support the same actions on every
 * platform: Twitch, Kick and Discord do delete/timeout/ban/unban, YouTube is
 * ban-only, TikTok is unsupported. Marketing and docs copy previously claimed a
 * blanket "delete, timeout and ban across Twitch, YouTube, Kick and Discord",
 * which is false. This test locks the copy so that overstatement cannot silently
 * return.
 *
 * It also guards the opposite failure, which is what actually happened next:
 * Kick's single-message delete DOES exist (`moderation:chat_message:manage`,
 * ADR-0048), so copy claiming Kick cannot delete is now an UNDERstatement, and
 * three surfaces carried it. Honesty runs both ways — a capability the product
 * has and denies is as wrong as one it claims and lacks.
 *
 * Source of truth: services/moderation-service/README.md capability matrix.
 * Parses source as text (repo convention, see token-contrast.test.ts).
 */

import { describe, expect, it } from 'vitest'
import { readFileSync } from 'node:fs'
import { join } from 'node:path'

const read = (rel: string) => readFileSync(join(__dirname, '..', rel), 'utf-8')

describe('moderation copy is honest per-platform (ADR-0017)', () => {
  const upgrade = read('upgrade/page.tsx')
  const onboarding = readFileSync(
    join(__dirname, '..', '..', 'components', 'onboarding', 'OnboardingChecklist.tsx'),
    'utf-8'
  )
  const docs = read('docs/page.tsx')

  it('no surface makes the false blanket "ban across Twitch, YouTube, Kick, and Discord" claim', () => {
    for (const src of [upgrade, onboarding, docs]) {
      expect(src).not.toMatch(/across Twitch, YouTube, Kick,? and Discord/)
    }
  })

  it('the upgrade page states YouTube is ban-only and TikTok is unsupported', () => {
    expect(upgrade).toMatch(/ban on YouTube/i)
    expect(upgrade).toMatch(/TikTok has no moderation API/i)
  })

  it('the docs page spells out the real per-platform limits', () => {
    expect(docs).toMatch(/Twitch, Kick and Discord do delete, timeout, ban and unban/)
    expect(docs).toMatch(/YouTube is\s+ban-only/)
    expect(docs).toMatch(/TikTok has no\s+moderation API/)
  })

  // Kick's single-message delete exists (ADR-0048). No surface may keep saying otherwise —
  // understating a capability sends streamers to a competitor for something All-Chat does.
  it('no surface still claims Kick cannot delete single messages', () => {
    for (const src of [upgrade, onboarding, docs]) {
      expect(src).not.toMatch(/Kick has no single-message/)
      expect(src).not.toMatch(/Kick does timeout/)
    }
  })
})
