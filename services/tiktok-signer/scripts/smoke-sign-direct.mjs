// Debug: initialize the signing session step by step to find the stall.
import { SigningSession } from '../dist/signing/session.js';

const session = new SigningSession({
  executablePath: '/nix/store/ds6wq0gzyimhl7c1kmzdvql0w4d2rcv4-chromium-153.0.8010.36/bin/chromium',
  userDataDir: '/tmp/tiktok-signer-debug-profile'
});

try {
  const started = Date.now();
  // signUrl triggers initialize() -> warm-up navigation
  const signed = await session.signUrl('https://webcast.tiktok.com/webcast/im/fetch/?aid=1988&room_id=7679022730909469458');
  console.log('signed in', Date.now() - started, 'ms');
  console.log('signedUrl starts:', signed.signedUrl.slice(0, 120));
  console.log('userAgent:', signed.userAgent.slice(0, 60));
  console.log('cookies present:', signed.cookies.length > 0);
} catch (error) {
  console.log('ERR after', error.message);
} finally {
  await session.close();
}
