import { describe, it, expect, vi, beforeEach } from 'vitest';
import type { Pool, QueryResult } from 'pg';

// Mock pg module
vi.mock('pg', () => {
  const mockQuery = vi.fn();
  const MockPool = vi.fn(() => ({ query: mockQuery }));
  return { Pool: MockPool, default: { Pool: MockPool } };
});

import { MemoryRepository, normalizeTags, extractTagsFromQuestion } from '../memory/repository.js';
import type { StoredMemory, ParsedMemoryMarker } from '../types.js';

function makeMockPool(): { pool: Pool; mockQuery: ReturnType<typeof vi.fn> } {
  const mockQuery = vi.fn();
  const pool = { query: mockQuery } as unknown as Pool;
  return { pool, mockQuery };
}

describe('normalizeTags', () => {
  it('trims whitespace, lowercases, and filters empty strings', () => {
    const result = normalizeTags(['  Kick-Listener ', ' auth-service', '']);
    expect(result).toEqual(['kick-listener', 'auth-service']);
  });

  it('returns empty array for all-empty input', () => {
    expect(normalizeTags(['', '  ', '   '])).toEqual([]);
  });
});

describe('extractTagsFromQuestion', () => {
  it('extracts known service names from question text', () => {
    const result = extractTagsFromQuestion('why is kick-listener crashing');
    expect(result).toContain('kick-listener');
  });

  it('extracts error keywords from question text', () => {
    const result = extractTagsFromQuestion('the service is in crashloop with timeout');
    expect(result).toContain('crashloop');
    expect(result).toContain('timeout');
  });

  it('extracts multiple service names from a question', () => {
    const result = extractTagsFromQuestion('auth-service and api-gateway both have issues');
    expect(result).toContain('auth-service');
    expect(result).toContain('api-gateway');
  });

  it('returns empty array when no known terms found', () => {
    const result = extractTagsFromQuestion('what is the meaning of life');
    expect(result).toEqual([]);
  });
});

