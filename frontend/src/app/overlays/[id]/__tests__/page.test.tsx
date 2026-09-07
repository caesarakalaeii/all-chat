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
 * Overlay editor page — preview-handshake freshness.
 *
 * The editor pushes settings into the preview iframe twice: once eagerly when
 * the iframe element attaches (`onIframeReady`) and again when the embed posts
 * `EMBED_READY` to say its own listener is live. Both of those fire from
 * long-lived callbacks, so if they close over a stale `visualSettings` the
 * preview silently shows the settings the user had when the editor loaded
 * instead of the ones they just changed. These tests pin the payload that
 * reaches `postMessage` after an edit; they are the guard for the freshness
 * mechanism, whatever that mechanism happens to be.
 */

// @vitest-environment jsdom
import '@testing-library/jest-dom/vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import OverlayEditorPage from '@/app/overlays/[id]/page'

// vi.hoisted, because vi.mock's factory is lifted above the imports.
const api = vi.hoisted(() => ({
  get: vi.fn(),
  getSources: vi.fn(),
  getConfig: vi.fn(),
  getTTSConfig: vi.fn(),
}))

vi.mock('@/lib/api/overlays', async () => {
  const actual = await vi.importActual<typeof import('@/lib/api/overlays')>('@/lib/api/overlays')
  return { ...actual, overlaysApi: { ...actual.overlaysApi, ...api } }
})

vi.mock('@/lib/api/shares', () => ({
  sharesApi: { getAcceptedShares: vi.fn().mockResolvedValue([]) },
}))

vi.mock('@/lib/stores/auth-store', () => ({
  useAuthStore: () => ({ user: { id: 'u1', username: 'streamer' } }),
}))

vi.mock('@/hooks/useNotificationSocket', () => ({ useNotificationSocket: () => {} }))

vi.mock('next/navigation', () => ({
  useRouter: () => ({ push: vi.fn(), replace: vi.fn(), prefetch: vi.fn() }),
  usePathname: () => '/overlays/o1',
  useSearchParams: () => new URLSearchParams(),
}))

vi.mock('@/components/AppNav', () => ({ AppNav: () => null }))

const postMessage = vi.hoisted(() => vi.fn())

/**
 * SplitView stub: the real one mounts an iframe, and jsdom's iframe has no
 * usable `contentDocument.documentElement` for the `getComputedStyle` probe in
 * `handleIframeReady`. The stub hands the page a fake iframe whose
 * `contentWindow.postMessage` is the spy these tests assert on, and exposes the
 * attach as a button so a test can replay the handshake after an edit.
 */
vi.mock('@/components/SplitView', () => ({
  SplitView: ({
    children,
    onIframeReady,
  }: {
    children: React.ReactNode
    onIframeReady?: (iframe: HTMLIFrameElement) => void
  }) => {
    const iframe = {
      contentWindow: { postMessage },
      contentDocument: null,
    } as unknown as HTMLIFrameElement
    return (
      <div>
        <button type="button" onClick={() => onIframeReady?.(iframe)}>
          attach preview iframe
        </button>
        {children}
      </div>
    )
  },
}))

// jsdom lacks window.matchMedia; useReducedMotion (reached via the mock-message
// preview rows) calls it. Static non-matching stub.
if (typeof window.matchMedia !== 'function') {
  window.matchMedia = (query: string) =>
    ({
      matches: false,
      media: query,
      addEventListener: () => {},
      removeEventListener: () => {},
    }) as unknown as MediaQueryList
}

/**
 * The page reads its overlay id from a promise prop via React's `use()`. A
 * thenable that already carries React's fulfilled bookkeeping is unwrapped
 * without suspending, which keeps these tests out of Suspense: the page holds
 * async work open indefinitely (`next/dynamic` chunks, debounces), so an
 * `await act(...)` around the render never settles.
 */
function resolvedParams(id: string): Promise<{ id: string }> {
  return Object.assign(Promise.resolve({ id }), {
    status: 'fulfilled' as const,
    value: { id },
  })
}

/** Messages of one `type` posted to the preview iframe, oldest first. */
function postedMessages(type: string): Record<string, unknown>[] {
  return postMessage.mock.calls
    .map((call) => call[0] as Record<string, unknown>)
    .filter((message) => message?.type === type)
}

function latestPostedMessage(type: string): Record<string, unknown> | undefined {
  const messages = postedMessages(type)
  return messages[messages.length - 1]
}

