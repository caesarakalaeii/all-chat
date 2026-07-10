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
 * rewriteThemeFontImports rewrites Google Fonts `@import url('https://fonts.googleapis.com/css2?...')`
 * stylesheet references in bundled-theme CSS to the same-origin `/font-proxy/css`
 * proxy (audit #11).
 *
 * The proxy lives under `/font-proxy/*` (NOT `/api/*`): the ingress routes
 * `/api/*` to the api-gateway and next.config rewrites `/api/*` there too, so a
 * proxy under `/api/fonts` was shadowed and 404'd in production.
 *
 * Why: the overlay CSP (`style-src 'self' 'unsafe-inline'`) blocks a direct
 * `@import` from fonts.googleapis.com, so themed overlays silently fell back to
 * the system font. Whitelisting Google in the CSP is NOT acceptable — the proxy
 * exists specifically so viewer IPs never hit Google directly (DSGVO / "Google
 * Fonts" ruling). The proxy validates each font family against an allowlist and
 * additionally rewrites the gstatic font binaries to `/font-proxy/file`, so after
 * this rewrite no overlay viewer ever connects to a Google host.
 *
 * Only the css2 stylesheet URL is rewritten; the query string (family axes +
 * display) is preserved verbatim so the proxy receives the exact same request.
 */
export function rewriteThemeFontImports(css: string): string {
  if (!css) return css
  return css.replace(/https:\/\/fonts\.googleapis\.com\/css2\?([^'")\s]+)/g, '/font-proxy/css?$1')
}
