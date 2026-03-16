import { describe, it, expect, vi, beforeEach } from 'vitest'

const mockGet = vi.fn()
const mockDelete = vi.fn()
const mockPatch = vi.fn()

vi.mock('@/lib/api/client', () => ({
  apiClient: { get: mockGet, delete: mockDelete, patch: mockPatch },
}))

// Import after mock setup
const { getGuilds, getGuildChannels, disconnectGuild, updateSourceConfig } = await import(
  '@/lib/api/discord'
)

describe('discord API module', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('getGuilds calls GET /api/v1/auth/guilds', async () => {
    mockGet.mockResolvedValue([])
    await getGuilds()
    expect(mockGet).toHaveBeenCalledWith('/api/v1/auth/guilds')
  })

  it('getGuildChannels calls GET /api/v1/auth/guilds/{id}/channels', async () => {
    mockGet.mockResolvedValue({ categories: [] })
    await getGuildChannels('guild-123')
    expect(mockGet).toHaveBeenCalledWith('/api/v1/auth/guilds/guild-123/channels')
  })

  it('disconnectGuild calls DELETE /api/v1/auth/guilds/{id}', async () => {
    mockDelete.mockResolvedValue(undefined)
    await disconnectGuild('guild-456')
    expect(mockDelete).toHaveBeenCalledWith('/api/v1/auth/guilds/guild-456')
  })

  it('updateSourceConfig calls PATCH /api/v1/overlays/{overlayId}/sources/{sourceId}', async () => {
    mockPatch.mockResolvedValue(undefined)
    const config = {
      guild_id: 'g',
      inbound_channel_id: 'c',
      relay_enabled: false,
      relay_channel_id: null,
    }
    await updateSourceConfig('overlay-1', 'source-2', config)
    expect(mockPatch).toHaveBeenCalledWith('/api/v1/overlays/overlay-1/sources/source-2', {
      config,
    })
  })
})
