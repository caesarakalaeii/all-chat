// Smoke: drive the listener's SelfSigner against the running tiktok-signer
// service and print the decoded SignResult. Room offline is still a pass for
// the plumbing: what matters is HTTP 200 + protobuf decode, and TikTok's
// Set-Cookie arriving.
// Usage: node scripts/smoke-self-signer.mjs [roomId]
import { SelfSigner } from '../dist/sign/self.js';

const roomId = process.argv[2] ?? '7679022730909469458';
const signer = new SelfSigner({ baseUrl: 'http://localhost:8092' });

try {
  const result = await signer.sign({ roomId, userAgent: 'smoke' });
  const fetchResult = result.fetchResult;
  const cursor = fetchResult?.cursor ?? '(none)';
  console.log('sign OK');
  console.log('  decoded ProtoMessageFetchResult cursor:', cursor);
  console.log('  fetchResultCookieHeader:', result.fetchResultCookieHeader.slice(0, 50));
  console.log('  fetchResultRoomId:', result.fetchResultRoomId ?? '(none)');
} catch (error) {
  console.log('sign FAILED:', error.message);
  process.exit(1);
}
