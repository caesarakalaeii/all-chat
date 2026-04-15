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

import { describe, it, expect } from 'vitest';
import { getUsernameSpanProps } from '@/lib/utils/usernameSpan';

describe('Overlay gradient render', () => {
  it('renders flat color when name_gradient absent', () => {
    const props = getUsernameSpanProps({ color: '#ff0000' });
    expect(props.className).toBe('font-semibold text-sm');
    expect(props.style).toEqual({ color: '#ff0000' });
    expect(props.style).not.toHaveProperty('backgroundImage');
  });

  it('applies bg-clip-text text-transparent when name_gradient present', () => {
    const props = getUsernameSpanProps({
      color: '#ff0000',
      name_gradient: { type: 'linear', colors: ['#9146ff', '#00b5ad'], angle: 90 },
    });
    expect(props.className).toContain('bg-clip-text');
    expect(props.className).toContain('text-transparent');
  });

  it('backgroundImage contains linear-gradient when name_gradient present', () => {
    const props = getUsernameSpanProps({
      color: '#ff0000',
      name_gradient: { type: 'linear', colors: ['#9146ff', '#00b5ad'], angle: 90 },
    });
    expect(props.style).toHaveProperty('backgroundImage');
    expect((props.style as { backgroundImage: string }).backgroundImage).toContain('linear-gradient');
  });
});
