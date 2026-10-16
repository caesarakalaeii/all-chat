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
import { readFileSync } from 'node:fs'
import path from 'node:path'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'

const sendTestAlert = vi.fn()
vi.mock('@/lib/api/overlays', () => ({
  overlaysApi: { sendTestAlert: (...args: unknown[]) => sendTestAlert(...args) },
}))
const toast = vi.fn()
vi.mock('@/lib/toast', () => ({ toastManager: { add: (t: unknown) => toast(t) } }))

import { AlertTestPanel } from '@/components/overlays/AlertTestPanel'
import { ALERT_EVENT_TYPES } from '@/lib/types/alert'

afterEach(() => {
  cleanup()
  vi.clearAllMocks()
})

describe('AlertTestPanel', () => {
  it('sends a test alert of the default event type and confirms it', async () => {
    sendTestAlert.mockResolvedValue({ alert_id: 'a1' })
    render(<AlertTestPanel overlayId="ov1" />)

    fireEvent.click(screen.getByRole('button', { name: 'Send test alert' }))

    await waitFor(() => expect(sendTestAlert).toHaveBeenCalledWith('ov1', 'subscription'))
    await waitFor(() =>
      expect(toast).toHaveBeenCalledWith(expect.objectContaining({ type: 'success' }))
    )
  })

  it('reports a failed send', async () => {
    sendTestAlert.mockRejectedValue(new Error('boom'))
    render(<AlertTestPanel overlayId="ov1" />)

    fireEvent.click(screen.getByRole('button', { name: 'Send test alert' }))

    await waitFor(() =>
      expect(toast).toHaveBeenCalledWith(expect.objectContaining({ type: 'error' }))
    )
  })

  it('offers exactly the event types shared/alerts knows', () => {
    // The server owns the list (shared/alerts eventDefs) and 400s an unknown
    // type; this keeps the picker from drifting out of sync with it.
    const goSource = readFileSync(
      path.join(__dirname, '../../../../../shared/alerts/alerts.go'),
      'utf8'
    )
    const goTypes = [...goSource.matchAll(/^\s*\{"([a-z_]+)", "[a-z]+", "/gm)].map((m) => m[1])
    expect(goTypes.length).toBeGreaterThan(0)
    expect([...ALERT_EVENT_TYPES]).toEqual(goTypes)
  })
})
