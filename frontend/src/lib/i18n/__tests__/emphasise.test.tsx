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

// @vitest-environment jsdom
import '@testing-library/jest-dom/vitest'
import { cleanup, render, screen } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { emphasise, interpolateElements } from '@/lib/i18n/emphasise'

afterEach(cleanup)

describe('emphasise', () => {
  it('wraps the emphasised run and leaves the text either side of it alone', () => {
    render(
      <p data-testid="sentence">
        {emphasise('Bubble palettes are a Premium feature.', 'Premium', (run) => (
          <strong>{run}</strong>
        ))}
      </p>
    )

    // The rendered sentence must read exactly as the pre-migration literal did:
    // migrating copy into the catalog is not licence to change what is on screen.
    expect(screen.getByTestId('sentence')).toHaveTextContent(
      'Bubble palettes are a Premium feature.'
    )
    expect(screen.getByText('Premium').tagName).toBe('STRONG')
  })

  it('renders the sentence plainly when the emphasis is not in it', () => {
    // A drifted emphasis key must not cost the reader the sentence. Losing the
    // bold is cosmetic; losing the copy is a blank paragraph in production.
    render(
      <p data-testid="sentence">
        {emphasise('No emphasis to find here.', 'Premium', (run) => (
          <strong>{run}</strong>
        ))}
      </p>
    )

    expect(screen.getByTestId('sentence')).toHaveTextContent('No emphasis to find here.')
    expect(screen.queryByText('Premium')).toBeNull()
  })

  it('wraps only the first occurrence when the run repeats', () => {
    // Emphasising every occurrence would bold words the pre-migration markup
    // left plain, which is a rendered-output change.
    render(
      <p data-testid="sentence">
        {emphasise('Premium is Premium.', 'Premium', (run) => (
          <strong>{run}</strong>
        ))}
      </p>
    )

    expect(screen.getByTestId('sentence')).toHaveTextContent('Premium is Premium.')
    expect(screen.getAllByText('Premium')).toHaveLength(1)
  })
})

describe('interpolateElements', () => {
  it('wraps each named placeholder in its own element', () => {
    render(
      <p data-testid="sentence">
        {interpolateElements('Join from chat ({voteCommand} or just {bareVote}) to play.', {
          voteCommand: <code>!vote 2</code>,
          bareVote: <code>2</code>,
        })}
      </p>
    )

    expect(screen.getByTestId('sentence')).toHaveTextContent(
      'Join from chat (!vote 2 or just 2) to play.'
    )
    expect(screen.getByText('!vote 2').tagName).toBe('CODE')
    expect(screen.getByText('2').tagName).toBe('CODE')
  })

  it('keeps a placeholder whose value is a substring of another distinct', () => {
    // `2` occurs inside `!vote 2`. Substituting the values first and searching
    // for them afterwards would wrap the wrong run, which is why the split is
    // done on the unresolved template instead.
    render(
      <p data-testid="sentence">
        {interpolateElements('{long} then {short}', {
          long: <code>!vote 2</code>,
          short: <em>2</em>,
        })}
      </p>
    )

    expect(screen.getByTestId('sentence')).toHaveTextContent('!vote 2 then 2')
    expect(screen.getByText('!vote 2').tagName).toBe('CODE')
    expect(screen.getByText('2').tagName).toBe('EM')
  })

  it('leaves an unknown placeholder in place rather than dropping the run', () => {
    // Mirrors t()'s never-throws rule: a drifted key must cost the reader as
    // little as possible, and a visible {brace} is a louder bug report than a
    // silently shortened sentence.
    render(
      <p data-testid="sentence">
        {interpolateElements('a {known} b {missing} c', { known: <code>ok</code> })}
      </p>
    )

    expect(screen.getByTestId('sentence')).toHaveTextContent('a ok b {missing} c')
  })
  it('renders a placeholder used twice without a duplicate React key', () => {
    // /docs/api names the same wire field twice in one sentence
    // ('... a {status} frame per configured source ... {status} data:'), which
    // is copy a translator must be able to reorder. Keying each substitution by
    // placeholder name alone makes the second one collide with the first, and
    // React reports that as an error rather than throwing, so a rendered-text
    // assertion alone would not catch it.
    const errors = vi.spyOn(console, 'error').mockImplementation(() => {})

    render(
      <p data-testid="sentence">{interpolateElements('a {x} b {x} c', { x: <code>X</code> })}</p>
    )

    expect(screen.getByTestId('sentence')).toHaveTextContent('a X b X c')
    expect(errors).not.toHaveBeenCalled()
    errors.mockRestore()
  })
})
