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
 * In-memory (non-persisted) viewer token storage — SECURITY (audit H3).
 *
 * The streamer/admin token paths were removed in the H3 cookie-auth migration
 * (the streamer access token now lives only in an httpOnly cookie). This store
 * now holds ONLY the viewer access token, which the viewer flow
 * (`viewer-auth-store.ts`, `api/viewer.ts`) still mirrors here so the viewer
 * API client can read it without touching localStorage. The viewer token is
 * also still written to localStorage for legacy readers; full removal is
 * deferred to a later viewer cookie-auth migration.
 *
 * Storing the viewer token in a module-level variable means an XSS payload
 * cannot read it from storage: it must be actively running while the token is
 * in memory, and the token is irrecoverably lost when the tab is closed or
 * refreshed.
 */

let viewerAccessToken: string | null = null

export const inMemoryTokens = {
  // ---- Viewer access token — mirrored. ----
  getViewerAccessToken(): string | null {
    return viewerAccessToken
  },
  setViewerAccessToken(token: string | null): void {
    viewerAccessToken = token
  },
}
