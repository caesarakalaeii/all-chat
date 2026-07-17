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
 * AcceptModal Component Tests
 *
 * Tests for the share request acceptance modal with form validation.
 */

// @vitest-environment jsdom
import '@testing-library/jest-dom/vitest'
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, waitFor, fireEvent, cleanup } from '@testing-library/react'
import { AcceptModal } from '../AcceptModal'
import { sharesApi } from '@/lib/api/shares'
import { overlaysApi } from '@/lib/api/overlays'
import type { ShareRequest } from '@/lib/types/share'
import type { Overlay } from '@/lib/types/overlay'

// Mock APIs
vi.mock('@/lib/api/shares')
vi.mock('@/lib/api/overlays')
vi.mock('@/lib/toast', () => ({
  toastManager: {
    add: vi.fn(),
  },
}))

// The dialog renders into document.body via a portal; unmount between tests
// so stale portals do not leak into the next test's queries.
afterEach(() => cleanup())

const mockRequest: ShareRequest = {
  id: 'share-123',
  sender_user_id: 'user-456',
  sender_overlay_id: 'overlay-789',
  recipient_user_id: 'user-current',
  status: 'pending',
  created_at: '2026-03-09T12:00:00Z',
  expires_at: '2026-03-16T12:00:00Z',
  sender: {
    id: 'user-456',
    username: 'streamer123',
    display_name: 'Streamer 123',
    profile_image_url: 'https://example.com/avatar.png',
  },
  overlay_sources: [
    { platform: 'twitch', channel_name: 'channel1' },
    { platform: 'youtube', channel_name: 'channel2' },
  ],
}

const mockOverlays: Overlay[] = [
  {
    id: 'overlay-1',
    user_id: 'user-current',
    name: 'My Gaming Overlay',
    is_active: true,
    is_public_for_viewers: false,
    created_at: '2026-03-01T12:00:00Z',
    updated_at: '2026-03-01T12:00:00Z',
  },
  {
    id: 'overlay-2',
    user_id: 'user-current',
    name: 'My IRL Overlay',
    is_active: false,
    is_public_for_viewers: false,
    created_at: '2026-03-02T12:00:00Z',
    updated_at: '2026-03-02T12:00:00Z',
  },
]

describe('AcceptModal', () => {
  const mockOnClose = vi.fn()
  const mockOnAccepted = vi.fn()

  beforeEach(() => {
    vi.clearAllMocks()
    vi.mocked(overlaysApi.list).mockResolvedValue(mockOverlays)
  })

  // Test 1: Modal displays sender name and platform badges
  it('renders sender name and platform badges', async () => {
    render(<AcceptModal request={mockRequest} onClose={mockOnClose} onAccepted={mockOnAccepted} />)

    // Rendered via portal into document.body as an accessible dialog
    expect(screen.getByRole('dialog')).toBeInTheDocument()

    // Check sender name in title
    expect(screen.getByText(/Streamer 123 wants to share with you/i)).toBeInTheDocument()

    // Wait for overlays to load
    await waitFor(() => {
      expect(screen.getByRole('combobox')).toBeInTheDocument()
    })
  })

  // Test 2: Overlay dropdown populates with user's overlays
  it('fetches and displays user overlays in dropdown', async () => {
    render(<AcceptModal request={mockRequest} onClose={mockOnClose} onAccepted={mockOnAccepted} />)

    await waitFor(() => {
      expect(overlaysApi.list).toHaveBeenCalled()
    })

    // Check dropdown contains overlays
    const select = screen.getByRole('combobox')
    expect(select).toBeInTheDocument()

    // Check options are present (checking in the document)
    await waitFor(() => {
      expect(screen.getByText('My Gaming Overlay')).toBeInTheDocument()
      expect(screen.getByText('My IRL Overlay')).toBeInTheDocument()
    })
  })

  // Test 3: "This stream" expiry option is pre-selected by default
  it('defaults to "This stream" expiry option', async () => {
    render(<AcceptModal request={mockRequest} onClose={mockOnClose} onAccepted={mockOnAccepted} />)

    await waitFor(() => {
      expect(screen.getByRole('combobox')).toBeInTheDocument()
    })

    // Find the "This stream" radio button and check it's selected
    const thisStreamRadio = screen.getByLabelText(/This stream/i) as HTMLInputElement
    expect(thisStreamRadio).toBeChecked()
  })

  // Test 4: Custom duration shows inline error when value < 1 or > 168
  it('validates custom hours input (boundary cases)', async () => {
    render(<AcceptModal request={mockRequest} onClose={mockOnClose} onAccepted={mockOnAccepted} />)

    await waitFor(() => {
      expect(screen.getByRole('combobox')).toBeInTheDocument()
    })

    // Select custom duration option
    const customRadio = screen.getByLabelText(/Custom duration/i)
    fireEvent.click(customRadio)

    // Get the hours input
    const hoursInput = screen.getByPlaceholderText(/hours/i) || screen.getByRole('spinbutton')

    // Test: 0 hours (invalid)
    fireEvent.change(hoursInput, { target: { value: '0' } })
    await waitFor(() => {
      expect(screen.getByText(/Must be between 1 and 168 hours/i)).toBeInTheDocument()
    })

    // Test: 169 hours (invalid)
    fireEvent.change(hoursInput, { target: { value: '169' } })
    await waitFor(() => {
      expect(screen.getByText(/Must be between 1 and 168 hours/i)).toBeInTheDocument()
    })

    // Test: 1 hour (valid)
    fireEvent.change(hoursInput, { target: { value: '1' } })
    await waitFor(() => {
      expect(screen.queryByText(/Must be between 1 and 168 hours/i)).not.toBeInTheDocument()
    })

    // Test: 168 hours (valid)
    fireEvent.change(hoursInput, { target: { value: '168' } })
    await waitFor(() => {
      expect(screen.queryByText(/Must be between 1 and 168 hours/i)).not.toBeInTheDocument()
    })
  })

  // Test 5: Accept button disabled when validation fails
  it('disables Accept button when validation fails', async () => {
    render(<AcceptModal request={mockRequest} onClose={mockOnClose} onAccepted={mockOnAccepted} />)

    await waitFor(() => {
      expect(screen.getByRole('combobox')).toBeInTheDocument()
    })

    // Select custom duration option
    const customRadio = screen.getByLabelText(/Custom duration/i)
    fireEvent.click(customRadio)

    const hoursInput = screen.getByPlaceholderText(/hours/i) || screen.getByRole('spinbutton')
    const acceptButton = screen.getByRole('button', { name: /Accept/i })

    // Invalid value: button should be disabled
    fireEvent.change(hoursInput, { target: { value: '0' } })
    await waitFor(() => {
      expect(acceptButton).toBeDisabled()
    })

    // Valid value: button should be enabled
    fireEvent.change(hoursInput, { target: { value: '24' } })
    await waitFor(() => {
      expect(acceptButton).not.toBeDisabled()
    })
  })

  // Test 6: Calls onAccepted with sender_overlay_id on success
  it('calls onAccepted callback on successful acceptance', async () => {
    const mockResponse = {
      share: { ...mockRequest, status: 'accepted' as const },
      sender_overlay_id: 'overlay-789',
    }
    vi.mocked(sharesApi.acceptRequest).mockResolvedValue(mockResponse)

    render(<AcceptModal request={mockRequest} onClose={mockOnClose} onAccepted={mockOnAccepted} />)

    await waitFor(() => {
      expect(screen.getByRole('combobox')).toBeInTheDocument()
    })

    // Click Accept button
    const acceptButton = screen.getByRole('button', { name: /Accept/i })
    fireEvent.click(acceptButton)

    await waitFor(() => {
      expect(sharesApi.acceptRequest).toHaveBeenCalledWith(
        'share-123',
        'overlay-1', // First overlay auto-selected
        'this_stream',
        undefined
      )
      expect(mockOnAccepted).toHaveBeenCalledWith('overlay-789')
    })
  })

  // Test 7: Shows error when no overlays exist
  it('shows error message when user has no overlays', async () => {
    vi.mocked(overlaysApi.list).mockResolvedValue([])

    render(<AcceptModal request={mockRequest} onClose={mockOnClose} onAccepted={mockOnAccepted} />)

    await waitFor(() => {
      expect(screen.getByText(/Create an overlay first to accept shares/i)).toBeInTheDocument()
    })
  })
})

