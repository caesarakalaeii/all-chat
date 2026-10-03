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
 * TikTok native emote extraction for chat messages.
 *
 * TikTok embeds its own emotes inline in a chat comment as bracketed tokens
 * like "[laughcry]". The chat payload (`WebcastChatMessage` from the v3 proto,
 * delivered raw by TikTokLiveConnection) carries one `EmoteWithIndex` entry
 * per token: `{ index, emote: { emoteId, image: { urlList } } }`. Older
 * schemas (v1/v2) called the same fields `placeInComment` and
 * `image.imageUrl`, and the connector's deprecated legacy `WebcastPushConnection`
 * flattens them to `{ emoteId, emoteImageUrl, placeInComment }` — this service
 * uses TikTokLiveConnection and sees the raw v3 shape only.
 *
 * These emotes are platform-native, like Twitch native emotes: the third-party
 * emote enricher (7TV/BTTV/FFZ/Twitch-global) cannot resolve them, so they are
 * extracted here and forwarded as the `emote_data` tag — the same contract the
 * YouTube InnerTube listener uses — for the message-processor TikTok normalizer
 * to turn into Emote entries.
 *
 * The entry carries no text name, so the visible token must be recovered from
 * the comment text at the emote's start index. That index does not document its
 * unit (bytes, code points or UTF-16 code units are all plausible), and the
 * three agree whenever the text before the emote is ASCII. Recovery therefore
 * tries each interpretation and accepts the token only when they agree; an
 * emote whose token cannot be pinned down is dropped rather than forwarded
 * with a wrong span. The Go side locates the token again by substring search,
 * so Positions stay byte-exact no matter which unit TikTok meant.
 */

/** Minimal structural view of the v3 `EmoteModel` fields we consume. */
export interface TikTokEmoteModelView {
  emoteId?: string;
  image?: { urlList?: string[] };
}

/** Minimal structural view of one v3 `EmoteWithIndex` chat emote entry. */
export interface TikTokChatEmoteView {
  index?: number;
  emote?: TikTokEmoteModelView;
}

/** One entry of the emote_data tag (mirrors the YouTube listener's shape). */
interface EmoteDataEntry {
  code: string; // visible token in the comment, e.g. "[laughcry]"
  url: string;
  id: string;
}

/**
 * Serializes a chat message's native emotes into the `emote_data` tag value:
 * a JSON array of `{code, url, id}` entries in payload order. Returns
 * undefined when nothing serializes, so the caller can omit the tag entirely
 * and downstream keeps its empty-slice behaviour for plain messages.
 */
export function serializeEmoteData(
  content: string,
  emotes: TikTokChatEmoteView[] | undefined
): string | undefined {
  const entries: EmoteDataEntry[] = [];
  for (const chatEmote of emotes ?? []) {
    const code = recoverToken(content, chatEmote.index);
    const url = firstImageUrl(chatEmote.emote?.image);
    if (code === null || url === '') {
      continue;
    }
    entries.push({ code, url, id: chatEmote.emote?.emoteId ?? '' });
  }
  return entries.length > 0 ? JSON.stringify(entries) : undefined;
}

/**
 * Recovers the visible token (e.g. "[laughcry]") the emote occupies in the
 * comment text, or null when no single token can be trusted. See the unit
 * caveat in the file header: every plausible unit of `index` is tried and the
 * interpretations must agree on the token.
 */
function recoverToken(content: string, index: number | undefined): string | null {
  if (typeof index !== 'number' || !Number.isInteger(index) || index < 0) {
    return null;
  }
  const candidates = new Set<string>();
  for (const offset of candidateOffsets(content, index)) {
    const code = bracketedTokenAt(content, offset);
    if (code !== null) {
      candidates.add(code);
    }
  }
  // 0 candidates: the index points at no token. 2+: the units name different
  // tokens and one of them is wrong. Exactly 1: all interpretations that
  // resolved agree, so the token is safe to forward.
  if (candidates.size !== 1) {
    return null;
  }
  return candidates.values().next().value ?? null;
}

/**
 * The JS string offsets where the emote token could start if `index` counts
 * UTF-16 code units (a raw offset into the JS string), Unicode code points, or
 * UTF-8 bytes. Counts that do not land on a character boundary resolve to
 * nothing and are skipped.
 */
function candidateOffsets(content: string, index: number): number[] {
  const offsets = [
    index <= content.length ? index : null,
    offsetForUnit(content, index, 'codepoints'),
    offsetForUnit(content, index, 'bytes'),
  ];
  return offsets.filter((offset): offset is number => offset !== null);
}

function offsetForUnit(
  content: string,
  index: number,
  unit: 'codepoints' | 'bytes'
): number | null {
  let count = 0;
  for (let offset = 0; offset < content.length; ) {
    if (count === index) {
      return offset;
    }
    const point = content.codePointAt(offset)!;
    count += unit === 'bytes' ? utf8Length(point) : 1;
    offset += point > 0xffff ? 2 : 1; // surrogate pair
  }
  return count === index ? content.length : null;
}

function utf8Length(point: number): number {
  if (point < 0x80) return 1;
  if (point < 0x800) return 2;
  if (point < 0x10000) return 3;
  return 4;
}

function bracketedTokenAt(content: string, offset: number): string | null {
  if (content[offset] !== '[') {
    return null;
  }
  const close = content.indexOf(']', offset + 1);
  if (close === -1 || close === offset + 1) {
    return null;
  }
  return content.slice(offset, close + 1);
}

/**
 * Empty strings are skipped (mirroring pickAvatarUrl) so a malformed entry
 * cannot suppress a usable one.
 */
function firstImageUrl(image: { urlList?: string[] } | undefined): string {
  return image?.urlList?.find((url) => typeof url === 'string' && url.length > 0) ?? '';
}