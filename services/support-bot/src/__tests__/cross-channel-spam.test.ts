import { describe, it, expect } from 'vitest';
import { CrossChannelSpamDetector } from '../moderation/cross-channel-spam.js';

describe('CrossChannelSpamDetector.record', () => {
  it('returns false the first time a user posts a message', () => {
    const det = new CrossChannelSpamDetector();
    expect(det.record('u1', 'c1', 'hello world')).toBe(false);
  });

  it('returns false when the same user posts the same content in 2 channels', () => {
    const det = new CrossChannelSpamDetector();
    const msg = 'check out my stream';
    expect(det.record('u1', 'c1', msg)).toBe(false);
    expect(det.record('u1', 'c2', msg)).toBe(false);
  });

  it('returns true when the same user posts the same content in 3 distinct channels', () => {
    const det = new CrossChannelSpamDetector();
    const msg = 'free nitro at evil.example';
    expect(det.record('u1', 'c1', msg)).toBe(false);
    expect(det.record('u1', 'c2', msg)).toBe(false);
    expect(det.record('u1', 'c3', msg)).toBe(true);
  });

  it('triggers on short text alone in 3 channels (no min-length filter)', () => {
    const det = new CrossChannelSpamDetector();
    expect(det.record('u1', 'c1', 'bro')).toBe(false);
    expect(det.record('u1', 'c2', 'bro')).toBe(false);
    expect(det.record('u1', 'c3', 'bro')).toBe(true);
  });

  it('does NOT trigger when the same content is posted 3 times in the same channel', () => {
    const det = new CrossChannelSpamDetector();
    const msg = 'repeated in one channel';
    expect(det.record('u1', 'c1', msg)).toBe(false);
    expect(det.record('u1', 'c1', msg)).toBe(false);
    expect(det.record('u1', 'c1', msg)).toBe(false);
  });

  it('tracks distinct content separately per user', () => {
    const det = new CrossChannelSpamDetector();
    expect(det.record('u1', 'c1', 'message AAA')).toBe(false);
    expect(det.record('u1', 'c2', 'message AAA')).toBe(false);
    expect(det.record('u1', 'c3', 'message BBB')).toBe(false);
    expect(det.record('u1', 'c3', 'message AAA')).toBe(true);
  });

  it('does not bleed counts across users', () => {
    const det = new CrossChannelSpamDetector();
    const msg = 'shared spam line';
    expect(det.record('u1', 'c1', msg)).toBe(false);
    expect(det.record('u2', 'c2', msg)).toBe(false);
    expect(det.record('u1', 'c3', msg)).toBe(false); // u1 only in c1+c3 = 2
  });

  it('treats content case-insensitively', () => {
    const det = new CrossChannelSpamDetector();
    expect(det.record('u1', 'c1', 'Hello World')).toBe(false);
    expect(det.record('u1', 'c2', 'hello world')).toBe(false);
    expect(det.record('u1', 'c3', 'HELLO WORLD')).toBe(true);
  });

  it('trims surrounding whitespace before comparing', () => {
    const det = new CrossChannelSpamDetector();
    expect(det.record('u1', 'c1', '   content here   ')).toBe(false);
    expect(det.record('u1', 'c2', 'content here')).toBe(false);
    expect(det.record('u1', 'c3', 'content here\n')).toBe(true);
  });

  it('skips messages with no text and no attachments', () => {
    const det = new CrossChannelSpamDetector();
    expect(det.record('u1', 'c1', '')).toBe(false);
    expect(det.record('u1', 'c2', '')).toBe(false);
    expect(det.record('u1', 'c3', '   ')).toBe(false);
  });

  it('triggers when short text + same attachment is posted in 3 channels (current scam meta)', () => {
    const det = new CrossChannelSpamDetector();
    const sizes = [123_456]; // identical scam image re-uploaded gets same size
    expect(det.record('u1', 'c1', 'bro', sizes)).toBe(false);
    expect(det.record('u1', 'c2', 'bro', sizes)).toBe(false);
    expect(det.record('u1', 'c3', 'bro', sizes)).toBe(true);
  });

  it('treats different attachment sizes as different messages', () => {
    const det = new CrossChannelSpamDetector();
    expect(det.record('u1', 'c1', 'bro', [100])).toBe(false);
    expect(det.record('u1', 'c2', 'bro', [200])).toBe(false);
    expect(det.record('u1', 'c3', 'bro', [300])).toBe(false);
  });

  it('treats attachment-only messages (no text) as fingerprintable', () => {
    const det = new CrossChannelSpamDetector();
    expect(det.record('u1', 'c1', '', [42_000])).toBe(false);
    expect(det.record('u1', 'c2', '', [42_000])).toBe(false);
    expect(det.record('u1', 'c3', '', [42_000])).toBe(true);
  });

  it('treats attachment order as irrelevant (same set, different order)', () => {
    const det = new CrossChannelSpamDetector();
    expect(det.record('u1', 'c1', 'bro', [100, 200])).toBe(false);
    expect(det.record('u1', 'c2', 'bro', [200, 100])).toBe(false);
    expect(det.record('u1', 'c3', 'bro', [100, 200])).toBe(true);
  });

  it('expires entries older than the sliding window', () => {
    let now = 1_000_000;
    const det = new CrossChannelSpamDetector({ windowMs: 60_000, now: () => now });
    expect(det.record('u1', 'c1', 'expiring spam')).toBe(false);
    expect(det.record('u1', 'c2', 'expiring spam')).toBe(false);
    now += 61_000;
    expect(det.record('u1', 'c3', 'expiring spam')).toBe(false);
  });

  it('does not retrigger after the user has already been actioned', () => {
    const det = new CrossChannelSpamDetector();
    const msg = 'compromised account spam';
    det.record('u1', 'c1', msg);
    det.record('u1', 'c2', msg);
    expect(det.record('u1', 'c3', msg)).toBe(true);
    expect(det.record('u1', 'c4', msg)).toBe(false);
    expect(det.record('u1', 'c5', 'different content')).toBe(false);
  });

  it('respects a custom channelThreshold', () => {
    const det = new CrossChannelSpamDetector({ channelThreshold: 2 });
    expect(det.record('u1', 'c1', 'two-channel rule')).toBe(false);
    expect(det.record('u1', 'c2', 'two-channel rule')).toBe(true);
  });
});
