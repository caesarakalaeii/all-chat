// Debug: fetch the profile page HTML and extract live room state.
import { request } from 'undici';

const response = await request('https://www.tiktok.com/@tiktok/live', {
  headers: {
    'User-Agent':
      'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.6 Safari/605.1.15'
  }
});
const html = await response.body.text();

// Dump every distinct *Status / state field around the room id.
for (const key of ['liveStatusCode', 'statusStr', '"status"', 'liveCore', 'isLIVE', 'liveRoomStatus']) {
  const match = new RegExp(`${key}[^,}]{0,40}`).exec(html);
  if (match) console.log(key, '=>', match[0]);
}
const roomId = /"roomId":"?(\d{15,})"?/.exec(html);
console.log('roomId:', roomId?.[1]);
