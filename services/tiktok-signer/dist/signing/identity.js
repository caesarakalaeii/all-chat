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
/** Safari on macOS: the X-Bogus signing session's identity (/v1/sign-url path). */
export const SIGNING_IDENTITY = {
    // Everything TikTok sees (User-Agent, browser_* query params, the
    // fingerprint X-Gnarly encodes) must agree with this, so it is one object
    // and callers read it from /identity instead of guessing.
    userAgent: 'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.6 Safari/605.1.15',
    browserPlatform: 'MacIntel',
    os: 'mac',
    screenWidth: 1920,
    screenHeight: 1080
};
/** Linux Chrome/144: the viewer pages, their captures and the WS session lease. */
export const VIEWER_IDENTITY = {
    userAgent: 'Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/144.0.0.0 Safari/537.36',
    browserPlatform: 'Linux x86_64',
    os: 'linux',
    screenWidth: 1920,
    screenHeight: 1080
};
/** The UA the viewer pages run with; must match what the listener pins. */
export const VIEWER_UA = VIEWER_IDENTITY.userAgent;
/** OS family each known browser platform token implies. */
const OS_BY_PLATFORM = {
    MacIntel: 'mac',
    'Linux x86_64': 'linux',
    'Linux armv7l': 'linux',
    'Linux aarch64': 'linux',
    Win32: 'windows'
};
/**
 * Whether an identity describes one browser: the os token and the platform
 * token agree with the UA's OS section (between the first and second
 * semicolon — not anywhere in the UA, which would let a stray mention
 * pass), and the screen dimensions are positive. Exported so tests can
 * assert the negative — an inconsistent identity is the standing bug class
 * this module exists to prevent (the hardcoded MacIntel/mac that
 * contradicted a Linux UA).
 */
export function identityIsConsistent(identity) {
    const { userAgent, browserPlatform, os, screenWidth, screenHeight } = identity;
    const osSection = (userAgent.split(';')[1] ?? '').toLowerCase();
    if (!osSection.includes(os.toLowerCase()))
        return false;
    // The platform token (e.g. 'MacIntel') never appears verbatim in a UA
    // ('Macintosh' does); what must agree is the OS family it implies.
    const impliedOs = OS_BY_PLATFORM[browserPlatform];
    if (impliedOs === undefined)
        return false;
    if (!osSection.includes(impliedOs))
        return false;
    return screenWidth > 0 && screenHeight > 0;
}
/**
 * The browser_version param im/fetch carries: the version segment of the UA
 * (Chrome/x for Chromium UAs, Version/x for Safari). A UA without either
 * segment describes no browser version TikTok recognizes — empty string.
 */
export function browserVersionFromUserAgent(ua) {
    const chrome = ua.match(/Chrome\/([\d.]+)/);
    if (chrome)
        return chrome[1];
    const safari = ua.match(/Version\/([\d.]+)/);
    if (safari)
        return safari[1];
    return '';
}
//# sourceMappingURL=identity.js.map