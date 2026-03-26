import type { Pool } from 'pg';
import type { StoredMemory, ParsedMemoryMarker, MemoryType } from '../types.js';

const KNOWN_SERVICES = [
  'twitch-listener',
  'youtube-listener',
  'youtube-innertube-listener',
  'kick-listener',
  'tiktok-listener',
  'discord-listener',
  'api-gateway',
  'auth-service',
  'emote-service',
  'message-processor',
  'overlay-manager',
  'source-manager',
  'token-refresh-service',
  'support-bot',
];

const KNOWN_ERROR_KEYWORDS = [
  'oomkill',
  'crashloop',
  'timeout',
  'quota',
  'rate-limit',
  '429',
  '500',
  '502',
  '503',
  'connection',
  'websocket',
  'redis',
  'postgres',
  'database',
];

export function normalizeTags(raw: string[]): string[] {
  return raw.map((t) => t.trim().toLowerCase()).filter((t) => t.length > 0);
}

export function extractTagsFromQuestion(question: string): string[] {
  const lower = question.toLowerCase();
  const found: string[] = [];

  for (const service of KNOWN_SERVICES) {
    if (lower.includes(service)) {
      found.push(service);
    }
  }

  for (const keyword of KNOWN_ERROR_KEYWORDS) {
    if (lower.includes(keyword)) {
      found.push(keyword);
    }
  }

  return found;
}

const STALENESS_FORMULA =
  `EXTRACT(epoch FROM NOW() - updated_at) / 86400.0 - (access_count * 2.0)`;

interface RawMemoryRow {
  id: number;
  type: MemoryType;
  tags: string[];
  content: string;
  access_count: number;
  updated_at: Date;
}

export class MemoryRepository {
  private pool: Pool;

  constructor(pool: Pool) {
    this.pool = pool;
  }

  async retrieveMemories(tags: string[]): Promise<StoredMemory[]> {
    try {
      const result = await this.pool.query<RawMemoryRow>(
        `SELECT id, type, tags, content, access_count, updated_at
         FROM bot_memories
         WHERE tags && $1
         ORDER BY ${STALENESS_FORMULA} ASC
         LIMIT 10`,
        [tags],
      );

      const rows = result.rows;

      if (rows.length === 0) {
        return [];
      }

      const ids = rows.map((r: RawMemoryRow) => r.id);
      await this.pool.query(
        `UPDATE bot_memories SET access_count = access_count + 1 WHERE id = ANY($1)`,
        [ids],
      );

      return rows.map((r: RawMemoryRow) => ({
        id: r.id,
        type: r.type,
        tags: r.tags,
        content: r.content,
        accessCount: r.access_count,
        updatedAt: r.updated_at,
      }));
    } catch (err) {
      console.warn('[MemoryRepository] retrieveMemories error:', err);
      return [];
    }
  }

  async storeMemory(marker: ParsedMemoryMarker): Promise<void> {
    try {
      const normalizedTags = normalizeTags(marker.tags);
      const truncatedContent = marker.content.slice(0, 500);

      const existing = await this.pool.query<{ id: number }>(
        `SELECT id FROM bot_memories
         WHERE type = $1 AND cardinality(tags & $2) >= 2
         LIMIT 1`,
        [marker.type, normalizedTags],
      );

      if (existing.rows.length > 0) {
        await this.pool.query(
          `UPDATE bot_memories
           SET content = $1, tags = $2, updated_at = NOW()
           WHERE id = $3`,
          [truncatedContent, normalizedTags, existing.rows[0].id],
        );
      } else {
        await this.pool.query(
          `INSERT INTO bot_memories (type, tags, content)
           VALUES ($1, $2, $3)`,
          [marker.type, normalizedTags, truncatedContent],
        );
      }

      await this.pruneIfNeeded();
    } catch (err) {
      console.warn('[MemoryRepository] storeMemory error:', err);
    }
  }

  async updateMemory(id: number, content: string): Promise<void> {
    try {
      const truncatedContent = content.slice(0, 500);
      await this.pool.query(
        `UPDATE bot_memories SET content = $1, updated_at = NOW() WHERE id = $2`,
        [truncatedContent, id],
      );
    } catch (err) {
      console.warn('[MemoryRepository] updateMemory error:', err);
    }
  }

  private async pruneIfNeeded(): Promise<void> {
    const countResult = await this.pool.query<{ count: string }>(
      `SELECT COUNT(*) AS count FROM bot_memories`,
    );
    const count = parseInt(countResult.rows[0].count, 10);

    if (count > 500) {
      await this.pool.query(
        `DELETE FROM bot_memories
         WHERE id = (
           SELECT id FROM bot_memories
           ORDER BY ${STALENESS_FORMULA} DESC
           LIMIT 1
         )`,
      );
    }
  }
}
