/**
 * AddSourceModal Component Tests
 *
 * Tests for the add-source prompt modal shown after accepting a share.
 */

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor, fireEvent } from '@testing-library/react';
import { AddSourceModal } from './AddSourceModal';
import { overlaysApi } from '@/lib/api/overlays';
import type { Overlay } from '@/lib/types/overlay';

// Mock APIs
vi.mock('@/lib/api/overlays');
vi.mock('react-hot-toast', () => ({
  default: {
    success: vi.fn(),
    error: vi.fn(),
  },
}));

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
];

describe('AddSourceModal', () => {
  const mockOnClose = vi.fn();
  const mockOnAdded = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(overlaysApi.list).mockResolvedValue(mockOverlays);
  });

  // Test 1: Modal displays sender name and shared overlay preview
  it('renders sender name in title', async () => {
    render(
      <AddSourceModal
        senderName="Streamer 123"
        senderOverlayId="overlay-789"
        onClose={mockOnClose}
        onAdded={mockOnAdded}
      />
    );

    expect(screen.getByText(/Add Streamer 123's overlay to one of yours/i)).toBeInTheDocument();

    // Wait for overlays to load
    await waitFor(() => {
      expect(screen.getByRole('combobox')).toBeInTheDocument();
    });
  });

  // Test 2: Overlay dropdown shows all user's overlays
  it('fetches and displays all user overlays in dropdown', async () => {
    render(
      <AddSourceModal
        senderName="Streamer 123"
        senderOverlayId="overlay-789"
        onClose={mockOnClose}
        onAdded={mockOnAdded}
      />
    );

    await waitFor(() => {
      expect(overlaysApi.list).toHaveBeenCalled();
    });

    // Check dropdown contains overlays
    const select = screen.getByRole('combobox');
    expect(select).toBeInTheDocument();

    await waitFor(() => {
      expect(screen.getByText('My Gaming Overlay')).toBeInTheDocument();
      expect(screen.getByText('My IRL Overlay')).toBeInTheDocument();
    });
  });

  // Test 3: Add button adds shared source to selected overlay
  it('calls addSource API when Add button clicked', async () => {
    vi.mocked(overlaysApi.addSource).mockResolvedValue({
      id: 'source-new',
      overlay_id: 'overlay-1',
      platform: 'twitch',
      channel_id: 'overlay-789',
      created_at: '2026-03-09T12:00:00Z',
      updated_at: '2026-03-09T12:00:00Z',
    });

    render(
      <AddSourceModal
        senderName="Streamer 123"
        senderOverlayId="overlay-789"
        onClose={mockOnClose}
        onAdded={mockOnAdded}
      />
    );

    await waitFor(() => {
      expect(screen.getByRole('combobox')).toBeInTheDocument();
    });

    // Click Add button
    const addButton = screen.getByRole('button', { name: /Add/i });
    fireEvent.click(addButton);

    await waitFor(() => {
      // For Phase 15, we're just logging (Phase 16 will implement API)
      // So we check onAdded and onClose are called
      expect(mockOnAdded).toHaveBeenCalled();
      expect(mockOnClose).toHaveBeenCalled();
    });
  });

  // Test 4: Skip button closes modal without action
  it('closes modal without action when Skip button clicked', async () => {
    render(
      <AddSourceModal
        senderName="Streamer 123"
        senderOverlayId="overlay-789"
        onClose={mockOnClose}
        onAdded={mockOnAdded}
      />
    );

    await waitFor(() => {
      expect(screen.getByRole('combobox')).toBeInTheDocument();
    });

    // Click Skip button
    const skipButton = screen.getByRole('button', { name: /Skip/i });
    fireEvent.click(skipButton);

    await waitFor(() => {
      expect(mockOnClose).toHaveBeenCalled();
      expect(mockOnAdded).not.toHaveBeenCalled();
    });
  });

  // Test 5: Add button calls overlaysApi.addSource with shared_overlay platform
  // RED state (Wave 0): handleAdd uses console.log, NOT overlaysApi.addSource → FAILS
  // GREEN state (Wave 1+): handleAdd calls overlaysApi.addSource → PASSES
  it('calls overlaysApi.addSource with shared_overlay platform when Add clicked', async () => {
    vi.mocked(overlaysApi.addSource).mockResolvedValue({
      id: 'source-new',
      overlay_id: 'overlay-1',
      platform: 'shared_overlay',
      channel_id: 'sender-overlay-uuid',
      channel_name: "xqc's overlay",
      auth_required: false,
      config: {},
      is_active: false,
      created_at: '2026-03-10T12:00:00Z',
      updated_at: '2026-03-10T12:00:00Z',
    });

    render(
      <AddSourceModal
        senderName="xqc"
        senderOverlayId="sender-overlay-uuid"
        onClose={mockOnClose}
        onAdded={mockOnAdded}
      />
    );

    await waitFor(() => {
      expect(screen.getByRole('combobox')).toBeInTheDocument();
    });

    // Click Add button
    const addButton = screen.getByRole('button', { name: /Add/i });
    fireEvent.click(addButton);

    await waitFor(() => {
      // At Wave 0: handleAdd calls console.log, NOT overlaysApi.addSource → assertion FAILS RED
      // At Wave 1+: handleAdd calls overlaysApi.addSource with correct args → PASSES GREEN
      expect(overlaysApi.addSource).toHaveBeenCalledWith('overlay-1', {
        platform: 'shared_overlay',
        channel_id: 'sender-overlay-uuid',
        channel_name: "xqc's overlay",
      });
    });
  });
});