/** Replay the `onIframeReady` attach the real SplitView performs on mount. */
function attachPreviewIframe(): void {
  fireEvent.click(screen.getByRole('button', { name: 'attach preview iframe' }))
}

/** Replay the embed's "my listener is live" handshake. */
function postEmbedReady(): void {
  fireEvent(
    window,
    new MessageEvent('message', { data: { type: 'EMBED_READY' }, origin: window.location.origin })
  )
}

async function renderEditor(): Promise<void> {
  render(<OverlayEditorPage params={resolvedParams('o1')} />)
  await screen.findByRole('button', { name: 'attach preview iframe' })
}

describe('overlay editor preview handshake', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    api.get.mockResolvedValue({ id: 'o1', name: 'Main overlay', is_public_for_viewers: false })
    api.getSources.mockResolvedValue([])
    api.getConfig.mockResolvedValue({ display_settings: {} })
    api.getTTSConfig.mockResolvedValue({})
  })

  afterEach(() => {
    cleanup()
    vi.restoreAllMocks()
  })

  it('sends the visual settings edited since load when the iframe re-attaches', async () => {
    await renderEditor()
    attachPreviewIframe()
    expect(latestPostedMessage('VISUAL_CSS_UPDATE')?.css).not.toContain('--chat-show-avatars: none')

    fireEvent.click(screen.getByRole('button', { name: 'Visibility' }))
    fireEvent.click(screen.getByRole('switch', { name: 'Show avatars' }))

    postMessage.mockClear()
    attachPreviewIframe()

    expect(latestPostedMessage('VISUAL_CSS_UPDATE')?.css).toContain('--chat-show-avatars: none')
  })

  it('sends the visual settings edited since load when the embed reports ready', async () => {
    await renderEditor()
    attachPreviewIframe()

    fireEvent.click(screen.getByRole('button', { name: 'Visibility' }))
    fireEvent.click(screen.getByRole('switch', { name: 'Show avatars' }))

    postMessage.mockClear()
    postEmbedReady()

    expect(latestPostedMessage('VISUAL_CSS_UPDATE')?.css).toContain('--chat-show-avatars: none')
  })

  it('sends the filter settings edited since load when the embed reports ready', async () => {
    await renderEditor()
    attachPreviewIframe()

    fireEvent.click(screen.getByRole('button', { name: 'Filters' }))
    fireEvent.click(screen.getByRole('switch', { name: 'Hide bot commands (!)' }))

    postMessage.mockClear()
    postEmbedReady()

    expect(latestPostedMessage('FILTER_SETTINGS_UPDATE')?.filterSettings).toMatchObject({
      hide_commands: true,
    })
  })
})

/**
 * The two URLs the editor hands out are not interchangeable: the OBS URL is the
 * public overlay a browser SOURCE renders on stream, and the dock URL is the
 * auth-gated monitor a browser DOCK renders beside the mixer. Pasting one where
 * the other belongs is a support ticket, so each has its own button and the
 * dock's carries the `dock=1` flag that makes the monitor fit a narrow panel.
 */
describe('overlay editor OBS and dock URLs', () => {
  const writeText = vi.fn().mockResolvedValue(undefined)

  beforeEach(() => {
    vi.clearAllMocks()
    api.get.mockResolvedValue({ id: 'o1', name: 'Main overlay', is_public_for_viewers: false })
    api.getSources.mockResolvedValue([])
    api.getConfig.mockResolvedValue({ display_settings: {} })
    api.getTTSConfig.mockResolvedValue({})
    Object.defineProperty(navigator, 'clipboard', {
      value: { writeText },
      configurable: true,
    })
  })

  afterEach(() => {
    cleanup()
    vi.restoreAllMocks()
  })

  it('copies the public overlay URL, with no dock flag on it', async () => {
    await renderEditor()

    fireEvent.click(screen.getByRole('button', { name: 'Copy OBS URL' }))

    expect(writeText).toHaveBeenCalledWith(`${window.location.origin}/overlay/o1`)
  })

  it('copies the monitor URL with dock=1 for a browser dock', async () => {
    await renderEditor()

    fireEvent.click(screen.getByRole('button', { name: 'Copy dock URL' }))

    expect(writeText).toHaveBeenCalledWith(`${window.location.origin}/overlay/o1/view?dock=1`)
  })
})
