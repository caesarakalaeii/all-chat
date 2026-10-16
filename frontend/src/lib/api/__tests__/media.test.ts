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

import { afterEach, describe, expect, it, vi } from 'vitest'

const post = vi.fn()
vi.mock('../client', () => ({ apiClient: { post: (...a: unknown[]) => post(...a) } }))

import { MEDIA_AUDIO_TYPES, MEDIA_MAX_BYTES, uploadMedia } from '../media'

class FakeXHR {
  static last: FakeXHR
  method = ''
  url = ''
  headers: Record<string, string> = {}
  body: unknown
  status = 200
  upload: {
    onprogress?: (e: { lengthComputable: boolean; loaded: number; total: number }) => void
  } = {}
  onload?: () => void
  onerror?: () => void
  constructor() {
    FakeXHR.last = this
  }
  open(method: string, url: string) {
    this.method = method
    this.url = url
  }
  setRequestHeader(k: string, v: string) {
    this.headers[k] = v
  }
  send(body: unknown) {
    this.body = body
    this.upload.onprogress?.({ lengthComputable: true, loaded: 5, total: 10 })
    queueMicrotask(() => this.onload?.())
  }
}

const file = (type: string, size = 10) => ({ name: 'airhorn.mp3', type, size }) as unknown as File

afterEach(() => {
  vi.unstubAllGlobals()
  vi.clearAllMocks()
})

describe('uploadMedia', () => {
  it('presigns, PUTs the bytes straight to the upload URL, and returns the public URL', async () => {
    vi.stubGlobal('XMLHttpRequest', FakeXHR)
    post.mockResolvedValue({
      object_key: 'u/1/airhorn.mp3',
      upload_url: 'https://media.allch.at/allchat-media/u/1/airhorn.mp3?X-Amz-Signature=s',
      public_url: 'https://media.allch.at/allchat-media/u/1/airhorn.mp3',
    })
    const progress = vi.fn()
    const f = file('audio/mpeg')

    await expect(uploadMedia(f, MEDIA_AUDIO_TYPES, progress)).resolves.toBe(
      'https://media.allch.at/allchat-media/u/1/airhorn.mp3'
    )
    expect(post).toHaveBeenCalledWith('/api/v1/media/presign', {
      filename: 'airhorn.mp3',
      content_type: 'audio/mpeg',
      size: 10,
    })
    expect(FakeXHR.last.method).toBe('PUT')
    expect(FakeXHR.last.url).toContain('X-Amz-Signature')
    expect(FakeXHR.last.headers['Content-Type']).toBe('audio/mpeg')
    expect(FakeXHR.last.body).toBe(f)
    expect(progress).toHaveBeenCalledWith(0.5)
    expect(progress).toHaveBeenLastCalledWith(1)
  })

  it('refuses a wrong type or an oversized file before any request', async () => {
    await expect(uploadMedia(file('video/mp4'), MEDIA_AUDIO_TYPES)).rejects.toMatchObject({
      reason: 'type',
    })
    await expect(
      uploadMedia(file('audio/mpeg', MEDIA_MAX_BYTES + 1), MEDIA_AUDIO_TYPES)
    ).rejects.toMatchObject({
      reason: 'size',
    })
    expect(post).not.toHaveBeenCalled()
  })

  it('fails when the storage PUT is rejected', async () => {
    class Rejecting extends FakeXHR {
      status = 403
    }
    vi.stubGlobal('XMLHttpRequest', Rejecting)
    post.mockResolvedValue({
      object_key: 'k',
      upload_url: 'https://x/u',
      public_url: 'https://x/p',
    })
    await expect(uploadMedia(file('audio/mpeg'), MEDIA_AUDIO_TYPES)).rejects.toMatchObject({
      reason: 'failed',
    })
  })
})
