import { NextResponse } from 'next/server';
import { BADGE_CACHE_TTL_MS, fetchBadgeSets } from '../../fetchBadgeSets';

type RouteParams = {
  params: {
    roomId?: string;
  };
};

export async function GET(_request: Request, { params }: RouteParams) {
  const roomId = params.roomId?.trim();
  if (!roomId) {
    return NextResponse.json({ badge_sets: {} }, { status: 400 });
  }

  try {
    const badgeSets = await fetchBadgeSets(`/channels/${roomId}/display`);
    return NextResponse.json(badgeSets, {
      headers: {
        'Cache-Control': `public, max-age=${Math.floor(BADGE_CACHE_TTL_MS / 1000)}`,
      },
    });
  } catch (err) {
    console.error('[Badges] Failed to proxy Twitch channel badges', { roomId, err });
    return NextResponse.json({ badge_sets: {} }, { status: 502 });
  }
}
