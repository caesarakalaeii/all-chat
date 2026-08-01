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

import type { CSSProperties } from 'react';
import { NameGradient } from '@/lib/types/message';
import { buildGradientCSS } from '@/lib/utils/gradient';
import { resolveUsernameColor, type UsernameColorUser } from '@/lib/utils/usernameColor';

/**
 * User info subset needed to compute username span rendering props.
 */
export interface UsernameSpanUser extends UsernameColorUser {
  name_gradient?: NameGradient;
}

/**
 * Returns the className and style props for a username span element.
 *
 * When name_gradient is present, returns bg-clip-text text-transparent with
 * a backgroundImage CSS property (pure CSS gradient, no JavaScript animation).
 * When name_gradient is absent, falls back to inline color style.
 */
export function getUsernameSpanProps(user: UsernameSpanUser): {
  className: string;
  style: CSSProperties;
} {
  if (user.name_gradient) {
    return {
      className: 'font-semibold text-sm bg-clip-text text-transparent',
      style: { backgroundImage: buildGradientCSS(user.name_gradient) },
    };
  }
  return {
    className: 'font-semibold text-sm',
    // Shared resolver so this can't drift from the overlay render sites (ADR-0047).
    style: { color: resolveUsernameColor(user) },
  };
}
