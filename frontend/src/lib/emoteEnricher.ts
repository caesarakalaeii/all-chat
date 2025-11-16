/**
 * Client-Side Emote Enricher
 *
 * Parses message text and enriches it with emote data (positions, URLs).
 * Mirrors the backend emote enrichment logic for client-side mock messages.
 */

import type { Emote } from './types/message';
import type { EmoteData } from './api/emotes';

interface WordOccurrence {
  word: string;
  start: number;
  end: number;
}

/**
 * Check if a character is a word boundary
 */
function isBoundaryChar(c: string): boolean {
  return c === ' ' || c === '\n' || c === '\t' || c === ',' || c === '.' || c === '!' || c === '?';
}

/**
 * Check if a substring at a given position is a complete word (not part of another word)
 */
function isWordBoundary(text: string, pos: number, length: number): boolean {
  // Check before
  if (pos > 0 && !isBoundaryChar(text[pos - 1])) {
    return false;
  }

  // Check after
  const endPos = pos + length;
  if (endPos < text.length && !isBoundaryChar(text[endPos])) {
    return false;
  }

  return true;
}

/**
 * Find all occurrences of a word in text with their positions
 */
function findWordOccurrences(text: string, word: string): WordOccurrence[] {
  const occurrences: WordOccurrence[] = [];
  let pos = 0;

  while (pos < text.length) {
    const idx = text.indexOf(word, pos);
    if (idx === -1) {
      break;
    }

    // Check if this is a word boundary
    if (isWordBoundary(text, idx, word.length)) {
      occurrences.push({
        word,
        start: idx,
        end: idx + word.length - 1
      });
    }

    pos = idx + 1;
  }

  return occurrences;
}

/**
 * Enrich message text with emote data
 *
 * @param text - The message text to enrich
 * @param availableEmotes - Array of available emotes for the channel
 * @returns Array of emote objects with positions
 */
export function enrichMessageWithEmotes(text: string, availableEmotes: EmoteData[]): Emote[] {
  if (!text || availableEmotes.length === 0) {
    return [];
  }

  // Build a map of emote code -> emote for quick lookup
  const emoteMap = new Map<string, EmoteData>();
  for (const emote of availableEmotes) {
    emoteMap.set(emote.code, emote);
  }

  const enrichedEmotes: Emote[] = [];

  // Tokenize by whitespace and check each word
  const words = text.split(/\s+/);
  const processedWords = new Set<string>();

  for (const word of words) {
    // Skip if we've already processed this word
    if (processedWords.has(word)) {
      continue;
    }
    processedWords.add(word);

    const emoteData = emoteMap.get(word);
    if (!emoteData) {
      continue;
    }

    // Find all occurrences of this emote in the text
    const occurrences = findWordOccurrences(text, word);
    if (occurrences.length === 0) {
      continue;
    }

    // Create emote object with all positions
    enrichedEmotes.push({
      code: emoteData.code,
      provider: emoteData.provider,
      url: emoteData.url,
      positions: occurrences.map(occ => [occ.start, occ.end])
    });
  }

  return enrichedEmotes;
}

/**
 * Cache for channel emotes to avoid repeated API calls
 */
const emoteCache = new Map<string, { emotes: EmoteData[]; timestamp: number }>();
const CACHE_TTL = 5 * 60 * 1000; // 5 minutes

/**
 * Get cached emotes or return null if cache miss/expired
 */
export function getCachedEmotes(channel: string): EmoteData[] | null {
  const cached = emoteCache.get(channel);
  if (!cached) {
    return null;
  }

  const now = Date.now();
  if (now - cached.timestamp > CACHE_TTL) {
    emoteCache.delete(channel);
    return null;
  }

  return cached.emotes;
}

/**
 * Cache emotes for a channel
 */
export function setCachedEmotes(channel: string, emotes: EmoteData[]): void {
  emoteCache.set(channel, {
    emotes,
    timestamp: Date.now()
  });
}
