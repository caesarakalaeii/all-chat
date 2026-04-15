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
import type { NameGradient } from '@/lib/types/message';

// Inline the guard logic under test — mirrors the implementation in page.tsx
function applyGradientParseGuard(user: { name_gradient?: NameGradient | string }): void {
  if (user?.name_gradient && typeof user.name_gradient === 'string') {
    user.name_gradient = JSON.parse(user.name_gradient as unknown as string) as NameGradient;
  }
}

describe('ws.onmessage gradient parse guard', () => {
  it('converts JSON string to NameGradient object', () => {
    const gradient: NameGradient = { type: 'linear', colors: ['#9146ff', '#00b5ad'], angle: 90 };
    const user: { name_gradient?: NameGradient | string } = {
      name_gradient: JSON.stringify(gradient),
    };

    applyGradientParseGuard(user);

    expect(typeof user.name_gradient).toBe('object');
    const parsed = user.name_gradient as NameGradient;
    expect(parsed.type).toBe('linear');
    expect(parsed.colors).toEqual(['#9146ff', '#00b5ad']);
    expect(parsed.angle).toBe(90);
  });

  it('leaves NameGradient object unchanged', () => {
    const gradient: NameGradient = { type: 'linear', colors: ['#ff0000', '#0000ff'], angle: 45 };
    const user: { name_gradient?: NameGradient | string } = {
      name_gradient: gradient,
    };

    applyGradientParseGuard(user);

    expect(user.name_gradient).toBe(gradient); // Same reference — not re-parsed
    expect((user.name_gradient as NameGradient).colors).toEqual(['#ff0000', '#0000ff']);
  });

  it('leaves undefined unchanged', () => {
    const user: { name_gradient?: NameGradient | string } = {};

    applyGradientParseGuard(user);

    expect(user.name_gradient).toBeUndefined();
  });
});
