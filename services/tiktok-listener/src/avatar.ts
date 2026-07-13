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
 * TikTok profile-picture URL selection.
 *
 * The v3 `User` proto carries a viewer's avatar as up to three image models
 * (`avatarThumb`, `avatarMedium`, `avatarLarge`), each exposing a `urlList` of
 * several CDN URLs for the *same* picture in different formats and sizes —
 * including a low-resolution "shrink" placeholder that TikTok reuses across
 * many accounts.
 *
 * Blindly taking `avatarThumb.urlList[0]` (as this service briefly did after
 * the 2.4.0 upgrade) is unreliable: that first entry is frequently the generic
 * shrink placeholder, so every viewer collapsed to the *same* avatar instead of
 * their own. This mirrors the connector library's own `getPreferredPictureFormat`
 * heuristic — prefer a real 100x100 WebP/JPEG, avoid the shrink placeholder, and
 * fall back to the raw first entry only as a last resort.
 */

/** Minimal structural view of a TikTok avatar image model. */
export interface AvatarImageModel {
  urlList?: string[];
}

/** Minimal structural view of the avatar-bearing fields of a TikTok user. */
export interface AvatarUser {
  avatarThumb?: AvatarImageModel;
  avatarMedium?: AvatarImageModel;
  avatarLarge?: AvatarImageModel;
}

/**
 * Picks the most broadly loadable profile-picture URL from a single avatar
 * image model, or an empty string when the model carries no usable URL.
 */
export function pickAvatarUrl(image: AvatarImageModel | undefined): string {
  const urls = image?.urlList?.filter(
    (url): url is string => typeof url === 'string' && url.length > 0
  );
  if (!urls || urls.length === 0) {
    return '';
  }

  return (
    urls.find((url) => url.includes('100x100') && url.includes('.webp')) ||
    urls.find((url) => url.includes('100x100') && url.includes('.jpeg')) ||
    urls.find((url) => !url.includes('shrink')) ||
    // Every remaining URL is a "shrink" placeholder (shared across accounts).
    // Return '' rather than the placeholder so the caller falls through to the
    // next image model, and ultimately to the viewer's initial — never the
    // shared image that caused this regression in the first place.
    ''
  );
}

/**
 * Reads a viewer's profile-picture URL from the v3 avatar image models,
 * falling through thumb → medium → large so a viewer still gets a real,
 * per-user avatar when the smallest model is absent or only carries the
 * shared shrink placeholder. Returns an empty string when none is available,
 * which the overlay renders as the viewer's initial.
 */
export function tiktokAvatarUrl(user: AvatarUser | undefined): string {
  return (
    pickAvatarUrl(user?.avatarThumb) ||
    pickAvatarUrl(user?.avatarMedium) ||
    pickAvatarUrl(user?.avatarLarge)
  );
}
