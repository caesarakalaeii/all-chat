/**
 * This file is part of All-Chat.
 * Copyright (C) 2026 caesarakalaeii
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as
 * published by the Free Software Foundation, either version 3 of the
 * License, or (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program. If not, see <https://www.gnu.org/licenses/>.
 */

import { describe, expect, it } from 'vitest';
import {
  SIGNING_IDENTITY,
  VIEWER_IDENTITY,
  browserVersionFromUserAgent,
  identityIsConsistent,
  type SignerIdentity
} from './identity.js';
import { imFetchParams } from '../api.js';

describe('shipped identities', () => {
  it('are internally consistent', () => {
    expect(identityIsConsistent(SIGNING_IDENTITY)).toBe(true);
    expect(identityIsConsistent(VIEWER_IDENTITY)).toBe(true);
  });

  it('rejects a deliberately inconsistent identity', () => {
    // The standing bug class: params claiming a platform the UA contradicts.
    const macParamsOnLinuxUa: SignerIdentity = {
      ...VIEWER_IDENTITY,
      browserPlatform: 'MacIntel',
      os: 'mac'
    };
    expect(identityIsConsistent(macParamsOnLinuxUa)).toBe(false);

    const badScreen: SignerIdentity = { ...SIGNING_IDENTITY, screenWidth: 0 };
    expect(identityIsConsistent(badScreen)).toBe(false);

    const macUaClaimingLinux: SignerIdentity = {
      ...SIGNING_IDENTITY,
      os: 'linux'
    };
    expect(identityIsConsistent(macUaClaimingLinux)).toBe(false);
  });
});

describe('browserVersionFromUserAgent', () => {
  it('extracts the Chrome segment for Chromium UAs', () => {
    expect(browserVersionFromUserAgent(VIEWER_IDENTITY.userAgent)).toBe('144.0.0.0');
  });

  it('extracts the Version segment for Safari UAs', () => {
    expect(browserVersionFromUserAgent(SIGNING_IDENTITY.userAgent)).toBe('18.6');
  });

  it('returns the empty string for a UA with neither segment', () => {
    expect(browserVersionFromUserAgent('Mozilla/5.0 (compatible; bot)')).toBe('');
  });
});

describe('imFetchParams describes its identity', () => {
  it('derives the self-describing params from the signing identity', () => {
    const params = imFetchParams('42', undefined, SIGNING_IDENTITY);
    expect(params.get('browser_version')).toBe('18.6');
    expect(params.get('browser_platform')).toBe('MacIntel');
    expect(params.get('os')).toBe('mac');
    expect(params.get('screen_width')).toBe('1920');
    expect(params.get('screen_height')).toBe('1080');
  });

  it('derives the self-describing params from the viewer identity — no MacIntel on a Linux UA', () => {
    const params = imFetchParams('42', undefined, VIEWER_IDENTITY);
    expect(params.get('browser_version')).toBe('144.0.0.0');
    expect(params.get('browser_platform')).toBe('Linux x86_64');
    expect(params.get('os')).toBe('linux');
  });
});
