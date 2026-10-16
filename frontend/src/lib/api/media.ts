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
 * Alert media uploads (media-service, ADR-0068): presign through the gateway,
 * then PUT the bytes straight to MinIO (media.allch.at) — they never pass
 * through a Go pod. The returned public_url is what a profile stores.
 */

import { apiClient } from './client'

/** media-service's allow-list and cap (services/media-service/README.md). */
export const MEDIA_IMAGE_TYPES = ['image/png', 'image/jpeg', 'image/gif', 'image/webp'] as const
export const MEDIA_AUDIO_TYPES = ['audio/mpeg', 'audio/ogg', 'audio/wav', 'audio/webm'] as const
export const MEDIA_MAX_BYTES = 10 * 1024 * 1024

interface PresignResponse {
  object_key: string
  upload_url: string
  public_url: string
}

/** Why an upload was refused before any request, or 'failed' after one. */
export type MediaUploadError = 'type' | 'size' | 'failed'

export class MediaUploadRejected extends Error {
  constructor(public reason: MediaUploadError) {
    super(reason)
    this.name = 'MediaUploadRejected'
  }
}

/**
 * Upload one file and resolve to its public URL. `onProgress` gets 0-1.
 * Client-side checks mirror media-service so a bad file fails without a
 * registry row; the server re-checks.
 */
export async function uploadMedia(
  file: File,
  accept: readonly string[],
  onProgress?: (fraction: number) => void
): Promise<string> {
  if (!accept.includes(file.type)) throw new MediaUploadRejected('type')
  if (file.size <= 0 || file.size > MEDIA_MAX_BYTES) throw new MediaUploadRejected('size')

  const presigned = await apiClient.post<PresignResponse>('/api/v1/media/presign', {
    filename: file.name,
    content_type: file.type,
    size: file.size,
  })

  // XHR, not fetch: fetch has no upload progress.
  await new Promise<void>((resolve, reject) => {
    const xhr = new XMLHttpRequest()
    xhr.open('PUT', presigned.upload_url)
    xhr.setRequestHeader('Content-Type', file.type)
    xhr.upload.onprogress = (e) => {
      if (e.lengthComputable) onProgress?.(e.loaded / e.total)
    }
    xhr.onload = () =>
      xhr.status >= 200 && xhr.status < 300 ? resolve() : reject(new MediaUploadRejected('failed'))
    xhr.onerror = () => reject(new MediaUploadRejected('failed'))
    xhr.send(file)
  })
  onProgress?.(1)
  return presigned.public_url
}
