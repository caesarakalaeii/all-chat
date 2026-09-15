// Debug: scan a list of accounts for one that is LIVE right now.
import { request } from 'undici';

const UA =
  'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.6 Safari/605.1.15';

const candidates = [
  'tiktok', 'nasa', 'mrbeast', 'khaby.lame', 'zachking', 'washingtonpost',
  'nfl', 'nba', 'espn', 'bleacherreport', 'barstoolsports', 'fanum', 'isdin',
  'duolingo', 'oreo', 'netflix', 'samsung', 'redbull'
];

for (const uniqueId of candidates) {
  try {
    const response = await request(`https://www.tiktok.com/@${uniqueId}/live`, {
      headers: { 'User-Agent': UA }
    });
    const html = await response.body.text();
    const liveRoomStatus = /liveRoomStatus":(\d+)/.exec(html)?.[1];
    const roomId = /"roomId":"?(\d{15,})"?/.exec(html)?.[1];
    console.log(uniqueId, 'liveRoomStatus:', liveRoomStatus, 'roomId:', roomId);
    if (liveRoomStatus === '1' && roomId) {
      console.log('LIVE NOW:', uniqueId, roomId);
      break;
    }
  } catch (error) {
    console.log(uniqueId, 'ERR:', error.message);
  }
}
