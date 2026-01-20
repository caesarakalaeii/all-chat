import { NextResponse } from 'next/server';

/**
 * Health check endpoint for Kubernetes probes
 * Returns build information and validates the server is functioning
 */
export async function GET() {
  return NextResponse.json({
    status: 'healthy',
    timestamp: new Date().toISOString(),
    build: {
      date: process.env.NEXT_PUBLIC_BUILD_DATE || 'unknown',
      commit: process.env.NEXT_PUBLIC_GIT_COMMIT || 'unknown'
    }
  });
}