describe('MemoryRepository', () => {
  let repo: MemoryRepository;
  let mockQuery: ReturnType<typeof vi.fn>;
  let pool: Pool;

  beforeEach(() => {
    vi.clearAllMocks();
    const mock = makeMockPool();
    pool = mock.pool;
    mockQuery = mock.mockQuery;
    repo = new MemoryRepository(pool);
  });

  describe('retrieveMemories', () => {
    it('returns StoredMemory[] from pool.query result rows', async () => {
      const fakeRows = [
        {
          id: 1,
          type: 'error_pattern',
          tags: ['kick-listener', 'crashloop'],
          content: 'kick-listener crashes when rate limited',
          access_count: 3,
          updated_at: new Date('2026-01-01'),
        },
      ];
      // First call: SELECT query, second call: UPDATE access_count
      mockQuery
        .mockResolvedValueOnce({ rows: fakeRows } as QueryResult)
        .mockResolvedValueOnce({ rows: [] } as QueryResult);

      const result = await repo.retrieveMemories(['kick-listener']);

      expect(result).toHaveLength(1);
      expect(result[0].id).toBe(1);
      expect(result[0].type).toBe('error_pattern');
      expect(result[0].tags).toEqual(['kick-listener', 'crashloop']);
      expect(result[0].content).toBe('kick-listener crashes when rate limited');
      expect(result[0].accessCount).toBe(3);
      expect(result[0].updatedAt).toEqual(new Date('2026-01-01'));
    });

    it('returns empty array when pool.query throws (error swallowed, logged)', async () => {
      mockQuery.mockRejectedValueOnce(new Error('DB connection failed'));

      const result = await repo.retrieveMemories(['kick-listener']);

      expect(result).toEqual([]);
    });

    it('bumps access_count for returned memory IDs', async () => {
      const fakeRows = [
        {
          id: 42,
          type: 'codebase_insight',
          tags: ['api-gateway'],
          content: 'api-gateway handles WebSocket connections',
          access_count: 1,
          updated_at: new Date(),
        },
      ];
      mockQuery
        .mockResolvedValueOnce({ rows: fakeRows } as QueryResult)
        .mockResolvedValueOnce({ rows: [] } as QueryResult);

      await repo.retrieveMemories(['api-gateway']);

      expect(mockQuery).toHaveBeenCalledTimes(2);
      const secondCall = mockQuery.mock.calls[1];
      expect(secondCall[0]).toMatch(/UPDATE.*bot_memories/i);
      // params are passed as [ids] where ids is an array, so secondCall[1] is [[42]]
      const idsParam = (secondCall[1] as number[][])[0];
      expect(idsParam).toContain(42);
    });

    it('returns empty array when no rows match (no access_count bump needed)', async () => {
      mockQuery.mockResolvedValueOnce({ rows: [] } as QueryResult);

      const result = await repo.retrieveMemories(['nonexistent-tag']);

      expect(result).toEqual([]);
      // No second query needed when no rows returned
      expect(mockQuery).toHaveBeenCalledTimes(1);
    });
  });

  describe('storeMemory', () => {
    it('inserts new row when no existing memory matches type+tags', async () => {
      const marker: ParsedMemoryMarker = {
        type: 'error_pattern',
        tags: ['kick-listener'],
        content: 'New error pattern',
      };
      // check for existing: returns nothing, insert, pruneIfNeeded count query
      mockQuery
        .mockResolvedValueOnce({ rows: [] } as QueryResult) // SELECT existing
        .mockResolvedValueOnce({ rows: [] } as QueryResult) // INSERT
        .mockResolvedValueOnce({ rows: [{ count: '10' }] } as QueryResult); // prune count check

      await repo.storeMemory(marker);

      expect(mockQuery).toHaveBeenCalledTimes(3);
      const insertCall = mockQuery.mock.calls[1];
      expect(insertCall[0]).toMatch(/INSERT INTO bot_memories/i);
    });

    it('updates existing row when same type + 2+ overlapping tags found', async () => {
      const marker: ParsedMemoryMarker = {
        type: 'error_pattern',
        tags: ['kick-listener', 'crashloop'],
        content: 'Updated error pattern',
      };
      mockQuery
        .mockResolvedValueOnce({ rows: [{ id: 5 }] } as QueryResult) // SELECT existing
        .mockResolvedValueOnce({ rows: [] } as QueryResult) // UPDATE
        .mockResolvedValueOnce({ rows: [{ count: '10' }] } as QueryResult); // prune count check

      await repo.storeMemory(marker);

      expect(mockQuery).toHaveBeenCalledTimes(3);
      const updateCall = mockQuery.mock.calls[1];
      expect(updateCall[0]).toMatch(/UPDATE bot_memories/i);
    });

    it('swallows pool.query errors (logs warning, does not throw)', async () => {
      const marker: ParsedMemoryMarker = {
        type: 'correction',
        tags: ['auth-service'],
        content: 'Some correction',
      };
      mockQuery.mockRejectedValueOnce(new Error('DB error'));

      await expect(repo.storeMemory(marker)).resolves.toBeUndefined();
    });

    it('triggers pruning when count > 500', async () => {
      const marker: ParsedMemoryMarker = {
        type: 'codebase_insight',
        tags: ['message-processor'],
        content: 'Some insight',
      };
      mockQuery
        .mockResolvedValueOnce({ rows: [] } as QueryResult) // SELECT existing
        .mockResolvedValueOnce({ rows: [] } as QueryResult) // INSERT
        .mockResolvedValueOnce({ rows: [{ count: '501' }] } as QueryResult) // prune count check
        .mockResolvedValueOnce({ rows: [] } as QueryResult); // DELETE oldest

      await repo.storeMemory(marker);

      expect(mockQuery).toHaveBeenCalledTimes(4);
      const deleteCall = mockQuery.mock.calls[3];
      expect(deleteCall[0]).toMatch(/DELETE FROM bot_memories/i);
    });
  });

  describe('updateMemory', () => {
    it('updates content and updated_at for given id', async () => {
      mockQuery.mockResolvedValueOnce({ rows: [] } as QueryResult);

      await repo.updateMemory(7, 'New content here');

      expect(mockQuery).toHaveBeenCalledTimes(1);
      const updateCall = mockQuery.mock.calls[0];
      expect(updateCall[0]).toMatch(/UPDATE bot_memories/i);
      expect(updateCall[1]).toContain(7);
      expect(updateCall[1]).toContain('New content here'.slice(0, 500));
    });

    it('swallows errors gracefully', async () => {
      mockQuery.mockRejectedValueOnce(new Error('DB error'));

      await expect(repo.updateMemory(1, 'content')).resolves.toBeUndefined();
    });
  });
});