describe('AcceptModal — senderPlatform Kick disable', () => {
  const mockOnClose = vi.fn()
  const mockOnAccepted = vi.fn()

  beforeEach(() => {
    vi.clearAllMocks()
    vi.mocked(overlaysApi.list).mockResolvedValue(mockOverlays)
  })

  // Test: Kick disables "This stream" radio
  it('disables "This stream" option when senderPlatform is kick', async () => {
    render(
      <AcceptModal
        request={mockRequest}
        onClose={mockOnClose}
        onAccepted={mockOnAccepted}
        senderPlatform="kick"
      />
    )

    await waitFor(() => {
      expect(screen.getByRole('combobox')).toBeInTheDocument()
    })

    const thisStreamRadio = screen.getByRole('radio', { name: /this stream/i })
    expect(thisStreamRadio).toBeDisabled()
  })

  // Test: Kick shows explanatory note
  it('shows explanatory note when senderPlatform is kick', async () => {
    render(
      <AcceptModal
        request={mockRequest}
        onClose={mockOnClose}
        onAccepted={mockOnAccepted}
        senderPlatform="kick"
      />
    )

    await waitFor(() => {
      expect(screen.getByRole('combobox')).toBeInTheDocument()
    })

    expect(screen.getByText(/not available for Kick/i)).toBeInTheDocument()
  })

  // Test: Non-kick platform does not disable "This stream"
  it('does not disable "This stream" for non-Kick platforms', async () => {
    render(
      <AcceptModal
        request={mockRequest}
        onClose={mockOnClose}
        onAccepted={mockOnAccepted}
        senderPlatform="twitch"
      />
    )

    await waitFor(() => {
      expect(screen.getByRole('combobox')).toBeInTheDocument()
    })

    const thisStreamRadio = screen.getByRole('radio', { name: /this stream/i })
    expect(thisStreamRadio).not.toBeDisabled()
  })

  // Test: Undefined senderPlatform does not disable "This stream"
  it('does not disable "This stream" when senderPlatform is undefined', async () => {
    render(<AcceptModal request={mockRequest} onClose={mockOnClose} onAccepted={mockOnAccepted} />)

    await waitFor(() => {
      expect(screen.getByRole('combobox')).toBeInTheDocument()
    })

    const thisStreamRadio = screen.getByRole('radio', { name: /this stream/i })
    expect(thisStreamRadio).not.toBeDisabled()
  })

  // Test: Kick switches default to 'unlimited'
  it('selects "unlimited" by default when senderPlatform is kick', async () => {
    render(
      <AcceptModal
        request={mockRequest}
        onClose={mockOnClose}
        onAccepted={mockOnAccepted}
        senderPlatform="kick"
      />
    )

    await waitFor(() => {
      expect(screen.getByRole('combobox')).toBeInTheDocument()
    })

    const unlimitedRadio = screen.getByRole('radio', { name: /unlimited/i })
    expect(unlimitedRadio).toBeChecked()
  })
})
