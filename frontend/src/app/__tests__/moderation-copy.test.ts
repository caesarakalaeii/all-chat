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
 * platform: Twitch, Kick and Discord do delete/timeout/ban/unban, YouTube does
 * timeout and ban (no delete, no unban — neither has a usable id there, see
 * moderation-service/clients/youtube.go), TikTok is unsupported. Marketing and
 * docs copy previously claimed a blanket "delete, timeout and ban across Twitch,
 * YouTube, Kick and Discord", which is false. This test locks the copy so that
 * overstatement cannot silently return.
 *
 * It also guards the opposite failure, which is what actually happened next:
 * Kick's single-message delete DOES exist (`moderation:chat_message:manage`,
 * ADR-0048), so copy claiming Kick cannot delete is now an UNDERstatement, and
 * three surfaces carried it. Honesty runs both ways — a capability the product
 * has and denies is as wrong as one it claims and lacks.
 *
 * Source of truth: services/moderation-service/README.md capability matrix.
 * Parses source as text (repo convention, see token-contrast.test.ts).
 *
 * The upgrade, onboarding and docs copy now lives in the i18n catalog (#799),
 * so this gate reads those namespace files rather than the render sites. It
 * must follow the copy: a claim is equally false wherever it is stored, and a
 * gate pointed at a file the words have left passes for the wrong reason.
 */

import { describe, expect, it } from 'vitest'
import { readFileSync } from 'node:fs'
import { join } from 'node:path'

const readCatalog = (namespace: string) =>
  readFileSync(join(__dirname, '..', '..', 'lib', 'i18n', 'messages', 'en', namespace), 'utf-8')

describe('moderation copy is honest per-platform (ADR-0017)', () => {
  const upgrade = readCatalog('marketing.ts')
  const onboarding = readCatalog('onboarding.ts')
  const docs = readCatalog('docs.ts')

  it('no surface makes the false blanket "ban across Twitch, YouTube, Kick, and Discord" claim', () => {
    for (const src of [upgrade, onboarding, docs]) {
      expect(src).not.toMatch(/across Twitch, YouTube, Kick,? and Discord/)
    }
  })

  it('the upgrade page names YouTube’s narrower set and TikTok as unsupported', () => {
    expect(upgrade).toMatch(/timeout and ban on YouTube/i)
    expect(upgrade).toMatch(/TikTok has no moderation API/i)
  })

  it('the docs page spells out the real per-platform limits', () => {
    // \s+ rather than a literal space: the claim used to straddle a JSX line
    // wrap and now sits on one catalog line, and the gate should not care which.
    expect(docs).toMatch(/Twitch, Kick and Discord do delete, timeout, ban and unban/)
    expect(docs).toMatch(/YouTube does\s+timeout and ban/)
    expect(docs).toMatch(/TikTok has no\s+moderation API/)
  })

  // YouTube gained timeout, so nothing may still call it ban-only — and no surface may promise
  // delete or unban there, which remain impossible for want of a usable id.
  it('no surface calls YouTube ban-only, or promises delete/unban there', () => {
    for (const src of [upgrade, onboarding, docs]) {
      expect(src).not.toMatch(/YouTube is\s+ban-only/)
      expect(src).not.toMatch(/delete[^.]{0,40}on YouTube/)
    }
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
