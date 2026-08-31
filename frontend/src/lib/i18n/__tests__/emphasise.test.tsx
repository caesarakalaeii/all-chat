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
import { afterEach, describe, expect, it } from 'vitest'

import { emphasise } from '@/lib/i18n/emphasise'

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
