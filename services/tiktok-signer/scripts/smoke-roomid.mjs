// Smoke: resolve a room ID through the connector's direct routes (no Euler)
// and then ask the signer service for a signed /im/fetch/ exchange.
// Usage: node scripts/smoke-roomid.mjs <username>
import { TikTokLiveConnection } from '../../tiktok-listener/node_modules/tiktok-live-connector/dist/index.js';

const username = process.argv[2] ?? 'nasa';
const connection = new TikTokLiveConnection(username, {});
try {
  const roomId = await connection.fetchRoomId();
  console.log('roomId:', roomId);
  const info = await connection.fetchRoomInfo();
  console.log('status:', info.status, 'title:', info.title);
} catch (error) {
  console.log('ERR:', error.message);
}
