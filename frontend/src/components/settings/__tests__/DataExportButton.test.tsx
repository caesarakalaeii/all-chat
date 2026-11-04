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

// @vitest-environment jsdom
import '@testing-library/jest-dom/vitest'
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, waitFor, fireEvent, cleanup } from '@testing-library/react'
import { DataExportButton } from '../DataExportButton'
import { toastManager } from '@/lib/toast'

vi.mock('@/lib/toast', () => ({ toastManager: { add: vi.fn() } }))

describe('DataExportButton', () => {
  let saved: { name: string; blob: Blob } | null
  beforeEach(() => {
    saved = null
    vi.mocked(toastManager.add).mockReset()
    // jsdom has no object URLs and does not navigate on an anchor click, so capture
    // what would have been saved instead.
    const blobs = new Map<string, Blob>()
    vi.stubGlobal('URL', {
      ...URL,
      createObjectURL: (blob: Blob) => {
        blobs.set('blob:export', blob)
        return 'blob:export'
      },
      revokeObjectURL: vi.fn(),
    })
    vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(function (
      this: HTMLAnchorElement
    ) {
      saved = { name: this.download, blob: blobs.get(this.href)! }
    })
  })
  afterEach(() => {
    cleanup()
    vi.unstubAllGlobals()
    vi.restoreAllMocks()
  })

  it('saves what the export endpoint returned as a dated JSON file', async () => {
    const load = vi.fn().mockResolvedValue({ format_version: 2, user: { username: 'tester' } })
    render(<DataExportButton load={load} filename="allchat-data-export" />)

    fireEvent.click(screen.getByRole('button', { name: 'Download my data' }))

    await waitFor(() => expect(saved).not.toBeNull())
    expect(load).toHaveBeenCalledOnce()
    expect(saved!.name).toMatch(/^allchat-data-export-\d{4}-\d{2}-\d{2}\.json$/)
    expect(saved!.blob.type).toBe('application/json')
    expect(JSON.parse(await saved!.blob.text())).toEqual({
      format_version: 2,
      user: { username: 'tester' },
    })
    expect(screen.getByRole('button', { name: 'Download my data' })).toBeEnabled()
  })

  it('reports a failed export and lets the user try again', async () => {
    const load = vi.fn().mockRejectedValue(new Error('500'))
    render(<DataExportButton load={load} filename="allchat-data-export" />)

    fireEvent.click(screen.getByRole('button', { name: 'Download my data' }))

    await waitFor(() =>
      expect(toastManager.add).toHaveBeenCalledWith(
        expect.objectContaining({ title: 'Could not download your data', type: 'error' })
      )
    )
    expect(saved).toBeNull()
    expect(screen.getByRole('button', { name: 'Download my data' })).toBeEnabled()
  })
})
