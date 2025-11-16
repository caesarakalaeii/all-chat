import { NextResponse } from 'next/server';
import { BADGE_CACHE_TTL_MS, fetchBadgeSets } from '../fetchBadgeSets';

export async function GET() {
  try {
    const badgeSets = await fetchBadgeSets('/global/display');
    return NextResponse.json(badgeSets, {
      headers: {
        'Cache-Control': `public, max-age=${Math.floor(BADGE_CACHE_TTL_MS / 1000)}`,
      },
    });
  } catch (err) {
    console.error('[Badges] Failed to proxy Twitch global badges', err);
    return NextResponse.json({ badge_sets: {} }, { status: 502 });
  }
}
